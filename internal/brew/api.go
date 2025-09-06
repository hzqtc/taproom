package brew

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
)

var cacheDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache"
	}
	return filepath.Join(home, ".cache", "taproom")
}()

const (
	apiFormulaURL             = "https://formulae.brew.sh/api/formula.json"
	apiCaskURL                = "https://formulae.brew.sh/api/cask.json"
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaCache              = "formula.json"
	casksCache                = "cask.json"
	formulaeAnalytics90dCache = "formulae-analytics-90d.json"
	casksAnalytics90dCache    = "casks-analytics-90d.json"

	urlCacheTtl = 6 * time.Hour
)

var flagInvalidateCache = pflag.BoolP("invalidate-cache", "i", false, "Invalidate cache and force re-downloading data")

// Structs for parsing Homebrew API Json
type apiFormula struct {
	Name     string `json:"name"`
	Tap      string `json:"tap"`
	Desc     string `json:"desc"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
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
}

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
}

type apiFormulaAnalytics struct {
	Items []struct {
		Name  string `json:"formula"`
		Count string `json:"count"`
	} `json:"items"`
}

type apiCaskAnalytics struct {
	Items []struct {
		Name  string `json:"cask"`
		Count string `json:"count"`
	} `json:"items"`
}

func readCacheData(cachePath string) []byte {
	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < urlCacheTtl {
		file, err := os.Open(cachePath)
		if err == nil {
			defer file.Close()
			body, err := io.ReadAll(file)
			if err == nil {
				return body
			}
		}
	}

	return nil
}

// fetchJsonWithCache is a generic function to fetch and decode Json from a URL, with caching.
func fetchJsonWithCache[T any](url, filename string, target *T, dataChan chan T, errChan chan error) {
	cachePath := filepath.Join(cacheDir, filename)

	// Attempt to load from cache first
	if !*flagInvalidateCache {
		if cacheData := readCacheData(cachePath); cacheData != nil {
			if err := json.Unmarshal(cacheData, &target); err == nil {
				log.Printf("Loaded %s from cache file %s", url, filename)
				dataChan <- *target
				return
			}
		}
	}

	// If cache is invalid or missing, fetch from URL
	resp, err := http.Get(url)
	if err != nil {
		errChan <- fmt.Errorf("failed to fetch %s: %w", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errChan <- fmt.Errorf("bad HTTP status fetching %s: %s", url, resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errChan <- fmt.Errorf("failed to read body from %s: %w", url, err)
		return
	}

	// Save to cache
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		if err := os.WriteFile(cachePath, body, 0644); err != nil {
			// Log caching error but don't fail the request
			log.Printf("Failed to write to cache at %s: %+v", cachePath, err)
		}
	}

	if err := json.Unmarshal(body, &target); err != nil {
		errChan <- fmt.Errorf("failed to decode json from %s: %w", url, err)
		return
	}
	log.Printf("Downloaded %s", url)
	dataChan <- *target
}
