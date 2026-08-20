package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// chatMessage is a single rendered entry in the chat log: a user message or a
// system line (logs).
type chatMessage struct {
	sender  string
	content string
	system  bool
}

type ChatViewConfig struct {
	nickname string
	messages []chatMessage
	viewport viewport.Model
}

func newChatModel(nickname string) ChatViewConfig {
	vp := viewport.New()
	vp.FillHeight = true
	return ChatViewConfig{
		nickname: nickname,
		messages: make([]chatMessage, 0, 64),
		viewport: vp,
	}
}

func (m ChatViewConfig) Init() tea.Cmd { return nil }

func (m ChatViewConfig) Update(msg tea.Msg) (Section, tea.Cmd) {
	switch msg := msg.(type) {
	case logMsg:
		m.messages = append(m.messages, chatMessage{content: string(msg), system: true})
		m.rebuild()

	case chatMsg:
		m.messages = append(m.messages, chatMessage{sender: msg.Sender, content: msg.Content})
		m.rebuild()

	case nicknameMsg:
		m.nickname = msg.Nickname
		m.rebuild()

	case chatResizeMsg:
		m.viewport.SetWidth(msg.width)
		m.viewport.SetHeight(msg.height)
		m.rebuild()

	case scrollMsg:
		if msg < 0 {
			m.viewport.ScrollUp(int(-msg))
		} else {
			m.viewport.ScrollDown(int(msg))
		}
	}

	return m, nil
}

// rebuild re-renders all messages into the viewport at the current width and
// jumps to the bottom so new messages stay visible.
func (m *ChatViewConfig) rebuild() {
	if m.viewport.Width() <= 0 || m.viewport.Height() <= 0 {
		return
	}

	contentW := m.viewport.Width()
	lines := make([]string, 0, len(m.messages)*2)

	for _, msg := range m.messages {
		if msg.system {
			lines = append(lines, renderSystemLine(msg.content, contentW))
			continue
		}
		lines = append(lines, renderMessage(msg, m.nickname, contentW)...)
	}

	if len(lines) == 0 {
		lines = []string{lipgloss.NewStyle().
			Foreground(faintC).
			Italic(true).
			Render("No messages yet — say hello!")}
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.viewport.GotoBottom()
}

// View renders the message area. The surrounding border, divider and composer
// are assembled by the parent model so the chat panel reads as one unit.
func (m ChatViewConfig) View(width, height int, focused bool) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	// Re-size defensively so a render before the first resize still works.
	if m.viewport.Width() != width || m.viewport.Height() != height {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(height)
		m.rebuild()
	}
	return m.viewport.View()
}

// renderMessage renders a single message block: name label + bubble, aligned
// left (received) or right (sent).
func renderMessage(msg chatMessage, ownNickname string, width int) []string {
	mine := msg.sender == ownNickname

	// Content width inside the bubble: total width minus the 1-cell side
	// margins, the 1-cell border and the 1-cell horizontal padding.
	contentMax := max(width-6, 4)

	content := msg.content
	if strings.TrimSpace(content) == "" {
		content = " "
	}
	bubble := renderBubble(content, contentMax, mine)

	if mine {
		return rightBlock(bubble, width)
	}

	name := lipgloss.NewStyle().Foreground(accentC).Bold(true).Render(msg.sender)
	return leftBlock(lipgloss.JoinVertical(lipgloss.Left, name, bubble))
}

// renderBubble wraps content, normalizes line widths so the bubble stays
// rectangular, and draws the border + padding.
func renderBubble(content string, contentMax int, mine bool) string {
	lines := wrapText(content, contentMax)
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Pad every line to the width of the longest line.
	longest := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > longest {
			longest = w
		}
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", longest-lipgloss.Width(l))
	}

	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder(), true)
	if mine {
		style = style.
			BorderForeground(borderHi).
			Foreground(textC)
	} else {
		style = style.
			BorderForeground(borderC).
			Foreground(textC)
	}

	return style.Render(strings.Join(lines, "\n"))
}

// wrapText wraps text into lines no wider than limit cells, hard-breaking any
// word that is itself longer than the limit.
func wrapText(text string, limit int) []string {
	if limit < 1 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if lipgloss.Width(cur)+1+lipgloss.Width(w) <= limit {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	lines = append(lines, cur)

	var out []string
	for _, l := range lines {
		if lipgloss.Width(l) <= limit {
			out = append(out, l)
			continue
		}
		out = append(out, hardBreak(l, limit)...)
	}
	return out
}

// hardBreak splits a single over-long string into chunks of at most limit cells.
func hardBreak(s string, limit int) []string {
	var (
		out   []string
		cur   []rune
		width int
	)
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if width+rw > limit && len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
			width = 0
		}
		cur = append(cur, r)
		width += rw
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// leftBlock indents a block by one cell (left margin).
func leftBlock(block string) []string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return lines
}

// rightBlock right-aligns a block with a one-cell margin from the right edge.
func rightBlock(block string, width int) []string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		w := lipgloss.Width(lines[i])
		if w < width-1 {
			lines[i] = strings.Repeat(" ", width-1-w) + lines[i]
		}
	}
	return lines
}

func renderSystemLine(content string, width int) string {
	line := lipgloss.NewStyle().Foreground(faintC).Italic(true).Render("· " + content)
	return lipgloss.PlaceHorizontal(width, lipgloss.Left, line)
}

func (m ChatViewConfig) Name() string   { return "chat" }
func (m ChatViewConfig) Chosen() string { return "" }
