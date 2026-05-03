package engine

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestPlayContinuousBackoff verifica que o PlayContinuous não cicla rapidamente
// quando o FFmpeg falha (simula crash com um comando que sai imediatamente).
func TestPlayContinuousBackoff(t *testing.T) {
	eng := &StreamEngine{FFmpegPath: "false"} // "false" é um binário que sempre retorna exit 1

	videoCount := 0
	startTimes := []time.Time{}
	maxVideos := 4

	stopCh := make(chan struct{})

	getNextVideo := func(current string) (string, error) {
		if videoCount >= maxVideos {
			close(stopCh)
			return "", nil
		}
		videoCount++
		return "test.mp4", nil
	}

	onVideoStart := func(name string, dur float64) {
		startTimes = append(startTimes, time.Now())
		t.Logf("Video start #%d: %s", len(startTimes), name)
	}

	// Override StreamSingle para não depender de FFmpeg real
	// Vamos testar diretamente o comportamento de backoff
	started := time.Now()

	go eng.PlayContinuous(
		"/tmp",
		"rtmp://fake",
		StreamOptions{Resolution: "1280x720"},
		getNextVideo,
		onVideoStart,
		nil,
		stopCh,
	)

	// Espera o teste terminar (max 60s)
	select {
	case <-stopCh:
	case <-time.After(60 * time.Second):
		t.Fatal("Test timed out")
	}

	elapsed := time.Since(started)
	t.Logf("Total elapsed: %v for %d video attempts", elapsed, videoCount)

	// Com backoff, 4 tentativas devem levar pelo menos uns 10 segundos
	// (3s + 6s + 9s + ...) — sem backoff levaria <2s
	if elapsed < 5*time.Second {
		t.Errorf("Backoff não está funcionando! Elapsed=%v, deveria ser >5s para %d tentativas", elapsed, videoCount)
	}
}

// TestStreamSingleCommand verifica que o comando FFmpeg é construído corretamente
func TestStreamSingleCommand(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg não encontrado, pulando teste")
	}

	eng := &StreamEngine{FFmpegPath: ffmpegPath}

	// Cria arquivos necessários
	os.WriteFile("chat_overlay.txt", []byte(""), 0644)
	os.WriteFile("scroll_text.txt", []byte(""), 0644)

	cmd, logFile := eng.StreamSingle("./videos", "test_mock_A.mp4", "pipe:1", StreamOptions{Resolution: "1280x720"})
	defer logFile.Close()

	if cmd == nil {
		t.Fatal("StreamSingle retornou nil")
	}

	// Verifica argumentos essenciais
	args := cmd.Args
	hasRe := false
	hasFlvflags := false
	for i, a := range args {
		if a == "-re" {
			hasRe = true
		}
		if a == "-flvflags" && i+1 < len(args) && args[i+1] == "no_duration_filesize" {
			hasFlvflags = true
		}
	}

	if !hasRe {
		t.Error("Faltando -re nos argumentos")
	}
	if !hasFlvflags {
		t.Error("Faltando -flvflags no_duration_filesize nos argumentos")
	}

	t.Logf("Comando: %v", cmd.Args[:5])
	t.Log("OK: argumentos do FFmpeg estão corretos")
}
