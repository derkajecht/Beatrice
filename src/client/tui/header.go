package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderConfig struct {
	Status      UserStatus
	FocusedChat string
	Nickname    string
	ErrorMsg    string
}

func newHeaderModel(nickname string) HeaderConfig {
	return HeaderConfig{
		Status:      1,
		Nickname:    nickname,
		FocusedChat: "",
	}
}

func (m HeaderConfig) Init() tea.Cmd { return nil }

func (m HeaderConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	switch msg := msg.(type) {
	case presenceMsg:
		m.Status = msg.Status
		m.ErrorMsg = ""
		return m, nil
	case errorMsg:
		if msg.Content == "" {
			return m, nil
		}
		m.ErrorMsg = msg.Content
		return m, nil
	case resetBar:
		if msg.Content {
			m.ErrorMsg = ""
		}
		return m, nil
	}
	return m, nil
}

func (m HeaderConfig) View(width, height int, focused bool) string {
	// Left: brand + connection status.
	dot := "●"
	dotColor := activeC
	label := "Connected"
	if m.Status == 0 {
		dot = "◐"
		dotColor = awayC
		label = "Away"
	}

	left := lipgloss.NewStyle().Bold(true).Foreground(mutedC).Render(m.Nickname) +
		" " +
		lipgloss.NewStyle().Foreground(dotColor).Render(dot) +
		" " +
		lipgloss.NewStyle().Foreground(mutedC).Render(label)

	// Right: hint, replaced by an active error message styled in errC.
	right := lipgloss.NewStyle().Foreground(faintC).Render("enter send · esc clear · ctrl+c quit")
	if m.ErrorMsg != "" {
		right = lipgloss.NewStyle().Foreground(errC).Italic(true).Render(m.ErrorMsg)
	}

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
