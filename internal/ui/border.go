package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Build a custom border top for lipgloss that embeds a title in it
func getBorderTopWithTitle(title string, width int) string {
	const filler = "─"
	const lead = 1

	if width <= 0 {
		return ""
	} else if width <= len(title) {
		return title[:width] // truncate if title too long
	}

	// Compute how many dashes go on each side
	var left, right int
	if width <= len(title)+lead {
		left = 1
	} else {
		left = lead
	}
	right = width - len(title) - left

	return strings.Repeat(filler, left) + title + strings.Repeat(filler, right)
}

func getRoundedBorderWithTitle(title string, width int) lipgloss.Border {
	return lipgloss.Border{
		Top:         getBorderTopWithTitle(title, width),
		Bottom:      roundedBorder.Bottom,
		Left:        roundedBorder.Left,
		Right:       roundedBorder.Right,
		TopLeft:     roundedBorder.TopLeft,
		TopRight:    roundedBorder.TopRight,
		BottomLeft:  roundedBorder.BottomLeft,
		BottomRight: roundedBorder.BottomRight,
	}
}
