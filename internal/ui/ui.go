package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"crom-mediastream/internal/daemon"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
	titleStyle  = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("170"))
	statusStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	activeTabStyle   = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Bold(true)
	inactiveTabStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("241"))
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

type tickMsg time.Time
type stateMsg daemon.DaemonState
type errorMsg string

type Model struct {
	list        list.Model
	state       daemon.DaemonState
	newsInput   textinput.Model
	activeTab   int
	cursor      int
	resolutions []string
	resIndex    int
	connected   bool
	errStr      string
}

func sendCommand(action string, video string, news string, res string) {
	payload := daemon.CommandPayload{
		Action:     action,
		Video:      video,
		NewsText:   news,
		Resolution: res,
	}
	body, _ := json.Marshal(payload)
	http.Post("http://localhost:8080/command", "application/json", bytes.NewBuffer(body))
}

func fetchStateCmd() tea.Cmd {
	return func() tea.Msg {
		var state daemon.DaemonState
		client := &http.Client{Timeout: 1 * time.Second}
		resp, err := client.Get("http://localhost:8080/state")
		if err != nil {
			return errorMsg("Connection refused. Is the Daemon running?")
		}
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&state)
		if err != nil {
			return errorMsg("Failed to decode daemon state")
		}
		return stateMsg(state)
	}
}

func NewModel() Model {
	l := list.New([]list.Item{}, itemDelegate{}, 40, 20)
	l.Title = "FILA DE REPRODUÇÃO (SYNCED)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1).
		Bold(true)

	ti := textinput.New()
	ti.Placeholder = "Digite a noticia e aperte Enter..."
	ti.CharLimit = 156
	ti.Width = 40

	res := []string{"1920x1080", "1280x720", "854x480"}

	return Model{
		list:        l,
		newsInput:   ti,
		resolutions: res,
		resIndex:    1,
		connected:   false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStateCmd(),
		m.tick(),
		textinput.Blink,
	)
}

