package main

import (
	"fmt"
	"slices"
	"strings"
	"taproom/internal/data"
	"taproom/internal/util"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
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
	sidePanelWidthMin    = 30
	tableAdditionalWidth = 30
	colSpacing           = 2
	outputMaxLines       = 10
)

const (
	disabledSymbol    = ""
	deprecatedSymbol  = ""
	uninstalledSymbol = "✗"
	installedSymbol   = "✓"
	outdatedSymbol    = ""
	pinnedSymbol      = ""
)

var (
	flagHideHelp = pflag.Bool("hide-help", false, "Hide the help text")
)

// --- Styles ---

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

	headerStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)

	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	searchStyle = baseStyle.
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */)

	tableStyle = baseStyle

	detailPanelStyle = baseStyle.
				Padding(0, 1)

	// The content style for details panel, width-2 to account for padding
	detailsContentStyle = lipgloss.NewStyle().
				Width(sidePanelWidthMin - 2)

	filterModeStyle = baseStyle.
			Width(sidePanelWidthMin).
			Padding(0, 1).
			Margin(1, 0)

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
		tableStyle.Render(m.table.View()),
		detailPanelStyle.Render(m.detailPanel.View()),
	)

	topContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		searchStyle.Render(m.search.View()),
		filterModeStyle.Render(m.filters.String()),
	)

	views := []string{
		topContent,
		mainContent,
		m.renderStats(),
	}
	if output := m.renderOutput(); output != "" {
		views = append(views, output)
	}
	if !*flagHideHelp {
		views = append(views, m.renderHelp())
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

func renderKey(k key.Binding) string {
	return keyStyle.Render(k.Keys()[0])
}

func (m *model) renderHelp() string {
	var b strings.Builder

	b.WriteString("General   : ")
	b.WriteString(renderKey(m.keys.Quit))
	b.WriteString(": quit ")
	b.WriteString(renderKey(m.keys.Refresh))
	b.WriteString(": refresh ")
	b.WriteString(renderKey(m.keys.SwitchFocus))
	b.WriteString(": switch focus ")
	b.WriteString(renderKey(m.keys.FocusSearch))
	b.WriteString(": search ")
	b.WriteString(renderKey(m.keys.Esc))
	b.WriteString(": clear search ")
	b.WriteString(renderKey(m.keys.Enter))
	b.WriteString(": exit search ")
	b.WriteString(renderKey(m.keys.SortByNext) + "/" + renderKey(m.keys.SortByPrev))
	b.WriteString(": sorting")
	b.WriteString("\n")
	b.WriteString("Navigation: ")
	b.WriteString(keyStyle.Render("j") + "/" + keyStyle.Render("↓"))
	b.WriteString(": cursor down ")
	b.WriteString(keyStyle.Render("k") + "/" + keyStyle.Render("↑"))
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
	b.WriteString("Filter    : ")
	b.WriteString(renderKey(m.keys.FilterAll))
	b.WriteString(": all (no filter) ")
	b.WriteString(renderKey(m.keys.FilterFormulae))
	b.WriteString(": formulae ")
	b.WriteString(renderKey(m.keys.FilterCasks))
	b.WriteString(": casks ")
	b.WriteString(renderKey(m.keys.FilterInstalled))
	b.WriteString(": installed ")
	b.WriteString(renderKey(m.keys.FilterOutdated))
	b.WriteString(": outdated ")
	b.WriteString(renderKey(m.keys.FilterExplicit))
	b.WriteString(": explicitly installed ")
	b.WriteString(renderKey(m.keys.FilterActive))
	b.WriteString(": active")
	b.WriteString("\n")
	b.WriteString("Commands  : ")
	b.WriteString(renderKey(m.keys.OpenHomePage))
	b.WriteString(": home page ")
	b.WriteString(renderKey(m.keys.OpenBrewUrl))
	b.WriteString(": brew.sh ")
	b.WriteString(renderKey(m.keys.OpenRelease))
	b.WriteString(": release page ")
	b.WriteString(renderKey(m.keys.UpgradeAll))
	b.WriteString(": upgrade all ")
	b.WriteString(renderKey(m.keys.Upgrade))
	b.WriteString(": upgrade ")
	b.WriteString(renderKey(m.keys.Install))
	b.WriteString(": install ")
	b.WriteString(renderKey(m.keys.Remove))
	b.WriteString(": uninstall ")
	b.WriteString(renderKey(m.keys.Pin))
	b.WriteString(": pin ")
	b.WriteString(renderKey(m.keys.Unpin))
	b.WriteString(": unpin ")
	b.WriteString(renderKey(m.keys.CleanUp))
	b.WriteString(": cleanup")

	return helpStyle.Render(b.String())
}

func getTableStyles() table.Styles {
	tableStyles := table.DefaultStyles()
	tableStyles.Header = tableStyles.Header.
		Foreground(highlightColor).
		BorderStyle(roundedBorder).
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
		searchStyle = searchStyle.BorderForeground(focusedBorderColor)
		tableStyle = tableStyle.BorderForeground(borderColor)
		detailPanelStyle = detailPanelStyle.BorderForeground(borderColor)
	case focusTable:
		searchStyle = searchStyle.BorderForeground(borderColor)
		tableStyle = tableStyle.BorderForeground(focusedBorderColor)
		detailPanelStyle = detailPanelStyle.BorderForeground(borderColor)
	case focusDetail:
		searchStyle = searchStyle.BorderForeground(borderColor)
		tableStyle = tableStyle.BorderForeground(borderColor)
		detailPanelStyle = detailPanelStyle.BorderForeground(focusedBorderColor)
	}
}

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

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	m.updateFocusBorder()

	// 2, 4, 6, 8 are used to account for border, margin and prompt width (search box only)
	outputStyle = outputStyle.Width(m.width - 2)
	helpStyle = helpStyle.Width(m.width - 2)

	sidePanelWidth := max(sidePanelWidthMin, m.width-tableWidthMax-4)
	m.search.Width = m.width - sidePanelWidth - 8
	filterModeStyle = filterModeStyle.
		BorderStyle(getRoundedBorderWithTitle("Filters", sidePanelWidth)).
		Width(sidePanelWidth)
	detailPanelStyle = detailPanelStyle.
		BorderStyle(getRoundedBorderWithTitle("Details", sidePanelWidth))
	m.detailPanel.Width = sidePanelWidth - 2
	detailsContentStyle = detailsContentStyle.Width(sidePanelWidth - 2)

	tableWidth := m.width - sidePanelWidth - 4
	m.table.SetWidth(tableWidth)

	mainHeight := m.height - 4
	mainHeight -= lipgloss.Height(searchStyle.Render(m.search.View()))
	mainHeight -= lipgloss.Height(m.renderStats())
	if !*flagHideHelp {
		mainHeight -= lipgloss.Height(m.renderHelp())
	}
	mainHeight -= lipgloss.Height(m.renderOutput())

	m.table.SetHeight(mainHeight)
	m.detailPanel.Height = mainHeight

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

	for _, col := range m.columns {
		colWidth := colWidthMap[col]
		if tableWidth > visibleColsWidth+colWidth+colSpacing {
			visibleCols = append(visibleCols, col)
			visibleColsWidth += colWidth + colSpacing
		}
	}

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

