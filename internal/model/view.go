package model

import (
	"taproom/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

const (
	sidePanelWidthMin = 30
)

var (
	flagHideHelp = pflag.Bool("hide-help", false, "Hide the help text")
)

func (m model) View() string {
	if loading := m.loadingView.View(); loading != "" {
		return loading
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
	if output := m.outputView.View(); output != "" {
		views = append(views, output)
	}
	if !*flagHideHelp {
		views = append(views, m.helpView.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, views...)
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
	m.outputView.SetWidth(m.width - 2)
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
	if output := m.outputView.View(); output != "" {
		mainHeight -= lipgloss.Height(output)
	}

	m.filterView.SetWidth(sidePanelWidth)
	m.search.SetWidth(m.width - sidePanelWidth - 8)
	m.table.SetDimensions(tableWidth, mainHeight)
	m.detailPanel.SetDimension(sidePanelWidth-2, mainHeight)
}
