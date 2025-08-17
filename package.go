package main

import (
	"fmt"
	"strings"
	"time"
)

// Package holds all combined information for a formula or cask.
type Package struct {
	Name                  string // Used as a unique key
	Tap                   string
	Version               string
	InstalledVersion      string
	Desc                  string
	Homepage              string
	Urls                  []string
	License               string
	Dependencies          []string
	BuildDependencies     []string
	Dependents            []string
	Conflicts             []string
	InstallCount90d       int
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
	ReleaseInfo           *ReleaseNote // Only set when package is outdated
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

const (
	negativeKwPrefix = "-"

	kwPrefixName = "n:"
	kwPrefixDesc = "d:"
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

func (pkg *Package) ShortVersion() string {
	if pkg.IsOutdated {
		return fmt.Sprintf("%s (New)", pkg.Version)
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
	} else {
		return pkg.Version
	}
}

func (pkg *Package) LongVersion() string {
	if pkg.IsOutdated {
		return fmt.Sprintf("%s -> %s", pkg.InstalledVersion, pkg.Version)
	} else if pkg.IsPinned {
		return fmt.Sprintf("%s (Pinned)", pkg.InstalledVersion)
	} else {
		return pkg.Version
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

// Test if a package matches the keywords
func (pkg *Package) MatchKeywords(kws []string) bool {
	for _, kw := range kws {
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
	if kw, match := strings.CutPrefix(kw, kwPrefixName); match {
		return strings.Contains(strings.ToLower(pkg.Name), kw)
	} else if kw, match := strings.CutPrefix(kw, kwPrefixDesc); match {
		return strings.Contains(strings.ToLower(pkg.Desc), kw)
	} else {
		return strings.Contains(strings.ToLower(pkg.Name), kw) ||
			strings.Contains(strings.ToLower(pkg.Desc), kw) ||
			strings.Contains(strings.ToLower(pkg.Homepage), kw)
	}
}
