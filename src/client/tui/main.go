// Package tui
package tui

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea2 "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// "github.com/charmbracelet/lipgloss"
	"github.com/derkajecht/Beatrice/src/shared"
)

// Composer constants.
const (
	composerPrompt  = " ❯ "
	composerPromptW = 3 // visual cells of the prompt
	headerHeight    = 1
)

type model struct {
	packetCh      <-chan shared.GeneralPacket
	logCh         <-chan []byte
	nickname      string
	send          func(content string) error
	sendPresence  func(status string) error
	active        bool
	activityTimer time.Time
	viewport      viewport.Model
	sidebar       Section
	chat          Section
	header        Section
	input         textinput.Model
	tooSmall      bool
}

type (
	logMsg            string
	packetMsg         shared.GeneralPacket
	inactivityTickMsg struct{}
)

// chatMsg carries a single incoming or outgoing message. Own-ness (sent vs
// received) is derived from the model's nickname when it is rendered.
type chatMsg struct {
	Sender  string
	Content string
}

type leaveMsg struct {
	Nickname string
	Content  string
}

type joinMsg struct {
	Nickname string
}

type nicknameMsg struct {
	Nickname string
}

// presenceMsg carries a peer's (or the local user's) presence status into
// the sidebar.
type presenceMsg struct {
	Nickname string
	Status   UserStatus
}

// scrollMsg scrolls the chat history. A negative value scrolls up.
type scrollMsg int

// chatResizeMsg re-flows the chat viewport when the terminal resizes.
type chatResizeMsg struct {
	width  int
	height int
}

// NewModel builds the TUI model. nickname and send wire up the message
// composer: on Enter, the composer text is handed to send as plaintext; the
// client backend handles encryption and per-recipient fan-out. sendPresence
// is the separate plaintext presence path used by the inactivity timer.
func NewModel(packetCh <-chan shared.GeneralPacket, logCh <-chan []byte, nickname string, send func(content string) error, sendPresence func(status string) error) model {
	input := newComposer()
	input.Focus()
	return model{
		packetCh:      packetCh,
		logCh:         logCh,
		nickname:      nickname,
		active:        true,
		activityTimer: time.Now(),
		send:          send,
		sendPresence:  sendPresence,
		sidebar:       newSidebarModel(),
		chat:          newChatModel(nickname),
		header:        newHeaderModel(),
		input:         input,
	}
}

// newComposer builds a styled single-line message input.
func newComposer() textinput.Model {
	in := textinput.New()
	in.Prompt = composerPrompt
	in.Placeholder = "Type a message…"
	in.CharLimit = 2000

	// TODO: make colours user configurable. Default to false
	styles := textinput.DefaultDarkStyles()
	styles.Cursor.Blink = true
	styles.Cursor.Color = lg.Color("13")
	styles.Focused.Prompt = lgStyle("13", true)
	styles.Focused.Text = lgStyle("default", false)
	styles.Focused.Placeholder = lgStyle("8", false)
	styles.Blurred.Prompt = lgStyle("7", false)
	styles.Blurred.Text = lgStyle("default", false)
	styles.Blurred.Placeholder = lgStyle("8", false)
	in.SetStyles(styles)
	return in
}

// lgStyle builds a v2 lipgloss style for the bubbles v2 textinput, which uses
// a different lipgloss version than the rest of the TUI.
func lgStyle(hex string, bold bool) lg.Style {
	s := lg.NewStyle().Foreground(lg.Color(hex))
	if bold {
		s = s.Bold(true)
	}
	return s
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitForLog(m.logCh), waitForPacket(m.packetCh), waitForInactivity())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height)
		m.tooSmall = msg.Width < 44 || msg.Height < 10
		var cmd tea.Cmd
		m, cmd = m.markActive()
		m = m.resize()
		return m, cmd

	case tea.KeyMsg:
		var cmd tea.Cmd
		m, cmd = m.markActive()
		next, keyCmd := m.handleKey(msg)
		return next, tea.Batch(cmd, keyCmd)

	case logMsg:
		updated, _ := m.chat.Update(msg)
		m.chat = updated
		return m, waitForLog(m.logCh)

	case packetMsg:
		var cmd tea.Cmd
		m, cmd = m.handlePacket(msg)
		return m, tea.Batch(waitForPacket(m.packetCh), cmd)

	case inactivityTickMsg:
		// The tick is started once in Init and re-armed here on every fire,
		// so exactly one timer is ever pending. On expiry, transition
		// active -> away exactly once and announce it via the presence
		// path
		if inactivityCheck(&m) && m.active {
			m.active = false
			return m, tea.Batch(m.presenceCmd(shared.PresenceAway), waitForInactivity())
		}
		return m, waitForInactivity()

	}
	return m, nil
}

