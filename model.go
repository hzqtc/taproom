package main

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
)

// viewMode defines which subset of packages is currently being viewed.
type viewMode int

const (
	viewAll viewMode = iota
	viewFormulae
	viewCasks
	viewInstalled
	viewOutdated
	viewExplicitlyInstalled
	viewNonDisabled
)

func (v viewMode) String() string {
	switch v {
	case viewAll:
		return "All"
	case viewFormulae:
		return "Formulae"
	case viewCasks:
		return "Casks"
	case viewInstalled:
		return "Installed"
	case viewOutdated:
		return "Outdated"
	case viewExplicitlyInstalled:
		return "Installed (no deps)"
	case viewNonDisabled:
		return "Hiding disabled"
	default:
		return "Unknown"
	}
}

// sortMode defines the current sorting method for the package list.
type sortMode int

const (
	sortByName sortMode = iota
	sortByPopularity
)

// columnName is a type for identifying table columns.
type columnName int

const (
	colSymbol      columnName = iota // Symbol to differentiate formula vs cask
	colName                          // Name of the formula or token of the cask, unique identifier when combine with IsCask
	colVersion                       // Newest version
	colTap                           // Homebrew tap
	colDescription                   // Brief description
	colInstalls                      // Number of installs in the last 90 days
	colStatus                        // Calculated status such as deprecated, installed, outdated, pinned
)

// focusMode defines which component is currently focused
type focusMode int

const focusModeLen = 3

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
	table    table.Model
	viewport viewport.Model
	search   textinput.Model
	spinner  spinner.Model

	// State
	isLoading      bool
	loadingMsg     string
	focusMode      focusMode
	viewMode       viewMode
	sortMode       sortMode
	errorMsg       string
	width          int
	height         int
	visibleColumns []columnName

	// Keybindings
	keys keyMap

	// Command execution
	isExecuting bool
	output      []string
	cmdChan     chan tea.Msg
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

	// Main table
	tbl := table.New(
		table.WithFocused(true),
	)
	tbl.SetStyles(getTableStyles())

	return model{
		search:    searchInput,
		spinner:   s,
		table:     tbl,
		isLoading: true,
		keys:      defaultKeyMap(),
	}
}

