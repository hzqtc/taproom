package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
)

// focusMode defines which component is currently focused
type focusMode int

const (
	focusTable focusMode = iota
	focusDetail
	focusSearch
)

// model holds the entire state of the application.
type model struct {
	// Package data
	allPackages  []Package  // The complete list of all packages, sorted by name
	viewPackages []*Package // The filtered and sorted list of packages to display

	// UI Components from the bubbles library
	table     table.Model
	viewport  viewport.Model
	search    textinput.Model
	spinner   spinner.Model
	stopwatch stopwatch.Model

	// State
	isLoading      bool
	loadingMsg     string
	focusMode      focusMode
	filters        filterGroup
	sortColumn     columnName
	errorMsg       string
	width          int
	height         int
	columns        []columnName // Enabled table columns
	visibleColumns []columnName // Columns currently visible in the UI, depending on screen width

	// Keybindings
	keys keyMap

	// Command execution
	isExecuting bool
	output      []string
}

// initialModel creates the starting state of the application.
func initialModel() model {
	// Search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search packages..."
	searchInput.Prompt = " / "

	// Spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot

	var sw stopwatch.Model
	if *showLoadTimer {
		sw = stopwatch.NewWithInterval(time.Millisecond)
	}

	// Main table
	tbl := table.New(
		table.WithFocused(true),
	)
	tbl.SetStyles(getTableStyles())

	// Parse hidden columns from command line flag into a set
	hiddenColumns := make(map[columnName]struct{})
	for _, c := range *hiddenCols {
		if col, err := parseColumnName(c); err == nil {
			hiddenColumns[col] = struct{}{}
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Add all non-hidden columns
	columns := []columnName{}
	for i := 0; i < int(totalNumColumns); i++ {
		col := columnName(i)
		if _, hidden := hiddenColumns[col]; !hidden {
			columns = append(columns, col)
		}
	}

	return model{
		search:     searchInput,
		spinner:    s,
		stopwatch:  sw,
		table:      tbl,
		isLoading:  true,
		loadingMsg: "",
		sortColumn: colName,
		columns:    columns,
		keys:       defaultKeyMap(),
	}
}

// Init is the first command that is run when the application starts.
func (m model) Init() tea.Cmd {
	// Start the spinner and load the data from Homebrew APIs.
	cmds := []tea.Cmd{m.spinner.Tick, m.loadData()}
	if *showLoadTimer {
		cmds = append(cmds, m.stopwatch.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles all incoming messages and returns a new model and command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	// Window was resized
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.updateTable()

	// Data has been successfully loaded
	case dataLoadedMsg:
		m.isLoading = false
		m.loadingMsg = ""
		if *showLoadTimer {
			cmds = append(cmds, m.stopwatch.Stop())
		}
		m.allPackages = msg.packages
		m.filterAndSortPackages()
		m.updateLayout()
		m.updateTable()

	// An error occurred during data loading
	case dataLoadingErrMsg:
		m.isLoading = false
		if *showLoadTimer {
			cmds = append(cmds, m.stopwatch.Stop())
		}
		// Data loading error is fatal
		m.errorMsg = msg.err.Error()

	case loadingProgressMsg:
		if msg.message != "" {
			m.loadingMsg = m.loadingMsg + "\n" + msg.message
		}
		cmds = append(cmds, streamLoadingProgress(msg.ch))

	// Spinner tick (for animation)
	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case stopwatch.TickMsg, stopwatch.StartStopMsg:
		if m.isLoading {
			m.stopwatch, cmd = m.stopwatch.Update(msg)
			cmds = append(cmds, cmd)
		}

	// Command execution start
	case commandStartMsg:
		m.isExecuting = true
		m.output = []string{}

	// Command execution output
	case commandOutputMsg:
		if msg.line != "" {
			m.output = append(m.output, msg.line)
			m.updateLayout()
		}
		cmds = append(cmds, streamOutput(msg.ch))

	// Command execution finish
	case commandFinishMsg:
		m.isExecuting = false
		if msg.err == nil {
			// Command was successful, clear output and update package state
			m.output = m.output[:0]
			m.updatePackageForAction(msg.action, msg.pkgs)
			m.updateTable()
		}
		// If there are error, it should already be displayed in the output
		m.updateLayout()

	// A key was pressed
	case tea.KeyMsg:
		if m.focusMode == focusSearch {
			cmds = append(cmds, m.handleSearchInputKeys(msg))
		} else {
			// General keys when focus is not on search
			switch {
			case key.Matches(msg, m.keys.SwitchFocus):
				// Tab switches focus between table and viewport
				if m.focusMode == focusTable {
					m.focusMode = focusDetail
				} else if m.focusMode == focusDetail {
					m.focusMode = focusTable
				}
				m.updateFocusBorder()
			case key.Matches(msg, m.keys.FocusSearch):
				m.focusMode = focusSearch
				m.search.Focus()
				m.updateFocusBorder()
				cmds = append(cmds, textinput.Blink)
			case key.Matches(msg, m.keys.Refresh):
				m.isLoading = true
				m.output = []string{}
				cmds = append(cmds, m.loadData())
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			default:
				if m.focusMode == focusDetail {
					cmds = append(cmds, m.handleViewportKeys(msg))
				} else if m.focusMode == focusTable {
					cmds = append(cmds, m.handleTableKeys(msg))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) handleSearchInputKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, m.keys.Enter) || key.Matches(msg, m.keys.SwitchFocus):
		m.search.Blur()
		m.focusMode = focusTable
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.search.Blur()
		m.focusMode = focusTable
		m.updateFocusBorder()
		m.search.SetValue("")
		m.filterAndSortPackages()
		m.updateTable()
	default:
		m.search, cmd = m.search.Update(msg)
		m.filterAndSortPackages()
		m.updateTable()
	}
	return cmd
}

func (m *model) handleTableKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	var selectedPkg *Package
	if len(m.viewPackages) > 0 && m.table.Cursor() >= 0 {
		selectedPkg = m.viewPackages[m.table.Cursor()]
	}

	switch {
	case key.Matches(msg, m.keys.Enter):
		m.focusMode = focusDetail
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.search.SetValue("")
		m.filterAndSortPackages()
		m.updateTable()

	// Sorting & Filtering
	case key.Matches(msg, m.keys.SortByNext):
		// Sort by the next sortable and visible column
		for {
			m.sortColumn = (m.sortColumn + 1) % totalNumColumns
			if m.sortColumn.Sortable() && m.isColumnVisible(m.sortColumn) {
				break
			}
		}
		m.updateLayout() // Needs to update table column header
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.SortByPrev):
		// Sort by the previous sortable and visible column
		for {
			m.sortColumn = m.sortColumn - 1
			if m.sortColumn < 0 {
				m.sortColumn = totalNumColumns - 1
			}
			if m.sortColumn.Sortable() && m.isColumnVisible(m.sortColumn) {
				break
			}
		}
		m.updateLayout() // Needs to update table column header
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterAll):
		m.filters.reset()
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterFormulae):
		m.filters.toggleFilter(filterFormulae)
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterCasks):
		m.filters.toggleFilter(filterCasks)
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterInstalled):
		m.filters.toggleFilter(filterInstalled)
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterOutdated):
		m.filters.toggleFilter(filterOutdated)
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterExplicit):
		m.filters.toggleFilter(filterExplicitlyInstalled)
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterActive):
		m.filters.toggleFilter(filterActive)
		m.filterAndSortPackages()
		m.updateTable()

	// Commands
	case key.Matches(msg, m.keys.OpenHomePage):
		if selectedPkg != nil && selectedPkg.Homepage != "" {
			browser.OpenURL(selectedPkg.Homepage)
		}
	case key.Matches(msg, m.keys.OpenBrewUrl):
		if selectedPkg != nil {
			browser.OpenURL(selectedPkg.BrewUrl())
		}
	case key.Matches(msg, m.keys.UpgradeAll):
		outdatedPkgs := m.getOutdatedPackages()
		if !m.isExecuting && len(outdatedPkgs) > 0 {
			cmd = upgradeAllPackages(outdatedPkgs)
		}
	case key.Matches(msg, m.keys.Upgrade):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsOutdated && !selectedPkg.IsPinned {
			cmd = upgradePackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Install):
		if !m.isExecuting && selectedPkg != nil && !selectedPkg.IsInstalled {
			cmd = installPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Remove):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsInstalled {
			cmd = uninstallPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Pin):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsInstalled && !selectedPkg.IsCask && !selectedPkg.IsPinned {
			cmd = pinPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Unpin):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsPinned {
			cmd = unpinPackage(selectedPkg)
		}

	default:
		// Let table itself handle the rest of keys
		m.table, cmd = m.table.Update(msg)
		m.updateViewport()
	}

	return cmd
}

