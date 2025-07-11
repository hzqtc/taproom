package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Constants & Data Structures ---
// TODO: make caching configurable
var cacheDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("could not determine home directory, using relative path for cache: %+v\n", err)
		return ".cache"
	}
	return filepath.Join(home, ".cache", "taproom")
}()

var brewPrefix = func() string {
	brewPrefixBytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		log.Printf("failed to identify brew prefix: %+v\n", err)
		return ""
	}
	return strings.TrimSpace(string(brewPrefixBytes))
}()

const (
	apiFormulaURL             = "https://formulae.brew.sh/api/formula.json"
	apiCaskURL                = "https://formulae.brew.sh/api/cask.json"
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaCacheFile           = "formula.json"
	casksCacheFile             = "cask.json"
	formulaeAnalyticsCacheFile = "formulae-analytics.json"
	casksAnalyticsCacheFile    = "casks-analytics.json"
	formulaeSizesCacheFile     = "formulae-sizes.json"
	casksSizesCacheFile        = "casks-sizes.json"

	urlCacheTtl      = 6 * time.Hour
	pkgSizesCacheTtl = 24 * time.Hour
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
	IsDeprecated          bool
	IsDisabled            bool
	InstalledAsDependency bool
	Size                  int64  // Size in bytes
	FormattedSize         string // Formated size like 24.5MB, 230KB
	InstalledDate         string
}

// Structs for parsing Homebrew API Json
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
	Deprecated        bool     `json:"deprecated"`
	Disabled          bool     `json:"disabled"`
	Installed         []struct {
		Version        string `json:"version"`
		Time           int64  `json:"time"`
		InstalledAsDep bool   `json:"installed_as_dependency"`
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
	Deprecated       bool   `json:"deprecated"`
	Disabled         bool   `json:"disabled"`
	InstalledVersion string `json:"installed"`
	InstalledTime    int64  `json:"installed_time"`
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
	formulaSizesChan := make(chan map[string]int64)
	caskSizesChan := make(chan map[string]int64)
	errChan := make(chan error, 7)

	go fetchJsonWithCache(
		apiFormulaURL,
		formulaCacheFile,
		&[]apiFormula{},
		formulaeChan,
		errChan,
	)
	go fetchJsonWithCache(
		apiCaskURL,
		casksCacheFile,
		&[]apiCask{},
		casksChan,
		errChan,
	)
	go fetchJsonWithCache(
		apiFormulaAnalytics90dURL,
		formulaeAnalyticsCacheFile,
		&apiFormulaAnalytics{},
		formulaAnalyticsChan,
		errChan,
	)
	go fetchJsonWithCache(
		apiCaskAnalytics90dURL,
		casksAnalyticsCacheFile,
		&apiCaskAnalytics{},
		caskAnalyticsChan,
		errChan,
	)
	go fetchInstalled(installedChan, errChan)
	go fetchFormulaSizes(formulaSizesChan, errChan)
	go fetchCaskSizes(caskSizesChan, errChan)

	var allFormulae []apiFormula
	var allCasks []apiCask
	var formulaAnalytics apiFormulaAnalytics
	var caskAnalytics apiCaskAnalytics
	var allInstalled installedInfo
	var formulaSizes map[string]int64
	var caskSizes map[string]int64

	for i := 0; i < cap(errChan); i++ {
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
		case sizes := <-formulaSizesChan:
			formulaSizes = sizes
		case sizes := <-caskSizesChan:
			caskSizes = sizes
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
		formulaSizes,
		caskSizes,
	)
	return dataLoadedMsg{packages: packages}
}

// fetchJsonWithCache is a generic function to fetch and decode Json from a URL, with caching.
func fetchJsonWithCache[T any](url, filename string, target *T, dataChan chan T, errChan chan error) {
	cachePath := filepath.Join(cacheDir, filename)

	// Attempt to load from cache first
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < urlCacheTtl {
		file, err := os.Open(cachePath)
		if err == nil {
			defer file.Close()
			body, err := io.ReadAll(file)
			if err == nil {
				if err := json.Unmarshal(body, &target); err == nil {
					log.Printf("Loaded %s from cache file %s", url, filename)
					dataChan <- *target
					return
				}
			}
		}
	}

	// If cache is invalid or missing, fetch from URL
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

	// Save to cache
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		if err := os.WriteFile(cachePath, body, 0644); err != nil {
			// Log caching error but don't fail the request
			log.Printf("Failed to write to cache at %s: %+v", cachePath, err)
		}
	}

	if err := json.Unmarshal(body, &target); err != nil {
		errChan <- fmt.Errorf("failed to decode json from %s: %w", url, err)
		return
	}
	log.Printf("Downloaded %s", url)
	dataChan <- *target
}

