package ui

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"taproom/internal/data"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

var (
	flagHideCols = pflag.StringSlice(
		"hide-columns",
		[]string{},
		"Hide specific columns seprated by comma (no spaces): Version, Tap, Description, Installs, Size, Status",
	)
	flagSortColumn = pflag.StringP(
		"sort-column",
		"s",
		"Name",
		"Choose which column (Name, Tap, Installs, Size, Status) to sort by initially",
	)
)

const (
	tableAdditionalWidth = 30
	colSpacing           = 2
)

var MaxTableWidth = func() int {
	maxWidth := 0
	for _, colWidth := range colWidthMap {
		maxWidth += colWidth + colSpacing
	}
	return maxWidth + tableAdditionalWidth // Allow table to expand up to the additional width
}()

var (
	tableStyle = baseStyle.BorderForeground(focusedBorderColor)
)

type TableSelectionChangedMsg struct {
	Selected *data.Package
}

type PackageTableModel struct {
	// Data
	packages []*data.Package

	// UI component
	table table.Model

	// State
	sortColumn     packageTableColumn
	columns        []packageTableColumn // Enabled table columns
	visibleColumns []packageTableColumn // Columns currently visible in the UI, depending on screen width

	// Key bindings
	sortNext key.Binding
	sortPrev key.Binding
}

