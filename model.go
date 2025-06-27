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

// sortMode defines the current sorting method for the package list.
type sortMode int

const (
	sortByName sortMode = iota
	sortByPopularity
)

// model holds the entire state of the application.
type model struct {
	// Package data
	allPackages  []Package // The complete list of all packages
	viewPackages []Package // The filtered and sorted list of packages to display

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

	// A key was pressed
	case tea.KeyMsg:
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
		} else { // Otherwise, handle global keybindings.
			switch {
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
				return m, textinput.Blink

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
			}
		}
	}

	// Update the viewport model.
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) getPackage(name string) *Package {
	if name == "" {
		return nil
	}

	for _, pkg := range m.allPackages {
		if pkg.Name == name {
			return &pkg
		}
	}
	return nil
}

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterAndSortPackages() {
	m.viewPackages = []Package{}
	searchQuery := strings.ToLower(m.search.Value())

	for _, pkg := range m.allPackages {
		if searchQuery != "" &&
			!strings.Contains(strings.ToLower(pkg.Name), searchQuery) &&
			!strings.Contains(strings.ToLower(pkg.Desc), searchQuery) {
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

	if m.sortMode == sortByName {
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].Name < m.viewPackages[j].Name
		})
	} else if m.sortMode == sortByPopularity {
		sort.Slice(m.viewPackages, func(i, j int) bool {
			return m.viewPackages[i].InstallCount90d > m.viewPackages[j].InstallCount90d
		})
	}
}
