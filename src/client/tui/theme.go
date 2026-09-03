package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ThemeConfig is the user-configurable palette for the TUI. Values are
// color specifiers compatible with lipgloss: ANSI palette indices ("8",
// "13") or hex strings ("#ff00aa").
type ThemeConfig struct {
	Border      string `yaml:"border"`
	BorderFocus string `yaml:"border_focus"`
	Accent      string `yaml:"accent"`
	Text        string `yaml:"text"`
	Muted       string `yaml:"muted"`
	Faint       string `yaml:"faint"`
	Active      string `yaml:"active"`
	Away        string `yaml:"away"`
	Error       string `yaml:"error"`
}

// DefaultTheme returns the current visual defaults — the same ANSI palette
// values the TUI has used since first release. New code paths that build
// styles should source from this rather than hardcoding color strings.
func DefaultTheme() ThemeConfig {
	return ThemeConfig{
		Border:      "8",
		BorderFocus: "13",
		Accent:      "13",
		Text:        "15",
		Muted:       "7",
		Faint:       "8",
		Active:      "10",
		Away:        "11",
		Error:       "9",
	}
}

// lipgloss v1 color vars. These are consumed by the v1 chat, sidebar and
// header views (which use the v1 lipgloss module).
var (
	borderC  = lipgloss.Color("8")
	borderHi = lipgloss.Color("13")
	accentC  = lipgloss.Color("13")
	textC    = lipgloss.Color("15")
	mutedC   = lipgloss.Color("7")
	faintC   = lipgloss.Color("8")
	activeC  = lipgloss.Color("10")
	awayC    = lipgloss.Color("11")
	errC     = lipgloss.Color("9")
)

// Raw color strings retained for the v2 lipgloss composer (the bubbles v2
// textinput uses a different lipgloss version whose Color type expects raw
// strings, not the v1 lipgloss.Color wrapper).
var (
	rawBorder      = "8"
	rawBorderFocus = "13"
	rawAccent      = "13"
	rawText        = "15"
	rawMuted       = "7"
	rawFaint       = "8"
	rawActive      = "10"
	rawAway        = "11"
	rawErr         = "9"
)

// ApplyTheme updates both the v1 lipgloss color vars and the raw string
// values that the v2 composer reads. Empty fields leave the existing values
// untouched, so partial config updates are safe.
func ApplyTheme(t ThemeConfig) {
	setThemeColor(&borderC, &rawBorder, t.Border)
	setThemeColor(&borderHi, &rawBorderFocus, t.BorderFocus)
	setThemeColor(&accentC, &rawAccent, t.Accent)
	setThemeColor(&textC, &rawText, t.Text)
	setThemeColor(&mutedC, &rawMuted, t.Muted)
	setThemeColor(&faintC, &rawFaint, t.Faint)
	setThemeColor(&activeC, &rawActive, t.Active)
	setThemeColor(&awayC, &rawAway, t.Away)
	setThemeColor(&errC, &rawErr, t.Error)
}

func setThemeColor(target *lipgloss.Color, raw *string, value string) {
	if value == "" {
		return
	}
	*target = lipgloss.Color(value)
	*raw = value
}

// rawThemeSnapshot returns the current raw color strings. It exists so the
// v2 composer can read them without taking a dependency on the v1 color
// vars above.
func rawThemeSnapshot() (border, borderFocus, accent, text, muted, faint string) {
	return rawBorder, rawBorderFocus, rawAccent, rawText, rawMuted, rawFaint
}