func NewPackageTableModel() PackageTableModel {
	// Main table
	tbl := table.New(
		table.WithFocused(true),
		table.WithStyles(getTableStyles()),
	)

	// Parse hidden columns from command line flag into a set
	hiddenColumns := make(map[packageTableColumn]bool)
	for _, c := range *flagHideCols {
		if col, err := parseColumnName(c); err == nil {
			if col.hideable() {
				hiddenColumns[col] = true
			} else {
				fmt.Fprintf(os.Stderr, "Column %s can not be hidden\n", col.String())
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Add all non-hidden columns
	columns := []packageTableColumn{}
	for i := range int(totalNumColumns) {
		col := packageTableColumn(i)
		if _, hidden := hiddenColumns[col]; !hidden {
			columns = append(columns, col)
		}
	}

	sortCol, err := parseColumnName(*flagSortColumn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else if !sortCol.sortable() {
		fmt.Fprintf(os.Stderr, "Can not sort by column: %s\n", sortCol.String())
		os.Exit(1)
	}

	return PackageTableModel{
		table:      tbl,
		sortColumn: sortCol,
		columns:    columns,
		sortNext:   key.NewBinding(key.WithKeys("s")),
		sortPrev:   key.NewBinding(key.WithKeys("S")),
	}
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

func (m PackageTableModel) Update(msg tea.Msg) (PackageTableModel, tea.Cmd) {
	selected := m.Selected()
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.sortNext):
			m.sortNextColumn()
		case key.Matches(msg, m.sortPrev):
			m.sortPrevColumn()
		}
	}
	m.table, _ = m.table.Update(msg)
	if m.Selected() != selected {
		cmd = m.sendSelectionChangedMsg()
	}
	return m, cmd
}

func (m PackageTableModel) View() string {
	return tableStyle.Render(m.table.View())
}

func (m *PackageTableModel) SetDimensions(width, height int) {
	m.table.SetWidth(width)
	m.table.SetHeight(height)
	m.updateColumns()
	m.UpdateRows()
}

func (m *PackageTableModel) SetPackages(pkgs []*data.Package) tea.Cmd {
	selected := m.Selected()
	m.packages = pkgs
	m.sortRows()
	if m.Selected() != selected {
		return m.sendSelectionChangedMsg()
	} else {
		return nil
	}
}

func (m *PackageTableModel) Packages() []*data.Package {
	return m.packages
}

func (m *PackageTableModel) SetFocused(focused bool) {
	if focused {
		tableStyle = tableStyle.BorderForeground(focusedBorderColor)
	} else {
		tableStyle = tableStyle.BorderForeground(borderColor)
	}
}

func (m *PackageTableModel) Selected() *data.Package {
	if m.table.Cursor() >= 0 && m.table.Cursor() < len(m.packages) {
		return m.packages[m.table.Cursor()]
	} else {
		return nil
	}
}

func (m *PackageTableModel) sendSelectionChangedMsg() tea.Cmd {
	return func() tea.Msg {
		return TableSelectionChangedMsg{
			Selected: m.Selected(),
		}
	}
}

func (m *PackageTableModel) ShowPackageInstalls() bool {
	return m.isColumnEnabled(colInstalls)
}

func (m *PackageTableModel) ShowPackageSizes() bool {
	return m.isColumnEnabled(colSize)
}

func (m *PackageTableModel) isColumnEnabled(c packageTableColumn) bool {
	return slices.Contains(m.columns, c)
}

func (m *PackageTableModel) isColumnVisible(c packageTableColumn) bool {
	return slices.Contains(m.visibleColumns, c)
}

func (m *PackageTableModel) sortNextColumn() {
	newCol := m.sortColumn.nextColumn()
	for !m.isColumnVisible(newCol) || !newCol.sortable() {
		newCol = newCol.nextColumn()
	}
	m.sortColumn = newCol
	// Needs to update column because sorting indicator changed
	m.updateColumns()
	m.sortRows()
}

func (m *PackageTableModel) sortPrevColumn() {
	newCol := m.sortColumn.prevColumn()
	for !m.isColumnVisible(newCol) || !newCol.sortable() {
		newCol = newCol.prevColumn()
	}
	m.sortColumn = newCol
	// Needs to update column because sorting indicator changed
	m.updateColumns()
	m.sortRows()
}

func (m *PackageTableModel) sortRows() {
	switch m.sortColumn {
	case colName:
		sort.Slice(m.packages, func(i, j int) bool {
			return m.packages[i].Name < m.packages[j].Name
		})
	case colTap:
		sort.Slice(m.packages, func(i, j int) bool {
			return m.packages[i].Tap < m.packages[j].Tap
		})
	case colInstalls:
		sort.Slice(m.packages, func(i, j int) bool {
			return m.packages[i].Installs90d > m.packages[j].Installs90d
		})
	case colSize:
		sort.Slice(m.packages, func(i, j int) bool {
			return m.packages[i].Size > m.packages[j].Size
		})
	case colStatus:
		sort.Slice(m.packages, func(i, j int) bool {
			return m.packages[i].Status() < m.packages[j].Status()
		})
	}
	m.UpdateRows()
}

func (m *PackageTableModel) updateColumns() {
	var newCols []table.Column
	m.visibleColumns, newCols = m.getVisibleCols()
	columnsChanged := len(newCols) != len(m.table.Columns())
	if columnsChanged {
		// Number of columns changed, clear rows before setting columns
		m.table.SetRows([]table.Row{})
	}
	m.table.SetColumns(newCols)
	if columnsChanged {
		// Re-compute rows
		m.UpdateRows()
	}
}

func (m *PackageTableModel) getVisibleCols() ([]packageTableColumn, []table.Column) {
	visibleCols := []packageTableColumn{}
	remainingWidth := m.table.Width()
	for _, col := range m.columns {
		colWidth := col.width()
		if remainingWidth >= colWidth+colSpacing {
			visibleCols = append(visibleCols, col)
			remainingWidth -= colWidth + colSpacing
		}
	}

	columns := []table.Column{}
	for _, col := range visibleCols {
		colTitle := col.String()
		colWidth := col.width()
		// Add sort indicator
		if col == m.sortColumn {
			if col.reverseSort() {
				colTitle = fmt.Sprintf("↓ %s", colTitle)
			} else {
				colTitle = fmt.Sprintf("↑ %s", colTitle)
			}
		}
		// Right align columns
		if col.rightAligned() {
			colTitle = fmt.Sprintf("%*s", colWidth, colTitle)
		}
		// Adjust column width to use remainingWidth
		// If desc column is not visible, the name column takes all remaining width
		if col == colName {
			if !slices.Contains(visibleCols, colDescription) {
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
	return visibleCols, columns
}

func (m *PackageTableModel) UpdateRows() {
	rows := make([]table.Row, len(m.packages))
	for i, pkg := range m.packages {
		rowData := []string{}
		for _, col := range m.visibleColumns {
			colData := col.getColumnData(pkg)
			if col.rightAligned() {
				colData = fmt.Sprintf("%*s", col.width(), colData)
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
}
