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
	if msg, ok := msg.(struct{ CurrentUsers map[string][]byte }); ok {
		m.UserList = make([]string, 0, len(msg.CurrentUsers))
		for k := range msg.CurrentUsers {
			m.UserList = append(m.UserList, k)
		}
	}
	return m, nil
}

func (m SidebarConfig) View(width, height int, focused bool) string {
	return lipgloss.NewStyle().
		Width(width - 2).
		Height(height).
		// BorderRight(true).
		Border(lipgloss.RoundedBorder()).
		Render("Active Chats")
}

func (m SidebarConfig) Name() string    { return "chat" }
func (m SidebarConfig) Chosen() string  { return "" }
func (m SidebarConfig) TotalItems() int { return len(m.UserList) }
func (m SidebarConfig) Width() float64  { return 1 }