func getFormattedStatus(pkg *data.Package) string {
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

func getColData(c columnName, pkg *data.Package) string {
	switch c {
	case colSymbol:
		return pkg.Symbol()
	case colName:
		return pkg.Name
	case colVersion:
		return pkg.ShortVersion()
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

	m.updateDetailsPanel()
}

// updateDetailsPanel sets the content of the details panel based on the selected package.
func (m *model) updateDetailsPanel() {
	if len(m.viewPackages) == 0 {
		m.detailPanel.SetContent("No packages match the current filter.")
		return
	}

	pkg := m.getSelectedPackage()
	if pkg == nil {
		m.detailPanel.SetContent("No packages selected.")
		return
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s %s", pkg.Symbol(), pkg.Name)))
	b.WriteString(fmt.Sprintf("\n%s\n\n", pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", pkg.LongVersion()))
	b.WriteString(fmt.Sprintf("Tap: %s\n", pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n", pkg.License))
	if m.isColumnEnabled(colInstalls) {
		b.WriteString(fmt.Sprintf("Installs (90d): %d\n", pkg.InstallCount90d))
	}

	b.WriteString(fmt.Sprintf("\nStatus: %s\n", getFormattedStatus(pkg)))
	if pkg.IsInstalled {
		if m.isColumnEnabled(colSize) {
			b.WriteString(fmt.Sprintf("Size: %s\n", pkg.FormattedSize))
		}
		b.WriteString(fmt.Sprintf("Installed on: %s\n", pkg.InstalledDate))
		if release := pkg.ReleaseInfo; release != nil {
			b.WriteString(fmt.Sprintf("Released on: %s\n", release.PublishDate.Format(time.DateOnly)))
		}
	}

	if len(pkg.Conflicts) > 0 {
		b.WriteString("\nConflicts:\n")
		for _, c := range pkg.Conflicts {
			if conflictPkg := m.getPackage(c); conflictPkg != nil && conflictPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), c))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), c))
			}
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
				recursiveMissingDeps := util.SortAndUniq(m.getRecursiveMissingDeps(dep))
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

	m.detailPanel.SetContent(detailsContentStyle.Render(b.String()))
	m.detailPanel.GotoTop()
}

func (m *model) renderStats() string {
	var formulaeNum, casksNum int
	var installedFormulaeNum, installedFormulaeDepNum, installedCasksNum int
	var formulaeSize, casksSize int64
	for _, pkg := range m.viewPackages {
		if pkg.IsCask {
			casksNum++
		} else {
			formulaeNum++
		}

		if !pkg.IsInstalled {
			continue
		}

		if pkg.IsCask {
			installedCasksNum++
			casksSize += pkg.Size
		} else {
			installedFormulaeNum++
			formulaeSize += pkg.Size
			if pkg.InstalledAsDependency {
				installedFormulaeDepNum++
			}
		}
	}
	return helpStyle.Render(
		fmt.Sprintf(
			"%s Formulae available | %s Casks available | %s Formulae (incl. %s deps) installed taking %s | %s Casks installed taking %s",
			keyStyle.Render(fmt.Sprintf("%d", formulaeNum)),
			keyStyle.Render(fmt.Sprintf("%d", casksNum)),
			keyStyle.Render(fmt.Sprintf("%d", installedFormulaeNum)),
			keyStyle.Render(fmt.Sprintf("%d", installedFormulaeDepNum)),
			keyStyle.Render(util.FormatSize(formulaeSize)),
			keyStyle.Render(fmt.Sprintf("%d", installedCasksNum)),
			keyStyle.Render(util.FormatSize(casksSize)),
		))
}
