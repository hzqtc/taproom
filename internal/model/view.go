package model

import (
	"fmt"
	"strings"
	"taproom/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

const (
	sidePanelWidthMin = 30
	outputMaxLines    = 10
)

var (
	flagHideHelp = pflag.Bool("hide-help", false, "Hide the help text")
)

// --- Styles ---

var (
	highlightColor  = lipgloss.Color("#FFD580")
	borderColor     = lipgloss.Color("#909090")
	errBorderColor  = deprecatedColor
	deprecatedColor = lipgloss.Color("#EF4444")

	roundedBorder = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			BorderStyle(roundedBorder).
			BorderForeground(borderColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	outputStyle = baseStyle.
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */).
			Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(highlightColor)
)

// --- View & Layout ---

// View is the main render function for the application.
func (m model) View() string {
	if m.errorMsg != "" {
		return fmt.Sprintf("An error occurred: %s\nPress 'q' to quit.", m.errorMsg)
	}

	if m.isLoading {
		var b strings.Builder
		m.spinner.Style = spinnerStyle
		b.WriteString(
			fmt.Sprintf(
				"%s\n%s\n\n%s Loading...",
				headerStyle.Render(logo),
				m.loadingPrgs.Progress(headerStyle.Render("Done")),
				m.spinner.View(),
			),
		)
		if m.loadTimer {
			b.WriteString(m.stopwatch.View())
		}
		return b.String()
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.table.View(),
		m.detailPanel.View(),
	)

	topContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.search.View(),
		m.filterView.View(),
	)

	views := []string{
		topContent,
		mainContent,
		m.statsView.View(),
	}
	if output := m.renderOutput(); output != "" {
		views = append(views, output)
	}
	if !*flagHideHelp {
		views = append(views, m.helpView.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func (m *model) renderOutput() string {
	if len(m.output) == 0 {
		return ""
	}

	var output string
	if len(m.output) > outputMaxLines {
		output = strings.Join(m.output[len(m.output)-outputMaxLines:], "\n")
	} else {
		output = strings.Join(m.output, "\n")
	}

	if m.commandErr {
		return outputStyle.BorderForeground(errBorderColor).Render(output)
	} else {
		return outputStyle.Render(output)
	}
}

func (m *model) updateFocusBorder() {
	switch m.focusMode {
	case focusSearch:
		m.search.SetFocused(true)
		m.table.SetFocused(false)
		m.detailPanel.SetFocused(false)
	case focusTable:
		m.search.SetFocused(false)
		m.table.SetFocused(true)
		m.detailPanel.SetFocused(false)
	case focusDetail:
		m.search.SetFocused(false)
		m.table.SetFocused(false)
		m.detailPanel.SetFocused(true)
	}
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	m.updateFocusBorder()

	// 2, 4, 6, 8 are used to account for border, margin and prompt width (search box only)
	outputStyle = outputStyle.Width(m.width - 2)
	m.statsView.SetWidth(m.width - 2)
	m.helpView.SetWidth(m.width - 2)

	sidePanelWidth := max(sidePanelWidthMin, m.width-ui.MaxTableWidth-4)
	tableWidth := m.width - sidePanelWidth - 4

	mainHeight := m.height - 4
	mainHeight -= lipgloss.Height(m.search.View())
	mainHeight -= lipgloss.Height(m.statsView.View())
	if !*flagHideHelp {
		mainHeight -= lipgloss.Height(m.helpView.View())
	}
	mainHeight -= lipgloss.Height(m.renderOutput())

	m.filterView.SetWidth(sidePanelWidth)
	m.search.SetWidth(m.width - sidePanelWidth - 8)
	m.table.SetDimensions(tableWidth, mainHeight)
	m.detailPanel.SetDimension(sidePanelWidth-2, mainHeight)
}
