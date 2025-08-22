package model

import (
	"strings"
	"taproom/internal/brew"
	"taproom/internal/data"
	"taproom/internal/loading"
	"taproom/internal/ui"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
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
)

// model holds the entire state of the application.
type model struct {
	// Package data
	allPackages []*data.Package // The complete list of all packages, sorted by name

	// UI Components from the bubbles library
	table       ui.PackageTableModel
	detailPanel ui.DetailsPanelModel
	search      ui.SearchInputModel
	filterView  ui.FilterViewModel
	helpView    ui.HelpModel
	statsView   ui.StatsModel
	spinner     spinner.Model
	stopwatch   stopwatch.Model

	// State
	isLoading   bool
	loadTimer   bool
	loadingPrgs *loading.LoadingProgress
	focusMode   focusMode
	errorMsg    string
	width       int
	height      int

	// Keybindings
	keys keyMap

	// Command execution
	isExecuting bool
	output      []string
	commandErr  bool
}

func InitialModel() model {

	// Spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot

	var sw stopwatch.Model
	if *flagShowLoadTimer {
		sw = stopwatch.NewWithInterval(time.Millisecond)
	}

	return model{
		spinner:     s,
		stopwatch:   sw,
		table:       ui.NewPackageTableModel(),
		detailPanel: ui.NewDetailsPanelModel(),
		search:      ui.NewSearchInputModel(),
		filterView:  ui.NewFilterViewModel(),
		helpView:    ui.NewHelpModel(),
		statsView:   ui.NewStatsModel(),
		isLoading:   true,
		loadTimer:   *flagShowLoadTimer,
		loadingPrgs: loading.NewLoadingProgress(),
		keys:        defaultKeyMap(),
	}
}

// Init is the first command that is run when the application starts.
func (m model) Init() tea.Cmd {
	// Start the spinner and load the data from Homebrew APIs.
	cmds := []tea.Cmd{m.spinner.Tick, brew.LoadData(m.table.ShowPackageInstalls(), m.table.ShowPackageSizes(), m.loadingPrgs)}
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

	// Data has been successfully loaded
	case brew.DataLoadedMsg:
		m.isLoading = false
		m.loadingPrgs.Reset()
		if m.loadTimer {
			cmds = append(cmds, m.stopwatch.Stop(), m.stopwatch.Reset())
		}
		m.allPackages = msg.Packages
		cmds = append(cmds, m.filterPackages())
		m.updateLayout()

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
			brew.UpdatePackageForAction(msg.Command, msg.Pkgs)
			m.table.UpdateRows()
		} else {
			m.commandErr = true
		}
		// If there are error, it should already be displayed in the output
		m.updateLayout()

	case ui.TableSelectionChangedMsg:
		m.detailPanel.SetPackage(msg.Selected)

	case ui.SearchMsg:
		cmds = append(cmds, m.filterPackages())

	case ui.FilterChangedMsg:
		cmds = append(cmds, m.filterPackages())

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
				m.updateFocusBorder()
				cmds = append(cmds, textinput.Blink)
			case key.Matches(msg, m.keys.Refresh):
				m.isLoading = true
				m.output = []string{}
				cmds = append(cmds, m.spinner.Tick, brew.LoadData(m.table.ShowPackageInstalls(), m.table.ShowPackageSizes(), m.loadingPrgs))
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
		m.focusMode = focusTable
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.focusMode = focusTable
		m.updateFocusBorder()
		cmd = m.search.Clear()
	default:
		m.search, cmd = m.search.Update(msg)
	}
	return cmd
}

func (m *model) handleTableKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	selectedPkg := m.table.Selected()

	switch {
	case key.Matches(msg, m.keys.Enter):
		m.focusMode = focusDetail
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.search.Clear()
		m.output = []string{}
		m.commandErr = false
		cmd = m.filterPackages()
		m.updateLayout()

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
		outdatedPkgs := brew.GetOutdatedPackages()
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
		m.filterView, cmd = m.filterView.Update(msg)
		if cmd == nil {
			m.table, cmd = m.table.Update(msg)
		}
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

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterPackages() tea.Cmd {
	viewPackages := []*data.Package{}

	searchQuery := strings.ToLower(m.search.Value())
	keywords := strings.Fields(searchQuery)

	for i := range m.allPackages {
		pkg := m.allPackages[i]

		if !pkg.MatchKeywords(keywords) {
			continue
		}

		passesFilter := true
		for _, f := range m.filterView.Value() {
			switch f {
			case ui.FilterFormulae:
				passesFilter = !pkg.IsCask
			case ui.FilterCasks:
				passesFilter = pkg.IsCask
			case ui.FilterInstalled:
				passesFilter = pkg.IsInstalled
			case ui.FilterOutdated:
				passesFilter = pkg.IsOutdated
			case ui.FilterExplicitlyInstalled:
				passesFilter = pkg.IsInstalled && !pkg.InstalledAsDependency
			case ui.FilterActive:
				passesFilter = !pkg.IsDisabled && !pkg.IsDeprecated
			}
			// A package needs to pass all filters, so break early when it doesn't pass any filter
			if !passesFilter {
				break
			}
		}

		if passesFilter {
			viewPackages = append(viewPackages, pkg)
		}
	}

	m.statsView.SetPackages(viewPackages)
	return m.table.SetPackages(viewPackages)
}
