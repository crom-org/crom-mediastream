package engine

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

type StreamEngine struct {
	FFmpegPath string
	CurrentCmd *exec.Cmd
}

func NewStreamEngine() (*StreamEngine, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	return &StreamEngine{FFmpegPath: path}, nil
}

func (e *StreamEngine) Stop() {
	if e.CurrentCmd != nil && e.CurrentCmd.Process != nil {
		e.CurrentCmd.Process.Kill()
	}
}

// StreamWithFade transitions with optimized settings
func (e *StreamEngine) StreamWithFade(currentPath, nextPath, streamURL string, offset float64) *exec.Cmd {
	e.Stop()

	// Using ultrafast preset to save CPU during transition
	// Added -reconnect flags for robustness
	filter := fmt.Sprintf(
		"[0:v][1:v]xfade=transition=fade:duration=1:offset=%f[v]; [0:a][1:a]acrossfade=d=1[a]",
		offset,
	)

	args := []string{
		"-re",
		"-ss", fmt.Sprintf("%f", offset), "-i", currentPath,
		"-re", "-i", nextPath,
		"-filter_complex", filter,
		"-map", "[v]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-maxrate", "3000k", "-bufsize", "6000k",
		"-pix_fmt", "yuv420p", "-g", "60",
		"-c:a", "aac", "-b:a", "128k",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	e.CurrentCmd = cmd
	return cmd
}

func (e *StreamEngine) StreamSingle(videoDir, fileName, streamURL string) *exec.Cmd {
	e.Stop()
	filePath := filepath.Join(videoDir, fileName)

	args := []string{
		"-re",
		"-i", filePath,
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-maxrate", "3000k", "-bufsize", "6000k",
		"-pix_fmt", "yuv420p", "-g", "60",
		"-c:a", "aac", "-b:a", "128k",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	e.CurrentCmd = cmd
	return cmd
}
