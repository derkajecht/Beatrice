package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// TODO: make this user configurable
// Terminal-aware palette.
//
// Text and panels use the terminal's own foreground/background (no explicit
// color set). Borders, accents, and status indicators use ANSI palette colors
// (0–15) so they follow the user's terminal theme.
var (
	// Panels & borders — transparent backgrounds, ANSI gray borders.
	borderC  = lipgloss.Color("8")  // bright black (theme gray)
	borderHi = lipgloss.Color("13") // bright magenta (focused composer)

	// Accents — follow the terminal palette.
	accentC = lipgloss.Color("13") // bright magenta (brand, focus, sender names)

	// Text — terminal default foreground where possible, ANSI gray for secondary.
	textC  = lipgloss.Color("15") // bright white
	mutedC = lipgloss.Color("7")  // white / light gray
	faintC = lipgloss.Color("8")  // bright black / dark gray

	// Status — semantic ANSI colors.
	activeC = lipgloss.Color("10") // bright green
	awayC   = lipgloss.Color("11") // bright yellow
	errC    = lipgloss.Color("9")  // bright red
)
