package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Constants & Data Structures ---
const (
	apiFormulaURL             = "https://formulae.brew.sh/api/formula.json"
	apiCaskURL                = "https://formulae.brew.sh/api/cask.json"
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"
)

// Package holds all combined information for a formula or cask.
type Package struct {
	Name                  string // Used as a unique key
	Tap                   string
	Version               string
	InstalledVersion      string
	Desc                  string
	Homepage              string
	License               string
	Dependencies          []string
	BuildDependencies     []string
	Dependents            []string
	InstallCount90d       int
	IsCask                bool
	IsInstalled           bool
	IsOutdated            bool
	IsPinned              bool
	InstalledAsDependency bool
	Status                string
}

// Structs for parsing Homebrew API JSON
type apiFormula struct {
	Name     string `json:"name"`
	Tap      string `json:"tap"`
	Desc     string `json:"desc"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Homepage          string   `json:"homepage"`
	License           string   `json:"license"`
	Dependencies      []string `json:"dependencies"`
	BuildDependencies []string `json:"build_dependencies"`
	Outdated          bool     `json:"outdated"`
	Pinned            bool     `json:"pinned"`
	Installed         []struct {
		Version        string `json:"version"`
		InstalledAsReq bool   `json:"installed_as_dependency"`
	} `json:"installed"`
}

type apiCask struct {
	Name         string `json:"token"`
	Tap          string `json:"tap"`
	Desc         string `json:"desc"`
	Version      string `json:"version"`
	Homepage     string `json:"homepage"`
	Dependencies struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"depends_on"`
	Outdated         bool   `json:"outdated"`
	InstalledVersion string `json:"installed"`
}

type apiFormulaAnalytics struct {
	Items []struct {
		Name  string `json:"formula"`
		Count string `json:"count"`
	} `json:"items"`
}

type apiCaskAnalytics struct {
	Items []struct {
		Name  string `json:"cask"`
		Count string `json:"count"`
	} `json:"items"`
}

// Structs for parsing `brew info --json=v2 --installed`
type installedInfo struct {
	Formulae []apiFormula `json:"formulae"`
	Casks    []apiCask    `json:"casks"`
}

// --- Data Fetching & Processing Logic ---

// Message types for tea.Cmd
type dataLoadedMsg struct{ packages []Package }
type dataLoadingErr struct{ err error }

// loadData is a tea.Cmd that fetches all data concurrently.
func loadData() tea.Msg {
	formulaeChan := make(chan []apiFormula)
	casksChan := make(chan []apiCask)
	formulaAnalyticsChan := make(chan apiFormulaAnalytics)
	caskAnalyticsChan := make(chan apiCaskAnalytics)
	installedChan := make(chan installedInfo)
	errChan := make(chan error, 5)

	go fetchJSON(
		apiFormulaURL,
		&[]apiFormula{},
		formulaeChan,
		errChan,
	)
	go fetchJSON(
		apiCaskURL,
		&[]apiCask{},
		casksChan,
		errChan,
	)
	go fetchJSON(
		apiFormulaAnalytics90dURL,
		&apiFormulaAnalytics{},
		formulaAnalyticsChan,
		errChan,
	)
	go fetchJSON(
		apiCaskAnalytics90dURL,
		&apiCaskAnalytics{},
		caskAnalyticsChan,
		errChan,
	)
	go fetchInstalled(installedChan, errChan)

	var allFormulae []apiFormula
	var allCasks []apiCask
	var formulaAnalytics apiFormulaAnalytics
	var caskAnalytics apiCaskAnalytics
	var allInstalled installedInfo

	for i := 0; i < 5; i++ {
		select {
		case f := <-formulaeChan:
			allFormulae = f
		case c := <-casksChan:
			allCasks = c
		case fa := <-formulaAnalyticsChan:
			formulaAnalytics = fa
		case ca := <-caskAnalyticsChan:
			caskAnalytics = ca
		case inst := <-installedChan:
			allInstalled = inst
		case err := <-errChan:
			return dataLoadingErr{err}
		}
	}

	packages := processAllData(
		allInstalled,
		allFormulae,
		allCasks,
		formulaAnalytics,
		caskAnalytics,
	)
	return dataLoadedMsg{packages: packages}
}

