package brew

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"taproom/internal/data"
)

const coreTap = "homebrew/core"

var (
	versionRegex = regexp.MustCompile(`v?(\d+(?:\.\d+)+[a-zA-Z0-9\-\.]*)`)
	sourceExts   = []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip"}
)

// Get a package from locally cloned custom tap data (*.rb files)
// Ideally this should be called after `brew update`
func getCustomTapPackage(info *installInfo) (*data.Package, error) {
	pkg := data.Package{
		Name: info.name,
		Tap:  info.tap,
	}

	// This reads the .rb file located in /opt/homebrew/Library/Taps/
	data, err := os.ReadFile(info.path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", info.path, err)
	}
	content := string(data)

	// Version
	if m := regexp.MustCompile(`version\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Version = m[1]
	}
	if m := regexp.MustCompile(`tag:\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Version = normalizeVersion(m[1])
	}

	// Revision
	if m := regexp.MustCompile(`revision\s+([0-9]+)`).FindStringSubmatch(content); m != nil {
		pkg.Revision, _ = strconv.Atoi(m[1])
	}

	// Desc
	if m := regexp.MustCompile(`desc\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Desc = m[1]
	}

	// Homepage
	if m := regexp.MustCompile(`homepage\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Homepage = m[1]
	}

	// Urls
	urlRe := regexp.MustCompile(`url\s+["']([^"']+)["']`)
	for _, m := range urlRe.FindAllStringSubmatch(content, -1) {
		url := m[1]
		pkg.Urls = append(pkg.Urls, url)

		// Try infer version from url
		if pkg.Version == "" {
			if v := parseVersionFromUrl(url); v != "" {
				pkg.Version = v
			}
		}
	}

	// License
	if m := regexp.MustCompile(`license\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.License = m[1]
	}

	// Dependencies
	depRe := regexp.MustCompile(`depends_on\s+["']([^"']+)["'](?:\s*=>\s*(.*))?`)
	for _, m := range depRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		attrs := m[2]

		if strings.Contains(attrs, ":build") {
			pkg.BuildDependencies = append(pkg.BuildDependencies, name)
		} else {
			pkg.Dependencies = append(pkg.Dependencies, name)
		}
	}

	// Conflicts
	conflictRe := regexp.MustCompile(`conflicts_with\s+["']([^"']+)["']`)
	for _, m := range conflictRe.FindAllStringSubmatch(content, -1) {
		pkg.Conflicts = append(pkg.Conflicts, m[1])
	}

	// Flags
	if strings.Contains(content, "disable!") {
		pkg.IsDisabled = true
	}
	if strings.Contains(content, "deprecate!") {
		pkg.IsDeprecated = true
	}

	// Final validation on required fields
	if pkg.Version == "" {
		return nil, fmt.Errorf("no version found in %s", info.path)
	} else if pkg.Desc == "" {
		return nil, fmt.Errorf("no desc found in %s", info.path)
	} else if pkg.Homepage == "" {
		return nil, fmt.Errorf("no homepage found in %s", info.path)
	} else {
		return &pkg, nil
	}
}

func parseVersionFromUrl(url string) string {
	base := path.Base(url)
	for _, ext := range sourceExts {
		if strings.HasSuffix(base, ext) {
			base = strings.TrimSuffix(base, ext)
			break
		}
	}
	if m := versionRegex.FindStringSubmatch(base); m != nil {
		return normalizeVersion(m[1])
	} else {
		return ""
	}
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
