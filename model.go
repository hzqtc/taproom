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
	isLoading  bool
	loadingMsg string
	viewMode   viewMode
	sortMode   sortMode
	errorMsg   string
	width      int
	height     int

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

	// Data has been successfully loaded
	case dataLoadedMsg:
		m.isLoading = false
		m.allPackages = msg.packages
		m.filterAndSortPackages()
		m.updateTable()
		m.updateLayout()

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
			if msg.action == actionUpgradeAll {
				// Do a full reload for 'upgrade all' since it's the simplest
				// TODO: maybe we can just re-fetch installed packages
				m.viewMode = viewAll
				m.search.SetValue("")
				m.isLoading = true
				cmds = append(cmds, loadData)
			} else {
				// Otherwise update package according to the executed command and refresh table.
				m.updatePackageForAction(msg.action, msg.pkg)
				m.filterAndSortPackages()
				m.updateTable()
			}
		}
		m.updateLayout()

	// A key was pressed
	case tea.KeyMsg:
		// When a command is running, ignore keys
		if m.isExecuting {
			break
		}

		// If the search bar is focused, handle input there.
		if m.search.Focused() {
			switch {
			case key.Matches(msg, m.keys.ExitSearch):
				m.search.Blur()
			case key.Matches(msg, m.keys.ClearSearch):
				m.search.Blur()
				m.search.SetValue("")
				m.filterAndSortPackages()
				m.updateTable()
			default:
				m.search, cmd = m.search.Update(msg)
				m.filterAndSortPackages()
				m.updateTable()
				cmds = append(cmds, cmd)
			}
		} else {
			var selectedPkg *Package
			if len(m.viewPackages) > 0 && m.table.Cursor() >= 0 {
				selectedPkg = m.viewPackages[m.table.Cursor()]
			}

			switch {
			//General
			// TODO: allow switching focus betwen panels using Tab and scrolling in the view port
			case key.Matches(msg, m.keys.Refresh):
				m.search.SetValue("")
				m.isLoading = true
				cmds = append(cmds, loadData)
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit

			// Navigation
			case key.Matches(msg, m.keys.Up):
				m.table.MoveUp(1)
				m.updateViewport()
			case key.Matches(msg, m.keys.Down):
				m.table.MoveDown(1)
				m.updateViewport()
			case key.Matches(msg, m.keys.PageUp):
				m.table.MoveUp(m.table.Height())
				m.updateViewport()
			case key.Matches(msg, m.keys.PageDown):
				m.table.MoveDown(m.table.Height())
				m.updateViewport()
			case key.Matches(msg, m.keys.GoToTop):
				m.table.SetCursor(0)
				m.updateViewport()
			case key.Matches(msg, m.keys.GoToBottom):
				m.table.SetCursor(len(m.viewPackages) - 1)
				m.updateViewport()

			// Search
			case key.Matches(msg, m.keys.FocusSearch):
				m.search.Focus()
				cmds = append(cmds, textinput.Blink)

			// Sorting & Filtering
			case key.Matches(msg, m.keys.ToggleSort):
				if m.sortMode == sortByName {
					m.sortMode = sortByPopularity
				} else {
					m.sortMode = sortByName
				}
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

				// Commands
			case key.Matches(msg, m.keys.UpgradeAll):
				cmds = append(cmds, upgradeAllPackages())
			case key.Matches(msg, m.keys.Upgrade):
				if selectedPkg != nil && selectedPkg.IsOutdated && !selectedPkg.IsPinned {
					cmds = append(cmds, upgradePackage(selectedPkg))
				}
			case key.Matches(msg, m.keys.Install):
				if selectedPkg != nil && !selectedPkg.IsInstalled {
					cmds = append(cmds, installPackage(selectedPkg))
				}
			case key.Matches(msg, m.keys.Remove):
				if selectedPkg != nil && selectedPkg.IsInstalled {
					cmds = append(cmds, uninstallPackage(selectedPkg))
				}
			case key.Matches(msg, m.keys.Pin):
				if selectedPkg != nil && selectedPkg.IsInstalled && !selectedPkg.IsCask && !selectedPkg.IsPinned {
					cmds = append(cmds, pinPackage(selectedPkg))
				}
			case key.Matches(msg, m.keys.Unpin):
				if selectedPkg != nil && selectedPkg.IsPinned {
					cmds = append(cmds, unpinPackage(selectedPkg))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
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

func (m *model) updatePackageForAction(action commandAction, pkg *Package) {
	switch action {
	case actionUpgrade:
		pkg.InstalledVersion = pkg.Version
		pkg.IsOutdated = false
	case actionInstall:
		pkg.IsInstalled = true
		pkg.InstalledVersion = pkg.Version
	case actionUninstall:
		pkg.IsInstalled = false
		pkg.InstalledVersion = ""
		pkg.IsOutdated = false
		pkg.IsPinned = false
		pkg.InstalledAsDependency = false
	case actionPin:
		pkg.IsPinned = true
	case actionUnpin:
		pkg.IsPinned = false
	}

	// After updating fields, we need to update the Status field as well.
	pkg.Status = getPackageStatus(pkg)
}