// fetchInstalled runs the `brew info` command and parses its Json output.
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

func fetchSizesWithCache(
	cacheFile string,
	fetcher func() (map[string]int64, error),
	sizesChan chan map[string]int64,
	errChan chan error,
) {
	cachePath := filepath.Join(cacheDir, cacheFile)

	// Attempt to load from cache first
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < pkgSizesCacheTtl {
		file, err := os.Open(cachePath)
		if err == nil {
			defer file.Close()
			body, err := io.ReadAll(file)
			if err == nil {
				var sizes map[string]int64
				if err := json.Unmarshal(body, &sizes); err == nil {
					log.Printf("Loaded package sizes from cache file %s", cacheFile)
					sizesChan <- sizes
					return
				}
			}
		}
	}

	// Fetch new data if cache is stale or invalid
	sizes, err := fetcher()
	if err != nil {
		errChan <- err
		return
	}

	// Save to cache
	if body, err := json.Marshal(sizes); err == nil {
		if err := os.MkdirAll(cacheDir, 0755); err == nil {
			if err := os.WriteFile(cachePath, body, 0644); err != nil {
				// Log caching error but don't fail the request
				log.Printf("Failed to write to cache at %s: %+v", cachePath, err)
			}
		}
	}

	sizesChan <- sizes
}

func fetchFormulaSizes(sizesChan chan map[string]int64, errChan chan error) {
	fetcher := func() (map[string]int64, error) {
		sizes := make(map[string]int64)
		// -k flag instructs du to output in KB
		cmd := exec.Command("du", "-k", "-d", "1", fmt.Sprintf("%s/Cellar", brewPrefix))
		output, err := cmd.Output()

		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					size, _ := strconv.ParseInt(fields[0], 10, 64)
					name := filepath.Base(fields[1])
					sizes[name] = size * 1024 // Convert KB to Bytes
				}
			}
		}
		return sizes, nil
	}
	fetchSizesWithCache(formulaeSizesCacheFile, fetcher, sizesChan, errChan)
}

func fetchCaskSizes(sizesChan chan map[string]int64, errChan chan error) {
	fetcher := func() (map[string]int64, error) {
		sizes := make(map[string]int64)

		// Step 1: Run `brew list --cask`
		listCmd := exec.Command("brew", "list", "--cask")
		listOutput, err := listCmd.Output()
		if err != nil {
			return sizes, fmt.Errorf("error running brew list --cask: %w", err)
		}

		// Step 2: Split output into cask names
		caskNames := strings.Fields(string(listOutput))
		if len(caskNames) == 0 {
			return sizes, nil
		}

		// Step 3: Run `brew info --cask <name1> <name2> ...`
		infoCmd := exec.Command("brew", append([]string{"info", "--cask"}, caskNames...)...)
		infoOutput, err := infoCmd.Output()
		if err != nil {
			return sizes, fmt.Errorf("error running brew info: %w", err)
		}

		// Step 4: Extract cask name to app size
		// Try to match following lines from brew info output:
		// /opt/homebrew/Caskroom/(cask name)/(version) (size)
		re := regexp.MustCompile(regexp.QuoteMeta(brewPrefix) + `/Caskroom/([^/]+)/[^ )]+ \(([^)]+)\)`)
		matches := re.FindAllStringSubmatch(string(infoOutput), -1)
		for _, match := range matches {
			appName := match[1]
			appSize := parseSizeToBytes(match[2])
			sizes[appName] = appSize
		}

		return sizes, nil
	}
	fetchSizesWithCache(casksSizesCacheFile, fetcher, sizesChan, errChan)
}

