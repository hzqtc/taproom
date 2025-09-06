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
		Path string `json:"path"`
	} `json:"source"`
}

var brewPrefix = func() string {
	brewPrefixBytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		panic(fmt.Sprintf("failed to locate homebrew path: %v", err))
	}
	return strings.TrimSpace(string(brewPrefixBytes))
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

func fetchInstalledPackages(fetchSize bool, cask bool, resultCh chan []*installInfo, errCh chan error) {
	var dir string
	if cask {
		dir = filepath.Join(brewPrefix, "Caskroom")
	} else {
		dir = filepath.Join(brewPrefix, "Cellar")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		errCh <- fmt.Errorf("failed to read dir %s: %w", dir, err)
	}

	installInfoCh := make(chan *installInfo, 16 /* chan buffer */)
	numPackages := 0
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden file and symlinks
		if name == "" || name[0] == '.' || entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		path := filepath.Join(dir, name)
		numPackages++
		go func() {
			if cask {
				installInfoCh <- getCaskInstallInfo(fetchSize, path)
			} else {
				installInfoCh <- getFormulaInstallInfo(fetchSize, path)
			}
		}()
	}

	infoList := []*installInfo{}
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

	receipt := parseInstallReceipt(filepath.Join(path, "INSTALL_RECEIPT.json"))

	return &installInfo{
		name:      name,
		tap:       receipt.Source.Tap,
		version:   receipt.Source.Versions.Stable,
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

	receipt := parseInstallReceipt(filepath.Join(path, ".metadata", "INSTALL_RECEIPT.json"))

	return &installInfo{
		name:      filepath.Base(path),
		version:   version,
		pinned:    false, // Cask can not be pinned
		size:      size,
		asDep:     receipt.InstalledAsDep,
		timestamp: timestamp,
	}
}

func parseInstallReceipt(path string) *installReceipt {
	var receipt installReceipt
	file, err := os.Open(path)
	if err != nil {
		log.Printf("failed to open INSTALL_RECEIPT.json in: %s", path)
		return nil
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&receipt); err != nil {
		log.Printf("failed to parse INSTALL_RECEIPT.json in: %s", path)
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
