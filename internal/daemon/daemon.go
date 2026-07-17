package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"crom-mediastream/internal/api"
	"crom-mediastream/internal/config"
	"crom-mediastream/internal/engine"
	"crom-mediastream/internal/queue"
)

type DaemonState struct {
	CurrentVideo  string   `json:"current_video"`
	Duration      float64  `json:"duration"`
	Elapsed       float64  `json:"elapsed"`
	Streaming     bool     `json:"streaming"`
	AutoDJEnabled bool     `json:"auto_dj_enabled"`
	LoopEnabled   bool     `json:"loop_enabled"`
	ChatEnabled   bool     `json:"chat_enabled"`
	ScrollEnabled bool     `json:"scroll_enabled"`
	NewsText      string   `json:"news_text"`
	Resolution    string   `json:"resolution"`
	StatusMessage string   `json:"status_message"`
	Playlist      []string `json:"playlist"`
}

type CommandPayload struct {
	Action     string `json:"action"`
	Video      string `json:"video,omitempty"`
	NewsText   string `json:"news_text,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type Daemon struct {
	cfg       *config.Config
	eng       *engine.StreamEngine
	queue     *queue.VideoQueue
	twitch    *api.TwitchAPI
	chatMon   *api.ChatMonitor
	state     DaemonState
	startTime time.Time
	mu        sync.Mutex

	// Controle do playback contínuo
	stopCh    chan struct{}
	isPlaying bool
}

func NewDaemon(cfg *config.Config, eng *engine.StreamEngine, q *queue.VideoQueue) *Daemon {
	// Garantir que os arquivos de runtime existam para o FFmpeg não dar crash
	if _, err := os.Stat("chat_overlay.txt"); os.IsNotExist(err) {
		os.WriteFile("chat_overlay.txt", []byte(""), 0644)
	}
	if _, err := os.Stat("scroll_text.txt"); os.IsNotExist(err) {
		os.WriteFile("scroll_text.txt", []byte(""), 0644)
	}

	tAPI := api.NewTwitchAPI(cfg.TwitchClientID, cfg.TwitchToken, cfg.TwitchUserID)
	cMon := api.NewChatMonitor("mrjcrom")

	d := &Daemon{
		cfg:     cfg,
		eng:     eng,
		queue:   q,
		twitch:  tAPI,
		chatMon: cMon,
		state: DaemonState{
			AutoDJEnabled: cfg.AutoDJEnabled,
			LoopEnabled:   cfg.LoopEnabled,
			ChatEnabled:   cfg.ChatEnabled,
			ScrollEnabled: cfg.ScrollEnabled,
			Resolution:    cfg.Resolution,
			StatusMessage: "Daemon initialized.",
		},
	}

	videos, _ := q.ListVideos()
	d.state.Playlist = videos

	scrollText, _ := os.ReadFile("scroll_text.txt")
	d.state.NewsText = string(scrollText)

	return d
}

func (d *Daemon) Start() {
	// Goroutine para atualizar elapsed time e playlist
	go d.elapsedTicker()

	http.HandleFunc("/state", d.handleState)
	http.HandleFunc("/command", d.handleCommand)

	// Auto-start: se auto_dj estiver ativo e houver vídeos, começa a live automaticamente
	if d.state.AutoDJEnabled {
		videos, err := d.queue.ListVideos()
		if err == nil && len(videos) > 0 {
			log.Printf("[Daemon] Auto-DJ ativo. Iniciando live automaticamente com: %s", videos[0])
			go d.startContinuousPlayback(videos[0])
		} else {
			log.Printf("[Daemon] Auto-DJ ativo mas nenhum vídeo encontrado na pasta.")
		}
	}

	fmt.Printf("Crom MediaStream Daemon rodando na porta %s...\n", d.cfg.Port)
	log.Fatal(http.ListenAndServe(":"+d.cfg.Port, nil))
}


// elapsedTicker atualiza o tempo decorrido e a playlist a cada segundo
func (d *Daemon) elapsedTicker() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		d.mu.Lock()

		if d.state.Streaming && d.state.CurrentVideo != "" {
			d.state.Elapsed = time.Since(d.startTime).Seconds()
		}

		videos, _ := d.queue.ListVideos()
		d.state.Playlist = videos

		d.mu.Unlock()
	}
}

// startContinuousPlayback inicia a goroutine de playback contínuo.
// Cada vídeo toca inteiro, e ao terminar, o próximo começa imediatamente.
// SEM matar o FFmpeg entre transições — gap mínimo de ~500ms.
func (d *Daemon) startContinuousPlayback(startVideo string) {
	// Se já está tocando, para primeiro
	if d.isPlaying {
		d.stopPlayback()
	}

	d.stopCh = make(chan struct{})
	d.isPlaying = true

	d.mu.Lock()
	d.state.Streaming = true
	d.state.StatusMessage = fmt.Sprintf("Starting continuous playback from %s...", startVideo)
	d.mu.Unlock()

	opts := engine.StreamOptions{Resolution: d.state.Resolution}
	firstVideo := startVideo

	getNextVideo := func(current string) (string, error) {
		// Na primeira chamada, retorna o vídeo solicitado
		if current == "" && firstVideo != "" {
			v := firstVideo
			firstVideo = "" // Consume o firstVideo
			return v, nil
		}

		d.mu.Lock()
		autoDJ := d.state.AutoDJEnabled
		loopEnabled := d.state.LoopEnabled
		d.mu.Unlock()

		if !autoDJ {
			// Sem AutoDJ, para após o vídeo atual
			return "", nil
		}

		nextVid, err := d.queue.GetNextVideo(current)
		if err != nil {
			return "", err
		}

		// Se loop está desligado e voltamos ao primeiro, para
		if !loopEnabled {
			videos, _ := d.queue.ListVideos()
			if len(videos) > 0 && nextVid == videos[0] && current != "" {
				return "", nil
			}
		}

		return nextVid, nil
	}

	onVideoStart := func(videoName string, duration float64) {
		d.mu.Lock()
		defer d.mu.Unlock()

		d.state.CurrentVideo = videoName
		d.state.Duration = duration
		d.state.Elapsed = 0
		d.state.Streaming = true
		d.startTime = time.Now()
		d.state.StatusMessage = fmt.Sprintf("▶ Playing: %s", videoName)

		// Atualiza metadata da Twitch em background
		title := "Streaming: " + videoName
		go d.twitch.UpdateStreamMetadata(title)
	}

	onVideoEnd := func(videoName string) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.state.StatusMessage = fmt.Sprintf("✓ Finished: %s", videoName)
		log.Printf("[Daemon] ✓ Vídeo finalizado: %s", videoName)
	}

	go func() {
		d.eng.PlayContinuous(
			d.cfg.VideoDir,
			d.cfg.GetFullStreamURL(),
			opts,
			getNextVideo,
			onVideoStart,
			onVideoEnd,
			d.stopCh,
		)

		// Playback terminou (playlist acabou ou stop recebido)
		d.mu.Lock()
		d.state.Streaming = false
		d.state.CurrentVideo = ""
		d.state.Duration = 0
		d.state.Elapsed = 0
		d.state.StatusMessage = "Playback ended."
		d.isPlaying = false
		d.mu.Unlock()
		log.Printf("[Daemon] Playback contínuo encerrado.")
	}()
}

// stopPlayback para o playback contínuo graciosamente
func (d *Daemon) stopPlayback() {
	if d.stopCh != nil {
		close(d.stopCh)
	}
	// Espera até 5s para o playback parar
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			d.eng.Stop() // Force stop
			return
		case <-ticker.C:
			d.mu.Lock()
			playing := d.isPlaying
			d.mu.Unlock()
			if !playing {
				return
			}
		}
	}
}

func (d *Daemon) handleState(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.state)
}

func (d *Daemon) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload CommandPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d.mu.Lock()

	switch payload.Action {
	case "start":
		if payload.Video != "" {
			d.mu.Unlock()
			d.startContinuousPlayback(payload.Video)
			w.WriteHeader(http.StatusOK)
			return
		}
	case "stop":
		d.mu.Unlock()
		d.stopPlayback()
		d.mu.Lock()
		d.state.Streaming = false
		d.state.CurrentVideo = ""
		d.state.Duration = 0
		d.state.StatusMessage = "Stream stopped."
		d.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	case "toggle_autodj":
		d.state.AutoDJEnabled = !d.state.AutoDJEnabled
	case "toggle_loop":
		d.state.LoopEnabled = !d.state.LoopEnabled
	case "toggle_chat":
		d.state.ChatEnabled = !d.state.ChatEnabled
		if d.state.ChatEnabled {
			d.chatMon.Start()
			d.state.StatusMessage = "Chat overlay enabled"
		} else {
			d.chatMon.Stop()
			os.WriteFile("chat_overlay.txt", []byte(""), 0644)
			d.state.StatusMessage = "Chat overlay disabled"
		}
	case "toggle_scroll":
		d.state.ScrollEnabled = !d.state.ScrollEnabled
		if d.state.ScrollEnabled {
			os.WriteFile("scroll_text.txt", []byte(d.state.NewsText), 0644)
			d.state.StatusMessage = "Scroll News enabled"
		} else {
			os.WriteFile("scroll_text.txt", []byte(""), 0644)
			d.state.StatusMessage = "Scroll News disabled"
		}
	case "update_news":
		d.state.NewsText = payload.NewsText
		if d.state.ScrollEnabled {
			os.WriteFile("scroll_text.txt", []byte(d.state.NewsText), 0644)
		}
		d.state.StatusMessage = "News text saved and applied."
	case "set_resolution":
		d.state.Resolution = payload.Resolution
	}

	d.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}
