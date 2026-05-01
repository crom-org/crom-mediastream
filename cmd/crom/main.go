package main

import (
	"fmt"
	"log"
	"os"

	"crom-mediastream/internal/config"
	"crom-mediastream/internal/engine"
	"crom-mediastream/internal/queue"
	"crom-mediastream/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 1. Load Config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Initialize Engine
	eng, err := engine.NewStreamEngine()
	if err != nil {
		log.Fatalf("Error initializing engine: %v", err)
	}

	// 3. Initialize Queue
	q := queue.NewVideoQueue(cfg.VideoDir)

	// 4. Start TUI
	m := ui.NewModel(cfg, eng, q)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
