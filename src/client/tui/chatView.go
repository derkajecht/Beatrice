package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatViewConfig struct {
	Messages []string `toml:"messages"`
}

func newChatModel(config ChatViewConfig) ChatViewConfig {
	return ChatViewConfig{
		Messages: config.Messages,
	}
}

func (m ChatViewConfig) Init() tea.Cmd { return nil }

func (m ChatViewConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	if log, ok := msg.(logMsg); ok {
		m.Messages = append(m.Messages, string(log))
	}
	if cm, ok := msg.(chatMsg); ok {
		m.Messages = append(m.Messages, string(cm))
	}
	return m, nil
}

func (m ChatViewConfig) View(width, height int, focused bool) string {
	content := "No messages yet."
	if len(m.Messages) > 0 {
		content = strings.Join(m.Messages, "\n")
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		Render(content)
}

func (m ChatViewConfig) Name() string    { return "chat" }
func (m ChatViewConfig) Chosen() string  { return "" }
func (m ChatViewConfig) TotalItems() int { return len(m.Messages) }
func (m ChatViewConfig) Width() float64  { return 1 }
