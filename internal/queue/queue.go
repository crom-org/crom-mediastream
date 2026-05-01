package queue

import (
	"os"
	"path/filepath"
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