func (m Model) tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case errorMsg:
		m.connected = false
		m.errStr = string(msg)
	case stateMsg:
		m.connected = true
		m.state = daemon.DaemonState(msg)
		// Sync playlist
		var items []list.Item
		for _, v := range m.state.Playlist {
			items = append(items, item(v))
		}
		m.list.SetItems(items)

		// Sync news text initial load
		if m.newsInput.Value() == "" && m.state.NewsText != "" {
			m.newsInput.SetValue(m.state.NewsText)
		}

		// Sync resolution index
		for i, r := range m.resolutions {
			if r == m.state.Resolution {
				m.resIndex = i
			}
		}

	case tea.KeyMsg:
		if m.activeTab == 1 && m.cursor == 4 {
			if msg.String() == "up" || msg.String() == "down" || msg.String() == "tab" || msg.String() == "esc" {
				m.newsInput.Blur()
			} else if msg.String() == "enter" {
				sendCommand("update_news", "", m.newsInput.Value(), "")
				return m, fetchStateCmd()
			} else {
				m.newsInput, cmd = m.newsInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if !(m.activeTab == 1 && m.cursor == 4) {
				return m, tea.Quit // Apenas fecha a UI, o Daemon continua rodando!
			}

		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			return m, nil

		case "up", "k":
			if m.activeTab == 1 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = 5
				}
				if m.cursor == 4 {
					m.newsInput.Focus()
				} else {
					m.newsInput.Blur()
				}
			}
		case "down", "j":
			if m.activeTab == 1 {
				m.cursor++
				if m.cursor > 5 {
					m.cursor = 0
				}
				if m.cursor == 4 {
					m.newsInput.Focus()
				} else {
					m.newsInput.Blur()
				}
			}
		case "left", "h":
			if m.activeTab == 1 && m.cursor == 5 {
				m.resIndex--
				if m.resIndex < 0 {
					m.resIndex = len(m.resolutions) - 1
				}
				sendCommand("set_resolution", "", "", m.resolutions[m.resIndex])
			}
		case "right", "l":
			if m.activeTab == 1 && m.cursor == 5 {
				m.resIndex = (m.resIndex + 1) % len(m.resolutions)
				sendCommand("set_resolution", "", "", m.resolutions[m.resIndex])
			}
		case "enter", " ":
			if m.activeTab == 1 {
				if m.cursor == 0 {
					sendCommand("toggle_autodj", "", "", "")
				} else if m.cursor == 1 {
					sendCommand("toggle_loop", "", "", "")
				} else if m.cursor == 2 {
					sendCommand("toggle_chat", "", "", "")
				} else if m.cursor == 3 {
					sendCommand("toggle_scroll", "", "", "")
				}
				return m, fetchStateCmd()
			}

			if m.activeTab == 0 && msg.String() == "enter" {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					sendCommand("start", string(i), "", "")
					return m, fetchStateCmd()
				}
			}

		case "s":
			if m.activeTab == 0 {
				sendCommand("stop", "", "", "")
				return m, fetchStateCmd()
			}

		case "r":
			return m, fetchStateCmd()
		}

	case tickMsg:
		return m, tea.Batch(m.tick(), fetchStateCmd())

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-8)
	}

	if m.activeTab == 0 {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if !m.connected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(2, 4).Render(
			"❌ FATAL ERROR: " + m.errStr + "\n\n" +
				"Por favor, inicie o servidor primeiro rodando o comando em background:\n" +
				"  $ ./crom-mediastream daemon &\n\n" +
				"Aperte Q para sair.",
		)
	}

	tabs := ""
	if m.activeTab == 0 {
		tabs = activeTabStyle.Render(" MONITOR ") + " " + inactiveTabStyle.Render(" SETTINGS ")
	} else {
		tabs = inactiveTabStyle.Render(" MONITOR ") + " " + activeTabStyle.Render(" SETTINGS ")
	}
	tabs += "\n\n"

	content := ""
	if m.activeTab == 0 {
		content = m.list.View()
	} else {
		content = "  CROM MEDIASTREAM SETTINGS\n\n"

		opts := []string{
			fmt.Sprintf("[%s] Enable Auto-DJ (Loop playlist automatically)", map[bool]string{true: "x", false: " "}[m.state.AutoDJEnabled]),
			fmt.Sprintf("[%s] Enable Loop Mode", map[bool]string{true: "x", false: " "}[m.state.LoopEnabled]),
			fmt.Sprintf("[%s] Enable Chat Overlay (Top-Left)", map[bool]string{true: "x", false: " "}[m.state.ChatEnabled]),
			fmt.Sprintf("[%s] Enable Scroll News (Bottom)", map[bool]string{true: "x", false: " "}[m.state.ScrollEnabled]),
			"    News Text     : " + m.newsInput.View(),
			fmt.Sprintf("    Resolution    : < %s >", m.resolutions[m.resIndex]),
		}

		for i, opt := range opts {
			if m.cursor == i {
				content += lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render("▶ " + opt) + "\n"
			} else {
				content += "  " + opt + "\n"
			}
		}
		content += "\n  Use Up/Down to navigate, Left/Right to change values, Enter to toggle."
	}

	statusText := "⚪ IDLE"
	color := "240"
	if m.state.Streaming {
		durDur := time.Duration(m.state.Duration) * time.Second
		elapDur := time.Duration(m.state.Elapsed) * time.Second
		durStr := durDur.Truncate(time.Second).String()
		elapStr := elapDur.Truncate(time.Second).String()
		statusText = fmt.Sprintf("🔴 LIVE: %s [%s / %s]", m.state.CurrentVideo, elapStr, durStr)
		color = "1"
	}

	status := statusStyle.Copy().
		Background(lipgloss.Color(color)).
		Foreground(lipgloss.Color("15")).
		Render(statusText)

	help := infoStyle.Render("\nTab: Trocar Aba • Enter: Ação • R: Atualizar • S: Parar • Q: Desconectar (Live Continua)")

	footer := lipgloss.JoinVertical(lipgloss.Left,
		status,
		help,
	)

	if m.state.StatusMessage != "" {
		footer += "\n" + infoStyle.Render("Log: "+m.state.StatusMessage)
	}

	return docStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			tabs,
			content,
			"\n",
			footer,
		),
	)
}
