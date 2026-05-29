# Platform Support Feature Design

## Overview

Add platform/environment support information to Taproom, allowing users to see which platforms (macOS, Linux) and architectures (arm64, x86_64) a package supports, and filter packages by compatibility with their current system.

## Goals

1. Display platform support info in the details panel
2. Add a "Compatible" filter to show only packages that work on the current system
3. Parse platform data from Homebrew API for both formulae and casks

## Data Sources

### Formulae

Platform support comes from `bottle.stable.files` in the formula JSON:

```json
{
  "bottle": {
    "stable": {
      "files": {
        "arm64_sequoia": { ... },
        "arm64_sonoma": { ... },
        "sonoma": { ... },
        "arm64_linux": { ... },
        "x86_64_linux": { ... }
      }
    }
  }
}
```

### Casks

Platform support comes from `variations` keys, plus `depends_on.macos` for minimum version:

```json
{
  "depends_on": {
    "macos": { ">=": ["10.15"] }
  },
  "variations": {
    "arm64_linux": { ... },
    "x86_64_linux": { ... },
    "sequoia": { ... },
    "sonoma": { ... }
  }
}
```

## Data Model

### New Types (`internal/data/package.go`)

```go
type Platform struct {
    OS   string  // "macOS" or "Linux"
    Arch string  // "arm64" or "x86_64"
}
```

### Package Struct Additions

```go
type Package struct {
    // ... existing fields ...
    Platforms       []Platform  // Parsed platform support
    MinMacOSVersion string      // e.g., "10.15" - empty if no requirement
}
```

## Platform Key Parsing

Bottle/variation keys follow these patterns:

| Key Pattern | OS | Arch |
|-------------|-----|------|
| `arm64_sequoia`, `arm64_sonoma`, etc. | macOS | arm64 |
| `sequoia`, `sonoma`, `ventura`, etc. | macOS | x86_64 |
| `arm64_linux` | Linux | arm64 |
| `x86_64_linux` | Linux | x86_64 |

### Parsing Logic

```go
func parsePlatforms(keys []string) []Platform {
    platformSet := make(map[Platform]bool)

    for _, key := range keys {
        if strings.HasSuffix(key, "_linux") {
            if strings.HasPrefix(key, "arm64") {
                platformSet[Platform{"Linux", "arm64"}] = true
            } else {
                platformSet[Platform{"Linux", "x86_64"}] = true
            }
        } else {
            // macOS version names
            if strings.HasPrefix(key, "arm64_") {
                platformSet[Platform{"macOS", "arm64"}] = true
            } else {
                platformSet[Platform{"macOS", "x86_64"}] = true
            }
        }
    }
    // Convert set to slice
}
```

## API Struct Changes

### `internal/brew/api.go`

```go
type apiFormula struct {
    // ... existing fields ...
    Bottle struct {
        Stable struct {
            Files map[string]struct {
                Cellar string `json:"cellar"`
                Url    string `json:"url"`
                Sha256 string `json:"sha256"`
            } `json:"files"`
        } `json:"stable"`
    } `json:"bottle"`
}

type apiCask struct {
    // ... existing fields ...
    DependsOn struct {
        Macos struct {
            Gte []string `json:">="`
        } `json:"macos"`
    } `json:"depends_on"`
    Variations map[string]interface{} `json:"variations"`
}
```

## Filter Implementation

### System Detection (`internal/util/util.go`)

```go
func CurrentPlatform() Platform {
    os := "macOS"
    if runtime.GOOS == "linux" {
        os = "Linux"
    }

    arch := "x86_64"
    if runtime.GOARCH == "arm64" {
        arch = "arm64"
    }

    return Platform{OS: os, Arch: arch}
}
```

### Compatibility Check (`internal/data/package.go`)

```go
func (pkg *Package) IsCompatibleWith(platform Platform) bool {
    if len(pkg.Platforms) == 0 {
        return false  // No bottle info = exclude from filter
    }
    for _, p := range pkg.Platforms {
        if p.OS == platform.OS && p.Arch == platform.Arch {
            return true
        }
    }
    return false
}
```

### New Filter Option

Add "Compatible" to the filter sidebar. When active, show only packages where `IsCompatibleWith(CurrentPlatform())` returns true.

## Details Panel Display

### Format Method (`internal/data/package.go`)

```go
func (pkg *Package) PlatformString() string {
    if len(pkg.Platforms) == 0 {
        return ""  // Omit entirely if no data
    }

    // Group by OS
    macArch := []string{}
    linuxArch := []string{}

    for _, p := range pkg.Platforms {
        if p.OS == "macOS" {
            macArch = append(macArch, p.Arch)
        } else {
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
        parts = append(parts, "Linux (" + strings.Join(linuxArch, ", ") + ")")
    }

    return strings.Join(parts, ", ")
}
```

### Example Outputs

- `Platforms: macOS (arm64, x86_64), Linux (arm64, x86_64)`
- `Platforms: macOS (arm64, x86_64) requires 10.15+, Linux (arm64, x86_64)`
- `Platforms: macOS (arm64, x86_64)` (cask without Linux support)
- *(field omitted if no bottle data)*

## Files to Modify

| File | Changes |
|------|---------|
| `internal/brew/api.go` | Add `Bottle` to `apiFormula`, add `DependsOn`/`Variations` to `apiCask` |
| `internal/data/package.go` | Add `Platform` struct, `Platforms`, `MinMacOSVersion` fields, helper methods |
| `internal/brew/data.go` | Parse bottle/variations keys during data loading |
| `internal/ui/details.go` | Display platform info in details panel |
| `internal/ui/filter.go` | Add "Compatible" filter option |
| `internal/ui/filterview.go` | Render the new filter option |
| `internal/model/model.go` | Apply compatible filter logic |
| `internal/util/util.go` | Add `CurrentPlatform()` helper |

## Out of Scope (Future Work)

- Manual platform selection filters (macOS, Linux, ARM64, x86_64)
- Search prefix support (`p:linux`)
- Table column for platforms
- "Likely works on Linux" heuristics for casks without explicit Linux support

## Edge Cases

- **No bottle data**: Omit Platforms field from details, exclude from "Compatible" filter
- **Casks without variations**: Assume macOS-only (arm64 + x86_64)
- **Build-from-source formulae**: Treated as "no bottle data"