// Init is the first command that is run when the application starts.
func (m model) Init() tea.Cmd {
	// Start the spinner and load the data from Homebrew APIs.
	return tea.Batch(m.spinner.Tick, loadData)
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
		m.allPackages = msg.packages
		m.filterAndSortPackages()
		m.updateLayout()
		m.updateTable()

	// An error occurred during data loading
	case dataLoadingErr:
		m.isLoading = false
		m.errorMsg = msg.err.Error()

	// Spinner tick (for animation)
	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	// Command execution start
	case commandStartMsg:
		m.isExecuting = true
		m.output = []string{}

	// Command execution message with channel
	case commandExecMsg:
		m.cmdChan = msg.ch
		cmds = append(cmds, waitForOutput(m.cmdChan))

	// Command execution output
	case commandOutputMsg:
		m.output = append(m.output, msg.line)
		m.updateLayout()
		cmds = append(cmds, waitForOutput(m.cmdChan))

	// Command execution finish
	case commandFinishMsg:
		m.isExecuting = false
		m.cmdChan = nil
		if msg.stderr != "" {
			m.errorMsg = msg.stderr
		} else if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			// Command was successful, update package state
			m.updatePackageForAction(msg.action, msg.pkgs)
			m.filterAndSortPackages()
			m.updateTable()
		}
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
				m.search.SetValue("")
				m.isLoading = true
				cmds = append(cmds, loadData)
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
	case key.Matches(msg, m.keys.Enter):
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
	case key.Matches(msg, m.keys.ToggleSort):
		if m.sortMode == sortByName {
			m.sortMode = sortByPopularity
		} else {
			m.sortMode = sortByName
		}
		m.updateLayout() // Needs to update table column header
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterAll):
		m.viewMode = viewAll
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterFormulae):
		m.viewMode = viewFormulae
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterCasks):
		m.viewMode = viewCasks
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterInstalled):
		m.viewMode = viewInstalled
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterOutdated):
		m.viewMode = viewOutdated
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterExplicit):
		m.viewMode = viewExplicitlyInstalled
		m.filterAndSortPackages()
		m.updateTable()
	case key.Matches(msg, m.keys.FilterDisabled):
		m.viewMode = viewNonDisabled
		m.filterAndSortPackages()
		m.updateTable()

	// Commands
	case key.Matches(msg, m.keys.OpenHomePage):
		if selectedPkg != nil && selectedPkg.Homepage != "" {
			browser.OpenURL(selectedPkg.Homepage)
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

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterAndSortPackages() {
	m.viewPackages = []*Package{}

	// Search with user query then filter by view mode
	searchQuery := strings.ToLower(m.search.Value())
	// Split query to tokens and match each token separately
	tokens := strings.Fields(searchQuery)
	for i := range m.allPackages {
		pkg := &m.allPackages[i]
		matches := true
		// Requires a package's name or its description to contain all tokens in the query
		for _, t := range tokens {
			if !strings.Contains(strings.ToLower(pkg.Name), t) && !strings.Contains(strings.ToLower(pkg.Desc), t) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}

		passesFilter := false
		switch m.viewMode {
		case viewAll:
			passesFilter = true
		case viewFormulae:
			passesFilter = !pkg.IsCask
		case viewCasks:
			passesFilter = pkg.IsCask
		case viewInstalled:
			passesFilter = pkg.IsInstalled
		case viewOutdated:
			passesFilter = pkg.IsOutdated
		case viewExplicitlyInstalled:
			passesFilter = pkg.IsInstalled && !pkg.InstalledAsDependency
		case viewNonDisabled:
			passesFilter = !pkg.IsDisabled && !pkg.IsDeprecated
		}

		if passesFilter {
			m.viewPackages = append(m.viewPackages, pkg)
		}
	}

	// No need to sort by name becuase m.allPackages are sorted by name
	if m.sortMode == sortByPopularity {
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].InstallCount90d > m.viewPackages[j].InstallCount90d
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

func (m *model) markInstalled(pkg *Package) {
	pkg.IsInstalled = true
	pkg.IsOutdated = false
	pkg.InstalledVersion = pkg.Version
}

func (m *model) markInstalledAsDep(pkg *Package) {
	m.markInstalled(pkg)
	pkg.InstalledAsDependency = true
}

func (m *model) markUninstalled(pkg *Package) {
	pkg.IsInstalled = false
	pkg.InstalledVersion = ""
	pkg.IsOutdated = false
	pkg.IsPinned = false
	pkg.InstalledAsDependency = false
}

func (m *model) markPinned(pkg *Package) {
	pkg.IsPinned = true
}

func (m *model) markUnpinned(pkg *Package) {
	pkg.IsPinned = false
}

func (m *model) updatePackageForAction(action commandAction, pkgs []*Package) {
	switch action {
	case actionUpgradeAll, actionUpgrade:
		for _, pkg := range pkgs {
			m.markInstalled(pkg)
		}
	case actionInstall:
		for _, pkg := range pkgs {
			m.markInstalled(pkg)
			// Also mark uninstalled dependencies as installed
			for _, depName := range m.getRecursiveMissingDeps(pkg.Name) {
				m.markInstalled(m.getPackage(depName))
			}
		}
	case actionUninstall:
		for _, pkg := range pkgs {
			m.markUninstalled(pkg)
		}
	case actionPin:
		for _, pkg := range pkgs {
			m.markPinned(pkg)
		}
	case actionUnpin:
		for _, pkg := range pkgs {
			m.markUnpinned(pkg)
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
