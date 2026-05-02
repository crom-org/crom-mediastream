package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	go d.stateMachine()

	http.HandleFunc("/state", d.handleState)
	http.HandleFunc("/command", d.handleCommand)

	fmt.Printf("Crom MediaStream Daemon rodando na porta %s...\n", d.cfg.Port)
	log.Fatal(http.ListenAndServe(":"+d.cfg.Port, nil))
}

func (d *Daemon) stateMachine() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		d.mu.Lock()

		if d.state.Streaming && d.state.CurrentVideo != "" {
			d.state.Elapsed = time.Since(d.startTime).Seconds()
		}

		if d.state.Streaming && d.state.AutoDJEnabled && d.state.Duration > 0 {
			remaining := d.state.Duration - d.state.Elapsed

			if remaining <= 2.0 && remaining > 0 {
				nextVid, _ := d.queue.GetNextVideo(d.state.CurrentVideo)

				if !d.state.LoopEnabled {
					videos, _ := d.queue.ListVideos()
					if len(videos) > 0 && nextVid == videos[0] {
						nextVid = ""
					}
				}

				if nextVid != "" && nextVid != d.state.CurrentVideo {
					title := "Streaming: " + nextVid
					go d.twitch.UpdateStreamMetadata(title)

					currentPath := filepath.Join(d.cfg.VideoDir, d.state.CurrentVideo)
					nextPath := filepath.Join(d.cfg.VideoDir, nextVid)
					dur, _ := d.eng.GetVideoDuration(nextPath)

					opts := engine.StreamOptions{Resolution: d.state.Resolution}
					d.state.StatusMessage = fmt.Sprintf("AutoDJ: Fading to %s...", nextVid)

					cmd := d.eng.StreamWithFade(currentPath, nextPath, d.cfg.GetFullStreamURL(), d.state.Elapsed, opts)
					go cmd.Run()

					d.state.CurrentVideo = nextVid
					d.startTime = time.Now()
					d.state.Duration = dur
					d.state.Elapsed = 0
				}
			}
		}

		videos, _ := d.queue.ListVideos()
		d.state.Playlist = videos

		d.mu.Unlock()
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
	defer d.mu.Unlock()

	switch payload.Action {
	case "start":
		if payload.Video != "" {
			d.startVideo(payload.Video)
		}
	case "stop":
		d.eng.Stop()
		d.state.Streaming = false
		d.state.CurrentVideo = ""
		d.state.Duration = 0
		d.state.StatusMessage = "Stream stopped."
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

	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) startVideo(videoName string) {
	title := "Streaming: " + videoName
	go d.twitch.UpdateStreamMetadata(title)

	nextPath := filepath.Join(d.cfg.VideoDir, videoName)
	dur, _ := d.eng.GetVideoDuration(nextPath)

	opts := engine.StreamOptions{Resolution: d.state.Resolution}

	if d.state.Streaming && d.state.CurrentVideo != "" {
		d.state.StatusMessage = fmt.Sprintf("Fading to %s...", videoName)
		currentPath := filepath.Join(d.cfg.VideoDir, d.state.CurrentVideo)
		cmd := d.eng.StreamWithFade(currentPath, nextPath, d.cfg.GetFullStreamURL(), d.state.Elapsed, opts)
		go cmd.Run()
	} else {
		d.state.StatusMessage = fmt.Sprintf("Starting %s...", videoName)
		cmd := d.eng.StreamSingle(d.cfg.VideoDir, videoName, d.cfg.GetFullStreamURL(), opts)
		go cmd.Run()
	}

	d.state.CurrentVideo = videoName
	d.startTime = time.Now()
	d.state.Duration = dur
	d.state.Elapsed = 0
	d.state.Streaming = true
}
