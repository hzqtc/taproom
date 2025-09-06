package brew

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"taproom/internal/data"
)

const coreTap = "homebrew/core"

// Get a package using local data
func getLocalPackage(info *installInfo) (*data.Package, error) {
	pkg := data.Package{
		Name: info.name,
		Tap:  info.tap,
	}

	data, err := os.ReadFile(info.path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	// Version
	if m := regexp.MustCompile(`version\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Version = m[1]
	} else {
		return nil, fmt.Errorf("no version found in %s", info.path)
	}

	// Desc
	if m := regexp.MustCompile(`desc\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Desc = m[1]
	} else {
		return nil, fmt.Errorf("no desc found in %s", info.path)
	}

	// Homepage
	if m := regexp.MustCompile(`homepage\s+["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		pkg.Homepage = m[1]
	} else {
		return nil, fmt.Errorf("no homepage found in %s", info.path)
	}

	// Urls
	urlRe := regexp.MustCompile(`url\s+["']([^"']+)["']`)
	for _, m := range urlRe.FindAllStringSubmatch(content, -1) {
		pkg.Urls = append(pkg.Urls, m[1])
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

	return &pkg, nil
}
