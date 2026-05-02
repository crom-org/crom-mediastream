package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type StreamEngine struct {
	FFmpegPath string
	CurrentCmd *exec.Cmd
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

func (e *StreamEngine) Stop() {
	if e.CurrentCmd != nil && e.CurrentCmd.Process != nil {
		e.CurrentCmd.Process.Kill()
	}
}

func getOverlayFilters() (string, string) {
	drawChat := "drawtext=fontfile=assets/Roboto.ttf:textfile=chat_overlay.txt:reload=1:fontcolor=white:fontsize=24:x=10:y=10:box=1:boxcolor=black@0.5"
	// x=w-mod(t*150,w+tw) precisa de escape na virgula
	drawScroll := "drawtext=fontfile=assets/Roboto.ttf:textfile=scroll_text.txt:reload=1:fontcolor=white:fontsize=32:x=w-mod(t*150\\,w+tw):y=H-50:box=1:boxcolor=black@0.8"
	return drawChat, drawScroll
}

// StreamWithFade transitions with optimized settings
func (e *StreamEngine) StreamWithFade(currentPath, nextPath, streamURL string, offset float64, opts StreamOptions) *exec.Cmd {
	e.Stop()

	drawChat, drawScroll := getOverlayFilters()
	
	// Normaliza as duas mídias (Vídeo e Áudio) para evitar crash de timebase ou resolução no XFADE
	v0 := fmt.Sprintf("[0:v]scale=%s,setsar=1,fps=30,format=yuv420p,settb=AVTB[v0]", opts.Resolution)
	v1 := fmt.Sprintf("[1:v]scale=%s,setsar=1,fps=30,format=yuv420p,settb=AVTB[v1]", opts.Resolution)
	a0 := "[0:a]aresample=48000[a0]"
	a1 := "[1:a]aresample=48000[a1]"
	
	vFade := "[v0][v1]xfade=transition=fade:duration=1:offset=0[xfaded]"
	aFade := "[a0][a1]acrossfade=d=1[a]"
	
	filter := fmt.Sprintf("%s; %s; %s; %s; %s; %s; [xfaded]%s,%s[vout]", v0, v1, a0, a1, vFade, aFade, drawChat, drawScroll)

	args := []string{
		"-re",
		"-ss", fmt.Sprintf("%f", offset), "-t", "1.0", "-i", currentPath,
		"-re", "-i", nextPath,
		"-filter_complex", filter,
		"-map", "[vout]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-maxrate", "3000k", "-bufsize", "6000k",
		"-pix_fmt", "yuv420p", "-g", "60",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	logFile, _ := os.OpenFile("ffmpeg.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	e.CurrentCmd = cmd
	return cmd
}

func (e *StreamEngine) StreamSingle(videoDir, fileName, streamURL string, opts StreamOptions) *exec.Cmd {
	e.Stop()
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
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
		"-f", "flv", streamURL,
	}

	cmd := exec.Command(e.FFmpegPath, args...)
	logFile, _ := os.OpenFile("ffmpeg.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	e.CurrentCmd = cmd
	return cmd
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
