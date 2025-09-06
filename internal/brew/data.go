package brew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"taproom/internal/data"
	"taproom/internal/gh"
	"taproom/internal/loading"
	"taproom/internal/util"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

// Holding all packages
var allBrewPackages []*data.Package

var cacheDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("could not determine home directory, using relative path for cache: %+v\n", err)
		return ".cache"
	}
	return filepath.Join(home, ".cache", "taproom")
}()

const (
	apiFormulaURL             = "https://formulae.brew.sh/api/formula.json"
	apiCaskURL                = "https://formulae.brew.sh/api/cask.json"
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaCache              = "formula.json"
	casksCache                = "cask.json"
	formulaeAnalytics90dCache = "formulae-analytics-90d.json"
	casksAnalytics90dCache    = "casks-analytics-90d.json"

	urlCacheTtl = 6 * time.Hour
)

var (
	flagInvalidateCache  = pflag.BoolP("invalidate-cache", "i", false, "Invalidate cache and force re-downloading data")
	flagFetchReleaseInfo = pflag.Bool("fetch-release", false, "Fetching release data for installed packages")
)

// Structs for parsing Homebrew API Json
type apiFormula struct {
	Name     string `json:"name"`
	Tap      string `json:"tap"`
	Desc     string `json:"desc"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Homepage string `json:"homepage"`
	Urls     struct {
		Stable struct {
			Url string `json:"url"`
		} `json:"stable"`
		Head struct {
			Url string `json:"url"`
		} `json:"head"`
	} `json:"urls"`
	License           string   `json:"license"`
	Dependencies      []string `json:"dependencies"`
	BuildDependencies []string `json:"build_dependencies"`
	Conflicts         []string `json:"conflicts_with"`
	Deprecated        bool     `json:"deprecated"`
	Disabled          bool     `json:"disabled"`
}

type apiCask struct {
	Name         string `json:"token"`
	Tap          string `json:"tap"`
	Desc         string `json:"desc"`
	Version      string `json:"version"`
	Homepage     string `json:"homepage"`
	Url          string `json:"url"`
	Dependencies struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"depends_on"`
	Conflicts struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"conflicts_with"`
	AutoUpdate bool `json:"auto_updates"`
	Deprecated bool `json:"deprecated"`
	Disabled   bool `json:"disabled"`
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

// --- Data Fetching & Processing Logic ---

// Message types for tea.Cmd
type DataLoadedMsg struct {
	Packages []*data.Package
}

type DataLoadingErrMsg struct {
	Err error
}

// loadData returns a tea.Cmd that fetches all data concurrently.
func LoadData(fetchAnalytics, fetchSize bool, loadingPrgs *loading.LoadingProgress) tea.Cmd {
	return func() tea.Msg {
		formulaeChan := make(chan []apiFormula)
		casksChan := make(chan []apiCask)
		formulaAnalytics90dChan := make(chan apiFormulaAnalytics)
		caskAnalytics90dChan := make(chan apiCaskAnalytics)
		formulaInstallInfoChan := make(chan []*installInfo)
		caskInstallInfoChan := make(chan []*installInfo)
		loadingTasksNum := 6
		errChan := make(chan error, loadingTasksNum)

		var allFormulae []apiFormula
		var allCasks []apiCask
		var formulaAnalytics90d apiFormulaAnalytics
		var caskAnalytics90d apiCaskAnalytics
		var formulaInstallInfo, caskInstallInfo []*installInfo

		go fetchJsonWithCache(apiFormulaURL, formulaCache, &allFormulae, formulaeChan, errChan)
		loadingPrgs.AddTask(formulaeChan, "Loading all Formulae")
		go fetchJsonWithCache(apiCaskURL, casksCache, &allCasks, casksChan, errChan)
		loadingPrgs.AddTask(casksChan, "Loading all Casks")
		if fetchAnalytics {
			go fetchJsonWithCache(apiFormulaAnalytics90dURL, formulaeAnalytics90dCache, &formulaAnalytics90d, formulaAnalytics90dChan, errChan)
			loadingPrgs.AddTask(formulaAnalytics90dChan, "Loading Formulae 90d analytics")
			go fetchJsonWithCache(apiCaskAnalytics90dURL, casksAnalytics90dCache, &caskAnalytics90d, caskAnalytics90dChan, errChan)
			loadingPrgs.AddTask(caskAnalytics90dChan, "Loading Cask 90d analytics")
		} else {
			loadingTasksNum -= 2
			formulaAnalytics90d = apiFormulaAnalytics{}
			caskAnalytics90d = apiCaskAnalytics{}
		}
		go fetchInstalledFormulae(fetchSize, formulaInstallInfoChan, errChan)
		loadingPrgs.AddTask(formulaInstallInfoChan, "Loading formulae installation data")
		go fetchInstalledCasks(fetchSize, caskInstallInfoChan, errChan)
		loadingPrgs.AddTask(caskInstallInfoChan, "Loading casks installation data")

		// Update brew in the background, we don't depend on `brew` command to get data
		// But we need brew to be updated when install/upgrade packages
		go updateBrew()

		for range loadingTasksNum {
			select {
			case f := <-formulaeChan:
				allFormulae = f
				loadingPrgs.MarkCompleted(formulaeChan)
			case c := <-casksChan:
				allCasks = c
				loadingPrgs.MarkCompleted(casksChan)
			case fa := <-formulaAnalytics90dChan:
				formulaAnalytics90d = fa
				loadingPrgs.MarkCompleted(formulaAnalytics90dChan)
			case ca := <-caskAnalytics90dChan:
				caskAnalytics90d = ca
				loadingPrgs.MarkCompleted(caskAnalytics90dChan)
			case info := <-formulaInstallInfoChan:
				formulaInstallInfo = info
				loadingPrgs.MarkCompleted(formulaInstallInfoChan)
			case info := <-caskInstallInfoChan:
				caskInstallInfo = info
				loadingPrgs.MarkCompleted(caskInstallInfoChan)
			case err := <-errChan:
				return DataLoadingErrMsg{err}
			}
		}

		allBrewPackages = processAllData(
			allFormulae,
			allCasks,
			formulaAnalytics90d,
			caskAnalytics90d,
			formulaInstallInfo,
			caskInstallInfo,
		)
		return DataLoadedMsg{Packages: allBrewPackages}
	}
}

func readCacheData(cachePath string) []byte {
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < urlCacheTtl {
		file, err := os.Open(cachePath)
		if err == nil {
			defer file.Close()
			body, err := io.ReadAll(file)
			if err == nil {
				return body
			}
		}
	}

	return nil
}

// fetchJsonWithCache is a generic function to fetch and decode Json from a URL, with caching.
func fetchJsonWithCache[T any](url, filename string, target *T, dataChan chan T, errChan chan error) {
	cachePath := filepath.Join(cacheDir, filename)

	// Attempt to load from cache first
	if !*flagInvalidateCache {
		if cacheData := readCacheData(cachePath); cacheData != nil {
			if err := json.Unmarshal(cacheData, &target); err == nil {
				log.Printf("Loaded %s from cache file %s", url, filename)
				dataChan <- *target
				return
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
		errChan <- fmt.Errorf("bad HTTP status fetching %s: %s", url, resp.Status)
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

func updateBrew() {
	var errOutput bytes.Buffer
	updateCmd := exec.Command("brew", "update")
	updateCmd.Stderr = &errOutput
	err := updateCmd.Run()
	if err != nil {
		log.Printf("failed to update homebrew %v: %s", err, errOutput.String())
	}
}

// processAllData merges all data sources into a single slice of Package.
func processAllData(
	formulae []apiFormula,
	casks []apiCask,
	formulaAnalytics90d apiFormulaAnalytics,
	caskAnalytics90d apiCaskAnalytics,
	formulaInstallInfo, caskInstallInfo []*installInfo,
) []*data.Package {
	formulaInstalls90d := mapFormulaeInstalls(formulaAnalytics90d) // formula name to 90d installs
	caskInstalls90d := mapCaskInstalls(caskAnalytics90d)           // cask name to 90d installs
	installedFormulae := mapInstallInfo(formulaInstallInfo)        // formula name to *installInfo
	installedCasks := mapInstallInfo(caskInstallInfo)              // cask  name to *installInfo
	formulaDependents := make(map[string][]string)                 // formula name to packages that depends on it
	caskDependents := make(map[string][]string)                    // cask name to packages that depends on it

	packages := []*data.Package{}

	// Add formulae from custom taps
	for _, info := range formulaInstallInfo {
		if info.tap != coreTap {
			pkg, err := getLocalPackage(info)
			if err == nil {
				pkg.Installs90d = formulaInstalls90d[pkg.Name]
				pkg.InstallSupported = true
				pkg.IsCask = false
				pkg = updateInstallInfo(pkg, info)
				packages = append(packages, pkg)
				for _, dep := range pkg.Dependencies {
					formulaDependents[dep] = append(formulaDependents[dep], pkg.Name)
				}
			} else {
				log.Printf("failed to retrieve infomation for %s/%s: %v", info.tap, info.name, err)
			}
		}
	}

	// Add formulae
	for _, f := range formulae {
		packages = append(packages, packageFromFormula(&f, formulaInstalls90d[f.Name], installedFormulae[f.Name]))
		for _, dep := range f.Dependencies {
			formulaDependents[dep] = append(formulaDependents[dep], f.Name)
		}
	}

	// Add casks
	for _, c := range casks {
		packages = append(packages, packageFromCask(&c, caskInstalls90d[c.Name], installedCasks[c.Name]))
		for _, dep := range c.Dependencies.Formulae {
			formulaDependents[dep] = append(formulaDependents[dep], c.Name)
		}
		for _, dep := range c.Dependencies.Casks {
			caskDependents[dep] = append(caskDependents[dep], c.Name)
		}
	}

	// Post processing: fetch release info and populate dependents
	for _, pkg := range packages {
		if *flagFetchReleaseInfo && pkg.IsInstalled {
			// Fetch release note in background as non blocking go routines
			go func() {
				pkg.ReleaseInfo = gh.GetGithubReleaseInfo(pkg)
			}()
		}
		if pkg.IsCask {
			pkg.Dependents = util.SortAndUniq(caskDependents[pkg.Name])
		} else {
			pkg.Dependents = util.SortAndUniq(formulaDependents[pkg.Name])
		}
	}

	// Sort all packages by name for faster lookups later.
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	return packages
}

func mapFormulaeInstalls(formulaAnalytics apiFormulaAnalytics) map[string]int {
	formulaInstalls := make(map[string]int)
	for _, item := range formulaAnalytics.Items {
		formulaInstalls[item.Name] = parseInstallCount(item.Count)
	}
	return formulaInstalls
}

func mapCaskInstalls(caskAnalytics apiCaskAnalytics) map[string]int {
	caskInstalls := make(map[string]int)
	for _, item := range caskAnalytics.Items {
		caskInstalls[item.Name] = parseInstallCount(item.Count)
	}
	return caskInstalls
}

func parseInstallCount(str string) int {
	str = strings.ReplaceAll(str, ",", "")
	count, _ := strconv.Atoi(str)
	return count
}

func mapInstallInfo(info []*installInfo) map[string]*installInfo {
	installedMap := make(map[string]*installInfo)
	for _, item := range info {
		installedMap[item.name] = item
	}
	return installedMap
}

func packageFromFormula(f *apiFormula, installs90d int, inst *installInfo) *data.Package {
	pkg := data.Package{
		Name:              f.Name,
		Tap:               f.Tap,
		Version:           f.Versions.Stable,
		Desc:              f.Desc,
		Homepage:          f.Homepage,
		Urls:              []string{f.Urls.Stable.Url, f.Urls.Head.Url},
		License:           f.License,
		Dependencies:      util.SortAndUniq(f.Dependencies),
		BuildDependencies: f.BuildDependencies,
		Conflicts:         f.Conflicts,
		Installs90d:       installs90d,
		IsDeprecated:      f.Deprecated,
		IsDisabled:        f.Disabled,
		InstallSupported:  true,
	}

	if inst != nil {
		return updateInstallInfo(&pkg, inst)
	} else {
		return &pkg
	}
}

func packageFromCask(c *apiCask, installs90d int, inst *installInfo) *data.Package {
	pkg := data.Package{
		Name:         c.Name,
		Tap:          c.Tap,
		Version:      c.Version,
		Desc:         c.Desc,
		Homepage:     c.Homepage,
		Urls:         []string{c.Url},
		License:      "N/A",
		Dependencies: util.SortAndUniq(append(c.Dependencies.Formulae, c.Dependencies.Casks...)),
		Conflicts:    util.SortAndUniq(append(c.Conflicts.Formulae, c.Conflicts.Casks...)),
		Installs90d:  installs90d,
		IsCask:       true,
		AutoUpdate:   c.AutoUpdate,
		IsDeprecated: c.Deprecated,
		IsDisabled:   c.Disabled,
	}

	url := c.Url
	// Trim query param from the url
	if i := strings.Index(url, "?"); i != -1 {
		url = url[:i]
	}
	// Don't support installing casks in pkg format as they require sudo
	pkg.InstallSupported = !strings.HasSuffix(url, ".pkg")

	if inst != nil {
		return updateInstallInfo(&pkg, inst)
	} else {
		return &pkg
	}
}

func updateInstallInfo(pkg *data.Package, inst *installInfo) *data.Package {
	pkg.IsInstalled = true
	if pkg.IsCask && pkg.AutoUpdate {
		// Cask has auto update (not managed by brew), assume it is up-to-date
		pkg.InstalledVersion = pkg.Version
		pkg.IsOutdated = false
	} else {
		pkg.InstalledVersion = inst.version
		pkg.IsOutdated = inst.version != pkg.Version
	}
	pkg.IsPinned = inst.pinned
	pkg.InstalledAsDependency = inst.asDep
	pkg.Size = inst.size
	pkg.FormattedSize = util.FormatSize(inst.size)
	pkg.InstalledDate = time.Unix(inst.timestamp, 0).Format(time.DateOnly)
	return pkg
}

func GetPackage(name string) *data.Package {
	// allBrewPackages is sorted by name
	index := sort.Search(len(allBrewPackages), func(i int) bool {
		return allBrewPackages[i].Name >= name
	})

	if index < len(allBrewPackages) && allBrewPackages[index].Name == name {
		return allBrewPackages[index]
	}

	return nil
}

func GetOutdatedPackages() []*data.Package {
	outdatedPackages := []*data.Package{}
	for i := range allBrewPackages {
		if pkg := allBrewPackages[i]; pkg.IsOutdated {
			outdatedPackages = append(outdatedPackages, pkg)
		}
	}
	return outdatedPackages
}

// Recursively find uninstalled dependencies
func GetRecursiveMissingDeps(pkgName string) []string {
	pkg := GetPackage(pkgName)
	if pkg.IsInstalled {
		return []string{}
	} else {
		deps := pkg.Dependencies
		for _, dep := range pkg.Dependencies {
			deps = append(deps, GetRecursiveMissingDeps(dep)...)
		}
		return deps
	}
}

// Recursively find installed dependents
func GetRecursiveInstalledDependents(pkgName string) []string {
	pkg := GetPackage(pkgName)
	if !pkg.IsInstalled {
		return []string{}
	} else {
		deps := pkg.Dependents
		for _, dep := range pkg.Dependents {
			deps = append(deps, GetRecursiveInstalledDependents(dep)...)
		}
		return deps
	}
}
