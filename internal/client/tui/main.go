package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type keyMap struct {
	Quit   key.Binding
	Next   key.Binding
	Prev   key.Binding
	Select key.Binding
}

var keys = keyMap{
	Quit:   key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
	Next:   key.NewBinding(key.WithKeys("ctrl+l", "tab", "right"), key.WithHelp("ctrl+l", "next section")),
	Prev:   key.NewBinding(key.WithKeys("ctrl+h", "shift+tab", "left"), key.WithHelp("ctrl+h", "prev section")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
}

type model struct {
	logCh   <-chan []byte
	width   int
	height  int
	sidebar Section
	chat    Section
	header  Section
}

type logMsg string

func NewTUI(logCh <-chan []byte) {
	tea.NewProgram(newModel(logCh), tea.WithAltScreen()).Run()
}

func newModel(logCh <-chan []byte) model {
	return model{
		logCh:   logCh,
		sidebar: newSidebarModel(SidebarConfig{}),
		chat:    newChatModel(ChatViewConfig{}),
		header:  newHeaderModel(HeaderConfig{}),
	}
}

func (m model) Init() tea.Cmd {
	return waitForLog(m.logCh)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case logMsg:
		updated, _ := m.chat.Update(msg)
		m.chat = updated
		return m, waitForLog(m.logCh)
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	sidebarWidth := min(24, m.width/3)
	bodyWidth := m.width - sidebarWidth
	bodyHeight := max(1, m.height-4)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.sidebar.View(sidebarWidth, bodyHeight, false),
		m.chat.View(bodyWidth, bodyHeight, true),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(m.width, 1, true),
		body,
	)
}

func waitForLog(logCh <-chan []byte) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-logCh
		if !ok {
			return nil
		}
		return logMsg(strings.TrimSpace(string(message)))
	}
}
