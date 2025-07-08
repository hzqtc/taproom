package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

const (
	// TODO: make view port wider on large screens
	viewportWidth = 30

	colSymbolWidth   = 2
	colNameWidth     = 15
	colVersionWidth  = 15
	colTapWidth      = 15
	colDescWidth     = 30
	colInstallsWidth = 8
	colStatusWidth   = 15
	colSpacing       = 2

	outputMaxLines = 10
)

const (
	formulaSymbol = ""
	caskSymbol    = ""

	disabledSymbol    = ""
	deprecatedSymbol  = ""
	uninstalledSymbol = ""
	installedSymbol   = "✓"
	outdatedSymbol    = ""
	pinnedSymbol      = "✗"
)

// --- Styles ---

var (
	highlightColor          = lipgloss.Color("#FFD580")
	highlightForegroudColor = lipgloss.Color("#2E2E2E")
	borderColor             = lipgloss.Color("240")
	focusedBorderColor      = highlightColor
	installedColor          = lipgloss.Color("#22C55E")
	deprecatedColor         = lipgloss.Color("#EF4444")
	uninstalledColor        = lipgloss.Color("#FBBF24")
	pinnedColor             = lipgloss.Color("#B57EDC")

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
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

	headerStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)

	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	searchStyle = baseStyle.Copy().
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */)

	tableStyle = baseStyle.Copy()

	viewportStyle = baseStyle.Copy().
			Padding(0, 1)

	// The content style for viewport, width-2 to account for padding
	vpContentStyle = lipgloss.NewStyle().
			Width(viewportWidth - 2)

	viewModeStyle = baseStyle.Copy().
			Width(viewportWidth).
			Padding(0, 1).
			Margin(1, 0)

	outputStyle = baseStyle.Copy()

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
		tableStyle.Render(m.table.View()),
		viewportStyle.Render(m.viewport.View()),
	)

	topContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		searchStyle.Render(m.search.View()),
		viewModeStyle.Render(fmt.Sprintf("Viewing: %s", m.viewMode.String())),
	)

	views := []string{
		topContent,
		mainContent,
	}
	if output := m.renderOutput(); output != "" {
		views = append(views, output)
	}
	views = append(views, m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func (m *model) renderOutput() string {
	if !m.isExecuting {
		return ""
	}

	var output string
	if len(m.output) > outputMaxLines {
		output = strings.Join(m.output[len(m.output)-outputMaxLines:], "\n")
	} else {
		output = strings.Join(m.output, "\n")
	}
	return outputStyle.Render(output)
}

func (m *model) renderHelp() string {
	var b strings.Builder

	// TODO: update help based on current focused component
	b.WriteString("General      : ")
	b.WriteString(keyStyle.Render("q"))
	b.WriteString(": quit ")
	b.WriteString(keyStyle.Render("r"))
	b.WriteString(": refresh ")
	b.WriteString(keyStyle.Render("Tab"))
	b.WriteString(": switch focus ")
	b.WriteString(keyStyle.Render("/"))
	b.WriteString(": search ")
	b.WriteString(keyStyle.Render("Esc"))
	b.WriteString(": clear search ")
	b.WriteString(keyStyle.Render("Enter"))
	b.WriteString(": exit search")
	b.WriteString("\n")
	b.WriteString("Navigation   : ")
	b.WriteString(keyStyle.Render("j/↓"))
	b.WriteString(": cursor down ")
	b.WriteString(keyStyle.Render("k/↑"))
	b.WriteString(": cursor up ")
	b.WriteString(keyStyle.Render("PageUp"))
	b.WriteString(": prev page ")
	b.WriteString(keyStyle.Render("PageDown"))
	b.WriteString(": next page ")
	b.WriteString(keyStyle.Render("g"))
	b.WriteString(": go to top ")
	b.WriteString(keyStyle.Render("G"))
	b.WriteString(": go to bottom")
	b.WriteString("\n")
	b.WriteString("Filter & Sort: ")
	b.WriteString(keyStyle.Render("s"))
	b.WriteString(": sort by name/popularity ")
	b.WriteString(keyStyle.Render("a"))
	b.WriteString(": all ")
	b.WriteString(keyStyle.Render("f"))
	b.WriteString(": formulae ")
	b.WriteString(keyStyle.Render("c"))
	b.WriteString(": casks ")
	b.WriteString(keyStyle.Render("i"))
	b.WriteString(": installed ")
	b.WriteString(keyStyle.Render("o"))
	b.WriteString(": outdated ")
	b.WriteString(keyStyle.Render("e"))
	b.WriteString(": explicitly installed ")
	b.WriteString(keyStyle.Render("d"))
	b.WriteString(": hide disabled/deprecated")
	b.WriteString("\n")
	b.WriteString("Commands     : ")
	b.WriteString(keyStyle.Render("b"))
	b.WriteString(": open home page ")
	b.WriteString(keyStyle.Render("U"))
	b.WriteString(": upgrade all ")
	b.WriteString(keyStyle.Render("u"))
	b.WriteString(": upgrade selected ")
	b.WriteString(keyStyle.Render("t"))
	b.WriteString(": install selected ")
	b.WriteString(keyStyle.Render("x"))
	b.WriteString(": uninstall selected ")
	b.WriteString(keyStyle.Render("p"))
	b.WriteString(": pin selected ")
	b.WriteString(keyStyle.Render("P"))
	b.WriteString(": unpin selected")

	return helpStyle.Render(b.String())
}

func getTableStyles() table.Styles {
	tableStyles := table.DefaultStyles()
	tableStyles.Header = tableStyles.Header.
		Foreground(highlightColor).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true)
	tableStyles.Selected = tableStyles.Selected.
		Foreground(highlightForegroudColor).
		Background(highlightColor).
		Bold(true)
	return tableStyles
}

