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

// Platform represents a supported OS and architecture combination
type Platform struct {
	OS   string // "macOS" or "Linux"
	Arch string // "arm64" or "x86_64"
}

// Package holds all combined information for a formula or cask.
type Package struct {
	Name                  string // Used as a unique key
	Aliases               []string
	Tap                   string
	Version               string
	Revision              int
	InstalledVersion      string
	InstalledRevision     int
	Desc                  string
	Homepage              string
	Urls                  []string
	License               string
	Dependencies          []string
	BuildDependencies     []string
	Dependents            []string
	Conflicts             []string
	Installs90d           int
	AutoUpdate            bool
	IsCask                bool
	IsInstalled           bool
	IsOutdated            bool
	IsPinned              bool
	IsDeprecated          bool
	IsDisabled            bool
	InstalledAsDependency bool
	Size                  int64  // Size in kbs
	FormattedSize         string // Formated size like 24.5MB, 230KB
	InstallSupported      bool   // Whether installing the package is supported in taproom
	InstalledDate         string
	ReleaseInfo           *ReleaseInfo // Only set when package is outdated
	Platforms             []Platform   // Supported platforms parsed from bottles/variations
	MinMacOSVersion       string       // Minimum macOS version required (casks only)
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

func (pkg *Package) Status() string {
	if pkg.IsDisabled {
		return statusDisabled
	} else if pkg.IsDeprecated {
		return statusDeprecated
	} else if pkg.IsPinned {
		return statusPinned
	} else if pkg.IsOutdated {
		return statusOutdated
	} else if pkg.InstalledAsDependency {
		return statusInstalledAsDep
	} else if pkg.IsInstalled {
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
	if pkg.IsOutdated {
		return fmt.Sprintf("%s (New)", pkg.versionWithRev())
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pin)", pkg.installedVersionWithRev())
	} else {
		return pkg.versionWithRev()
	}
}

func (pkg *Package) LongVersion() string {
	if pkg.IsOutdated {
		return fmt.Sprintf("%s -> %s", pkg.installedVersionWithRev(), pkg.versionWithRev())
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pinned)", pkg.installedVersionWithRev())
	} else {
		return pkg.versionWithRev()
	}
}

func (pkg *Package) MarkInstalled() {
	pkg.IsInstalled = true
	pkg.IsOutdated = false
	pkg.InstalledVersion = pkg.Version
	pkg.InstalledDate = time.Now().Format(time.DateOnly)
}

func (pkg *Package) MarkInstalledAsDep() {
	pkg.MarkInstalled()
	pkg.InstalledAsDependency = true
}

func (pkg *Package) MarkUninstalled() {
	pkg.IsInstalled = false
	pkg.InstalledVersion = ""
	pkg.IsOutdated = false
	pkg.IsPinned = false
	pkg.InstalledAsDependency = false
}

func (pkg *Package) MarkPinned() {
	pkg.IsPinned = true
}

func (pkg *Package) MarkUnpinned() {
	pkg.IsPinned = false
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
