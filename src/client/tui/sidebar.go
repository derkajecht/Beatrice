package tui

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UserStatus is a user's presence state. Today everyone is active; "away" is
// wired up so it can be set later without changing the render path.
type UserStatus int

const (
	StatusAway UserStatus = iota
	StatusActive
)

// SidebarUser is a single participant with their presence status.
type SidebarUser struct {
	Name   string
	Status UserStatus
}

type SidebarConfig struct {
	Users []SidebarUser
}

func newSidebarModel() SidebarConfig {
	return SidebarConfig{
		Users: make([]SidebarUser, 0, 10),
	}
}

func (m SidebarConfig) Init() tea.Cmd { return nil }

func (m SidebarConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	switch msg := msg.(type) {
	case joinMsg:
		if msg.Nickname == "" {
			return m, nil
		}
		for _, u := range m.Users {
			if u.Name == msg.Nickname {
				return m, nil // already present, don't duplicate
			}
		}
		m.Users = append(m.Users, SidebarUser{Name: msg.Nickname, Status: StatusActive})
		return m, nil
	case leaveMsg:
		if msg.Nickname == "" {
			return m, nil
		}
		m.Users = slices.DeleteFunc(m.Users, func(u SidebarUser) bool {
			return u.Name == msg.Nickname
		})
	case presenceMsg:
		if msg.Nickname == "" {
			return m, nil
		}
		for i, u := range m.Users {
			if u.Name == msg.Nickname {
				m.Users[i].Status = msg.Status
				return m, nil
			}
		}
		return m, nil
	default:
		name, ok := msg.(string)
		if !ok {
			return m, nil
		}
		for _, u := range m.Users {
			if u.Name == msg {
				return m, nil // already present, don't duplicate
			}
		}
		m.Users = append(m.Users, SidebarUser{Name: name, Status: StatusActive})
		return m, nil
	}

	return m, nil
}

func (m SidebarConfig) View(width, height int, focused bool) string {
	contentW := max(width-2, 1) // border

	contentH := max(height-2, 1)

	var b strings.Builder
	title := lipgloss.NewStyle().Foreground(textC).Bold(true).Render("\nPeople")
	// b.WriteString(titleStyle.Render("People").Align(lipgloss.Center))

	if len(m.Users) == 0 {
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("No one here yet"))
	} else {
		b.WriteString("\n")
		for _, u := range m.Users {
			// b.WriteString("\n")
			b.WriteString(userRowStyle.Render(m.renderUser(u)))
		}
	}

	users := lipgloss.NewStyle().
		Width(contentW).
		Height(contentH - 2).
		Align(lipgloss.Left).
		Render(b.String())

	content := lipgloss.JoinVertical(lipgloss.Center, title, users)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderC).
		Render(content)
}

func (m SidebarConfig) renderUser(u SidebarUser) string {
	icon, color := statusIcon(u.Status)
	return lipgloss.NewStyle().Foreground(color).Render(icon) + " " + u.Name
}

func statusIcon(s UserStatus) (icon string, color lipgloss.TerminalColor) {
	switch s {
	case StatusAway:
		return "◐", awayC
	case StatusActive:
		return "●", activeC
	}
	return "", nil
}

func (m SidebarConfig) Name() string   { return "sidebar" }
func (m SidebarConfig) Chosen() string { return "" }

var (
	// titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentC).Align(lipgloss.Center)
	mutedStyle   = lipgloss.NewStyle().Foreground(faintC).Italic(true)
	userRowStyle = lipgloss.NewStyle().Foreground(textC).PaddingLeft(1)
)
