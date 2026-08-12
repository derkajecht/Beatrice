package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderConfig struct {
	Status      string `toml:"status"`
	FocusedChat string `toml:"focused_chat"`
}

func newHeaderModel(config HeaderConfig) HeaderConfig {
	return HeaderConfig{
		Status:      config.Status,
		FocusedChat: config.FocusedChat,
	}
}

func (m HeaderConfig) Init() tea.Cmd { return nil }

func (m HeaderConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	// logic to get the user list from the dir list
	return m, nil
}

func (m HeaderConfig) View(width, height int, focused bool) string {
	status := m.Status
	if status == "" {
		status = "Connected"
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(height - 2).
		Render(status)
}

func (m HeaderConfig) Name() string    { return "header" }
func (m HeaderConfig) Chosen() string  { return "" }
func (m HeaderConfig) TotalItems() int { return 1 }
func (m HeaderConfig) Width() float64  { return 1 }
