package main

import (
	"fmt"
	"strings"
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
	Conflicts             []string
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

const (
	statusDisabled       = "Disabled"
	statusDeprecated     = "Deprecated"
	statusPinned         = "Pinned"
	statusOutdated       = "Outdated"
	statusInstalledAsDep = "Installed (Dep)"
	statusInstalled      = "Installed"
	statusUninstalled    = "Uninstalled"
)

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
		// Requires the name or description to contain all keywords
		// So we can return false on any unmatched keyword
		if !strings.Contains(strings.ToLower(pkg.Name), kw) && !strings.Contains(strings.ToLower(pkg.Desc), kw) {
			return false
		}
	}
	return true
}
