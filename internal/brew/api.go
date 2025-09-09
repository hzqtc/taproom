package brew

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var brewCacheDir = func() string {
	brewCacheDirBytes, err := exec.Command("brew", "--cache").Output()
	if err == nil {
		return strings.TrimSpace(string(brewCacheDirBytes))
	} else {
		log.Printf("failed to locate homebrew cache path: %v", err)
		return taproomCacheDir
	}
}()

var taproomCacheDir = func() string {
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".cache", "taproom")
	} else {
		log.Printf("failed to locate user's home dir: %v", err)
		return ".cache"
	}
}()

const (
	apiFormulaURL             = "https://formulae.brew.sh/api/formula.jws.json"
	apiCaskURL                = "https://formulae.brew.sh/api/cask.jws.json"
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaJwsJson       = "formula.jws.json"
	caskJwsJson          = "cask.jws.json"
	formulaAnalyticsJson = "formula-analytics-90d.json"
	caskAnalyticsJson    = "cask-analytics-90d.json"

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

type jwsJson struct {
	Payload string `json:"payload"`
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

func fetchFormula(dataChan chan []*apiFormula, errChan chan error) {
	target := []*apiFormula{}
	fetchJwsJsonWithCache(
		apiFormulaURL,
		filepath.Join(brewCacheDir, formulaJwsJson),
		&target,
		dataChan,
		errChan)
}

func fetchCask(dataChan chan []*apiCask, errChan chan error) {
	target := []*apiCask{}
	fetchJwsJsonWithCache(
		apiCaskURL,
		filepath.Join(brewCacheDir, caskJwsJson),
		&target,
		dataChan,
		errChan)
}

func fetchFormulaAnalytics(dataChan chan apiFormulaAnalytics, errChan chan error) {
	target := apiFormulaAnalytics{}
	fetchJsonWithCache(
		apiFormulaAnalytics90dURL,
		filepath.Join(taproomCacheDir, formulaAnalyticsJson),
		&target,
		dataChan,
		errChan)
}

func fetchCaskAnalytics(dataChan chan apiCaskAnalytics, errChan chan error) {
	target := apiCaskAnalytics{}
	fetchJsonWithCache(
		apiCaskAnalytics90dURL,
		filepath.Join(taproomCacheDir, caskAnalyticsJson),
		&target,
		dataChan,
		errChan)
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

// Fetch a JWS json and parse its payload to target
func fetchJwsJsonWithCache[T any](url, cachePath string, target *T, dataChan chan T, errChan chan error) {
	data, err := fetchUrlWithCache(url, cachePath)
	if err != nil {
		errChan <- err
		return
	}
	jws := jwsJson{}
	if err := json.Unmarshal(data, &jws); err != nil {
		errChan <- fmt.Errorf("failed to decode jws json from %s: %w", url, err)
		return
	}
	if err := json.Unmarshal([]byte(jws.Payload), target); err != nil {
		errChan <- fmt.Errorf("failed to decode json from %s: %w", url, err)
		return
	}
	dataChan <- *target
}

// A generic function to fetch and decode Json from a URL, with caching.
func fetchJsonWithCache[T any](url, cachePath string, target *T, dataChan chan T, errChan chan error) {
	data, err := fetchUrlWithCache(url, cachePath)
	if err != nil {
		errChan <- err
		return
	}
	if err := json.Unmarshal(data, &target); err != nil {
		errChan <- fmt.Errorf("failed to decode json from %s: %w", url, err)
		return
	}
	dataChan <- *target
}

func fetchUrlWithCache(url, cachePath string) ([]byte, error) {
	var jsonData []byte
	if !*flagInvalidateCache {
		jsonData = readCacheData(cachePath)
	}
	if jsonData != nil {
		log.Printf("Loaded %s from cache %s", url, cachePath)
		return jsonData, nil
	}

	// If cache is invalid or missing, fetch from URL
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad HTTP status fetching %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body from %s: %w", url, err)
	}

	// Save to cache
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err == nil {
		if err := os.WriteFile(cachePath, body, 0644); err != nil {
			// Log caching error but don't fail the request
			log.Printf("Failed to write to cache at %s: %+v", cachePath, err)
		}
	}

	log.Printf("Downloaded %s", url)
	return body, nil
}
