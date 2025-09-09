package brew

import (
	"bytes"
	"log"
	"os/exec"
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

var flagFetchReleaseInfo = pflag.Bool("fetch-release", false, "Fetching release data for installed packages")

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

		go fetchFormula(&allFormulae, formulaeChan, errChan)
		loadingPrgs.AddTask(formulaeChan, "Loading all Formulae")
		go fetchCask(&allCasks, casksChan, errChan)
		loadingPrgs.AddTask(casksChan, "Loading all Casks")
		if fetchAnalytics {
			go fetchFormulaAnalytics(&formulaAnalytics90d, formulaAnalytics90dChan, errChan)
			loadingPrgs.AddTask(formulaAnalytics90dChan, "Loading Formulae 90d analytics")
			go fetchCaskAnalytics(&caskAnalytics90d, caskAnalytics90dChan, errChan)
			loadingPrgs.AddTask(caskAnalytics90dChan, "Loading Cask 90d analytics")
		} else {
			loadingTasksNum -= 2
		}
		go fetchInstalledFormula(fetchSize, formulaInstallInfoChan, errChan)
		loadingPrgs.AddTask(formulaInstallInfoChan, "Loading formulae installation data")
		go fetchInstalledCask(fetchSize, caskInstallInfoChan, errChan)
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

	// Add formulae from custom taps, since they're not in formula.json
	for _, info := range formulaInstallInfo {
		if info.tap != coreTap {
			pkg, err := getCustomTapPackage(info)
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
		Revision:          f.Revision,
		Desc:              f.Desc,
		Homepage:          f.Homepage,
		Urls:              []string{f.Urls.Stable.Url, f.Urls.Head.Url},
		License:           f.License,
		Dependencies:      util.Sort(f.Dependencies),
		BuildDependencies: util.Sort(f.BuildDependencies),
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
		Dependencies: util.Sort(append(c.Dependencies.Formulae, c.Dependencies.Casks...)),
		Conflicts:    util.Sort(append(c.Conflicts.Formulae, c.Conflicts.Casks...)),
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
		pkg.InstalledRevision = inst.revision
		pkg.IsOutdated = inst.version != pkg.Version || inst.revision < pkg.Revision
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
