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
	version   string
	asDep     bool
	pinned    bool
	timestamp int64
	size      int64
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
	} `json:"source"`
}

var brewPrefix = func() string {
	brewPrefixBytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		log.Printf("failed to identify brew prefix: %v\n", err)
		return ""
	}
	return strings.TrimSpace(string(brewPrefixBytes))
}()

func fetchInstalledFormulae(fetchSize bool, resultCh chan []*installInfo, errCh chan error) {
	dir := filepath.Join(brewPrefix, "var/homebrew/linked")
	entries, err := os.ReadDir(dir)
	if err != nil {
		errCh <- fmt.Errorf("failed to list linked formulae: %w", err)
	}

	installInfoCh := make(chan *installInfo, 16)
	numFormulae := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] == '.' {
			continue
		}

		path := filepath.Join(dir, name)
		resolvedPath := path
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				log.Printf("failed to resolve symlink: %s", path)
				installInfoCh <- nil
				continue
			}
			resolvedPath = target
		}

		numFormulae++
		go func() {
			installInfoCh <- getFormulaInstallInfo(fetchSize, name, resolvedPath)
		}()
	}

	pinnedFormulae := getPinnedFormulae()
	infoList := []*installInfo{}
	for range numFormulae {
		info := <-installInfoCh
		if info == nil {
			continue
		}
		info.pinned = pinnedFormulae[info.name]
		log.Printf("fetched formula install info: %+v", info)
		infoList = append(infoList, info)
	}
	resultCh <- infoList
}

func getFormulaInstallInfo(fetchSize bool, name, path string) *installInfo {
	var size int64
	if fetchSize {
		size = fetchDirSize(path, false)
	}

	var receipt installReceipt
	file, err := os.Open(filepath.Join(path, "INSTALL_RECEIPT.json"))
	if err != nil {
		log.Printf("failed to open INSTALL_RECEIPT.json in: %s", path)

	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&receipt); err != nil {
		log.Printf("failed to parse INSTALL_RECEIPT.json in: %s", path)
	}

	return &installInfo{
		name:      name,
		version:   receipt.Source.Versions.Stable,
		size:      size,
		asDep:     receipt.InstalledAsDep,
		timestamp: receipt.InstallTime,
	}
}

func getPinnedFormulae() map[string]bool {
	formulae := make(map[string]bool)

	dir := filepath.Join(brewPrefix, "var/homebrew/pinned")
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("failed to list pinned formulae: %v", err)
		return formulae
	}

	for _, entry := range entries {
		formulae[entry.Name()] = true
	}

	return formulae
}

func fetchInstalledCasks(fetchSize bool, resultCh chan []*installInfo, errCh chan error) {
	dir := filepath.Join(brewPrefix, "Caskroom")
	entries, err := os.ReadDir(dir)
	if err != nil {
		errCh <- fmt.Errorf("failed to list casks: %w", err)
	}

	installInfoCh := make(chan *installInfo, 16)
	numCasks := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] == '.' {
			continue
		}

		path := filepath.Join(dir, name)
		numCasks++
		go func() {
			installInfoCh <- getCaskInstallInfo(fetchSize, path)
		}()
	}

	infoList := []*installInfo{}
	for range numCasks {
		info := <-installInfoCh
		if info == nil {
			continue
		}
		log.Printf("fetched cask install info: %+v", info)
		infoList = append(infoList, info)
	}
	resultCh <- infoList
}

func getCaskInstallInfo(fetchSize bool, path string) *installInfo {
	var size int64
	if fetchSize {
		size = fetchDirSize(path, true)
	}

	var receipt installReceipt
	file, err := os.Open(filepath.Join(path, ".metadata", "INSTALL_RECEIPT.json"))
	if err != nil {
		log.Printf("failed to open INSTALL_RECEIPT.json in: %s", path)

	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&receipt); err != nil {
		log.Printf("failed to parse INSTALL_RECEIPT.json in: %s", path)
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
		}
	}

	return &installInfo{
		name:      filepath.Base(path),
		version:   version,
		pinned:    false, // Cask can not be pinned
		size:      size,
		asDep:     receipt.InstalledAsDep,
		timestamp: timestamp,
	}
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
