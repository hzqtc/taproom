package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

const (
	viewportWidth    = 30
	colNameWidth     = 15
	colVersionWidth  = 15
	colTapWidth      = 15
	colDescWidthMin  = 30
	colInstallsWidth = 12
	colStatusWidth   = 25
)

// --- Styles ---

var (
	headerColor             = lipgloss.Color("#FFD580")
	highlightColor          = headerColor
	highlightForegroudColor = lipgloss.Color("#2E2E2E")
	borderColor             = lipgloss.Color("240")
	installedColor          = lipgloss.Color("#22C55E")
	uninstalledColor        = lipgloss.Color("#EF4444")
	pinnedColor             = lipgloss.Color("#B57EDC")

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	installedStyle = lipgloss.NewStyle().
			Foreground(installedColor)

	uninstalledStyle = lipgloss.NewStyle().
				Foreground(uninstalledColor)

	outdatedStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	pinnedStyle = lipgloss.NewStyle().
			Foreground(pinnedColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(headerColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Padding(1 /* top */, 1 /* horizontal */, 0 /* bottom */)

	searchStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */)

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
		m.spinner.Style = spinnerStyle
		return fmt.Sprintf("\n %s Loading package data...\n\n", m.spinner.View())
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		baseStyle.Render(m.table.View()),
		baseStyle.Render(m.viewport.View()),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		searchStyle.Render(m.search.View()),
		mainContent,
		helpStyle.Render(m.help.View(m.keys)),
	)
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	availableWidth := m.width - 6
	m.search.Width = availableWidth
	m.help.Width = availableWidth

	tableWidth := availableWidth - viewportWidth - 5

	searchHeight := lipgloss.Height(searchStyle.Render(m.search.View()))
	helpHeight := lipgloss.Height(helpStyle.Render(m.help.View(m.keys)))
	mainHeight := m.height - helpHeight - searchHeight - 4

	m.viewport.Width = viewportWidth
	m.viewport.Height = mainHeight

	m.table.SetWidth(tableWidth)
	m.table.SetHeight(mainHeight)

	// Dynamically adjust the width of the description column.
	otherColsWidth := colNameWidth + colVersionWidth + colTapWidth + colInstallsWidth + colStatusWidth
	descWidth := tableWidth - otherColsWidth - 5 // Account for 5 column separators
	if descWidth < colDescWidthMin {
		descWidth = colDescWidthMin
	}
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: colNameWidth},
		{Title: "Version", Width: colVersionWidth},
		{Title: "Tap", Width: colTapWidth},
		{Title: "Description", Width: descWidth},
		{Title: "90d Installs", Width: colInstallsWidth},
		{Title: "Status", Width: colStatusWidth},
	})

	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		Foreground(headerColor).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(highlightForegroudColor).
		Background(highlightColor).
		Bold(true)
	m.table.SetStyles(tableStyle)
}

// updateTable populates the table with the current viewPackages.
func (m *model) updateTable() {
	rows := make([]table.Row, len(m.viewPackages))
	for i, pkg := range m.viewPackages {
		version := pkg.Version
		if pkg.IsPinned {
			version = fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
		} else if pkg.IsOutdated {
			version = fmt.Sprintf("%s (New)", pkg.Version)
		}
		rows[i] = table.Row{
			pkg.Name,
			version,
			pkg.Tap,
			pkg.Desc,
			fmt.Sprintf("%d", pkg.InstallCount90d),
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

	version := pkg.Version
	if pkg.IsOutdated {
		version = fmt.Sprintf("%s -> %s", pkg.InstalledVersion, pkg.Version)
	} else if pkg.IsPinned {
		version = fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
	}

	status := pkg.Status
	statusSymbol := ""
	if pkg.IsPinned {
		statusSymbol = pinnedStyle.Render("")
	} else if pkg.IsOutdated {
		statusSymbol = outdatedStyle.Render("")
	} else if pkg.IsInstalled {
		statusSymbol = installedStyle.Render("✓")
	} else {
		statusSymbol = uninstalledStyle.Render("✗")
	}
	status = fmt.Sprintf("%s %s\n", statusSymbol, status)

	var b strings.Builder
	b.WriteString(headerStyle.Render(pkg.Name))
	b.WriteString(fmt.Sprintf("\n%s\n\n", pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", version))
	b.WriteString(fmt.Sprintf("Tap: %s\n", pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n\n", pkg.License))
	b.WriteString(fmt.Sprintf("Status: %s\n", status))
	b.WriteString(fmt.Sprintf("90-Day Installs: %d\n\n", pkg.InstallCount90d))

	b.WriteString("Dependencies:\n")
	if len(pkg.Dependencies) > 0 {
		for _, dep := range pkg.Dependencies {
			depPkg := m.getPackage(dep)
			if depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	} else {
		b.WriteString("  None\n")
	}

	if len(pkg.BuildDependencies) > 0 {
		b.WriteString("\nBuild Dependencies:\n")
		for _, dep := range pkg.BuildDependencies {
			depPkg := m.getPackage(dep)
			if depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	}

	if pkg.IsInstalled && len(pkg.Dependents) > 0 {
		b.WriteString("\nRequired By:\n")
		for _, dep := range pkg.Dependents {
			b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
		}
	}

	vpStyle := lipgloss.NewStyle().Width(viewportWidth)
	m.viewport.SetContent(vpStyle.Render(b.String()))
	m.viewport.GotoTop() // Reset scroll position
}