// markActive records user activity: it resets the inactivity timer and, on
// the single away -> active transition, announces the change via the
// presence path.
func (m model) markActive() (model, tea.Cmd) {
	m.activityTimer = time.Now()
	if m.active {
		return m, nil
	}
	m.active = true
	return m, m.presenceCmd(shared.PresenceActive)
}

// presenceCmd sends a presence update off the update loop. Failures are
// logged; presence is best-effort and never blocks the TUI.
func (m model) presenceCmd(status string) tea.Cmd {
	return func() tea.Msg {
		if err := m.sendPresence(status); err != nil {
			slog.Error("failed to send presence update", "status", status, "err", err)
		}
		return nil
	}
}

// resize re-flows the chat viewport and composer to the current terminal size.
func (m model) resize() model {
	_, chatCW, chatCH, _ := m.layout()
	m.chat, _ = m.chat.Update(chatResizeMsg{width: chatCW, height: chatCH})
	m.input.SetWidth(chatCW - composerPromptW)
	return m
}

// handleKey routes key presses between the composer and chat scrolling.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits, whether or not the composer is focused.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.input.Focused() {
		return m.handleComposerKey(msg)
	}
	return m.handleIdleKey(msg)
}

// handleComposerKey handles input while the composer is focused.
func (m model) handleComposerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.submit()
	case "esc":
		if m.input.Value() == "" {
			m.input.Blur()
			return m, nil
		}
		m.input.Reset()
		return m, nil
	}

	var cmd tea2.Cmd
	m.input, cmd = m.input.Update(toV2Key(msg))
	_ = cmd // v2 cursor cmd; cursor is intentionally static
	return m, nil
}

// handleIdleKey handles keys while the composer is blurred.
func (m model) handleIdleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "enter":
		m.input.Focus()
		return m, nil
	case "up":
		m.chat, _ = m.chat.Update(scrollMsg(-1))
		return m, nil
	case "down":
		m.chat, _ = m.chat.Update(scrollMsg(1))
		return m, nil
	case "pgup":
		_, _, _, page := m.layout()
		m.chat, _ = m.chat.Update(scrollMsg(-max(1, page/2)))
		return m, nil
	case "pgdown":
		_, _, _, page := m.layout()
		m.chat, _ = m.chat.Update(scrollMsg(max(1, page/2)))
		return m, nil
	}

	// Any printable key re-focuses the composer and begins typing.
	if msg.Type == tea.KeyRunes {
		m.input.Focus()
		var cmd tea2.Cmd
		m.input, cmd = m.input.Update(toV2Key(msg))
		_ = cmd
	}
	return m, nil
}

// submit sends the current composer value as a plaintext message.
func (m model) submit() (tea.Model, tea.Cmd) {
	rawContent := strings.TrimSpace(m.input.Value())
	if rawContent == "" {
		return m, nil
	}

	// Plaintext is handed to the client backend, which encrypts and fans out
	// one MessagePacket per recipient. It is never serialized onto the wire.
	if err := m.send(rawContent); err != nil {
		slog.Error("failed to send message", "err", err)
		if h, ok := m.header.(HeaderConfig); ok {
			h.Status = "send failed: " + err.Error()
			m.header = h
		}
		return m, nil // keep the text so the user can retry
	}

	m.input.Reset()

	// Local echo: show the sender's own plaintext once, locally only.
	s, _ := m.chat.Update(chatMsg{Sender: m.nickname, Content: rawContent})
	m.chat = s
	return m, nil
}

