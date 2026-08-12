package tui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/derkajecht/Beatrice/src/shared"
)

type keyMap struct {
	Quit key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
}

type model struct {
	packetCh <-chan shared.GeneralPacket
	logCh    <-chan []byte

	viewport viewport.Model
	sidebar  Section
	chat     Section
	header   Section
	tooSmall bool
}

type logMsg string
type packetMsg shared.GeneralPacket
type chatMsg string

func NewModel(packetCh <-chan shared.GeneralPacket, logCh <-chan []byte) model {
	return model{
		packetCh: packetCh,
		logCh:    logCh,
		sidebar:  newSidebarModel(SidebarConfig{}),
		chat:     newChatModel(ChatViewConfig{}),
		header:   newHeaderModel(HeaderConfig{}),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitForLog(m.logCh), waitForPacket(m.packetCh))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height)
		m.viewport.GotoBottom()
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
	case logMsg:
		updated, _ := m.chat.Update(msg)
		m.chat = updated
		return m, waitForLog(m.logCh)
	case packetMsg:
		var cmd tea.Cmd
		m, cmd = m.handlePacket(msg)
		return m, tea.Batch(waitForPacket(m.packetCh), cmd)
	}
	return m, nil
}

func (m model) handlePacket(p packetMsg) (model, tea.Cmd) {
	switch p.Type {
	case "d":
		var dp shared.DirPacket
		if err := json.Unmarshal(p.Message, &dp); err != nil {
			slog.Debug("failed to unmarshal DirPacket", "err", err)
			return m, nil
		}
		s, cmd := m.sidebar.Update(struct{ CurrentUsers map[string][]byte }{CurrentUsers: dp.CurrentUsers})
		m.sidebar = s
		return m, cmd
	case "j":
		var jp shared.JoinPacket
		if err := json.Unmarshal(p.Message, &jp); err != nil {
			slog.Debug("failed to unmarshal JoinPacket", "err", err)
			return m, nil
		}
		s, cmd := m.sidebar.Update(struct{ CurrentUsers map[string][]byte }{CurrentUsers: map[string][]byte{jp.Nickname: jp.PubKey}}) // don't show pub key in the sidebar
		m.sidebar = s
		return m, cmd
	case "m":
		var mp shared.MessagePacket
		if err := json.Unmarshal(p.Message, &mp); err != nil {
			slog.Debug("failed to unmarshal MessagePacket", "err", err)
			return m, nil
		}
		s, cmd := m.chat.Update(chatMsg(fmt.Sprintf("%s: %s", mp.Sender, mp.Content)))
		m.chat = s
		return m, cmd
	case "e":
		var ep shared.ErrPacket
		if err := json.Unmarshal(p.Message, &ep); err != nil {
			slog.Debug("failed to unmarshal ErrPacket", "err", err)
			return m, nil
		}
		if h, ok := m.header.(HeaderConfig); ok {
			h.Status = ep.Message
			m.header = h
		}
		return m, nil
	default:
		slog.Debug("unknown packet type", "type", p.Type)
		return m, nil
	}
}

func (m model) View() string {
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

func waitForPacket(packetCh <-chan shared.GeneralPacket) tea.Cmd {
	return func() tea.Msg {
		packet, ok := <-packetCh
		if !ok {
			return nil
		}
		return packetMsg(packet)
	}
}
