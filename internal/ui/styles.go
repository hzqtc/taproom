package ui

import "github.com/charmbracelet/lipgloss"

var (
	highlightColor          = lipgloss.Color("#FFD580")
	highlightForegroudColor = lipgloss.Color("#2E2E2E")
	borderColor             = lipgloss.Color("#909090")
	focusedBorderColor      = highlightColor
	errBorderColor          = lipgloss.Color("#EF4444")

	roundedBorder = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			BorderStyle(roundedBorder).
			BorderForeground(borderColor)

	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)
)
