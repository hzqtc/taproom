package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---

var (
	// General
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Table
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))
	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	// Spinner
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Table Styles
	tableStyles = func() table.Styles {
		s := table.DefaultStyles()
		s.Header = headerStyle
		s.Selected = selectedRowStyle
		return s
	}()
)

// --- View & Layout ---

// View is the main render function for the application.
func (m model) View() string {
	if m.errorMsg != "" {
		return fmt.Sprintf("An error occurred: %s\nPress 'q' to quit.", m.errorMsg)
	}

	if m.isLoading {
		return fmt.Sprintf("\n %s Loading package data...\n\n", m.spinner.View())
	}

	helpView := m.help.View(m.keys)

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		baseStyle.Render(m.table.View()),
		baseStyle.Render(m.viewport.View()),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		m.search.View(),
		mainContent,
		helpStyle.Render(helpView),
	)
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	helpHeight := lipgloss.Height(m.help.View(m.keys))
	searchHeight := lipgloss.Height(m.search.View())
	mainHeight := m.height - helpHeight - searchHeight - 2 // -2 for margins/borders

	tableWidth := int(float64(m.width) * 0.6)
	viewportWidth := m.width - tableWidth - 4 // -4 for borders

	m.table.SetHeight(mainHeight)
	m.table.SetWidth(tableWidth)

	// Dynamically adjust the width of the description column.
	descWidth := tableWidth - 15 - 15 - 20 - 20 - 4 // name, tap, version, status, borders
	if descWidth < 10 {
		descWidth = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: 20},
		{Title: "Tap", Width: 15},
		{Title: "Description", Width: descWidth},
		{Title: "Version", Width: 15},
		{Title: "Status", Width: 20},
	})

	m.viewport.Width = viewportWidth
	m.viewport.Height = mainHeight
}

// updateTable populates the table with the current viewPackages.
func (m *model) updateTable() {
	rows := make([]table.Row, len(m.viewPackages))
	for i, pkg := range m.viewPackages {
		rows[i] = table.Row{
			pkg.Name,
			pkg.Tap,
			pkg.Desc,
			pkg.Version,
			pkg.Status,
		}
	}
	m.table.SetRows(rows)

	// Reset cursor if it's out of bounds
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(0)
	}

	m.updateViewport()
}

// updateViewport sets the content of the details panel based on the selected package.
func (m *model) updateViewport() {
	if len(m.viewPackages) == 0 {
		m.viewport.SetContent("No packages match the current filter.")
		return
	}

	// Ensure selected index is valid
	selectedIndex := m.table.Cursor()
	if selectedIndex < 0 || selectedIndex >= len(m.viewPackages) {
		selectedIndex = 0
	}

	pkg := m.viewPackages[selectedIndex]

	var b strings.Builder
	b.WriteString(headerStyle.Render(pkg.Name))
	b.WriteString(fmt.Sprintf("\n%s\n\n", pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", pkg.Version))
	b.WriteString(fmt.Sprintf("Tap: %s\n", pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n\n", pkg.License))
	b.WriteString(fmt.Sprintf("Status: %s\n", pkg.Status))
	b.WriteString(fmt.Sprintf("90-Day Installs: %d\n\n", pkg.InstallCount90d))

	if len(pkg.Dependencies) > 0 {
		b.WriteString("Dependencies:\n")
		for _, dep := range pkg.Dependencies {
			b.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}

	m.viewport.SetContent(b.String())
	m.viewport.GotoTop() // Reset scroll position
}
