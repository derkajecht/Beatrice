package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderConfig struct {
	Status      string
	FocusedChat string
}

func newHeaderModel() HeaderConfig {
	return HeaderConfig{
		Status:      "",
		FocusedChat: "",
	}
}

func (m HeaderConfig) Init() tea.Cmd { return nil }

func (m HeaderConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	return m, nil
}

func (m HeaderConfig) View(width, height int, focused bool) string {
	// Left: brand + connection status.
	dot := "●"
	dotColor := activeC
	label := "Connected"
	if m.Status != "" {
		dot = "●"
		dotColor = errC
		label = m.Status
	}

	left := lipgloss.NewStyle().Bold(true).Foreground(accentC).Render(" Beatrice 🐶") +
		"  " +
		lipgloss.NewStyle().Foreground(dotColor).Render(dot) +
		" " +
		lipgloss.NewStyle().Foreground(mutedC).Render(label)

	right := lipgloss.NewStyle().Foreground(faintC).Render("enter send · esc clear · ctrl+c quit")

	innerW := max(width-2, 1) // 1 cell padding on each side

	gutter := max(innerW-lipgloss.Width(left)-lipgloss.Width(right), 1)

	line := left + strings.Repeat(" ", gutter) + right

	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Render(line)
}

func (m HeaderConfig) Name() string   { return "header" }
func (m HeaderConfig) Chosen() string { return "" }
