package data

import (
	"fmt"
	"strings"
	"time"
)

type ReleaseInfo struct {
	Date    time.Time
	Version string
	Url     string
}

type InstallStatus string

const (
	InstalledDep        InstallStatus = "installed_dep"
	ExplicitlyInstalled InstallStatus = "explicitly_installed"
	Uninstalled         InstallStatus = "uninstalled"
)

type UpdateStatus string

const (
	NotApplicable UpdateStatus = "not_applicable"
	UpToDate      UpdateStatus = "up_to_date"
	Outdated      UpdateStatus = "outdated"
	Pinned        UpdateStatus = "pinned"
)

type DeprecationStatus string

const (
	Deprecated DeprecationStatus = "deprecated"
	Deleted    DeprecationStatus = "deleted"
	Active     DeprecationStatus = "active"
)

// Package holds all combined information for a formula or cask.
type Package struct {
	Name              string // Used as a unique key
	Aliases           []string
	Tap               string
	Version           string
	Revision          int
	InstalledVersion  string
	InstalledRevision int
	Desc              string
	Homepage          string
	Urls              []string
	License           string
	Dependencies      []string
	BuildDependencies []string
	Dependents        []string
	Conflicts         []string
	Installs90d       int
	AutoUpdate        bool
	IsCask            bool
	InstallStatus     InstallStatus
	UpdateStatus      UpdateStatus
	DeprecationStatus DeprecationStatus
	Size              int64  // Size in kbs
	FormattedSize     string // Formated size like 24.5MB, 230KB
	InstallSupported  bool   // Whether installing the package is supported in taproom
	InstalledDate     string
	ReleaseInfo       *ReleaseInfo // Only set when package is outdated
}

const (
	formulaSymbol = ""
	caskSymbol    = ""
)

const (
	statusDisabled       = "Disabled"
	statusDeprecated     = "Deprecated"
	statusPinned         = "Pinned"
	statusOutdated       = "Outdated"
	statusInstalledAsDep = "Installed (Dep)"
	statusInstalled      = "Installed"
	statusUninstalled    = "Uninstalled"
)

func (pkg *Package) Symbol() string {
	if pkg.IsCask {
		return caskSymbol
	} else {
		return formulaSymbol
	}
}

// / A simplified status string for display
func (pkg *Package) DisplayStatus() string {
	if pkg.DeprecationStatus == Deleted {
		return statusDisabled
	} else if pkg.DeprecationStatus == Deprecated {
		return statusDeprecated
	} else if pkg.UpdateStatus == Pinned {
		return statusPinned
	} else if pkg.UpdateStatus == Outdated {
		return statusOutdated
	} else if pkg.InstallStatus == InstalledDep {
		return statusInstalledAsDep
	} else if pkg.InstallStatus == ExplicitlyInstalled {
		return statusInstalled
	} else {
		return statusUninstalled
	}
}

func (pkg *Package) BrewUrl() string {
	if pkg.IsCask {
		return fmt.Sprintf("https://formulae.brew.sh/cask/%s", pkg.Name)
	} else {
		return fmt.Sprintf("https://formulae.brew.sh/formula/%s", pkg.Name)
	}
}

func (pkg *Package) versionWithRev() string {
	if pkg.Revision > 0 {
		return fmt.Sprintf("%s_%d", pkg.Version, pkg.Revision)
	} else {
		return pkg.Version
	}
}

func (pkg *Package) installedVersionWithRev() string {
	if pkg.InstalledRevision > 0 {
		return fmt.Sprintf("%s_%d", pkg.InstalledVersion, pkg.InstalledRevision)
	} else {
		return pkg.InstalledVersion
	}
}

func (pkg *Package) ShortVersion() string {
	switch pkg.UpdateStatus {
	case Outdated:
		return fmt.Sprintf("%s (New)", pkg.versionWithRev())
	case Pinned:
		return fmt.Sprintf("%s (Pin)", pkg.installedVersionWithRev())
	default:
		return pkg.versionWithRev()
	}
}

func (pkg *Package) LongVersion() string {
	switch pkg.UpdateStatus {
	case Outdated:
		return fmt.Sprintf("%s -> %s", pkg.installedVersionWithRev(), pkg.versionWithRev())
	case Pinned:
		return fmt.Sprintf("%s (Pinned)", pkg.installedVersionWithRev())
	default:
		return pkg.versionWithRev()
	}
}

func (pkg *Package) MarkInstalled() {
	pkg.InstallStatus = ExplicitlyInstalled
	pkg.UpdateStatus = UpToDate
	pkg.InstalledVersion = pkg.Version
	pkg.InstalledDate = time.Now().Format(time.DateOnly)
}

func (pkg *Package) MarkInstalledAsDep() {
	pkg.MarkInstalled()
	pkg.InstallStatus = InstalledDep
}

func (pkg *Package) MarkUninstalled() {
	pkg.InstallStatus = Uninstalled
	pkg.InstalledVersion = ""
	pkg.UpdateStatus = NotApplicable
}

func (pkg *Package) MarkPinned() {
	pkg.UpdateStatus = Pinned
}

func (pkg *Package) MarkUnpinned() {
	if pkg.InstalledVersion != pkg.Version || pkg.InstalledRevision < pkg.Revision {
		pkg.UpdateStatus = Outdated
	} else {
		pkg.UpdateStatus = UpToDate
	}
}

const (
	negativeKwPrefix = "-"

	kwPrefixName     = "n:"
	kwPrefixDesc     = "d:"
	kwPrefixTap      = "t:"
	kwPrefixHomePage = "h:"
)

// Test if a package matches the keywords
func (pkg *Package) MatchKeywords(kws []string) bool {
	for _, kw := range kws {
		kw = strings.ToLower(kw)
		if kw, negative := strings.CutPrefix(kw, negativeKwPrefix); negative {
			if pkg.matchKeyword(kw) {
				return false
			}
		} else if !pkg.matchKeyword(kw) {
			return false
		}
	}
	return true
}

func (pkg *Package) matchKeyword(kw string) bool {
	if kw, hasPrefix := strings.CutPrefix(kw, kwPrefixName); hasPrefix {
		return pkg.matchKeywordInName(kw)
	} else if kw, hasPrefix := strings.CutPrefix(kw, kwPrefixDesc); hasPrefix {
		return pkg.matchKeywordInDesc(kw)
	} else if kw, hasPrefix := strings.CutPrefix(kw, kwPrefixTap); hasPrefix {
		return pkg.matchKeywordInTap(kw)
	} else if kw, hasPrefix := strings.CutPrefix(kw, kwPrefixHomePage); hasPrefix {
		return pkg.matchKeywordInHomePage(kw)
	}
	return pkg.matchKeywordInName(kw) || pkg.matchKeywordInDesc(kw)
}

func (pkg *Package) matchKeywordInName(kw string) bool {
	if strings.Contains(strings.ToLower(pkg.Name), kw) {
		return true
	} else {
		for _, alias := range pkg.Aliases {
			if strings.Contains(strings.ToLower(alias), kw) {
				return true
			}
		}
		return false
	}
}

func (pkg *Package) matchKeywordInDesc(kw string) bool {
	return strings.Contains(strings.ToLower(pkg.Desc), kw)
}

func (pkg *Package) matchKeywordInTap(kw string) bool {
	return strings.Contains(strings.ToLower(pkg.Tap), kw)
}

func (pkg *Package) matchKeywordInHomePage(kw string) bool {
	return strings.Contains(strings.ToLower(pkg.Homepage), kw)
}
