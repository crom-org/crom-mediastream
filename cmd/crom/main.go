package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"crom-mediastream/internal/config"
	"crom-mediastream/internal/daemon"
	"crom-mediastream/internal/engine"
	"crom-mediastream/internal/queue"
	"crom-mediastream/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	isDaemon := false
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		isDaemon = true
		// Remove "daemon" para o flag.Parse funcionar normalmente
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	var configPath string
	var videoDir string
	var port string
	flag.StringVar(&configPath, "config", "", "Caminho para o arquivo YAML customizado (ex: config_prod.yaml)")
	flag.StringVar(&videoDir, "videos", "", "Pasta de videos customizada")
	flag.StringVar(&port, "port", "", "Porta HTTP do Daemon (ex: 8080)")
	flag.Parse()

	if isDaemon {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}

		if videoDir != "" {
			cfg.VideoDir = videoDir
		}
		if port != "" {
			cfg.Port = port
		}

		eng, err := engine.NewStreamEngine()
		if err != nil {
			log.Fatalf("Error initializing engine: %v", err)
		}

		q := queue.NewVideoQueue(cfg.VideoDir)

		d := daemon.NewDaemon(cfg, eng, q)
		d.Start()
	} else {
		cfg, _ := config.LoadConfig(configPath)
		if port != "" {
			cfg.Port = port
		}
		if cfg.Port == "" {
			cfg.Port = "8080"
		}

		client := http.Client{Timeout: 1 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/state", cfg.Port))
		if err != nil {
			// Replica os argumentos originais para o daemon (ex: --config config_prod.yaml)
			daemonArgs := []string{"daemon"}
			daemonArgs = append(daemonArgs, os.Args[1:]...)
			cmd := exec.Command(os.Args[0], daemonArgs...)
			cmd.Start()
			time.Sleep(1 * time.Second)
		} else {
			resp.Body.Close()
		}

		m := ui.NewModel(cfg.Port)
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}
}