func (m *model) updateFocusBorder() {
	switch m.focusMode {
	case focusSearch:
		searchStyle = searchStyle.Copy().BorderForeground(focusedBorderColor)
		tableStyle = tableStyle.Copy().BorderForeground(borderColor)
		viewportStyle = viewportStyle.Copy().BorderForeground(borderColor)
	case focusTable:
		searchStyle = searchStyle.Copy().BorderForeground(borderColor)
		tableStyle = tableStyle.Copy().BorderForeground(focusedBorderColor)
		viewportStyle = viewportStyle.Copy().BorderForeground(borderColor)
	case focusDetail:
		searchStyle = searchStyle.Copy().BorderForeground(borderColor)
		tableStyle = tableStyle.Copy().BorderForeground(borderColor)
		viewportStyle = viewportStyle.Copy().BorderForeground(focusedBorderColor)
	}
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	m.updateFocusBorder()

	// 2, 4, 6, 8 are used to account for border, margin and prompt width (search box only)
	outputStyle = outputStyle.Copy().Width(m.width - 2)
	helpStyle = helpStyle.Copy().Width(m.width - 2)

	m.search.Width = m.width - viewportWidth - 8

	tableWidth := m.width - viewportWidth - 4
	m.table.SetWidth(tableWidth)

	m.viewport.Width = viewportWidth - 2

	searchHeight := lipgloss.Height(searchStyle.Render(m.search.View()))
	helpHeight := lipgloss.Height(m.renderHelp())
	outputHeight := lipgloss.Height(m.renderOutput())
	mainHeight := m.height - helpHeight - searchHeight - outputHeight - 4

	m.table.SetHeight(mainHeight)
	m.viewport.Height = mainHeight

	cols, remainingWidth := getVisibleCols(tableWidth)
	m.visibleColumns = cols
	columns := getTableCols(cols, remainingWidth)

	if len(m.table.Columns()) != len(columns) {
		// Clear data when number of columns changes, this needs to be before SetColumns()
		m.table.SetRows([]table.Row{})
	}
	m.table.SetColumns(columns)
}

// Dynamically determine visible columns based on table width
// Returns the visible columns and unused width
func getVisibleCols(tableWidth int) ([]columnName, int) {
	// symbol and name column is always visible
	visibleCols := []columnName{colSymbol, colName}
	colsWidth := colSymbolWidth + colSpacing + colNameWidth + colSpacing

	// Add other columns: prefer the ones take less space, then by importance
	if tableWidth > colsWidth+colInstallsWidth+colSpacing {
		visibleCols = append(visibleCols, colInstalls)
		colsWidth += colInstallsWidth + colSpacing
	}
	if tableWidth > colsWidth+colStatusWidth+colSpacing {
		visibleCols = append(visibleCols, colStatus)
		colsWidth += colStatusWidth + colSpacing
	}
	if tableWidth > colsWidth+colVersionWidth+colSpacing {
		visibleCols = append(visibleCols, colVersion)
		colsWidth += colVersionWidth + colSpacing
	}
	if tableWidth > colsWidth+colTapWidth+colSpacing {
		visibleCols = append(visibleCols, colTap)
		colsWidth += colTapWidth + colSpacing
	}
	if tableWidth > colsWidth+colDescWidth+colSpacing {
		visibleCols = append(visibleCols, colDescription)
		colsWidth += colDescWidth + colSpacing
	}
	// sort visible columns by their order in the iota
	sort.Slice(visibleCols, func(i, j int) bool {
		return visibleCols[i] < visibleCols[j]
	})
	return visibleCols, tableWidth - colsWidth
}

// Build the columns for the table view
func getTableCols(cols []columnName, remainingWidth int) []table.Column {
	columns := []table.Column{}
	for _, col := range cols {
		switch col {
		case colSymbol:
			columns = append(columns, table.Column{Title: " ", Width: colSymbolWidth})
		case colName:
			if slices.Contains(cols, colDescription) {
				columns = append(columns, table.Column{Title: "Name", Width: colNameWidth})
			} else {
				// If desc column is not visible, the name column takes all remaining width
				columns = append(columns, table.Column{Title: "Name", Width: colNameWidth + remainingWidth})
				remainingWidth = 0
			}
		case colVersion:
			columns = append(columns, table.Column{Title: "Version", Width: colVersionWidth})
		case colTap:
			columns = append(columns, table.Column{Title: "Tap", Width: colTapWidth})
		case colDescription:
			// If desc column is visible, it takes all remaining width
			columns = append(columns, table.Column{Title: "Description", Width: colDescWidth + remainingWidth})
			remainingWidth = 0
		case colInstalls:
			columns = append(columns, table.Column{Title: "Installs", Width: colInstallsWidth})
		case colStatus:
			columns = append(columns, table.Column{Title: "Status", Width: colStatusWidth})
		}
	}
	return columns
}

