package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SidebarConfig struct {
	UserList []string `toml:"user_list"`
}

func newSidebarModel(config SidebarConfig) SidebarConfig {
	return SidebarConfig{
		UserList: config.UserList,
	}
}

func (m SidebarConfig) Init() tea.Cmd { return nil }

func (m SidebarConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	// logic to get the user list from the dir list
	return m, nil
}

func (m SidebarConfig) View(width, height int, focused bool) string {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		Render("Chats\n\n• General")
}

func (m SidebarConfig) Name() string    { return "chat" }
func (m SidebarConfig) Chosen() bool    { return true }
func (m SidebarConfig) TotalItems() int { return len(m.UserList) }
func (m SidebarConfig) Width() float64  { return 1 }

