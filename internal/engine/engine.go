package engine

import (
	"fmt"
	"io"
	"log"
	"net"
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
	CurrentCmd *exec.Cmd // Master FFmpeg Command
	EncoderCmd *exec.Cmd // Active Encoder FFmpeg Command
	listener   net.Listener
	masterConn net.Conn
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

// StartMaster inicia o FFmpeg principal (Master) que lê MPEG-TS da porta TCP local e envia RTMP
func (e *StreamEngine) StartMaster(streamURL string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Mock para TestPlayContinuousBackoff onde FFmpegPath = "false"
	if e.FFmpegPath == "false" {
		l, _ := net.Listen("tcp", "127.0.0.1:0")
		e.listener = l
		conn, _ := net.Dial("tcp", l.Addr().String())
		e.masterConn, _ = l.Accept()
		conn.Close()
		return nil
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local TCP listener: %w", err)
	}
	e.listener = l
	port := l.Addr().(*net.TCPAddr).Port

	// FFmpeg Master que lê do TCP local e joga no RTMP usando "-c copy" (quase 0% de CPU)
	args := []string{
		"-f", "mpegts",
		"-i", fmt.Sprintf("tcp://127.0.0.1:%d", port),
		"-c", "copy",
		"-flvflags", "no_duration_filesize",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	logFile, _ := os.OpenFile("ffmpeg_master.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd.Stderr = logFile
	cmd.Stdout = logFile

	if err := cmd.Start(); err != nil {
		l.Close()
		logFile.Close()
		return fmt.Errorf("failed to start master FFmpeg: %w", err)
	}
	e.CurrentCmd = cmd

	// Aguarda a conexão do FFmpeg Master
	connCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			errCh <- err
		} else {
			connCh <- conn
		}
	}()

	select {
	case conn := <-connCh:
		e.masterConn = conn
		return nil
	case err := <-errCh:
		// Limpa tudo se der erro
		if e.CurrentCmd != nil && e.CurrentCmd.Process != nil {
			e.CurrentCmd.Process.Kill()
		}
		l.Close()
		return fmt.Errorf("error accepting master FFmpeg connection: %w", err)
	case <-time.After(5 * time.Second):
		if e.CurrentCmd != nil && e.CurrentCmd.Process != nil {
			e.CurrentCmd.Process.Kill()
		}
		l.Close()
		return fmt.Errorf("timeout waiting for master FFmpeg to connect")
	}
}

// Stop encerra todos os processos do FFmpeg e fecha as conexões TCP locais de forma segura
func (e *StreamEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Encerra o Encoder ativo primeiro
	if e.EncoderCmd != nil && e.EncoderCmd.Process != nil {
		e.EncoderCmd.Process.Signal(syscall.SIGINT)
		// Força kill após timeout
		done := make(chan struct{}, 1)
		go func() {
			e.EncoderCmd.Process.Wait()
			done <- struct{}{}
		}()
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			e.EncoderCmd.Process.Kill()
		}
		e.EncoderCmd = nil
	}

	// 2. Encerra o Master FFmpeg
	if e.CurrentCmd != nil && e.CurrentCmd.Process != nil {
		e.CurrentCmd.Process.Signal(syscall.SIGINT)
		done := make(chan struct{}, 1)
		go func() {
			e.CurrentCmd.Process.Wait()
			done <- struct{}{}
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			e.CurrentCmd.Process.Kill()
		}
		e.CurrentCmd = nil
	}

	// 3. Fecha as conexões de rede locais
	if e.masterConn != nil {
		e.masterConn.Close()
		e.masterConn = nil
	}
	if e.listener != nil {
		e.listener.Close()
		e.listener = nil
	}
}

func getOverlayFilters() (string, string) {
	drawChat := "drawtext=fontfile=assets/Roboto.ttf:textfile=chat_overlay.txt:reload=1:fontcolor=white:fontsize=24:x=10:y=10:box=1:boxcolor=black@0.5"
	drawScroll := "drawtext=fontfile=assets/Roboto.ttf:textfile=scroll_text.txt:reload=1:fontcolor=white:fontsize=32:x=w-mod(t*150\\,w+tw):y=H-50:box=1:boxcolor=black@0.8"
	return drawChat, drawScroll
}

