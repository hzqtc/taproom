package model

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"taproom/internal/brew"
	"taproom/internal/data"
	"taproom/internal/loading"
	"taproom/internal/ui"
	"taproom/internal/util"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
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
	flagFilters       = pflag.StringSliceP(
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
	allPackages []*data.Package // The complete list of all packages, sorted by name

	// UI Components from the bubbles library
	table       ui.PackageTableModel
	detailPanel viewport.Model
	search      textinput.Model
	spinner     spinner.Model
	stopwatch   stopwatch.Model

	// State
	isLoading   bool
	loadTimer   bool
	loadingPrgs *loading.LoadingProgress
	focusMode   focusMode
	filters     filterGroup
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

	fg, err := parseFilterGroup(*flagFilters)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return model{
		search:      searchInput,
		spinner:     s,
		stopwatch:   sw,
		table:       ui.NewPackageTableModel(),
		isLoading:   true,
		loadTimer:   *flagShowLoadTimer,
		loadingPrgs: loading.NewLoadingProgress(),
		filters:     fg,
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
		m.filterPackages()
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
			m.updatePackageForAction(msg.Command, msg.Pkgs)
			m.table.UpdateRows()
		} else {
			m.commandErr = true
		}
		// If there are error, it should already be displayed in the output
		m.updateLayout()

	case ui.TableSelectionChangedMsg:
		m.updateDetailsPanel()

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
		m.search.Blur()
		m.focusMode = focusTable
		m.updateFocusBorder()
	case key.Matches(msg, m.keys.Esc):
		m.search.Blur()
		m.focusMode = focusTable
		m.updateFocusBorder()
		m.search.SetValue("")
		m.filterPackages()
	default:
		m.search, cmd = m.search.Update(msg)
		m.filterPackages()
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
		m.search.SetValue("")
		m.output = []string{}
		m.commandErr = false
		m.filterPackages()
		m.updateLayout()

	case key.Matches(msg, m.keys.FilterAll):
		m.filters.reset()
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterFormulae):
		m.filters.toggleFilter(filterFormulae)
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterCasks):
		m.filters.toggleFilter(filterCasks)
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterInstalled):
		m.filters.toggleFilter(filterInstalled)
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterOutdated):
		m.filters.toggleFilter(filterOutdated)
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterExplicit):
		m.filters.toggleFilter(filterExplicitlyInstalled)
		m.filterPackages()
	case key.Matches(msg, m.keys.FilterActive):
		m.filters.toggleFilter(filterActive)
		m.filterPackages()

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

func (m *model) getPackage(name string) *data.Package {
	index := sort.Search(len(m.allPackages), func(i int) bool {
		return m.allPackages[i].Name >= name
	})

	if index < len(m.allPackages) && m.allPackages[index].Name == name {
		return m.allPackages[index]
	}

	return nil
}

// filterAndSortPackages updates the viewPackages based on current filters and sort mode.
func (m *model) filterPackages() {
	viewPackages := []*data.Package{}

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
			viewPackages = append(viewPackages, pkg)
		}
	}

	m.table.SetPackages(viewPackages)
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
