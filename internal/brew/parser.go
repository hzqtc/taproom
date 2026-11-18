package brew

import (
  "encoding/json"
  "fmt"
  "strings"
)

// Structs for parsing Homebrew API Json
type dependency struct {
  name      string
  isBuild   bool
  isRuntime bool
}

func (d *dependency) UnmarshalJSON(data []byte) error {
  var s string
  if err := json.Unmarshal(data, &s); err == nil {
    d.name = s
    d.isBuild = false
    d.isRuntime = true
    return nil
  }

  var m map[string]json.RawMessage
  if err := json.Unmarshal(data, &m); err == nil {
    for k, rawVal := range m {
      d.name = k
      var tag string
      if err := json.Unmarshal(rawVal, &tag); err == nil {
        d.isBuild = (tag == ":build")
        d.isRuntime = (tag != ":build" && tag != ":test")
        break
      }
      var tags []string
      if err := json.Unmarshal(rawVal, &tags); err == nil {
        for _, t := range tags {
          if t == ":build" {
            d.isBuild = true
          } else if t != ":test" {
            d.isRuntime = true
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
  var raw any
  if err := json.Unmarshal(data, &raw); err != nil {
    *l = ""
    return nil
  }
  *l = license(formatLicenseExpression(raw, false, ""))
  return nil
}

func formatLicenseExpression(expr any, bracket bool, hashType string) string {
  switch v := expr.(type) {
  case string:
    if after, ok := strings.CutPrefix(v, ":"); ok {
      return "LicenseRef-Homebrew-" + strings.ReplaceAll(after, "_", "-")
    }
    return v
  case map[string]any:
    var expressions []string
    if len(v) == 1 {
      for k, val := range v {
        hashType = k
        if !strings.HasPrefix(k, ":") {
          if vm, ok := val.(map[string]any); ok {
            if w, ok := vm[":with"].(string); ok {
              expressions = append(expressions, fmt.Sprintf("%s WITH %s", k, w))
            }
          }
        } else if list, ok := val.([]any); ok {
          for _, item := range list {
            expressions = append(expressions, formatLicenseExpression(item, true, hashType))
          }
        }
      }
    } else {
      bracket = false
      for k, val := range v {
        single := map[string]any{k: val}
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
  DeprecateArgs      any           `json:"deprecate_args"`
  DisableArgs        any           `json:"disable_args"`
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
  AutoUpdates   bool `json:"auto_updates"`
  DeprecateArgs any  `json:"deprecate_args"`
  DisableArgs   any  `json:"disable_args"`
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

func parseJwsJson[T any](data []byte, target *T) error {
  jws := jwsJson{}
  if err := json.Unmarshal(data, &jws); err != nil {
    return fmt.Errorf("failed to decode jws json: %w", err)
  }
  if err := json.Unmarshal([]byte(jws.Payload), target); err != nil {
    return fmt.Errorf("failed to decode json payload: %w", err)
  }
  return nil
}

func parseJson[T any](data []byte, target *T) error {
  if err := json.Unmarshal(data, target); err != nil {
    return fmt.Errorf("failed to decode json: %w", err)
  }
  return nil
}
