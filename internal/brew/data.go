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

var brewPrefix = func() string {
	brewPrefixBytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		log.Printf("failed to identify brew prefix: %+v\n", err)
		return ""
	}
	return strings.TrimSpace(string(brewPrefixBytes))
}()

const (
	apiFormulaURL          = "https://formulae.brew.sh/api/formula.json"
	apiCaskURL             = "https://formulae.brew.sh/api/cask.json"
	apiFormulaAnalyticsURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalyticsURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaCache           = "formula.json"
	casksCache             = "cask.json"
	formulaeAnalyticsCache = "formulae-analytics-90d.json"
	casksAnalyticsCache    = "casks-analytics-90d.json"

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
	Url          string `json:"url"`
	Dependencies struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"depends_on"`
	Conflicts struct {
		Formulae []string `json:"formula"`
		Casks    []string `json:"cask"`
	} `json:"conflicts_with"`
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
		formulaAnalyticsChan := make(chan apiFormulaAnalytics)
		caskAnalyticsChan := make(chan apiCaskAnalytics)
		installedChan := make(chan installedInfo)
		formulaSizesChan := make(chan map[string]int64)
		caskSizesChan := make(chan map[string]int64)
		errChan := make(chan error, 7)

		var allFormulae []apiFormula
		var allCasks []apiCask
		var formulaAnalytics apiFormulaAnalytics
		var caskAnalytics apiCaskAnalytics
		var allInstalled installedInfo
		var formulaSizes map[string]int64
		var caskSizes map[string]int64

		loadingTasksNum := cap(errChan)

		go fetchJsonWithCache(apiFormulaURL, formulaCache, &[]apiFormula{}, formulaeChan, errChan)
		loadingPrgs.AddTask(formulaeChan, "Loading all Formulae")
		go fetchJsonWithCache(apiCaskURL, casksCache, &[]apiCask{}, casksChan, errChan)
		loadingPrgs.AddTask(casksChan, "Loading all Casks")
		if fetchAnalytics {
			go fetchJsonWithCache(apiFormulaAnalyticsURL, formulaeAnalyticsCache, &apiFormulaAnalytics{}, formulaAnalyticsChan, errChan)
			loadingPrgs.AddTask(formulaAnalyticsChan, "Loading Formulae 90d analytics")
			go fetchJsonWithCache(apiCaskAnalyticsURL, casksAnalyticsCache, &apiCaskAnalytics{}, caskAnalyticsChan, errChan)
			loadingPrgs.AddTask(caskAnalyticsChan, "Loading Cask 90d analytics")
		} else {
			loadingTasksNum -= 2
			formulaAnalytics = apiFormulaAnalytics{}
			caskAnalytics = apiCaskAnalytics{}
		}
		if fetchSize {
			go fetchDirectorySizes(formulaSizesChan, errChan, fmt.Sprintf("%s/Cellar", brewPrefix), false)
			loadingPrgs.AddTask(formulaSizesChan, "Loading installed Formulae sizes")
			go fetchDirectorySizes(caskSizesChan, errChan, fmt.Sprintf("%s/Caskroom", brewPrefix), true)
			loadingPrgs.AddTask(caskSizesChan, "Loading installed Casks sizes")
		} else {
			loadingTasksNum -= 2
			formulaSizes = map[string]int64{}
			caskSizes = map[string]int64{}
		}
		go fetchInstalled(installedChan, errChan)
		loadingPrgs.AddTask(installedChan, "Loading installation data")

		for i := 0; i < loadingTasksNum; i++ {
			select {
			case f := <-formulaeChan:
				allFormulae = f
				loadingPrgs.MarkCompleted(formulaeChan)
			case c := <-casksChan:
				allCasks = c
				loadingPrgs.MarkCompleted(casksChan)
			case fa := <-formulaAnalyticsChan:
				formulaAnalytics = fa
				loadingPrgs.MarkCompleted(formulaAnalyticsChan)
			case ca := <-caskAnalyticsChan:
				caskAnalytics = ca
				loadingPrgs.MarkCompleted(caskAnalyticsChan)
			case inst := <-installedChan:
				allInstalled = inst
				loadingPrgs.MarkCompleted(installedChan)
			case sizes := <-formulaSizesChan:
				formulaSizes = sizes
				loadingPrgs.MarkCompleted(formulaSizesChan)
			case sizes := <-caskSizesChan:
				caskSizes = sizes
				loadingPrgs.MarkCompleted(caskSizesChan)
			case err := <-errChan:
				return DataLoadingErrMsg{err}
			}
		}

		allBrewPackages = processAllData(
			allInstalled,
			allFormulae,
			allCasks,
			formulaAnalytics,
			caskAnalytics,
			formulaSizes,
			caskSizes,
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

// fetchInstalled runs the `brew info` command and parses its Json output.
func fetchInstalled(installedChan chan installedInfo, errChan chan error) {
	var errOutput bytes.Buffer
	cmd := exec.Command("brew", "info", "--json=v2", "--installed")
	cmd.Stderr = &errOutput
	output, err := cmd.Output()
	if err != nil {
		errChan <- fmt.Errorf("failed to get installed packages: %s", errOutput.String())
		return
	}

	var info installedInfo
	if err := json.Unmarshal(output, &info); err != nil {
		errChan <- fmt.Errorf("failed to decode brew info json: %w", err)
		return
	}
	installedChan <- info
}

func fetchDirectorySizes(sizesChan chan map[string]int64, errChan chan error, dir string, followSymbolLinks bool) {
	// -k: output in KB
	// -d 1: output size for each direct sub-directories
	// -L: follow symbol links (which is used for Casks)
	args := []string{"-k", "-d", "1"}
	if followSymbolLinks {
		args = append(args, "-L")
	}
	args = append(args, dir)

	var errOutput bytes.Buffer
	cmd := exec.Command("du", args...)
	cmd.Stderr = &errOutput
	output, err := cmd.Output()

	if err == nil {
		sizesChan <- parseDuCmdOutput(output)
	} else {
		errChan <- fmt.Errorf("failed to get package sizes in %s\n%s", dir, errOutput.String())
	}
}

func fetchPackageSize(pkg *data.Package) int64 {
	if !pkg.IsInstalled {
		return 0
	}

	// -k: output in KB
	// -s: output the total size
	// -L: follow symbol links (which is used for Casks)
	args := []string{"-k", "-s"}
	dir := brewPrefix
	if pkg.IsCask {
		args = append(args, "-L")
		dir += "/Caskroom/"
	} else {
		dir += "/Cellar/"
	}
	dir += pkg.Name
	args = append(args, dir)

	cmd := exec.Command("du", args...)
	output, err := cmd.Output()

	if err == nil {
		return parseDuCmdOutput(output)[pkg.Name]
	}
	return 0
}

func parseDuCmdOutput(output []byte) map[string]int64 {
	sizes := make(map[string]int64)
	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			size, _ := strconv.ParseInt(fields[0], 10, 64)
			name := filepath.Base(fields[1])
			sizes[name] = size
		}
	}
	return sizes
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
) []*data.Package {
	formulaInstalls := mapFormulaeInstalls(formulaAnalytics)
	caskInstalls := mapCaskInstalls(caskAnalytics)

	formulaDependents := make(map[string][]string) // formula name to packages that depends on it
	caskDependents := make(map[string][]string)    // cask name to packages that depends on it
	installedFormulae := make(map[string]struct{}) // track installed formulae to avoid duplicate
	installedCasks := make(map[string]struct{})    // track installed casks to avoid duplicate

	packages := make([]*data.Package, 0, len(installed.Formulae)+len(installed.Casks)+len(formulae)+len(casks))
	// Process installed formulae
	for _, f := range installed.Formulae {
		packages = append(packages, packageFromFormula(&f, formulaInstalls[f.Name], true, formulaSizes[f.Name]))
		installedFormulae[f.Name] = struct{}{}
		for _, dep := range f.Dependencies {
			formulaDependents[dep] = append(formulaDependents[dep], f.Name)
		}
	}
	// Process installed casks
	for _, c := range installed.Casks {
		packages = append(packages, packageFromCask(&c, caskInstalls[c.Name], true, caskSizes[c.Name]))
		installedCasks[c.Name] = struct{}{}
		for _, dep := range c.Dependencies.Formulae {
			formulaDependents[dep] = append(formulaDependents[dep], c.Name)
		}
		for _, dep := range c.Dependencies.Casks {
			caskDependents[dep] = append(caskDependents[dep], c.Name)
		}
	}
	// Add formulaes to packages, except for installed formulae
	for _, f := range formulae {
		if _, installed := installedFormulae[f.Name]; !installed {
			packages = append(packages, packageFromFormula(&f, formulaInstalls[f.Name], false, 0))
			for _, dep := range f.Dependencies {
				formulaDependents[dep] = append(formulaDependents[dep], f.Name)
			}
		}
	}
	// Add casks to packages, except for installed casks
	for _, c := range casks {
		if _, installed := installedCasks[c.Name]; !installed {
			packages = append(packages, packageFromCask(&c, caskInstalls[c.Name], false, 0))
			for _, dep := range c.Dependencies.Formulae {
				formulaDependents[dep] = append(formulaDependents[dep], c.Name)
			}
			for _, dep := range c.Dependencies.Casks {
				caskDependents[dep] = append(caskDependents[dep], c.Name)
			}
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

func packageFromFormula(f *apiFormula, installs int, installed bool, installedSize int64) *data.Package {
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
		InstallCount90d:   installs,
		IsCask:            false,
		IsDeprecated:      f.Deprecated,
		IsDisabled:        f.Disabled,
		InstallSupported:  true,
	}
	if installed {
		inst := f.Installed[0]
		pkg.IsInstalled = true
		pkg.InstalledVersion = inst.Version
		pkg.IsOutdated = f.Outdated
		pkg.IsPinned = f.Pinned
		pkg.InstalledAsDependency = inst.InstalledAsDep
		pkg.Size = installedSize
		pkg.FormattedSize = util.FormatSize(installedSize)
		pkg.InstalledDate = time.Unix(inst.Time, 0).Format(time.DateOnly)
	}

	return &pkg
}

func packageFromCask(c *apiCask, installs int, installed bool, installedSize int64) *data.Package {
	pkg := data.Package{
		Name:            c.Name,
		Tap:             c.Tap,
		Version:         c.Version,
		Desc:            c.Desc,
		Homepage:        c.Homepage,
		Urls:            []string{c.Url},
		License:         "N/A",
		Dependencies:    util.SortAndUniq(append(c.Dependencies.Formulae, c.Dependencies.Casks...)),
		Conflicts:       util.SortAndUniq(append(c.Conflicts.Formulae, c.Conflicts.Casks...)),
		InstallCount90d: installs,
		IsCask:          true,
		IsDeprecated:    c.Deprecated,
		IsDisabled:      c.Disabled,
	}

	url := c.Url
	// Trim query param from the url
	if i := strings.Index(url, "?"); i != -1 {
		url = url[:i]
	}
	// Don't support installing casks in pkg format as they require sudo
	pkg.InstallSupported = !strings.HasSuffix(url, ".pkg")

	if installed {
		pkg.IsInstalled = true
		pkg.InstalledVersion = c.InstalledVersion
		pkg.IsOutdated = c.Outdated
		// Casks can't be pinned or installed as dependencies
		pkg.IsPinned = false
		pkg.InstalledAsDependency = false
		pkg.Size = installedSize
		pkg.FormattedSize = util.FormatSize(installedSize)
		pkg.InstalledDate = time.Unix(c.InstalledTime, 0).Format(time.DateOnly)
	}

	return &pkg
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
