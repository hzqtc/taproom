package model

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"taproom/internal/brew"
	"taproom/internal/data"
	"taproom/internal/loading"
	"taproom/internal/util"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	"github.com/spf13/pflag"
)

// focusMode defines which component is currently focused
type focusMode int

const (
	focusTable focusMode = iota
	focusDetail
	focusSearch
)

var (
	flagShowLoadTimer = pflag.BoolP("load-timer", "t", false, "Show a timer in the loading screen")
	flagHideCols      = pflag.StringSlice(
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
	flagFilters = pflag.StringSliceP(
		"filters",
		"f",
		[]string{},
		"Filters to enable (comma separated no space).\n"+
			"Pick 0 or 1 filter from each group: (Formulae, Casks), (Installed, Outdated, Expl. Installed, Active)",
	)
)

// model holds the entire state of the application.
type model struct {
	// Package data
	allPackages  []*data.Package // The complete list of all packages, sorted by name
	viewPackages []*data.Package // The filtered and sorted list of packages to display

	// UI Components from the bubbles library
	table       table.Model
	detailPanel viewport.Model
	search      textinput.Model
	spinner     spinner.Model
	stopwatch   stopwatch.Model

	// State
	isLoading      bool
	loadTimer      bool
	loadingPrgs    *loading.LoadingProgress
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
	commandErr  bool
}

func InitialModel() model {
	// Search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search packages..."
	searchInput.Prompt = " / "

	// Spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot

	var sw stopwatch.Model
	if *flagShowLoadTimer {
		sw = stopwatch.NewWithInterval(time.Millisecond)
	}

	// Main table
	tbl := table.New(
		table.WithFocused(true),
	)
	tbl.SetStyles(getTableStyles())

	// Parse hidden columns from command line flag into a set
	hiddenColumns := make(map[columnName]bool)
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
	columns := []columnName{}
	for i := range int(totalNumColumns) {
		col := columnName(i)
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

	fg, err := parseFilterGroup(*flagFilters)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return model{
		search:      searchInput,
		spinner:     s,
		stopwatch:   sw,
		table:       tbl,
		isLoading:   true,
		loadTimer:   *flagShowLoadTimer,
		loadingPrgs: loading.NewLoadingProgress(),
		filters:     fg,
		sortColumn:  sortCol,
		columns:     columns,
		keys:        defaultKeyMap(),
	}
}

// Init is the first command that is run when the application starts.
func (m model) Init() tea.Cmd {
	// Start the spinner and load the data from Homebrew APIs.
	cmds := []tea.Cmd{m.spinner.Tick, brew.LoadData(m.isColumnEnabled(colInstalls), m.isColumnEnabled(colSize), m.loadingPrgs)}
	if m.loadTimer {
		cmds = append(cmds, m.stopwatch.Start())
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
	case brew.DataLoadedMsg:
		m.isLoading = false
		m.loadingPrgs.Reset()
		if m.loadTimer {
			cmds = append(cmds, m.stopwatch.Stop(), m.stopwatch.Reset())
		}
		m.allPackages = msg.Packages
		m.filterAndSortPackages()
		m.updateLayout()
		m.updateTable()

	// An error occurred during data loading
	case brew.DataLoadingErrMsg:
		m.isLoading = false
		if m.loadTimer {
			cmds = append(cmds, m.stopwatch.Stop())
		}
		// Data loading error is fatal
		m.errorMsg = msg.Err.Error()

	// Spinner tick (for animation)
	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case stopwatch.TickMsg:
		if m.isLoading {
			m.stopwatch, cmd = m.stopwatch.Update(msg)
			cmds = append(cmds, cmd)
		}

	case stopwatch.StartStopMsg, stopwatch.ResetMsg:
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		cmds = append(cmds, cmd)

	// Command execution start
	case brew.CommandStartMsg:
		m.isExecuting = true
		m.output = []string{}
		m.commandErr = false

	// Command execution output
	case brew.CommandOutputMsg:
		if msg.Line != "" {
			m.output = append(m.output, msg.Line)
			m.updateLayout()
		}
		cmds = append(cmds, brew.StreamOutput(msg.Ch))

	// Command execution finish
	case brew.CommandFinishMsg:
		m.isExecuting = false
		if msg.Err == nil {
			// Command was successful, clear output and update package state
			m.output = m.output[:0]
			m.updatePackageForAction(msg.Command, msg.Pkgs)
			m.updateTable()
		} else {
			m.commandErr = true
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
				switch m.focusMode {
				case focusTable:
					m.focusMode = focusDetail
				case focusDetail:
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
				cmds = append(cmds, m.spinner.Tick, brew.LoadData(m.isColumnEnabled(colInstalls), m.isColumnEnabled(colSize), m.loadingPrgs))
				if m.loadTimer {
					cmds = append(cmds, m.stopwatch.Start())
				}
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			default:
				switch m.focusMode {
				case focusDetail:
					cmds = append(cmds, m.handleDetailsPanelKeys(msg))
				case focusTable:
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
	selectedPkg := m.getSelectedPackage()

	switch {
	case key.Matches(msg, m.keys.Enter):
		m.focusMode = focusDetail
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.search.SetValue("")
		m.output = []string{}
		m.commandErr = false
		m.filterAndSortPackages()
		m.updateTable()
		m.updateLayout()

	// Sorting & Filtering
	case key.Matches(msg, m.keys.SortByNext):
		// Sort by the next sortable and visible column
		for {
			m.sortColumn = (m.sortColumn + 1) % totalNumColumns
			if m.sortColumn.sortable() && m.isColumnVisible(m.sortColumn) {
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
			if m.sortColumn.sortable() && m.isColumnVisible(m.sortColumn) {
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
	case key.Matches(msg, m.keys.OpenRelease):
		if selectedPkg != nil && selectedPkg.ReleaseInfo != nil {
			browser.OpenURL(selectedPkg.ReleaseInfo.Url)
		}
	case key.Matches(msg, m.keys.UpgradeAll):
		outdatedPkgs := m.getOutdatedPackages()
		if !m.isExecuting && len(outdatedPkgs) > 0 {
			cmd = brew.UpgradeAllPackages(outdatedPkgs)
		}
	case key.Matches(msg, m.keys.Upgrade):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsOutdated && !selectedPkg.IsPinned {
			cmd = brew.UpgradePackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Install):
		if !m.isExecuting && selectedPkg != nil && !selectedPkg.IsInstalled {
			cmd = brew.InstallPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Remove):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsInstalled {
			cmd = brew.UninstallPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Pin):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsInstalled && !selectedPkg.IsCask && !selectedPkg.IsPinned {
			cmd = brew.PinPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.Unpin):
		if !m.isExecuting && selectedPkg != nil && selectedPkg.IsPinned {
			cmd = brew.UnpinPackage(selectedPkg)
		}
	case key.Matches(msg, m.keys.CleanUp):
		cmd = brew.Cleanup()

	default:
		// Let table itself handle the rest of keys
		m.table, cmd = m.table.Update(msg)
		m.updateDetailsPanel()
	}

	return cmd
}

func (m *model) handleDetailsPanelKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, m.keys.Esc):
		m.focusMode = focusTable
		m.updateFocusBorder()
	default:
		m.detailPanel, cmd = m.detailPanel.Update(msg)
	}
	return cmd
}

func (m *model) getSelectedPackage() *data.Package {
	if len(m.viewPackages) > 0 && m.table.Cursor() >= 0 {
		return m.viewPackages[m.table.Cursor()]
	} else {
		return nil
	}
}

func (m *model) getPackage(name string) *data.Package {
	index := sort.Search(len(m.allPackages), func(i int) bool {
		return m.allPackages[i].Name >= name
	})

	if index < len(m.allPackages) && m.allPackages[index].Name == name {
		return m.allPackages[index]
	}

	return nil
}

func (m *model) isColumnEnabled(c columnName) bool {
	return slices.Contains(m.columns, c)
}

func (m *model) isColumnVisible(c columnName) bool {
	return slices.Contains(m.visibleColumns, c)
}

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterAndSortPackages() {
	m.viewPackages = []*data.Package{}

	searchQuery := strings.ToLower(m.search.Value())
	keywords := strings.Fields(searchQuery)

	for i := range m.allPackages {
		pkg := m.allPackages[i]

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

func (m *model) getOutdatedPackages() []*data.Package {
	outdatedPackages := []*data.Package{}
	for i := range m.allPackages {
		if pkg := m.allPackages[i]; pkg.IsOutdated {
			outdatedPackages = append(outdatedPackages, pkg)
		}
	}
	return outdatedPackages
}

func (m *model) updatePackageForAction(command brew.BrewCommand, pkgs []*data.Package) {
	switch command {
	case brew.BrewCommandUpgradeAll, brew.BrewCommandUpgrade:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
		}
	case brew.BrewCommandInstall:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
			// Also mark uninstalled dependencies as installed
			for _, depName := range m.getRecursiveMissingDeps(pkg.Name) {
				m.getPackage(depName).MarkInstalled()
			}

			pkg.Size = brew.FetchPackageSize(pkg)
			pkg.FormattedSize = util.FormatSize(pkg.Size)
		}
	case brew.BrewCommandUninstall:
		for _, pkg := range pkgs {
			pkg.MarkUninstalled()
		}
	case brew.BrewCommandPin:
		for _, pkg := range pkgs {
			pkg.MarkPinned()
		}
	case brew.BrewCommandUnpin:
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