func (m *model) handleViewportKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, m.keys.Esc):
		m.focusMode = focusTable
		m.updateFocusBorder()
	default:
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return cmd
}

func (m *model) getPackage(name string) *Package {
	index := sort.Search(len(m.allPackages), func(i int) bool {
		return m.allPackages[i].Name >= name
	})

	if index < len(m.allPackages) && m.allPackages[index].Name == name {
		return &m.allPackages[index]
	}

	return nil
}

func (m *model) isColumnEnabled(c columnName) bool {
	for _, col := range m.columns {
		if c == col {
			return true
		}
	}
	return false
}

func (m *model) isColumnVisible(c columnName) bool {
	for _, col := range m.visibleColumns {
		if c == col {
			return true
		}
	}
	return false
}

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterAndSortPackages() {
	m.viewPackages = []*Package{}

	searchQuery := strings.ToLower(m.search.Value())
	keywords := strings.Fields(searchQuery)

	for i := range m.allPackages {
		pkg := &m.allPackages[i]

		if !pkg.MatchKeywords(keywords) {
			continue
		}

		passesFilter := true
		for _, f := range m.filters.split() {
			switch f {
			case filterFormulae:
				passesFilter = !pkg.IsCask
			case filterCasks:
				passesFilter = pkg.IsCask
			case filterInstalled:
				passesFilter = pkg.IsInstalled
			case filterOutdated:
				passesFilter = pkg.IsOutdated
			case filterExplicitlyInstalled:
				passesFilter = pkg.IsInstalled && !pkg.InstalledAsDependency
			case filterActive:
				passesFilter = !pkg.IsDisabled && !pkg.IsDeprecated
			}
			// A package needs to pass all filters, so break early when it doesn't pass any filter
			if !passesFilter {
				break
			}
		}

		if passesFilter {
			m.viewPackages = append(m.viewPackages, pkg)
		}
	}

	switch m.sortColumn {
	case colName:
		// No need to sort by name becuase m.allPackages are sorted by name
	case colTap:
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].Tap < m.viewPackages[j].Tap
		})
	case colInstalls:
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].InstallCount90d > m.viewPackages[j].InstallCount90d
		})
	case colSize:
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].Size > m.viewPackages[j].Size
		})
	case colStatus:
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].Status() < m.viewPackages[j].Status()
		})
	}
}