func (m model) handlePacket(p packetMsg) (model, tea.Cmd) {
	switch p.Type {
	case "n":
		var np shared.NicknameUpdatePacket
		if err := json.Unmarshal(p.Message, &np); err != nil {
			slog.Debug("failed to unmarshal NickPacket", "err", err)
			return m, nil
		}
		m.nickname = np.Nickname
		m.chat, _ = m.chat.Update(nicknameMsg{Nickname: np.Nickname})
		return m, nil
	case "d":
		var dp shared.DirPacket
		if err := json.Unmarshal(p.Message, &dp); err != nil {
			slog.Debug("failed to unmarshal DirPacket", "err", err)
			return m, nil
		}
		for k := range dp.CurrentUsers {
			s, _ := m.sidebar.Update(k)
			m.sidebar = s
		}
		return m, nil
	case "l":
		var lp shared.LeavePacket
		if err := json.Unmarshal(p.Message, &lp); err != nil {
			slog.Debug("failed to unmarshal LeavePacket", "err", err)
			return m, nil
		}

		s, chatCmd := m.chat.Update(leaveMsg{Nickname: lp.Nickname, Content: "has left the chat."})
		m.chat = s

		s, sidebarCmd := m.sidebar.Update(leaveMsg{Nickname: lp.Nickname})
		m.sidebar = s

		return m, tea.Batch(chatCmd, sidebarCmd)
	case "j":
		var jp shared.JoinPacket
		if err := json.Unmarshal(p.Message, &jp); err != nil {
			slog.Debug("failed to unmarshal JoinPacket", "err", err)
			return m, nil
		}
		s, cmd := m.sidebar.Update(joinMsg{jp.Nickname})
		m.sidebar = s
		return m, cmd
	case "p":
		// Presence updates arrive with the nickname already bound to the
		// authenticated connection server-side; update the matching sidebar
		// user (including the local user, who is in the directory too).
		var pp shared.PresencePacket
		if err := json.Unmarshal(p.Message, &pp); err != nil {
			slog.Debug("failed to unmarshal PresencePacket", "err", err)
			return m, nil
		}
		s, cmd := m.sidebar.Update(presenceMsg{Nickname: pp.Nickname, Status: presenceStatus(pp.Status)})
		m.sidebar = s
		return m, cmd
	case "m":
		// Inbound messages arrive already decrypted by the client backend as
		// a local DecryptedMessage; the encrypted wire form never reaches here.
		var dm shared.DecryptedMessage
		if err := json.Unmarshal(p.Message, &dm); err != nil {
			slog.Debug("failed to unmarshal DecryptedMessage", "err", err)
			return m, nil
		}
		s, cmd := m.chat.Update(chatMsg{Sender: dm.Sender, Content: dm.Content})
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

// presenceStatus maps a wire presence status to the sidebar's UserStatus.
// Unknown values fall back to active so a malformed packet can't hide a user.
func presenceStatus(s string) UserStatus {
	if s == shared.PresenceAway {
		return StatusAway
	}
	return StatusActive
}

// layout returns (sidebarWidth, chatContentWidth, chatContentHeight, bodyHeight)
// where the chat content dimensions are measured inside the chat panel border.
func (m model) layout() (int, int, int, int) {
	termW := m.viewport.Width()
	termH := m.viewport.Height()

	bodyH := termH - headerHeight

	sidebarW := max(termW/5, 14)

	if termW-sidebarW < 34 {
		sidebarW = termW - 34
	}
	if sidebarW < 14 {
		sidebarW = 14
	}
	if sidebarW > termW-2 {
		sidebarW = termW - 2
	}

	chatW := termW - sidebarW
	chatCW := chatW - 2 // left + right border

	// Chat column: top border (1) + messages + divider (1) + composer (1) + bottom border (1).
	chatCH := max(bodyH-4, 1)

	return sidebarW, chatCW, chatCH, bodyH
}

func (m model) View() string {
	// bubbletea renders View() before the first WindowSizeMsg, when the
	// viewport dimensions are still 0. Bail out until we know the size.
	if m.viewport.Width() <= 0 || m.viewport.Height() <= 0 {
		return ""
	}

	if m.tooSmall {
		msg := lipgloss.NewStyle().Foreground(mutedC).Render("Terminal too small — enlarge to chat.")
		return lipgloss.Place(
			m.viewport.Width(), m.viewport.Height(),
			lipgloss.Center, lipgloss.Center,
			msg,
		)
	}

	sw, chatCW, chatCH, bodyH := m.layout()

	header := m.header.View(m.viewport.Width(), headerHeight, true)
	sidebar := m.sidebar.View(sw, bodyH, false)
	messages := m.chat.View(chatCW, chatCH, true)

	// Unified chat panel: messages share one border with the composer below.
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(borderC).
		Width(chatCW).
		Height(chatCH).
		Render(messages)

	// divider := lipgloss.NewStyle().
	// 	Width(chatCW).
	// 	Foreground(borderC).
	// 	Render("├" + strings.Repeat("─", chatCW-2) + "┤")

	composer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(m.composerBorderC()).
		Width(chatCW).
		Height(1).
		Render(m.input.View())

	chatColumn := lipgloss.JoinVertical(lipgloss.Left, panel, composer)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatColumn)

	return lipgloss.JoinVertical(lipgloss.Left, body, header)
}

// composerBorderC highlights the composer's border while it has focus, a small
// affordance that signals "you can type here".
func (m model) composerBorderC() lipgloss.TerminalColor {
	if m.input.Focused() {
		return borderHi
	}
	return borderC
}

// toV2Key bridges a bubbletea v1 KeyMsg to the bubbles v2 KeyPressMsg that the
// textinput component expects. The two libraries use different tea types.
func toV2Key(k tea.KeyMsg) tea2.KeyPressMsg {
	var out tea2.KeyPressMsg

	if k.Type == tea.KeyRunes {
		if len(k.Runes) == 0 {
			return out
		}
		if k.Alt {
			// Alt + rune is a chord (e.g. alt+d); keep Text empty so the key
			// renders as "alt+d" and can match modifier-aware bindings.
			out.Code = k.Runes[0]
			out.Mod |= tea2.ModAlt
			return out
		}
		out.Text = string(k.Runes)
		out.Code = k.Runes[0]
		return out
	}

	switch k.Type {
	case tea.KeyEnter:
		out.Code = tea2.KeyEnter
	case tea.KeyBackspace:
		out.Code = tea2.KeyBackspace
	case tea.KeyTab:
		out.Code = tea2.KeyTab
	case tea.KeyEsc:
		out.Code = tea2.KeyEsc
	case tea.KeyDelete:
		out.Code = tea2.KeyDelete
	case tea.KeyInsert:
		out.Code = tea2.KeyInsert
	case tea.KeySpace:
		out.Text = " "
		out.Code = tea2.KeySpace
	case tea.KeyUp:
		out.Code = tea2.KeyUp
	case tea.KeyDown:
		out.Code = tea2.KeyDown
	case tea.KeyRight:
		out.Code = tea2.KeyRight
	case tea.KeyLeft:
		out.Code = tea2.KeyLeft
	case tea.KeyHome:
		out.Code = tea2.KeyHome
	case tea.KeyEnd:
		out.Code = tea2.KeyEnd
	case tea.KeyPgUp:
		out.Code = tea2.KeyPgUp
	case tea.KeyPgDown:
		out.Code = tea2.KeyPgDown
	case tea.KeyShiftTab:
		out.Code = tea2.KeyTab
		out.Mod |= tea2.ModShift
	default:
		if k.Type >= tea.KeyCtrlA && k.Type <= tea.KeyCtrlZ {
			out.Code = rune('a') + rune(k.Type-tea.KeyCtrlA)
			out.Mod |= tea2.ModCtrl
		} else {
			out.Text = k.String()
		}
	}

	if k.Alt {
		out.Mod |= tea2.ModAlt
	}
	return out
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

// inactivityCheck returns true if 120s has passed since last user input
func inactivityCheck(m *model) bool {
	return time.Since(m.activityTimer) >= 120*time.Second
}

func waitForInactivity() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return inactivityTickMsg{}
	})
}
