package ui

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"crom-mediastream/internal/api"
	"crom-mediastream/internal/config"
	"crom-mediastream/internal/engine"
	"crom-mediastream/internal/queue"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
	titleStyle  = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("170"))
	statusStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type item string

func (i item) FilterValue() string { return string(i) }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := lipgloss.NewStyle().PaddingLeft(4).Render
	if index == m.Index() {
		fn = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170")).Bold(true).Render
		str = "▶ " + str
	}

	fmt.Fprint(w, fn(str))
}

type videosLoadedMsg []string
type tickMsg time.Time

type Model struct {
	list          list.Model
	cfg           *config.Config
	eng           *engine.StreamEngine
	queue         *queue.VideoQueue
	twitch        *api.TwitchAPI
	currentVideo  string
	startTime     time.Time
	streaming     bool
	statusMessage string
}

func NewModel(cfg *config.Config, eng *engine.StreamEngine, q *queue.VideoQueue) Model {
	videos, _ := q.ListVideos()
	var items []list.Item
	for _, v := range videos {
		items = append(items, item(v))
	}

	l := list.New(items, itemDelegate{}, 40, 20)
	l.Title = "CROM MEDIASTREAM MONITOR"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1).
		Bold(true)

	tAPI := api.NewTwitchAPI(cfg.TwitchClientID, cfg.TwitchToken, cfg.TwitchUserID)

	return Model{
		list:   l,
		cfg:    cfg,
		eng:    eng,
		queue:  q,
		twitch: tAPI,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshList(),
		m.tick(),
	)
}

func (m Model) tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.eng.Stop()
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				nextVideo := string(i)
				title := "Streaming: " + nextVideo
				go m.twitch.UpdateStreamMetadata(title)

				if m.streaming && m.currentVideo != "" {
					elapsed := time.Since(m.startTime).Seconds()
					m.statusMessage = fmt.Sprintf("Fading to %s...", nextVideo)
					currentPath := filepath.Join(m.cfg.VideoDir, m.currentVideo)
					nextPath := filepath.Join(m.cfg.VideoDir, nextVideo)
					
					cmd := m.eng.StreamWithFade(currentPath, nextPath, m.cfg.GetFullStreamURL(), elapsed)
					go cmd.Run()
				} else {
					m.statusMessage = fmt.Sprintf("Starting %s...", nextVideo)
					cmd := m.eng.StreamSingle(m.cfg.VideoDir, nextVideo, m.cfg.GetFullStreamURL())
					go cmd.Run()
				}
				
				m.currentVideo = nextVideo
				m.startTime = time.Now()
				m.streaming = true
			}

		case "s":
			m.eng.Stop()
			m.streaming = false
			m.currentVideo = ""
			m.statusMessage = "Stream stopped."

		case "r":
			return m, m.refreshList()
		}

	case videosLoadedMsg:
		var items []list.Item
		for _, v := range msg {
			items = append(items, item(v))
		}
		m.list.SetItems(items)
		return m, nil

	case tickMsg:
		return m, m.tick()

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-6)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) refreshList() tea.Cmd {
	return func() tea.Msg {
		videos, _ := m.queue.ListVideos()
		return videosLoadedMsg(videos)
	}
}

func (m Model) View() string {
	statusText := "⚪ IDLE"
	color := "240"
	if m.streaming {
		elapsed := time.Since(m.startTime).Truncate(time.Second)
		statusText = fmt.Sprintf("🔴 LIVE: %s [%s]", m.currentVideo, elapsed)
		color = "1"
	}

	status := statusStyle.Copy().
		Background(lipgloss.Color(color)).
		Foreground(lipgloss.Color("15")).
		Render(statusText)

	help := infoStyle.Render("\nEnter: Transmitir • R: Atualizar • S: Parar • Q: Sair")
	
	footer := lipgloss.JoinVertical(lipgloss.Left,
		status,
		help,
	)

	if m.statusMessage != "" {
		footer += "\n" + infoStyle.Render("Log: "+m.statusMessage)
	}

	return docStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.list.View(),
			"\n",
			footer,
		),
	)
}
