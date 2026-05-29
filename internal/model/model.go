package model

import (
	"strings"
	"taproom/internal/brew"
	"taproom/internal/data"
	"taproom/internal/ui"
	"taproom/internal/util"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
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

type model struct {
	// The complete list of all packages, sorted by name
	allPackages []*data.Package

	// UI Components from the bubbles library
	table       ui.PackageTableModel
	detailPanel ui.DetailsPanelModel
	search      ui.SearchInputModel
	filterView  ui.FilterViewModel
	helpView    ui.HelpModel
	statsView   ui.StatsModel
	outputView  ui.OutputModel
	loadingView ui.LoadingScreenModel

	// State
	isExecuting bool
	focusMode   focusMode
	width       int
	height      int

	// Keybindings
	keys keyMap
}

func InitialModel() model {
	return model{
		table:       ui.NewPackageTableModel(),
		detailPanel: ui.NewDetailsPanelModel(),
		search:      ui.NewSearchInputModel(),
		filterView:  ui.NewFilterViewModel(),
		helpView:    ui.NewHelpModel(),
		statsView:   ui.NewStatsModel(),
		outputView:  ui.NewOutputModel(),
		loadingView: ui.NewLoadingScreenModel(),
		keys:        defaultKeyMap(),
	}
}

func (m model) Init() tea.Cmd {
	return m.loadData()
}

func (m *model) loadData() tea.Cmd {
	return tea.Batch(
		m.loadingView.StartLoading(),
		brew.LoadData(m.table.ShowPackageInstalls(), m.table.ShowPackageSizes(), m.loadingView.Progress()),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case brew.DataLoadedMsg:
		m.allPackages = msg.Packages
		cmds = append(cmds, m.loadingView.StopLoading(), m.filterPackages())
		m.updateLayout()

	case brew.DataLoadingErrMsg:
		cmds = append(cmds, m.loadingView.SetError(msg.Err.Error()))

	case spinner.TickMsg, stopwatch.TickMsg, stopwatch.StartStopMsg, stopwatch.ResetMsg:
		m.loadingView, cmd = m.loadingView.Update(msg)
		cmds = append(cmds, cmd)

	case brew.CommandStartMsg:
		m.isExecuting = true
		m.outputView.Clear()

	case brew.CommandOutputMsg:
		if msg.Line != "" {
			m.outputView.Append(msg.Line)
			m.updateLayout()
		}
		cmds = append(cmds, brew.StreamOutput(msg.Ch))

	case brew.CommandFinishMsg:
		m.isExecuting = false
		if msg.Err == nil {
			// Command was successful, clear output and update package state
			m.outputView.Clear()
			brew.UpdatePackageForAction(msg.Command, msg.Pkgs)
			m.table.UpdateRows()
		} else {
			m.outputView.SetError()
		}
		// If there are error, it should already be displayed in the output
		m.updateLayout()

	case ui.TableSelectionChangedMsg:
		m.detailPanel.SetPackage(msg.Selected)

	case ui.SearchMsg:
		cmds = append(cmds, m.filterPackages())

	case ui.FilterChangedMsg:
		cmds = append(cmds, m.filterPackages())

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
				cmds = append(cmds, m.loadData())
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
		m.outputView.Clear()
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
			case ui.FilterCompatible:
				os, arch := util.CurrentPlatform()
				currentPlatform := data.Platform{OS: os, Arch: arch}
				passesFilter = pkg.IsCompatibleWith(currentPlatform)
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
