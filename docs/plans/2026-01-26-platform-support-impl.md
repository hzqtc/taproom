# Platform Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add platform/environment support info to Taproom with details panel display and "Compatible" filter.

**Architecture:** Parse bottle files (formulae) and variations (casks) from Homebrew API to extract supported platforms. Add Platform type to data model, display in details panel, and add filter to show only packages compatible with current system.

**Tech Stack:** Go 1.24, Bubble Tea TUI framework, Homebrew API

---

### Task 1: Add Platform Type and Package Fields

**Files:**
- Modify: `internal/data/package.go:1-46`

**Step 1: Add Platform type and Package fields**

Add after line 13 (after ReleaseInfo struct):

```go
// Platform represents a supported OS and architecture combination
type Platform struct {
	OS   string // "macOS" or "Linux"
	Arch string // "arm64" or "x86_64"
}
```

Add new fields to Package struct (after line 45, before the closing brace):

```go
	Platforms       []Platform // Supported platforms parsed from bottles/variations
	MinMacOSVersion string     // Minimum macOS version required (casks only)
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds with no errors

**Step 3: Commit**

```bash
git add internal/data/package.go
git commit -m "feat(data): add Platform type and package fields for platform support"
```

---

### Task 2: Add Platform Helper Methods

**Files:**
- Modify: `internal/data/package.go`

**Step 1: Add PlatformString method**

Add at end of file:

```go
// PlatformString returns a human-readable string of supported platforms
func (pkg *Package) PlatformString() string {
	if len(pkg.Platforms) == 0 {
		return ""
	}

	macArch := []string{}
	linuxArch := []string{}

	for _, p := range pkg.Platforms {
		if p.OS == "macOS" {
			macArch = append(macArch, p.Arch)
		} else if p.OS == "Linux" {
			linuxArch = append(linuxArch, p.Arch)
		}
	}

	parts := []string{}
	if len(macArch) > 0 {
		s := "macOS (" + strings.Join(macArch, ", ") + ")"
		if pkg.MinMacOSVersion != "" {
			s += " requires " + pkg.MinMacOSVersion + "+"
		}
		parts = append(parts, s)
	}
	if len(linuxArch) > 0 {
		parts = append(parts, "Linux ("+strings.Join(linuxArch, ", ")+")")
	}

	return strings.Join(parts, ", ")
}

