package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

var flagTheme = pflag.String("theme", "auto", "Color theme: auto, light, dark")

var (
	highlightColor           = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD580"}
	highlightForegroundColor = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#2E2E2E"}
	borderColor              = lipgloss.AdaptiveColor{Light: "#909090", Dark: "#909090"}
	focusedBorderColor       = highlightColor
	errBorderColor           = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#EF4444"}

	installedColor   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#22C55E"}
	deprecatedColor  = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#EF4444"}
	uninstalledColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	pinnedColor      = lipgloss.AdaptiveColor{Light: "#7E22CE", Dark: "#B57EDC"}

	roundedBorder = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			BorderStyle(roundedBorder).
			BorderForeground(borderColor)

	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)
)

func InitTheme() {
	switch strings.ToLower(*flagTheme) {
	case "light":
		lipgloss.SetHasDarkBackground(false)
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	case "auto":
		// Let lipgloss auto-detect
	default:
		fmt.Fprintf(os.Stderr, "Invalid theme: %s (expected auto, light, dark)\n", *flagTheme)
		os.Exit(1)
	}
}
