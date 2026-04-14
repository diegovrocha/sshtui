package ui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme name: "dark" or "light".
var activeTheme = "dark"

var (
	ColorCyan    lipgloss.Color
	ColorMagenta lipgloss.Color
	ColorGreen   lipgloss.Color
	ColorRed     lipgloss.Color
	ColorYellow  lipgloss.Color
	ColorDim     lipgloss.Color

	TitleStyle     lipgloss.Style
	SubtitleStyle  lipgloss.Style
	DimStyle       lipgloss.Style
	ActiveStyle    lipgloss.Style
	InactiveStyle  lipgloss.Style
	SeparatorStyle lipgloss.Style
	DescStyle      lipgloss.Style
	SuccessStyle   lipgloss.Style
	ErrorStyle     lipgloss.Style
	WarnStyle      lipgloss.Style
	BoxStyle       lipgloss.Style
)

// Version is set at build time via ldflags by GoReleaser.
// Falls back to "dev" when built locally without ldflags.
var Version = "dev"

func init() {
	// Env var override takes precedence over autodetection.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SSHTUI_THEME"))); v == "light" || v == "dark" {
		activeTheme = v
	} else {
		activeTheme = detectTheme()
	}
	applyTheme(activeTheme)
}

// detectTheme inspects $COLORFGBG to guess the terminal background.
// Format: "fg;bg" (or "fg;aux;bg"). bg <= 6 or in {0,8} → dark.
// bg >= 7 or in {7,15} → light. Defaults to dark if missing or unparseable.
func detectTheme() string {
	raw := os.Getenv("COLORFGBG")
	if raw == "" {
		return "dark"
	}
	parts := strings.Split(raw, ";")
	if len(parts) < 2 {
		return "dark"
	}
	bgStr := strings.TrimSpace(parts[len(parts)-1])
	bg, err := strconv.Atoi(bgStr)
	if err != nil {
		return "dark"
	}
	// Common conventions: 0,8 dark backgrounds; 7,15 light.
	if bg == 0 || bg == 8 {
		return "dark"
	}
	if bg == 7 || bg == 15 {
		return "light"
	}
	if bg <= 6 {
		return "dark"
	}
	return "light"
}

// ForceTheme overrides autodetection. Accepts "dark" or "light".
// Any other value is ignored.
func ForceTheme(name string) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n != "dark" && n != "light" {
		return
	}
	activeTheme = n
	applyTheme(activeTheme)
}

// ActiveTheme returns the currently active theme name.
func ActiveTheme() string {
	return activeTheme
}

func applyTheme(name string) {
	switch name {
	case "light":
		ColorCyan = lipgloss.Color("6")
		ColorMagenta = lipgloss.Color("5")
		ColorGreen = lipgloss.Color("2")
		ColorRed = lipgloss.Color("1")
		ColorYellow = lipgloss.Color("3")
		ColorDim = lipgloss.Color("238")
	default:
		// dark (current behavior)
		ColorCyan = lipgloss.Color("14")
		ColorMagenta = lipgloss.Color("5")
		ColorGreen = lipgloss.Color("2")
		ColorRed = lipgloss.Color("1")
		ColorYellow = lipgloss.Color("3")
		ColorDim = lipgloss.Color("8")
	}

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorCyan)

	SubtitleStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorMagenta)

	DimStyle = lipgloss.NewStyle().
		Faint(true)

	ActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorCyan)

	InactiveStyle = lipgloss.NewStyle()

	SeparatorStyle = lipgloss.NewStyle().
		Faint(true)

	DescStyle = lipgloss.NewStyle().
		Foreground(ColorMagenta)

	SuccessStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGreen)

	ErrorStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorRed)

	WarnStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorYellow)

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorCyan).
		Padding(1, 2)
}

func Banner() string {
	t := TitleStyle.Render
	d := DimStyle.Render
	s := SubtitleStyle.Render

	// All logo lines padded to 31 chars so right-side text aligns
	var b strings.Builder
	b.WriteString(t("         _   _____ _   _ ___  ") + "\n")
	b.WriteString(t(" ___ ___| |_|_   _| | | |_ _| ") + "  " + s("SSH + TUI") + "\n")
	b.WriteString(t("/ __/ __| '_ \\| | | | | || |  ") + "  " + s("Inspect and generate SSH keys") + "\n")
	b.WriteString(t("\\__ \\__ \\ | | | | | |_| || |  ") + "  " + s("and certificates.") + "\n")
	b.WriteString(t("|___/___/_| |_|_|  \\___/|___| ") + "  " + d("https://github.com/diegovrocha/sshtui") + "\n")
	b.WriteString(d("                                 v"+Version) + "\n")
	return b.String()
}
