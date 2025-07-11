package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var colWidthMap = map[columnName]int{
	colSymbol:      2,
	colName:        15,
	colVersion:     15,
	colTap:         15,
	colDescription: 30,
	colInstalls:    10,
	colSize:        8,
	colStatus:      15,
}

var tableWidthMax = func() int {
	widthMax := 0
	for _, colWidth := range colWidthMap {
		widthMax += colWidth + colSpacing
	}
	return widthMax + tableAdditionalWidth // Allow table to expand up to the additional width
}()

const (
	viewportWidthMin     = 30
	tableAdditionalWidth = 30
	colSpacing           = 2
	outputMaxLines       = 10
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
			Width(viewportWidthMin - 2)

	viewModeStyle = baseStyle.Copy().
			Width(viewportWidthMin).
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
		return fmt.Sprintf("%s\n\n%s Loading...", m.loadingMsg, m.spinner.View())
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
	b.WriteString(keyStyle.Render("s/S"))
	b.WriteString(": switch sort column ")
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
	b.WriteString(keyStyle.Render("h"))
	b.WriteString(": open home page ")
	b.WriteString(keyStyle.Render("b"))
	b.WriteString(": open brew.sh ")
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

	viewportWidth := max(viewportWidthMin, m.width-tableWidthMax-4)
	m.search.Width = m.width - viewportWidth - 8
	viewModeStyle = viewModeStyle.Copy().Width(viewportWidth)
	m.viewport.Width = viewportWidth - 2
	vpContentStyle = vpContentStyle.Copy().Width(viewportWidth - 2)

	tableWidth := m.width - viewportWidth - 4
	m.table.SetWidth(tableWidth)

	searchHeight := lipgloss.Height(searchStyle.Render(m.search.View()))
	helpHeight := lipgloss.Height(m.renderHelp())
	outputHeight := lipgloss.Height(m.renderOutput())
	mainHeight := m.height - helpHeight - searchHeight - outputHeight - 4

	m.table.SetHeight(mainHeight)
	m.viewport.Height = mainHeight

	cols, remainingWidth := m.getVisibleCols(tableWidth)
	m.visibleColumns = cols
	columns := m.getTableCols(remainingWidth)

	if len(m.table.Columns()) != len(columns) {
		// Clear data when number of columns changes, this needs to be before SetColumns()
		m.table.SetRows([]table.Row{})
	}
	m.table.SetColumns(columns)
}

// Dynamically determine visible columns based on table width
// Returns the visible columns and unused width
func (m *model) getVisibleCols(tableWidth int) ([]columnName, int) {
	visibleCols := []columnName{}
	visibleColsWidth := 0

	// Add columns in order of importance
	for _, col := range []columnName{colSymbol, colName, colStatus, colVersion, colTap, colDescription, colInstalls, colSize} {
		colWidth := colWidthMap[col]
		if tableWidth > visibleColsWidth+colWidth+colSpacing {
			visibleCols = append(visibleCols, col)
			visibleColsWidth += colWidth + colSpacing
		}
	}

	// sort visible columns by their order in the iota
	sort.Slice(visibleCols, func(i, j int) bool {
		return visibleCols[i] < visibleCols[j]
	})
	return visibleCols, tableWidth - visibleColsWidth
}

// Build the columns for the table view
func (m *model) getTableCols(remainingWidth int) []table.Column {
	columns := []table.Column{}
	for _, col := range m.visibleColumns {
		colTitle := col.String()
		colWidth := colWidthMap[col]
		// Add sort indicator
		if col == m.sortColumn {
			if col.ReverseSort() {
				colTitle = fmt.Sprintf("↓ %s", colTitle)
			} else {
				colTitle = fmt.Sprintf("↑ %s", colTitle)
			}
		}
		// Right align columns
		if col.RightAligned() {
			colTitle = fmt.Sprintf("%*s", colWidth, colTitle)
		}
		// Adjust column width to use remainingWidth
		// If desc column is not visible, the name column takes all remaining width
		if col == colName {
			if !slices.Contains(m.visibleColumns, colDescription) {
				colWidth += remainingWidth
				remainingWidth = 0
			}
		}
		// If desc column is visible, it takes all remaining width
		if col == colDescription {
			colWidth += remainingWidth
			remainingWidth = 0
		}
		columns = append(columns, table.Column{Title: colTitle, Width: colWidth})
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
	return fmt.Sprintf("%s %s", statusSymbol, pkg.Status())
}

func getColData(c columnName, pkg *Package) string {
	switch c {
	case colSymbol:
		if pkg.IsCask {
			return caskSymbol
		} else {
			return formulaSymbol
		}
	case colName:
		return pkg.Name
	case colVersion:
		return getSimpleVersion(pkg)
	case colTap:
		return pkg.Tap
	case colDescription:
		return pkg.Desc
	case colInstalls:
		return fmt.Sprintf("%d", pkg.InstallCount90d)
	case colSize:
		if pkg.IsInstalled {
			return pkg.FormattedSize
		} else {
			return "N/A"
		}
	case colStatus:
		return pkg.Status()
	default:
		return ""
	}
}

// updateTable populates the table with the current viewPackages.
func (m *model) updateTable() {
	rows := make([]table.Row, len(m.viewPackages))
	for i, pkg := range m.viewPackages {
		rowData := []string{}
		for _, col := range m.visibleColumns {
			colData := getColData(col, pkg)
			if col.RightAligned() {
				colData = fmt.Sprintf("%*s", colWidthMap[col], colData)
			}
			rowData = append(rowData, colData)
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
	b.WriteString(fmt.Sprintf("License: %s\n", pkg.License))
	b.WriteString(fmt.Sprintf("Installs (90d): %d\n", pkg.InstallCount90d))
	if pkg.IsInstalled {
		b.WriteString(fmt.Sprintf("Size: %s\n", pkg.FormattedSize))
	}

	b.WriteString(fmt.Sprintf("\nStatus: %s\n", getFormattedStatus(pkg)))
	if pkg.IsInstalled {
		b.WriteString(fmt.Sprintf("Installed on: %s\n", pkg.InstalledDate))
	}

	if len(pkg.Conflicts) > 0 {
		b.WriteString("\nConflicts:\n")
		for _, c := range pkg.Conflicts {
			b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), c))
		}
	}

	if len(pkg.Dependencies) > 0 {
		b.WriteString("\nDependencies:\n")
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
