package brew

import (
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
    tag, ok := strings.CutPrefix(entry.Name(), prefix)
    if !ok {
      continue
    }
    tag, ok = strings.CutSuffix(tag, suffix)
    if !ok || tag == "" {
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
  if err := parseJwsJson(data, target); err != nil {
    errChan <- fmt.Errorf("%w from %s", err, url)
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
  if err := parseJson(data, target); err != nil {
    errChan <- fmt.Errorf("%w from %s", err, url)
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
