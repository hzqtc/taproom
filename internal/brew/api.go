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
	apiFormulaAnalytics90dURL = "https://formulae.brew.sh/api/analytics/install-on-request/90d.json"
	apiCaskAnalytics90dURL    = "https://formulae.brew.sh/api/analytics/cask-install/90d.json"

	formulaAnalyticsJson = "formula-analytics-90d.json"
	caskAnalyticsJson    = "cask-analytics-90d.json"

	urlCacheTtl = 6 * time.Hour
)

var flagInvalidateCache = pflag.BoolP("invalidate-cache", "i", false, "Invalidate cache and force re-downloading data")

// Structs for parsing Homebrew API Json
type dependency struct {
	Name    string
	IsBuild bool
}

func (d *dependency) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		d.Name = s
		d.IsBuild = false
		return nil
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		for k, v := range m {
			d.Name = k
			d.IsBuild = (v == ":build")
			break
		}
		return nil
	}

	return fmt.Errorf("invalid dependency type: %s", string(data))
}

type license string

func (l *license) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*l = license(s)
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err == nil {
		var keys []string
		for k, v := range m {
			if vm, ok := v.(map[string]interface{}); ok {
				if w, ok := vm[":with"].(string); ok {
					keys = append(keys, fmt.Sprintf("%s with %s", k, w))
					continue
				}
			}
			keys = append(keys, k)
		}
		*l = license(strings.Join(keys, ", "))
		return nil
	}

	*l = ""
	return nil
}

type stableUrlArgs []string

func (s *stableUrlArgs) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var result []string
	for _, item := range raw {
		var str string
		if err := json.Unmarshal(item, &str); err == nil {
			result = append(result, str)
		}
	}
	*s = result
	return nil
}

type conflictsList []string

func (c *conflictsList) UnmarshalJSON(data []byte) error {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var result []string
	for _, item := range raw {
		if len(item) > 0 {
			var str string
			if err := json.Unmarshal(item[0], &str); err == nil {
				result = append(result, str)
			}
		}
	}
	*c = result
	return nil
}

type apiFormula struct {
	Desc               string        `json:"desc"`
	Homepage           string        `json:"homepage"`
	StableVersion      string        `json:"stable_version"`
	Revision           int           `json:"revision"`
	StableUrlArgs      stableUrlArgs `json:"stable_url_args"`
	License            license       `json:"license"`
	StableDependencies []dependency  `json:"stable_dependencies"`
	Conflicts          conflictsList `json:"conflicts"`
	DeprecateArgs      interface{}   `json:"deprecate_args"`
	DisableArgs        interface{}   `json:"disable_args"`
}

type apiCask struct {
	Desc     string   `json:"desc"`
	Homepage string   `json:"homepage"`
	Version  string   `json:"version"`
	UrlArgs  []string `json:"url_args"`
	Names    []string `json:"names"`
	Conflicts struct {
		Formula []string `json:":formula"`
		Cask    []string `json:":cask"`
	} `json:"conflicts"`
	DependsOnArgs struct {
		Formula []string `json:":formula"`
		Cask    []string `json:":cask"`
	} `json:"depends_on_args"`
	AutoUpdates   bool        `json:"auto_updates"`
	DeprecateArgs interface{} `json:"deprecate_args"`
	DisableArgs   interface{} `json:"disable_args"`
}

type internalPackages struct {
	Metadata struct {
		HomebrewVersion string `json:"homebrew_version"`
		BottleTag       string `json:"bottle_tag"`
		GeneratedAt     int64  `json:"generated_at"`
	} `json:"metadata"`
	Formulae       map[string]*apiFormula `json:"formulae"`
	Casks          map[string]*apiCask    `json:"casks"`
	FormulaAliases map[string]string      `json:"formula_aliases"`
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

func getBottleTag() (string, error) {
	cmd := exec.Command("brew", "ruby", "-e", "puts Utils::Bottles.tag.to_s")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fetchInternalPackages(dataChan chan internalPackages, errChan chan error) {
	tag, err := getBottleTag()
	if err != nil {
		errChan <- fmt.Errorf("failed to detect brew bottle tag: %w", err)
		return
	}

	url := fmt.Sprintf("https://formulae.brew.sh/api/internal/packages.%s.jws.json", tag)
	cacheName := fmt.Sprintf("packages.%s.jws.json", tag)
	cachePath := filepath.Join(taproomCacheDir, cacheName)

	target := &internalPackages{}
	fetchJwsJsonWithCache(
		url,
		cachePath,
		target,
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
