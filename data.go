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
	Name                  string
	FullName              string // Used as a unique key
	Tap                   string
	Version               string
	Desc                  string
	Homepage              string
	License               string
	Dependencies          []string
	InstallCount90d       int
	IsCask                bool
	Status                string
	IsInstalled           bool
	IsOutdated            bool
	IsPinned              bool
	InstalledAsDependency bool
}

// Structs for parsing Homebrew API JSON
type apiFormula struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Desc     string `json:"desc"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Homepage     string   `json:"homepage"`
	License      string   `json:"license"`
	Dependencies []string `json:"dependencies"`
	Tap          string   `json:"tap"`
}

type apiCask struct {
	Token        string   `json:"token"`
	FullName     string   `json:"full_token"`
	Name         []string `json:"name"`
	Desc         string   `json:"desc"`
	Version      string   `json:"version"`
	Homepage     string   `json:"homepage"`
	Dependencies struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"depends_on"`
	Tap string `json:"tap"`
}

type apiAnalytics struct {
	Items []struct {
		Formula string `json:"formula"`
		Cask    string `json:"cask"`
		Count   string `json:"count"`
	} `json:"items"`
}

// Structs for parsing `brew info --json=v2 --installed`
type brewInfo struct {
	Formulae []installedFormula `json:"formulae"`
	Casks    []installedCask    `json:"casks"`
}

type installedFormula struct {
	Name      string `json:"name"`
	FullName  string `json:"full_name"`
	Outdated  bool   `json:"outdated"`
	Pinned    bool   `json:"pinned"`
	Installed []struct {
		Version        string `json:"version"`
		InstalledAsReq bool   `json:"installed_as_dependency"`
	} `json:"installed"`
}

type installedCask struct {
	Token            string   `json:"token"`
	FullName         string   `json:"full_token"`
	Name             []string `json:"name"`
	Outdated         bool     `json:"outdated"`
	InstalledVersion string   `json:"installed"`
}

// --- Data Fetching & Processing Logic ---

// Message types for tea.Cmd
type dataLoadedMsg struct{ packages []Package }
type dataLoadingErr struct{ err error }

// loadData is a tea.Cmd that fetches all data concurrently.
func loadData() tea.Msg {
	formulaeChan := make(chan []apiFormula)
	casksChan := make(chan []apiCask)
	formulaAnalyticsChan := make(chan apiAnalytics)
	caskAnalyticsChan := make(chan apiAnalytics)
	installedChan := make(chan brewInfo)
	errChan := make(chan error, 5)

	go fetchJSON(apiFormulaURL, &[]apiFormula{}, formulaeChan, errChan)
	go fetchJSON(apiCaskURL, &[]apiCask{}, casksChan, errChan)
	go fetchJSON(apiFormulaAnalytics90dURL, &apiAnalytics{}, formulaAnalyticsChan, errChan)
	go fetchJSON(apiCaskAnalytics90dURL, &apiAnalytics{}, caskAnalyticsChan, errChan)
	go fetchInstalled(installedChan, errChan)

	var allFormulae []apiFormula
	var allCasks []apiCask
	var formulaAnalyticsData apiAnalytics
	var caskAnalyticsData apiAnalytics
	var installedData brewInfo

	for i := 0; i < 5; i++ {
		select {
		case f := <-formulaeChan:
			allFormulae = f
		case c := <-casksChan:
			allCasks = c
		case fa := <-formulaAnalyticsChan:
			formulaAnalyticsData = fa
		case ca := <-caskAnalyticsChan:
			caskAnalyticsData = ca
		case inst := <-installedChan:
			installedData = inst
		case err := <-errChan:
			return dataLoadingErr{err}
		}
	}

	// Merge analytics data
	mergedAnalytics := formulaAnalyticsData
	mergedAnalytics.Items = append(mergedAnalytics.Items, caskAnalyticsData.Items...)

	packages := processAllData(allFormulae, allCasks, mergedAnalytics, installedData)
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
func fetchInstalled(installedChan chan brewInfo, errChan chan error) {
	cmd := exec.Command("brew", "info", "--json=v2", "--installed")
	output, err := cmd.Output()
	if err != nil {
		// Not a fatal error if brew is not installed or no packages are installed.
		installedChan <- brewInfo{}
		return
	}

	var info brewInfo
	if err := json.Unmarshal(output, &info); err != nil {
		errChan <- fmt.Errorf("failed to decode brew info json: %w", err)
		return
	}
	installedChan <- info
}

// processAllData merges all data sources into a single slice of Package.
func processAllData(formulae []apiFormula, casks []apiCask, analytics apiAnalytics, installed brewInfo) []Package {
	packages := make([]Package, 0, len(formulae)+len(casks))
	analyticsMap := make(map[string]int)
	installedMap := make(map[string]struct {
		isOutdated   bool
		isPinned     bool
		isDependency bool
	})

	for _, item := range analytics.Items {
		countStr := strings.ReplaceAll(item.Count, ",", "")
		count, _ := strconv.Atoi(countStr)
		name := item.Formula
		if name == "" {
			name = item.Cask
		}
		analyticsMap[name] = count
	}
	for _, f := range installed.Formulae {
		isDependency := false
		if len(f.Installed) > 0 {
			isDependency = f.Installed[0].InstalledAsReq
		}
		installedMap[f.FullName] = struct {
			isOutdated   bool
			isPinned     bool
			isDependency bool
		}{f.Outdated, f.Pinned, isDependency}
	}
	for _, c := range installed.Casks {
		installedMap[c.FullName] = struct {
			isOutdated   bool
			isPinned     bool
			isDependency bool
		}{c.Outdated, false, false} // Casks can't be pinned or installed as dependencies
	}

	for _, f := range formulae {
		pkg := Package{
			Name:         f.Name,
			FullName:     f.FullName,
			Tap:          f.Tap,
			Version:      f.Versions.Stable,
			Desc:         f.Desc,
			Homepage:     f.Homepage,
			License:      f.License,
			Dependencies: f.Dependencies,
			IsCask:       false,
		}
		pkg.InstallCount90d = analyticsMap[pkg.Name]
		if inst, ok := installedMap[pkg.FullName]; ok {
			pkg.IsInstalled, pkg.IsOutdated, pkg.IsPinned, pkg.InstalledAsDependency = true, inst.isOutdated, inst.isPinned, inst.isDependency
			if inst.isOutdated {
				pkg.Status = "Outdated"
			} else if inst.isPinned {
				pkg.Status = "Pinned"
			} else {
				pkg.Status = "Installed"
			}
		} else {
			pkg.Status = "Not Installed"
		}
		packages = append(packages, pkg)
	}

	for _, c := range casks {
		pkg := Package{
			Name:         c.Token,
			FullName:     c.FullName,
			Tap:          c.Tap,
			Version:      c.Version,
			Desc:         c.Desc,
			Homepage:     c.Homepage,
			Dependencies: append(c.Dependencies.Formulae, c.Dependencies.Casks...),
			IsCask:       true,
		}
		pkg.InstallCount90d = analyticsMap[pkg.Name]
		if inst, ok := installedMap[pkg.FullName]; ok {
			pkg.IsInstalled, pkg.IsOutdated, pkg.IsPinned, pkg.InstalledAsDependency = true, inst.isOutdated, inst.isPinned, inst.isDependency
			if inst.isOutdated {
				pkg.Status = "Outdated"
			} else {
				pkg.Status = "Installed"
			}
		} else {
			pkg.Status = "Not Installed"
		}
		packages = append(packages, pkg)
	}
	return packages
}
