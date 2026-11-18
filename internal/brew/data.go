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
		packagesChan := make(chan internalPackages)
		formulaAnalytics90dChan := make(chan apiFormulaAnalytics)
		caskAnalytics90dChan := make(chan apiCaskAnalytics)
		formulaInstallInfoChan := make(chan []*installInfo)
		caskInstallInfoChan := make(chan []*installInfo)
		loadingTasksNum := 5
		errChan := make(chan error, loadingTasksNum)

		var pkgs internalPackages
		var formulaAnalytics90d apiFormulaAnalytics
		var caskAnalytics90d apiCaskAnalytics
		var formulaInstallInfo, caskInstallInfo []*installInfo

		go fetchInternalPackages(packagesChan, errChan)
		loadingPrgs.AddTask(packagesChan, "Loading all Packages")
		if fetchAnalytics {
			go fetchFormulaAnalytics(formulaAnalytics90dChan, errChan)
			loadingPrgs.AddTask(formulaAnalytics90dChan, "Loading Formulae 90d analytics")
			go fetchCaskAnalytics(caskAnalytics90dChan, errChan)
			loadingPrgs.AddTask(caskAnalytics90dChan, "Loading Cask 90d analytics")
		} else {
			loadingTasksNum -= 2
		}
		go fetchInstalledFormula(fetchSize, formulaInstallInfoChan)
		loadingPrgs.AddTask(formulaInstallInfoChan, "Loading formulae installation data")
		go fetchInstalledCask(fetchSize, caskInstallInfoChan)
		loadingPrgs.AddTask(caskInstallInfoChan, "Loading casks installation data")

		// Update brew in the background, we don't depend on `brew` command to get data
		// But we need brew to be updated when install/upgrade packages
		go updateBrew()

		for range loadingTasksNum {
			select {
			case pkgs = <-packagesChan:
				loadingPrgs.MarkCompleted(packagesChan)
			case formulaAnalytics90d = <-formulaAnalytics90dChan:
				loadingPrgs.MarkCompleted(formulaAnalytics90dChan)
			case caskAnalytics90d = <-caskAnalytics90dChan:
				loadingPrgs.MarkCompleted(caskAnalytics90dChan)
			case formulaInstallInfo = <-formulaInstallInfoChan:
				loadingPrgs.MarkCompleted(formulaInstallInfoChan)
			case caskInstallInfo = <-caskInstallInfoChan:
				loadingPrgs.MarkCompleted(caskInstallInfoChan)
			case err := <-errChan:
				return DataLoadingErrMsg{err}
			}
		}

		allBrewPackages = processAllData(
			pkgs,
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
	pkgs internalPackages,
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

	// Map aliases
	aliasesMap := make(map[string][]string)
	for alias, canonical := range pkgs.FormulaAliases {
		aliasesMap[canonical] = append(aliasesMap[canonical], alias)
	}

	for _, info := range formulaInstallInfo {
		if info.tap == coreTap {
			continue
		}
		// Add formulae from third-party taps, since they're not in formula.json
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
			log.Printf("failed to retrieve information for %s/%s: %v", info.tap, info.name, err)
		}
	}

	for _, info := range caskInstallInfo {
		if info.tap == caskTap {
			continue
		}
		// Add casks from third-party taps, since they're not in cask.json
		pkg, err := getCustomTapPackage(info)
		if err == nil {
			pkg.Installs90d = formulaInstalls90d[pkg.Name]
			pkg.IsCask = true
			pkg.InstallSupported = len(pkg.Urls) > 0 && isInstallSupported(pkg.Urls[0])
			pkg = updateInstallInfo(pkg, info)
			packages = append(packages, pkg)
		} else {
			log.Printf("failed to retrieve information for %s/%s: %v", info.tap, info.name, err)
		}
	}

	// Add formulae
	for name, f := range pkgs.Formulae {
		aliases := aliasesMap[name]
		pkg := packageFromFormula(name, aliases, f, formulaInstalls90d[name], installedFormulae[name])
		packages = append(packages, pkg)
		for _, dep := range pkg.Dependencies {
			formulaDependents[dep] = append(formulaDependents[dep], name)
		}
	}

	// Add casks
	for name, c := range pkgs.Casks {
		pkg := packageFromCask(name, c, caskInstalls90d[name], installedCasks[name])
		packages = append(packages, pkg)
		for _, dep := range c.DependsOnArgs.Formula {
			formulaDependents[dep] = append(formulaDependents[dep], name)
		}
		for _, dep := range c.DependsOnArgs.Cask {
			caskDependents[dep] = append(caskDependents[dep], name)
		}
	}

	// Post processing: fetch release info and populate dependents
	for _, pkg := range packages {
		if *flagFetchReleaseInfo && pkg.InstallStatus != data.Uninstalled {
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

func packageFromFormula(name string, aliases []string, f *apiFormula, installs90d int, inst *installInfo) *data.Package {
	var deps []string
	var buildDeps []string
	for _, dep := range f.StableDependencies {
		if dep.IsBuild {
			buildDeps = append(buildDeps, dep.Name)
		} else if dep.IsRuntime {
			deps = append(deps, dep.Name)
		}
	}

	var urls []string
	if len(f.StableUrlArgs) > 0 {
		urls = append(urls, f.StableUrlArgs[0])
	}
	if len(f.HeadUrlArgs) > 0 {
		urls = append(urls, f.HeadUrlArgs[0])
	}

	allAliases := util.SortAndUniq(append(aliases, f.Aliases...))

	pkg := data.Package{
		Name:              name,
		Aliases:           allAliases,
		Tap:               "homebrew/core",
		Version:           f.StableVersion,
		Revision:          f.Revision,
		Desc:              f.Desc,
		Homepage:          f.Homepage,
		Urls:              urls,
		License:           string(f.License),
		Dependencies:      util.Sort(deps),
		BuildDependencies: util.Sort(buildDeps),
		Conflicts:         util.Sort(f.Conflicts),
		Installs90d:       installs90d,
		DeprecationStatus: func() data.DeprecationStatus {
			if f.DisableArgs != nil {
				return data.Deleted
			}
			if f.DeprecateArgs != nil {
				return data.Deprecated
			}
			return data.Active
		}(),
		InstallSupported: true,
		InstallStatus:    data.Uninstalled,
		UpdateStatus:     data.NotApplicable,
	}

	if inst != nil {
		return updateInstallInfo(&pkg, inst)
	} else {
		return &pkg
	}
}

func packageFromCask(name string, c *apiCask, installs90d int, inst *installInfo) *data.Package {
	pkg := data.Package{
		Name:         name,
		Tap:          "homebrew/cask",
		Version:      c.Version,
		Desc:         c.Desc,
		Homepage:     c.Homepage,
		Urls:         c.UrlArgs,
		License:      "N/A",
		Dependencies: util.Sort(append(c.DependsOnArgs.Formula, c.DependsOnArgs.Cask...)),
		Conflicts:    util.Sort(append(c.Conflicts.Formula, c.Conflicts.Cask...)),
		Installs90d:  installs90d,
		IsCask:       true,
		InstallSupported: func() bool {
			if len(c.UrlArgs) > 0 {
				return isInstallSupported(c.UrlArgs[0])
			}
			return false
		}(),
		AutoUpdate:   c.AutoUpdates,
		DeprecationStatus: func() data.DeprecationStatus {
			if c.DisableArgs != nil {
				return data.Deleted
			}
			if c.DeprecateArgs != nil {
				return data.Deprecated
			}
			return data.Active
		}(),
		InstallStatus: data.Uninstalled,
		UpdateStatus:  data.NotApplicable,
	}

	if inst != nil {
		return updateInstallInfo(&pkg, inst)
	} else {
		return &pkg
	}
}

func isInstallSupported(url string) bool {
	// Trim query param from the url
	if i := strings.Index(url, "?"); i != -1 {
		url = url[:i]
	}
	// Don't support installing casks in pkg format as they require sudo
	return !strings.HasSuffix(url, ".pkg")
}

func updateInstallInfo(pkg *data.Package, inst *installInfo) *data.Package {
	if inst.asDep {
		pkg.InstallStatus = data.InstalledDep
	} else {
		pkg.InstallStatus = data.ExplicitlyInstalled
	}

	if pkg.IsCask && pkg.AutoUpdate {
		// Cask has auto update (not managed by brew), assume it is up-to-date
		pkg.InstalledVersion = pkg.Version
		pkg.UpdateStatus = data.UpToDate
	} else {
		pkg.InstalledVersion = inst.version
		pkg.InstalledRevision = inst.revision
		if inst.pinned {
			pkg.UpdateStatus = data.Pinned
		} else if inst.version != pkg.Version || inst.revision < pkg.Revision {
			pkg.UpdateStatus = data.Outdated
		} else {
			pkg.UpdateStatus = data.UpToDate
		}
	}
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
		if pkg := allBrewPackages[i]; pkg.UpdateStatus == data.Outdated {
			outdatedPackages = append(outdatedPackages, pkg)
		}
	}
	return outdatedPackages
}

// Recursively find uninstalled dependencies
func GetRecursiveMissingDeps(pkgName string) []string {
	pkg := GetPackage(pkgName)
	if pkg.InstallStatus != data.Uninstalled {
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
	if pkg.InstallStatus == data.Uninstalled {
		return []string{}
	} else {
		deps := pkg.Dependents
		for _, dep := range pkg.Dependents {
			deps = append(deps, GetRecursiveInstalledDependents(dep)...)
		}
		return deps
	}
}