// processAllData merges all data sources into a single slice of Package.
func processAllData(
	installed installedInfo,
	formulae []apiFormula,
	casks []apiCask,
	formulaAnalytics apiFormulaAnalytics,
	caskAnalytics apiCaskAnalytics,
	formulaSizes map[string]int64,
	caskSizes map[string]int64,
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
		packages = append(packages, packageFromFormula(&f, formulaAnalyticsMap[f.Name], true, formulaSizes[f.Name]))
		installedFormulae[f.Name] = struct{}{}
		for _, dep := range f.Dependencies {
			formulaDependentsMap[dep] = append(formulaDependentsMap[dep], f.Name)
		}
	}
	// Process installed casks
	for _, c := range installed.Casks {
		packages = append(packages, packageFromCask(&c, caskAnalyticsMap[c.Name], true, caskSizes[c.Name]))
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
			packages = append(packages, packageFromFormula(&f, formulaAnalyticsMap[f.Name], false, 0))
			for _, dep := range f.Dependencies {
				formulaDependentsMap[dep] = append(formulaDependentsMap[dep], f.Name)
			}
		}
	}
	// Add casks to packages, except for installed casks
	for _, c := range casks {
		if _, installed := installedCasks[c.Name]; !installed {
			packages = append(packages, packageFromCask(&c, caskAnalyticsMap[c.Name], false, 0))
			for _, dep := range c.Dependencies.Formulae {
				formulaDependentsMap[dep] = append(formulaDependentsMap[dep], c.Name)
			}
			for _, dep := range c.Dependencies.Casks {
				caskDependentsMap[dep] = append(caskDependentsMap[dep], c.Name)
			}
		}
	}

	// Populate dependents
	for i, pkg := range packages {
		if pkg.IsCask {
			packages[i].Dependents = sortAndUniq(caskDependentsMap[pkg.Name])
		} else {
			packages[i].Dependents = sortAndUniq(formulaDependentsMap[pkg.Name])
		}
	}

	// Sort all packages by name for faster lookups later.
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	return packages
}

func packageFromFormula(f *apiFormula, installs int, installed bool, installedSize int64) Package {
	pkg := Package{
		Name:              f.Name,
		Tap:               f.Tap,
		Version:           f.Versions.Stable,
		Desc:              f.Desc,
		Homepage:          f.Homepage,
		License:           f.License,
		Dependencies:      sortAndUniq(f.Dependencies),
		BuildDependencies: f.BuildDependencies,
		InstallCount90d:   installs,
		IsCask:            false,
		IsDeprecated:      f.Deprecated,
		IsDisabled:        f.Disabled,
	}
	if installed {
		inst := f.Installed[0]
		pkg.IsInstalled = true
		pkg.InstalledVersion = inst.Version
		pkg.IsOutdated = f.Outdated
		pkg.IsPinned = f.Pinned
		pkg.InstalledAsDependency = inst.InstalledAsDep
		pkg.Size = installedSize
		pkg.FormattedSize = formatSize(installedSize)
		pkg.InstalledDate = time.Unix(inst.Time, 0).Format(time.DateOnly)
	}

	return pkg
}

func packageFromCask(c *apiCask, installs int, installed bool, installedSize int64) Package {
	pkg := Package{
		Name:            c.Name,
		Tap:             c.Tap,
		Version:         c.Version,
		Desc:            c.Desc,
		Homepage:        c.Homepage,
		License:         "N/A",
		Dependencies:    sortAndUniq(append(c.Dependencies.Formulae, c.Dependencies.Casks...)),
		InstallCount90d: installs,
		IsCask:          true,
		IsDeprecated:    c.Deprecated,
		IsDisabled:      c.Disabled,
	}
	if installed {
		pkg.IsInstalled = true
		pkg.InstalledVersion = c.InstalledVersion
		pkg.IsOutdated = c.Outdated
		// Casks can't be pinned or installed as dependencies
		pkg.IsPinned = false
		pkg.InstalledAsDependency = false
		pkg.Size = installedSize
		pkg.FormattedSize = formatSize(installedSize)
		pkg.InstalledDate = time.Unix(c.InstalledTime, 0).Format(time.DateOnly)
	}

	return pkg
}
