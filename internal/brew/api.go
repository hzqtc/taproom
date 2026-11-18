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
	"regexp"
	"runtime"
	"strconv"
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

var brewInternalApiCacheDir = func() string {
	if out, err := exec.Command("brew", "--cache").Output(); err == nil {
		cacheDir := strings.TrimSpace(string(out))
		if cacheDir != "" {
			return filepath.Join(cacheDir, "api", "internal")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Caches", "Homebrew", "api", "internal")
		}
		return filepath.Join(home, ".cache", "Homebrew", "api", "internal")
	}
	return filepath.Join(taproomCacheDir, "api", "internal")
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
	Name      string
	IsBuild   bool
	IsRuntime bool
}

func (d *dependency) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		d.Name = s
		d.IsBuild = false
		d.IsRuntime = true
		return nil
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err == nil {
		for k, rawVal := range m {
			d.Name = k
			var tag string
			if err := json.Unmarshal(rawVal, &tag); err == nil {
				d.IsBuild = (tag == ":build")
				d.IsRuntime = (tag != ":build" && tag != ":test")
				break
			}
			var tags []string
			if err := json.Unmarshal(rawVal, &tags); err == nil {
				for _, t := range tags {
					if t == ":build" {
						d.IsBuild = true
					} else if t != ":test" {
						d.IsRuntime = true
					}
				}
				break
			}
		}
		return nil
	}

	return fmt.Errorf("invalid dependency type: %s", string(data))
}

type license string

func (l *license) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		*l = ""
		return nil
	}
	*l = license(formatLicenseExpression(raw, false, ""))
	return nil
}

func formatLicenseExpression(expr interface{}, bracket bool, hashType string) string {
	switch v := expr.(type) {
	case string:
		if after, ok := strings.CutPrefix(v, ":"); ok {
			return "LicenseRef-Homebrew-" + strings.ReplaceAll(after, "_", "-")
		}
		return v
	case map[string]interface{}:
		var expressions []string
		if len(v) == 1 {
			for k, val := range v {
				hashType = k
				if !strings.HasPrefix(k, ":") {
					if vm, ok := val.(map[string]interface{}); ok {
						if w, ok := vm[":with"].(string); ok {
							expressions = append(expressions, fmt.Sprintf("%s WITH %s", k, w))
						}
					}
				} else if list, ok := val.([]interface{}); ok {
					for _, item := range list {
						expressions = append(expressions, formatLicenseExpression(item, true, hashType))
					}
				}
			}
		} else {
			bracket = false
			for k, val := range v {
				single := map[string]interface{}{k: val}
				expressions = append(expressions, formatLicenseExpression(single, true, ""))
			}
		}
		operator := " AND "
		if hashType == ":any_of" {
			operator = " OR "
		}
		joined := strings.Join(expressions, operator)
		if bracket && len(expressions) > 1 {
			return "(" + joined + ")"
		}
		return joined
	default:
		return ""
	}
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
	Aliases            []string      `json:"aliases"`
	Desc               string        `json:"desc"`
	Homepage           string        `json:"homepage"`
	StableVersion      string        `json:"stable_version"`
	Revision           int           `json:"revision"`
	StableUrlArgs      stableUrlArgs `json:"stable_url_args"`
	HeadUrlArgs        stableUrlArgs `json:"head_url_args"`
	License            license       `json:"license"`
	StableDependencies []dependency  `json:"stable_dependencies"`
	Conflicts          conflictsList `json:"conflicts"`
	DeprecateArgs      interface{}   `json:"deprecate_args"`
	DisableArgs        interface{}   `json:"disable_args"`
}

type apiCask struct {
	Desc      string   `json:"desc"`
	Homepage  string   `json:"homepage"`
	Version   string   `json:"version"`
	UrlArgs   []string `json:"url_args"`
	Names     []string `json:"names"`
	Conflicts struct {
		Formula []string `json:":formula"`
		Cask    []string `json:":cask"`
	} `json:"conflicts_with_args"`
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

func getBottleTag(cacheDir string) (string, error) {
	if tag := findLatestCachedBottleTag(cacheDir); tag != "" {
		return tag, nil
	}
	return calculateBottleTag()
}

func findLatestCachedBottleTag(cacheDir string) string {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return ""
	}

	const prefix = "packages."
	const suffix = ".jws.json"

	var latestTag string
	var latestModTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		tag := name[len(prefix) : len(name)-len(suffix)]
		if tag == "" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latestTag == "" || info.ModTime().After(latestModTime) {
			latestTag = tag
			latestModTime = info.ModTime()
		}
	}

	return latestTag
}

var productVersionRegex = regexp.MustCompile(`<key>ProductVersion</key>\s*<string>([0-9.]+)</string>`)

func calculateBottleTag() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	}

	switch runtime.GOOS {
	case "darwin":
		if arch == "x86_64" {
			if out, err := exec.Command("sysctl", "-n", "hw.optional.arm64").Output(); err == nil && strings.TrimSpace(string(out)) == "1" {
				arch = "arm64"
			}
		}
		osName, err := macosVersionName()
		if err != nil {
			return "", err
		}
		if arch == "x86_64" {
			return osName, nil
		}
		return fmt.Sprintf("%s_%s", arch, osName), nil
	case "linux":
		return fmt.Sprintf("%s_linux", arch), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func macosVersionName() (string, error) {
	verStr := ""
	if data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
		if m := productVersionRegex.FindSubmatch(data); len(m) > 1 {
			verStr = string(m[1])
		}
	}
	if verStr == "" {
		out, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
		if err != nil {
			return "", fmt.Errorf("failed to detect macOS version: %w", err)
		}
		verStr = strings.TrimSpace(string(out))
	}

	parts := strings.Split(verStr, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid macOS version %q: %w", verStr, err)
	}

	switch {
	case major >= 27:
		return "golden_gate", nil
	case major >= 26:
		return "tahoe", nil
	case major >= 15:
		return "sequoia", nil
	case major >= 14:
		return "sonoma", nil
	case major >= 13:
		return "ventura", nil
	case major >= 12:
		return "monterey", nil
	case major >= 11:
		return "big_sur", nil
	default:
		return "", fmt.Errorf("unsupported macOS version: %s", verStr)
	}
}

func fetchInternalPackages(dataChan chan internalPackages, errChan chan error) {
	tag, err := getBottleTag(brewInternalApiCacheDir)
	if err != nil {
		errChan <- fmt.Errorf("failed to detect brew bottle tag: %w", err)
		return
	}

	url := fmt.Sprintf("https://formulae.brew.sh/api/internal/packages.%s.jws.json", tag)
	cacheName := fmt.Sprintf("packages.%s.jws.json", tag)
	cachePath := filepath.Join(brewInternalApiCacheDir, cacheName)

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
