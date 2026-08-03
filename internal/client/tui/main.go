package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
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
	Quit: key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
	Next: key.NewBinding(key.WithKeys("ctrl+l", "tab", "right"), key.WithHelp("ctrl+l", "next section")),
	Prev: key.NewBinding(key.WithKeys("ctrl+h", "shift+tab", "left"), key.WithHelp("ctrl+h", "prev section")),
	// Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
}

type model struct {
	logCh    <-chan []byte
	width    int
	height   int
	sections []Section
	focused  int
	chosen   string
	tooSmall bool
	viewport viewport.Model
	sidebar  Section
	chat     Section
	header   Section
}

type logMsg string

func NewTUI(logCh <-chan []byte) {
	tea.NewProgram(newModel(logCh), tea.WithAltScreen()).Run()
}

func newModel(logCh <-chan []byte) model {
	// TODO: read user defined config and set main chat width/height
	sections := BuildSections(ChatConfig{})

	return model{
		logCh:    logCh,
		sections: sections,
		sidebar:  newSidebarModel(SidebarConfig{}),
		chat:     newChatModel(ChatViewConfig{}),
		header:   newHeaderModel(HeaderConfig{}),
	}
}

func (m model) Init() tea.Cmd {
	// call log channel init and init all sections
	waitForLog(m.logCh)
	cmds := make([]tea.Cmd, len(m.sections))
	for i, s := range m.sections {
		cmds[i] = s.Init()
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// var cmd tea.Cmd

	switch msg := msg.(type) {
	// TODO: refine the key check, quite bad atm
	case tea.WindowSizeMsg:
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height)
		// m.width, m.height = msg.Width, msg.Height
		m.viewport.GotoBottom()
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Next):
			if len(m.sections) > 0 {
				m.focused = (m.focused + 1) % len(m.sections)
			}
		case key.Matches(msg, keys.Prev):
			if len(m.sections) > 0 {
				m.focused--
				if m.focused < 0 {
					m.focused = len(m.sections) - 1
				}
			}
			// case key.Matches(msg, keys.Select):
			// 	if len(m.sections) > 0 {
			// 		m.sections[m.focused], _ = m.sections[m.focused].Update(msg)
			// 		if chosen := m.sections[m.focused].Chosen(); chosen != "" {
			// 			m.chosen = chosen
			// 			return m, tea.Quit
			// 		}
			// 	}
		}
	case logMsg:
		// NOTE: re-enable this after building the tui
		updated, _ := m.chat.Update(msg)
		m.chat = updated
		return m, waitForLog(m.logCh)

	}
	return m, nil
}

func (m model) View() string {
	// if m.viewport.Width() == 0 {
	// 	return "Loading..."
	// }

	sidebarWidth := m.viewport.Width() / 5
	bodyWidth := m.viewport.Width() - sidebarWidth - 2
	bodyHeight := max(1, m.viewport.Height()-4)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.sidebar.View(sidebarWidth, bodyHeight, false),
		m.chat.View(bodyWidth, bodyHeight, true),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(m.viewport.Width(), 1, true),
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
