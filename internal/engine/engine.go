package engine

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type StreamEngine struct {
	FFmpegPath string
	CurrentCmd *exec.Cmd
	mu         sync.Mutex
}

type StreamOptions struct {
	Resolution string // ex: "1280x720"
}

func NewStreamEngine() (*StreamEngine, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	return &StreamEngine{FFmpegPath: path}, nil
}

// Stop encerra o FFmpeg graciosamente com SIGINT, permitindo que ele finalize
// a conexão RTMP corretamente antes de morrer. Fallback para Kill após timeout.
func (e *StreamEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.CurrentCmd == nil || e.CurrentCmd.Process == nil {
		return
	}

	// Tenta SIGINT primeiro (FFmpeg finaliza RTMP graciosamente)
	err := e.CurrentCmd.Process.Signal(syscall.SIGINT)
	if err != nil {
		// Processo já morreu
		e.CurrentCmd = nil
		return
	}

	// Espera até 3 segundos para o FFmpeg encerrar graciosamente
	done := make(chan struct{}, 1)
	go func() {
		e.CurrentCmd.Process.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		// FFmpeg encerrou graciosamente
	case <-time.After(3 * time.Second):
		// Timeout — força kill
		e.CurrentCmd.Process.Kill()
	}

	e.CurrentCmd = nil
}

func getOverlayFilters() (string, string) {
	drawChat := "drawtext=fontfile=assets/Roboto.ttf:textfile=chat_overlay.txt:reload=1:fontcolor=white:fontsize=24:x=10:y=10:box=1:boxcolor=black@0.5"
	// x=w-mod(t*150,w+tw) precisa de escape na virgula
	drawScroll := "drawtext=fontfile=assets/Roboto.ttf:textfile=scroll_text.txt:reload=1:fontcolor=white:fontsize=32:x=w-mod(t*150\\,w+tw):y=H-50:box=1:boxcolor=black@0.8"
	return drawChat, drawScroll
}

// StreamSingle cria o comando FFmpeg para um vídeo. Retorna o cmd SEM executar.
// O chamador deve chamar cmd.Start() e cmd.Wait() para controlar o ciclo de vida.
func (e *StreamEngine) StreamSingle(videoDir, fileName, streamURL string, opts StreamOptions) (*exec.Cmd, *os.File) {
	e.mu.Lock()
	defer e.mu.Unlock()

	filePath := filepath.Join(videoDir, fileName)

	drawChat, drawScroll := getOverlayFilters()
	videoFilter := fmt.Sprintf("scale=%s,setsar=1,fps=30,format=yuv420p,%s,%s", opts.Resolution, drawChat, drawScroll)

	args := []string{
		"-re",
		"-i", filePath,
		"-vf", videoFilter,
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-maxrate", "3000k", "-bufsize", "6000k",
		"-pix_fmt", "yuv420p", "-g", "60",
		"-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
		"-flvflags", "no_duration_filesize",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	logFile, _ := os.OpenFile("ffmpeg.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	e.CurrentCmd = cmd
	return cmd, logFile
}

// minRunTime é o tempo mínimo que o FFmpeg deve rodar para ser considerado sucesso.
// Se encerrar antes disso, é tratado como crash/erro.
const minRunTime = 5 * time.Second

// maxConsecutiveFailures é o máximo de falhas consecutivas antes de aguardar mais tempo.
const maxConsecutiveFailures = 5

// PlayContinuous executa uma playlist de vídeos de forma contínua e sequencial.
// Cada vídeo é transmitido completamente antes de iniciar o próximo.
// O gap entre vídeos é mínimo (~500ms de reconexão RTMP).
// Se o FFmpeg falhar rapidamente, re-tenta o MESMO vídeo com backoff exponencial.
// A função bloqueia até que stopCh seja fechado ou ocorra um erro fatal.
func (e *StreamEngine) PlayContinuous(
	videoDir string,
	streamURL string,
	opts StreamOptions,
	getNextVideo func(current string) (string, error),
	onVideoStart func(videoName string, duration float64),
	onVideoEnd func(videoName string),
	stopCh <-chan struct{},
) {
	currentVideo := ""
	consecutiveFailures := 0

	for {
		select {
		case <-stopCh:
			e.Stop()
			return
		default:
		}

		nextVideo, err := getNextVideo(currentVideo)
		if err != nil || nextVideo == "" {
			log.Printf("[Engine] Playlist vazia ou erro: %v. Aguardando 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		nextPath := filepath.Join(videoDir, nextVideo)
		dur, _ := e.GetVideoDuration(nextPath)

		if onVideoStart != nil {
			onVideoStart(nextVideo, dur)
		}

		log.Printf("[Engine] ▶ Iniciando: %s (%.0fs)", nextVideo, dur)

		cmd, logFile := e.StreamSingle(videoDir, nextVideo, streamURL, opts)

		if err := cmd.Start(); err != nil {
			log.Printf("[Engine] ❌ Erro ao iniciar FFmpeg para %s: %v", nextVideo, err)
			logFile.Close()
			consecutiveFailures++
			waitTime := time.Duration(consecutiveFailures) * 3 * time.Second
			if waitTime > 30*time.Second {
				waitTime = 30 * time.Second
			}
			log.Printf("[Engine] ⏳ Aguardando %v antes de re-tentar...", waitTime)
			time.Sleep(waitTime)
			continue
		}

		startedAt := time.Now()

		// Canal para detectar fim do processo
		cmdDone := make(chan error, 1)
		go func() {
			cmdDone <- cmd.Wait()
		}()

		select {
		case err := <-cmdDone:
			logFile.Close()
			elapsed := time.Since(startedAt)

			if err != nil {
				log.Printf("[Engine] ⚠ FFmpeg encerrou com erro para %s após %v: %v", nextVideo, elapsed, err)
			}

			// Se o FFmpeg rodou menos que minRunTime, é um crash — NÃO avança para o próximo vídeo
			if elapsed < minRunTime {
				consecutiveFailures++
				waitTime := time.Duration(consecutiveFailures) * 3 * time.Second
				if waitTime > 30*time.Second {
					waitTime = 30 * time.Second
				}
				log.Printf("[Engine] 🔄 FFmpeg durou apenas %v (mínimo: %v). Crash detectado. Retry #%d em %v...",
					elapsed.Round(time.Millisecond), minRunTime, consecutiveFailures, waitTime)

				// NÃO atualiza currentVideo — vai re-tentar o mesmo vídeo ou o próximo ciclo
				// Se já falhou demais, avança para o próximo vídeo
				if consecutiveFailures >= maxConsecutiveFailures {
					log.Printf("[Engine] ❌ %d falhas consecutivas. Avançando para o próximo vídeo...", consecutiveFailures)
					currentVideo = nextVideo
					consecutiveFailures = 0
				}

				time.Sleep(waitTime)
				continue
			}

			// Vídeo tocou normalmente — reseta contagem de falhas
			consecutiveFailures = 0

			if onVideoEnd != nil {
				onVideoEnd(nextVideo)
			}

		case <-stopCh:
			logFile.Close()
			log.Printf("[Engine] 🛑 Stop recebido durante %s", nextVideo)
			e.Stop()
			return
		}

		currentVideo = nextVideo

		// Micro-pausa para garantir que o socket RTMP anterior fechou
		time.Sleep(500 * time.Millisecond)
	}
}

func (e *StreamEngine) GetVideoDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error running ffprobe: %w", err)
	}

	durationStr := strings.TrimSpace(string(out))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing duration '%s': %w", durationStr, err)
	}

	return duration, nil
}