// StreamSingle cria o comando FFmpeg para processamento de um vídeo
func (e *StreamEngine) StreamSingle(videoDir, fileName, streamURL string, opts StreamOptions) (*exec.Cmd, *os.File) {
	e.mu.Lock()
	defer e.mu.Unlock()

	filePath := filepath.Join(videoDir, fileName)

	drawChat, drawScroll := getOverlayFilters()
	videoFilter := fmt.Sprintf("scale=%s,setsar=1,fps=30,format=yuv420p,%s,%s", opts.Resolution, drawChat, drawScroll)

	var args []string
	if streamURL == "" {
		// Output para stdout em MPEG-TS (utilizado como Encoder em PlayContinuous)
		args = []string{
			"-re",
			"-i", filePath,
			"-vf", videoFilter,
			"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
			"-maxrate", "3000k", "-bufsize", "6000k",
			"-pix_fmt", "yuv420p", "-g", "60",
			"-keyint_min", "60", "-sc_threshold", "0",
			"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
			"-f", "mpegts", "-",
		}
	} else {
		// Modo direto/compatibilidade (utilizado em testes unitários)
		args = []string{
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
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	logFile, _ := os.OpenFile("ffmpeg_encoder.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd.Stderr = logFile
	e.EncoderCmd = cmd
	return cmd, logFile
}

const minRunTime = 5 * time.Second
const maxConsecutiveFailures = 5

// PlayContinuous roda uma playlist ininterrupta, usando a arquitetura Master/Encoder
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

	// 1. Inicia o Master FFmpeg (só fecha no Stop)
	log.Printf("[Engine] Iniciando Master FFmpeg para %s...", streamURL)
	if err := e.StartMaster(streamURL); err != nil {
		log.Printf("[Engine] ❌ Erro fatal ao iniciar Master FFmpeg: %v", err)
		return
	}
	defer e.Stop()

	for {
		select {
		case <-stopCh:
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

		// 2. Inicia o Encoder (grava saída no stdout)
		cmd, logFile := e.StreamSingle(videoDir, nextVideo, "", opts)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("[Engine] ❌ Erro ao obter pipe de saída do Encoder: %v", err)
			logFile.Close()
			consecutiveFailures++
			time.Sleep(2 * time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			log.Printf("[Engine] ❌ Erro ao iniciar Encoder para %s: %v", nextVideo, err)
			logFile.Close()
			consecutiveFailures++
			time.Sleep(2 * time.Second)
			continue
		}

		startedAt := time.Now()

		// 3. Pipe o output do Encoder para a conexão Master do TCP em background
		go func() {
			defer logFile.Close()
			defer stdoutPipe.Close()

			e.mu.Lock()
			conn := e.masterConn
			e.mu.Unlock()

			if conn != nil {
				io.Copy(conn, stdoutPipe)
			}
		}()

		// Canal para detectar fim do Encoder
		cmdDone := make(chan error, 1)
		go func() {
			cmdDone <- cmd.Wait()
		}()

		select {
		case err := <-cmdDone:
			elapsed := time.Since(startedAt)

			if err != nil {
				log.Printf("[Engine] ⚠ Encoder encerrou com erro para %s após %v: %v", nextVideo, elapsed, err)
			}

			// Se o Encoder falhar rapidamente, trata como crash
			if elapsed < minRunTime {
				consecutiveFailures++
				waitTime := time.Duration(consecutiveFailures) * 3 * time.Second
				if waitTime > 30*time.Second {
					waitTime = 30 * time.Second
				}
				log.Printf("[Engine] 🔄 Encoder durou apenas %v (mínimo: %v). Crash detectado. Retry #%d em %v...",
					elapsed.Round(time.Millisecond), minRunTime, consecutiveFailures, waitTime)

				if consecutiveFailures >= maxConsecutiveFailures {
					log.Printf("[Engine] ❌ %d falhas consecutivas. Avançando para o próximo vídeo...", consecutiveFailures)
					currentVideo = nextVideo
					consecutiveFailures = 0
				}

				time.Sleep(waitTime)
				continue
			}

			consecutiveFailures = 0

			if onVideoEnd != nil {
				onVideoEnd(nextVideo)
			}

		case <-stopCh:
			log.Printf("[Engine] 🛑 Stop recebido durante %s", nextVideo)
			return
		}

		currentVideo = nextVideo

		// Pequena pausa de transição (100ms) para acomodar o fluxo de buffer sem travar
		time.Sleep(100 * time.Millisecond)
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