func getSimpleVersion(pkg *Package) string {
	if pkg.IsOutdated {
		return fmt.Sprintf("%s (New)", pkg.Version)
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
	} else {
		return pkg.Version
	}
}

func getFormattedVersion(pkg *Package) string {
	if pkg.IsOutdated {
		return fmt.Sprintf("%s -> %s", pkg.InstalledVersion, pkg.Version)
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pinned)", pkg.InstalledVersion)
	} else {
		return pkg.Version
	}
}

func getSimpleStatus(pkg *Package) string {
	if pkg.IsDisabled {
		return "Disabled"
	} else if pkg.IsDeprecated {
		return "Deprecated"
	} else if pkg.IsPinned {
		return "Pinned"
	} else if pkg.IsOutdated {
		return "Outdated"
	} else if pkg.InstalledAsDependency {
		return "Installed (Dep)"
	} else if pkg.IsInstalled {
		return "Installed"
	} else {
		return "Uninstalled"
	}
}

func getFormattedStatus(pkg *Package) string {
	var statusSymbol string
	if pkg.IsDisabled {
		statusSymbol = deprecatedStyle.Render(disabledSymbol)
	} else if pkg.IsDeprecated {
		statusSymbol = deprecatedStyle.Render(deprecatedSymbol)
	} else if pkg.IsPinned {
		statusSymbol = pinnedStyle.Render(pinnedSymbol)
	} else if pkg.IsOutdated {
		statusSymbol = outdatedStyle.Render(outdatedSymbol)
	} else if pkg.IsInstalled {
		statusSymbol = installedStyle.Render(installedSymbol)
	} else {
		statusSymbol = uninstalledStyle.Render(uninstalledSymbol)
	}
	return fmt.Sprintf("%s %s\n", statusSymbol, getSimpleStatus(pkg))
}

// updateTable populates the table with the current viewPackages.
func (m *model) updateTable() {
	rows := make([]table.Row, len(m.viewPackages))
	for i, pkg := range m.viewPackages {
		rowData := []string{}
		for _, col := range m.visibleColumns {
			switch col {
			case colSymbol:
				if pkg.IsCask {
					rowData = append(rowData, caskSymbol)
				} else {
					rowData = append(rowData, formulaSymbol)
				}
			case colName:
				rowData = append(rowData, pkg.Name)
			case colVersion:
				rowData = append(rowData, getSimpleVersion(pkg))
			case colTap:
				rowData = append(rowData, pkg.Tap)
			case colDescription:
				rowData = append(rowData, pkg.Desc)
			case colInstalls:
				rowData = append(rowData, fmt.Sprintf("%*d", colInstallsWidth, pkg.InstallCount90d))
			case colStatus:
				rowData = append(rowData, getSimpleStatus(pkg))
			}
		}
		rows[i] = table.Row(rowData)
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
		m.viewport.SetContent("No packages selected.")
		return
	}

	pkg := m.viewPackages[selectedIndex]
	var pkgSymbol string
	if pkg.IsCask {
		pkgSymbol = caskSymbol
	} else {
		pkgSymbol = formulaSymbol
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s %s", pkgSymbol, pkg.Name)))
	b.WriteString(fmt.Sprintf("\n%s\n\n", pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", getFormattedVersion(pkg)))
	b.WriteString(fmt.Sprintf("Tap: %s\n", pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n\n", pkg.License))
	b.WriteString(fmt.Sprintf("Status: %s\n", getFormattedStatus(pkg)))
	b.WriteString(fmt.Sprintf("90-Day Installs: %d\n\n", pkg.InstallCount90d))

	b.WriteString("Dependencies:\n")
	if len(pkg.Dependencies) > 0 {
		for _, dep := range pkg.Dependencies {
			if depPkg := m.getPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
				// For uninstall dependencies, show all recursive uninstalled dependencies
				recursiveMissingDeps := sortAndUniq(m.getRecursiveMissingDeps(dep))
				for _, d := range recursiveMissingDeps {
					if p := m.getPackage(d); !p.IsInstalled {
						b.WriteString(fmt.Sprintf("    %s %s\n", uninstalledStyle.Render("✗"), d))
					}
				}
			}
		}
	} else {
		b.WriteString("  None\n")
	}

	if len(pkg.BuildDependencies) > 0 {
		b.WriteString("\nBuild Dependencies:\n")
		for _, dep := range pkg.BuildDependencies {
			if depPkg := m.getPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	}

	if len(pkg.Dependents) > 0 {
		b.WriteString("\nRequired By:\n")
		for _, dep := range pkg.Dependents {
			if depPkg := m.getPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	}

	m.viewport.SetContent(vpContentStyle.Render(b.String()))
	m.viewport.GotoTop()
}

