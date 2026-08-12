package tui

import tea "github.com/charmbracelet/bubbletea"

type Section interface {
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Section, tea.Cmd)
	View(width, height int, focused bool) string
	Chosen() string
}
