package queue

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type VideoQueue struct {
	Dir string
}

func NewVideoQueue(dir string) *VideoQueue {
	return &VideoQueue{Dir: dir}
}

func (q *VideoQueue) ListVideos() ([]string, error) {
	files, err := os.ReadDir(q.Dir)
	if err != nil {
		return nil, err
	}

	var videos []string
	for _, f := range files {
		if !f.IsDir() {
			ext := strings.ToLower(filepath.Ext(f.Name()))
			if ext == ".mp4" || ext == ".mkv" || ext == ".mov" {
				videos = append(videos, f.Name())
			}
		}
	}
	sort.Strings(videos)
	return videos, nil
}

func (q *VideoQueue) WatchFolder(onChange func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					onChange()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return watcher.Add(q.Dir)
}

func (q *VideoQueue) GetNextVideo(currentVideo string) (string, error) {
	videos, err := q.ListVideos()
	if err != nil {
		return "", err
	}

	if len(videos) == 0 {
		return "", nil
	}

	if currentVideo == "" {
		return videos[0], nil
	}

	for i, v := range videos {
		if v == currentVideo {
			if i+1 < len(videos) {
				return videos[i+1], nil
			}
			// Loop back to first video
			return videos[0], nil
		}
	}

	// Se o currentVideo não estiver na lista (foi apagado), retorna o primeiro
	return videos[0], nil
}