func (m *model) getOutdatedPackages() []*Package {
	outdatedPackages := []*Package{}
	for i := range m.allPackages {
		if pkg := &m.allPackages[i]; pkg.IsOutdated {
			outdatedPackages = append(outdatedPackages, pkg)
		}
	}
	return outdatedPackages
}

func (m *model) updatePackageForAction(action commandAction, pkgs []*Package) {
	switch action {
	case actionUpgradeAll, actionUpgrade:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
		}
	case actionInstall:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
			// Also mark uninstalled dependencies as installed
			for _, depName := range m.getRecursiveMissingDeps(pkg.Name) {
				m.getPackage(depName).MarkInstalled()
			}
		}
	case actionUninstall:
		for _, pkg := range pkgs {
			pkg.MarkUninstalled()
		}
	case actionPin:
		for _, pkg := range pkgs {
			pkg.MarkPinned()
		}
	case actionUnpin:
		for _, pkg := range pkgs {
			pkg.MarkUnpinned()
		}
	}
}

func (m *model) getRecursiveMissingDeps(pkgName string) []string {
	pkg := m.getPackage(pkgName)
	if pkg.IsInstalled {
		return []string{}
	} else {
		deps := pkg.Dependencies
		depsCopy := append([]string{}, deps...)
		for _, dep := range depsCopy {
			deps = append(deps, m.getRecursiveMissingDeps(dep)...)
		}
		return deps
	}
}