// fetchJSON is a generic function to fetch and decode JSON from a URL.
func fetchJSON[T any](url string, target *T, dataChan chan T, errChan chan error) {
	resp, err := http.Get(url)
	if err != nil {
		errChan <- fmt.Errorf("failed to fetch %s: %w", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errChan <- fmt.Errorf("bad status from %s: %s", url, resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errChan <- fmt.Errorf("failed to read body from %s: %w", url, err)
		return
	}

	if err := json.Unmarshal(body, &target); err != nil {
		errChan <- fmt.Errorf("failed to decode json from %s: %w", url, err)
		return
	}
	dataChan <- *target
}

// fetchInstalled runs the `brew info` command and parses its JSON output.
func fetchInstalled(installedChan chan installedInfo, errChan chan error) {
	cmd := exec.Command("brew", "info", "--json=v2", "--installed")
	output, err := cmd.Output()
	if err != nil {
		// Not a fatal error if brew is not installed or no packages are installed.
		installedChan <- installedInfo{}
		return
	}

	var info installedInfo
	if err := json.Unmarshal(output, &info); err != nil {
		errChan <- fmt.Errorf("failed to decode brew info json: %w", err)
		return
	}
	installedChan <- info
}

// processAllData merges all data sources into a single slice of Package.
func processAllData(
	installed installedInfo,
	formulae []apiFormula,
	casks []apiCask,
	formulaAnalytics apiFormulaAnalytics,
	caskAnalytics apiCaskAnalytics,
) []Package {
	formulaAnalyticsMap := make(map[string]int)
	caskAnalyticsMap := make(map[string]int)
	// Process analytics data to be used to constuct Package struct
	for _, item := range formulaAnalytics.Items {
		countStr := strings.ReplaceAll(item.Count, ",", "")
		count, _ := strconv.Atoi(countStr)
		formulaAnalyticsMap[item.Name] = count
	}
	for _, item := range caskAnalytics.Items {
		countStr := strings.ReplaceAll(item.Count, ",", "")
		count, _ := strconv.Atoi(countStr)
		caskAnalyticsMap[item.Name] = count
	}

	formulaDependentsMap := make(map[string][]string) // formula name to packages that depends on it
	caskDependentsMap := make(map[string][]string)    // cask name to packages that depends on it
	installedFormulae := make(map[string]struct{})    // track installed formulae to avoid duplicate
	installedCasks := make(map[string]struct{})       // track installed casks to avoid duplicate

	packages := make([]Package, 0, len(installed.Formulae)+len(installed.Casks)+len(formulae)+len(casks))
	// Process installed formulae
	for _, f := range installed.Formulae {
		packages = append(packages, packageFromFormula(&f, formulaAnalyticsMap[f.Name]))
		installedFormulae[f.Name] = struct{}{}
		for _, dep := range f.Dependencies {
			formulaDependentsMap[dep] = append(formulaDependentsMap[dep], f.Name)
		}
	}
	// Process installed casks
	for _, c := range installed.Casks {
		packages = append(packages, packageFromCask(&c, caskAnalyticsMap[c.Name]))
		installedCasks[c.Name] = struct{}{}
		for _, dep := range c.Dependencies.Formulae {
			formulaDependentsMap[dep] = append(formulaDependentsMap[dep], c.Name)
		}
		for _, dep := range c.Dependencies.Casks {
			caskDependentsMap[dep] = append(caskDependentsMap[dep], c.Name)
		}
	}
	// Add formulaes to packages, except for installed formulae
	for _, f := range formulae {
		if _, installed := installedFormulae[f.Name]; !installed {
			packages = append(packages, packageFromFormula(&f, formulaAnalyticsMap[f.Name]))
		}
	}
	// Add casks to packages, except for installed casks
	for _, c := range casks {
		if _, installed := installedCasks[c.Name]; !installed {
			packages = append(packages, packageFromCask(&c, caskAnalyticsMap[c.Name]))
		}
	}

	// Populate dependents for each installed package
	for i, pkg := range packages {
		if pkg.IsInstalled {
			if pkg.IsCask {
				packages[i].Dependents = caskDependentsMap[pkg.Name]
			} else {
				packages[i].Dependents = formulaDependentsMap[pkg.Name]
			}
		}
	}

	return packages
}

func packageFromFormula(f *apiFormula, installs int) Package {
	pkg := Package{
		Name:              f.Name,
		Tap:               f.Tap,
		Version:           f.Versions.Stable,
		Desc:              f.Desc,
		Homepage:          f.Homepage,
		License:           f.License,
		Dependencies:      f.Dependencies,
		BuildDependencies: f.BuildDependencies,
		InstallCount90d:   installs,
		IsCask:            false,
	}
	if len(f.Installed) > 0 {
		inst := f.Installed[0]
		pkg.IsInstalled = true
		pkg.InstalledVersion = inst.Version
		pkg.IsOutdated = f.Outdated
		pkg.IsPinned = f.Pinned
		pkg.InstalledAsDependency = !inst.InstalledAsReq
	}
	pkg.Status = getPackageStatus(&pkg)

	return pkg
}

func packageFromCask(c *apiCask, installs int) Package {
	pkg := Package{
		Name:            c.Name,
		Tap:             c.Tap,
		Version:         c.Version,
		Desc:            c.Desc,
		Homepage:        c.Homepage,
		Dependencies:    append(c.Dependencies.Formulae, c.Dependencies.Casks...),
		InstallCount90d: installs,
		IsCask:          true,
	}
	if c.InstalledVersion != "" {
		pkg.IsInstalled = true
		pkg.InstalledVersion = c.InstalledVersion
		pkg.IsOutdated = c.Outdated
		// Casks can't be pinned or installed as dependencies
		pkg.IsPinned = false
		pkg.InstalledAsDependency = false
	}
	pkg.Status = getPackageStatus(&pkg)

	return pkg
}

func getPackageStatus(pkg *Package) string {
	if pkg.IsPinned {
		return "Pinned"
	} else if pkg.IsOutdated {
		return "Outdated"
	} else if pkg.InstalledAsDependency {
		return "Installed (Dep)"
	} else if pkg.IsInstalled {
		return "Installed"
	} else {
		return "Uninstalled"
	}
}
