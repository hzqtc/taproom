package brew

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type installInfo struct {
	name      string
	tap       string
	version   string
	revision  int
	asDep     bool
	pinned    bool
	timestamp int64
	size      int64
	path      string
}

// struct to parse INSTALL_RECEIPT.json
type installReceipt struct {
	InstalledAsDep bool  `json:"installed_as_dependency"`
	InstallTime    int64 `json:"time"`
	Source         struct {
		Version  string `json:"version"` // Cask only
		Versions struct {
			Stable string `json:"stable"` // Formula only
		} `json:"versions"`
		Tap  string `json:"tap"`
		Path string `json:"path"` // Path is a .rb file for custom tap packages
	} `json:"source"`
}

var brewPrefix = func() string {
	bytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		panic(fmt.Sprintf("failed to locate homebrew path: %v", err))
	}
	return strings.TrimSpace(string(bytes))
}()

var pinnedPackages = func() map[string]bool {
	formulae := make(map[string]bool)

	dir := filepath.Join(brewPrefix, "var/homebrew/pinned")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return formulae
	}

	for _, entry := range entries {
		formulae[entry.Name()] = true
	}
	return formulae
}()

func fetchInstalledFormula(fetchSize bool, resultCh chan []*installInfo) {
	fetchInstalledPackages(
		filepath.Join(brewPrefix, "Cellar"),
		func(path string) *installInfo { return getFormulaInstallInfo(fetchSize, path) },
		resultCh)
}

func fetchInstalledCask(fetchSize bool, resultCh chan []*installInfo) {
	fetchInstalledPackages(
		filepath.Join(brewPrefix, "Caskroom"),
		func(path string) *installInfo { return getCaskInstallInfo(fetchSize, path) },
		resultCh)
}

func fetchInstalledPackages(installDir string, fetcher func(string) *installInfo, resultCh chan []*installInfo) {
	infoList := []*installInfo{}
	installInfoCh := make(chan *installInfo, 16 /* chan buffer */)
	numPackages := 0

	entries, err := os.ReadDir(installDir)
	if err != nil {
		log.Printf("failed to read dir %s: %v", installDir, err)
	} else {
		for _, entry := range entries {
			name := entry.Name()
			// Skip hidden file and symlinks
			if name == "" || name[0] == '.' || entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			path := filepath.Join(installDir, name)
			numPackages++
			go func() {
				installInfoCh <- fetcher(path)
			}()
		}
	}

	for range numPackages {
		info := <-installInfoCh
		if info == nil {
			continue
		}
		info.pinned = pinnedPackages[info.name]
		infoList = append(infoList, info)
	}
	resultCh <- infoList
}

func getFormulaInstallInfo(fetchSize bool, path string) *installInfo {
	name := filepath.Base(path)
	entries, err := os.ReadDir(path)
	var subdir string
	if err != nil {
		log.Printf("failed to get formula install info from %s: %v", path, err)
		return nil
	} else {
		for _, entry := range entries {
			// Expect only one subdirectory, which name is the formula version
			subdir = entry.Name()
			if subdir == "" || subdir[0] == '.' {
				continue
			} else {
				break
			}
		}
	}
	path = filepath.Join(path, subdir)

	var size int64
	if fetchSize {
		size = fetchDirSize(path, false)
	}

	receipt := parseInstallReceipt(path)
	revision := 0
	// Get revision from subdir, e.g. 0.11.1_2 is vision 0.11.1 and revision is 2
	if s, _ := strings.CutPrefix(subdir, receipt.Source.Versions.Stable); len(s) > 0 && strings.HasPrefix(s, "_") {
		revision, _ = strconv.Atoi(s[1:])
	}

	return &installInfo{
		name:      name,
		tap:       receipt.Source.Tap,
		version:   receipt.Source.Versions.Stable,
		revision:  revision,
		size:      size,
		asDep:     receipt.InstalledAsDep,
		timestamp: receipt.InstallTime,
		path:      receipt.Source.Path,
	}
}

func getCaskInstallInfo(fetchSize bool, path string) *installInfo {
	var size int64
	if fetchSize {
		size = fetchDirSize(path, true)
	}

	var version string
	var timestamp int64
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("failed to get cask version and install time: %v", err)
	} else {
		for _, entry := range entries {
			name := entry.Name()
			if name == "" || name[0] == '.' {
				continue
			}

			// Expect only one subdirectory, which name is the cask version
			// This is more up-to-date than the version in INSTALL_RECEIPT.json
			version = name

			info, err := os.Stat(filepath.Join(path, name))
			if err != nil {
				log.Printf("failed to get cask install time: %v", err)
			} else {
				timestamp = info.ModTime().Unix()
			}

			break
		}
	}

	info := installInfo{
		name:      filepath.Base(path),
		version:   version,
		size:      size,
		timestamp: timestamp,
	}

	// Casks installed by older brew (before 4.4.0) does not have INSTALL_RECEIPT.json
	if receipt := parseInstallReceipt(filepath.Join(path, ".metadata")); receipt != nil {
		info.tap = receipt.Source.Tap
		info.asDep = receipt.InstalledAsDep
		info.path = receipt.Source.Path
		info.timestamp = receipt.InstallTime
	}

	return &info
}

func parseInstallReceipt(dir string) *installReceipt {
	const filename = "INSTALL_RECEIPT.json"
	var receipt installReceipt
	file, err := os.Open(filepath.Join(dir, filename))
	if err != nil {
		log.Printf("failed to open %s in: %s", filename, dir)
		return nil
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&receipt); err != nil {
		log.Printf("failed to parse %s in: %s", filename, dir)
		return nil
	}
	return &receipt
}

func fetchDirSize(path string, followSymlink bool) int64 {
	// -k: output in KB
	// -s: output the total size
	// -L: follow symbol links (which is needed for Casks)
	args := []string{"-k", "-s"}
	if followSymlink {
		args = append(args, "-L")
	}
	args = append(args, path)
	cmd := exec.Command("du", args...)
	output, err := cmd.Output()

	if err == nil {
		fields := strings.Fields(string(output))
		if len(fields) == 2 {
			size, _ := strconv.ParseInt(fields[0], 10, 64)
			return size
		}
	}
	return 0
}
