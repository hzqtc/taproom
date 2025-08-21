package ui

import "github.com/charmbracelet/lipgloss"

var (
	highlightColor          = lipgloss.Color("#FFD580")
	highlightForegroudColor = lipgloss.Color("#2E2E2E")
	borderColor             = lipgloss.Color("#909090")
	focusedBorderColor      = highlightColor
	errBorderColor          = deprecatedColor
	installedColor          = lipgloss.Color("#22C55E")
	deprecatedColor         = lipgloss.Color("#EF4444")
	uninstalledColor        = lipgloss.Color("#FBBF24")
	pinnedColor             = lipgloss.Color("#B57EDC")

	roundedBorder = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			BorderStyle(roundedBorder).
			BorderForeground(borderColor)

	installedStyle = lipgloss.NewStyle().
			Foreground(installedColor)

	deprecatedStyle = lipgloss.NewStyle().
			Foreground(deprecatedColor)

	uninstalledStyle = lipgloss.NewStyle().
				Foreground(uninstalledColor)

	outdatedStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	pinnedStyle = lipgloss.NewStyle().
			Foreground(pinnedColor)
)