// IsCompatibleWith checks if the package supports the given platform
func (pkg *Package) IsCompatibleWith(platform Platform) bool {
	if len(pkg.Platforms) == 0 {
		return false
	}
	for _, p := range pkg.Platforms {
		if p.OS == platform.OS && p.Arch == platform.Arch {
			return true
		}
	}
	return false
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/data/package.go
git commit -m "feat(data): add PlatformString and IsCompatibleWith methods"
```

---

### Task 3: Add CurrentPlatform Helper

**Files:**
- Modify: `internal/util/util.go`

**Step 1: Add runtime import and CurrentPlatform function**

Add `"runtime"` to imports, then add at end of file:

```go
// CurrentPlatform returns the current system's platform
func CurrentPlatform() (string, string) {
	os := "macOS"
	if runtime.GOOS == "linux" {
		os = "Linux"
	}

	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	return os, arch
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/util/util.go
git commit -m "feat(util): add CurrentPlatform helper for system detection"
```

---

### Task 4: Update API Structs for Bottle/Variations Parsing

**Files:**
- Modify: `internal/brew/api.go:55-99`

**Step 1: Update apiFormula struct**

Replace the apiFormula struct (lines 55-79) with:

```go
type apiFormula struct {
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases"`
	Tap      string   `json:"tap"`
	Desc     string   `json:"desc"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Revision int    `json:"revision"`
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
	Bottle            struct {
		Stable struct {
			Files map[string]struct {
				Cellar string `json:"cellar"`
				Url    string `json:"url"`
				Sha256 string `json:"sha256"`
			} `json:"files"`
		} `json:"stable"`
	} `json:"bottle"`
}
```

**Step 2: Update apiCask struct**

Replace the apiCask struct (lines 81-99) with:

```go
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
	MacOSReq   struct {
		Gte []string `json:">="`
	} `json:"macos"`
	Variations map[string]interface{} `json:"variations"`
}
```

**Step 3: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/brew/api.go
git commit -m "feat(api): add Bottle and Variations fields to API structs"
```

---

### Task 5: Add Platform Parsing Functions

**Files:**
- Modify: `internal/brew/data.go`

**Step 1: Add parsePlatformsFromBottle function**

Add after the imports section (after line 18):

```go
// parsePlatformsFromBottle extracts platforms from formula bottle file keys
func parsePlatformsFromBottle(files map[string]struct {
	Cellar string `json:"cellar"`
	Url    string `json:"url"`
	Sha256 string `json:"sha256"`
}) []data.Platform {
	platformSet := make(map[data.Platform]bool)

	for key := range files {
		if strings.HasSuffix(key, "_linux") {
			if strings.HasPrefix(key, "arm64") {
				platformSet[data.Platform{OS: "Linux", Arch: "arm64"}] = true
			} else {
				platformSet[data.Platform{OS: "Linux", Arch: "x86_64"}] = true
			}
		} else {
			// macOS version names (sequoia, sonoma, etc.)
			if strings.HasPrefix(key, "arm64_") {
				platformSet[data.Platform{OS: "macOS", Arch: "arm64"}] = true
			} else {
				platformSet[data.Platform{OS: "macOS", Arch: "x86_64"}] = true
			}
		}
	}

	platforms := make([]data.Platform, 0, len(platformSet))
	for p := range platformSet {
		platforms = append(platforms, p)
	}
	return platforms
}

// parsePlatformsFromVariations extracts platforms from cask variations keys
func parsePlatformsFromVariations(variations map[string]interface{}) []data.Platform {
	platformSet := make(map[data.Platform]bool)

	// Default: casks support macOS
	hasMacOS := false
	hasLinux := false

	for key := range variations {
		if strings.HasSuffix(key, "_linux") {
			hasLinux = true
			if strings.HasPrefix(key, "arm64") {
				platformSet[data.Platform{OS: "Linux", Arch: "arm64"}] = true
			} else {
				platformSet[data.Platform{OS: "Linux", Arch: "x86_64"}] = true
			}
		} else {
			// macOS version names
			hasMacOS = true
			if strings.HasPrefix(key, "arm64_") {
				platformSet[data.Platform{OS: "macOS", Arch: "arm64"}] = true
			} else {
				platformSet[data.Platform{OS: "macOS", Arch: "x86_64"}] = true
			}
		}
	}

	// If no variations, assume macOS with both architectures
	if !hasMacOS && !hasLinux {
		platformSet[data.Platform{OS: "macOS", Arch: "arm64"}] = true
		platformSet[data.Platform{OS: "macOS", Arch: "x86_64"}] = true
	}

	platforms := make([]data.Platform, 0, len(platformSet))
	for p := range platformSet {
		platforms = append(platforms, p)
	}
	return platforms
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/brew/data.go
git commit -m "feat(data): add platform parsing functions for bottles and variations"
```

---

### Task 6: Integrate Platform Parsing into Package Creation

**Files:**
- Modify: `internal/brew/data.go:239-290`

**Step 1: Update packageFromFormula function**

In `packageFromFormula` function (around line 239), add platform parsing. After line 257 (after `InstallSupported: true,`), add:

```go
		Platforms: parsePlatformsFromBottle(f.Bottle.Stable.Files),
```

**Step 2: Update packageFromCask function**

In `packageFromCask` function (around line 266), add platform and minMacOS parsing. After line 283 (after `IsDisabled: c.Disabled,`), add:

```go
		Platforms:       parsePlatformsFromVariations(c.Variations),
		MinMacOSVersion: getMinMacOSVersion(c.MacOSReq.Gte),
```

**Step 3: Add getMinMacOSVersion helper**

Add after the parsing functions:

```go
// getMinMacOSVersion extracts the minimum macOS version from requirements
func getMinMacOSVersion(versions []string) string {
	if len(versions) > 0 {
		return versions[0]
	}
	return ""
}
```

**Step 4: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/brew/data.go
git commit -m "feat(data): integrate platform parsing into package creation"
```

---

### Task 7: Display Platforms in Details Panel

**Files:**
- Modify: `internal/ui/details.go:120-205`

**Step 1: Add platform display to updatePanel**

In the `updatePanel` method, add platform info display. After line 133 (after `Installs (90d)` line), add:

```go
	if platformStr := m.pkg.PlatformString(); platformStr != "" {
		b.WriteString(fmt.Sprintf("Platforms: %s\n", platformStr))
	}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/ui/details.go
git commit -m "feat(ui): display platform support in details panel"
```

---

### Task 8: Add Compatible Filter Constant

**Files:**
- Modify: `internal/ui/filter.go:19-30`

**Step 1: Add FilterCompatible constant**

Update the filter constants (around line 21). Add after `FilterActive`:

```go
	FilterCompatible                             // 0100 0000
```

**Step 2: Update conflictFilters to include Compatible in the second group**

Update line 36 to include FilterCompatible:

```go
	filterGroup(FilterInstalled | FilterOutdated | FilterExplicitlyInstalled | FilterActive | FilterCompatible),
```

**Step 3: Update String method**

Add case in the `String()` method (around line 101):

```go
	case FilterCompatible:
		return "Compatible"
```

**Step 4: Update parseFilter function**

Add case in `parseFilter()` (around line 120):

```go
	case "Compatible":
		return FilterCompatible, nil
```

**Step 5: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/ui/filter.go
git commit -m "feat(filter): add FilterCompatible constant"
```

---

### Task 9: Add Compatible Filter Key Binding

**Files:**
- Modify: `internal/ui/filterview.go:13-53`

**Step 1: Add filterCompatible key binding field**

Add after line 23 (after `filterActive key.Binding`):

```go
	filterCompatible key.Binding
```

**Step 2: Initialize the key binding**

In `NewFilterViewModel()`, add after line 52 (after `filterActive` initialization):

```go
		filterCompatible: key.NewBinding(key.WithKeys("m")),
```

**Step 3: Handle the key in Update method**

In the `Update` method, add a case after line 74 (after the `filterActive` case):

```go
		case key.Matches(msg, m.filterCompatible):
			m.fg.toggleFilter(FilterCompatible)
```

**Step 4: Update flagFilters help text**

Update line 31 to include Compatible:

```go
		"Pick 0 or 1 filter from each group: (Formulae, Casks), (Installed, Outdated, Expl. Installed, Active, Compatible)",
```

**Step 5: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/ui/filterview.go
git commit -m "feat(ui): add Compatible filter key binding (m key)"
```

---

### Task 10: Implement Compatible Filter Logic

**Files:**
- Modify: `internal/model/model.go:262-305`

**Step 1: Add data import**

Add `"taproom/internal/util"` to imports if not already present.

**Step 2: Update filterPackages to handle Compatible filter**

In the `filterPackages` method, add a case for FilterCompatible. After line 290 (after the `FilterActive` case), add:

```go
			case ui.FilterCompatible:
				os, arch := util.CurrentPlatform()
				currentPlatform := data.Platform{OS: os, Arch: arch}
				passesFilter = pkg.IsCompatibleWith(currentPlatform)
```

**Step 3: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/model/model.go
git commit -m "feat(model): implement Compatible filter using current platform"
```

---

### Task 11: Update Help Text

**Files:**
- Modify: `internal/ui/help.go` (check for filter help)

**Step 1: Check if help.go has filter documentation**

Read the file to see if filter keys are documented.

**Step 2: If filter keys are documented, add 'm' for Compatible**

Add `m Compatible` to the filter key list if it exists.

**Step 3: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit (if changes made)**

```bash
git add internal/ui/help.go
git commit -m "docs(help): add Compatible filter to help text"
```

---

### Task 12: Test the Implementation

**Step 1: Run existing tests**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Build and run manually**

Run: `go build -o taproom . && ./taproom`
Expected: App launches successfully

**Step 3: Verify platform display**

- Select a package (e.g., ripgrep)
- Check details panel shows "Platforms: macOS (arm64, x86_64), Linux (arm64, x86_64)"

**Step 4: Verify Compatible filter**

- Press 'm' to enable Compatible filter
- Verify filter shows "Compatible" in filter view
- Verify packages are filtered to current platform

**Step 5: Commit any fixes if needed**

---

### Task 13: Final Commit and PR Preparation

**Step 1: Run final tests**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Run linting if available**

Run: `go vet ./...`
Expected: No issues

**Step 3: Review all changes**

Run: `git diff main...HEAD --stat`

**Step 4: Create final summary commit if needed**

If any cleanup is needed, commit it.

**Step 5: Push branch**

```bash
git push -u origin feature/platform-support
```

**Step 6: Create PR**

Create PR with title: "feat: Add platform support display and Compatible filter"

Body should include:
- Summary of changes
- Screenshots if possible
- Test instructions
