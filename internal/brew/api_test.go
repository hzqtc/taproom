package brew

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDependencyUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		isBuild   bool
		isRuntime bool
	}{
		{
			name:      "runtime string",
			input:     `"bdw-gc"`,
			wantName:  "bdw-gc",
			isBuild:   false,
			isRuntime: true,
		},
		{
			name:      "build map string",
			input:     `{"pkgconf": ":build"}`,
			wantName:  "pkgconf",
			isBuild:   true,
			isRuntime: false,
		},
		{
			name:      "test map string",
			input:     `{"pytest": ":test"}`,
			wantName:  "pytest",
			isBuild:   false,
			isRuntime: false,
		},
		{
			name:      "no_linkage map string",
			input:     `{"libfoo": ":no_linkage"}`,
			wantName:  "libfoo",
			isBuild:   false,
			isRuntime: true,
		},
		{
			name:      "build and test array",
			input:     `{"cmake": [":build", ":test"]}`,
			wantName:  "cmake",
			isBuild:   true,
			isRuntime: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d dependency
			if err := json.Unmarshal([]byte(tc.input), &d); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.name != tc.wantName || d.isBuild != tc.isBuild || d.isRuntime != tc.isRuntime {
				t.Errorf("got %+v, want name=%q isBuild=%v isRuntime=%v", d, tc.wantName, tc.isBuild, tc.isRuntime)
			}
		})
	}
}

func TestLicenseUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: `"MIT"`,
			want:  "MIT",
		},
		{
			name:  "symbol string",
			input: `":public_domain"`,
			want:  "LicenseRef-Homebrew-public-domain",
		},
		{
			name:  "with exception",
			input: `{"GPL-3.0-or-later": {":with": "GCC-exception-3.1"}}`,
			want:  "GPL-3.0-or-later WITH GCC-exception-3.1",
		},
		{
			name:  "all_of",
			input: `{":all_of": ["GPL-3.0-only", "MIT"]}`,
			want:  "GPL-3.0-only AND MIT",
		},
		{
			name:  "any_of",
			input: `{":any_of": ["Apache-2.0", "MIT"]}`,
			want:  "Apache-2.0 OR MIT",
		},
		{
			name:  "nested",
			input: `{":all_of": ["GPL-3.0-only", {":any_of": ["GPL-3.0-only", "MIT"]}]}`,
			want:  "GPL-3.0-only AND (GPL-3.0-only OR MIT)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var l license
			if err := json.Unmarshal([]byte(tc.input), &l); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(l) != tc.want {
				t.Errorf("got %q, want %q", string(l), tc.want)
			}
		})
	}
}

func TestFindLatestCachedBottleTag(t *testing.T) {
	tmpDir := t.TempDir()

	olderPath := filepath.Join(tmpDir, "packages.arm64_tahoe.jws.json")
	newerPath := filepath.Join(tmpDir, "packages.arm64_golden_gate.jws.json")
	sidecarPath := filepath.Join(tmpDir, "packages.arm64_tahoe.jws.json.payload")

	for _, p := range []string{olderPath, newerPath, sidecarPath} {
		if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", p, err)
		}
	}

	now := time.Now()
	_ = os.Chtimes(olderPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	_ = os.Chtimes(newerPath, now.Add(-1*time.Hour), now.Add(-1*time.Hour))
	_ = os.Chtimes(sidecarPath, now, now)

	tag := findLatestCachedBottleTag(tmpDir)
	if tag != "arm64_golden_gate" {
		t.Errorf("expected tag %q, got %q", "arm64_golden_gate", tag)
	}
}

func TestFetchInternalPackagesLiveCache(t *testing.T) {
	dataChan := make(chan internalPackages, 1)
	errChan := make(chan error, 1)

	go fetchInternalPackages(dataChan, errChan)

	select {
	case err := <-errChan:
		t.Fatalf("fetchInternalPackages failed: %v", err)
	case pkgs := <-dataChan:
		if len(pkgs.Formulae) == 0 {
			t.Error("expected non-empty formulae map")
		}
		if len(pkgs.Casks) == 0 {
			t.Error("expected non-empty casks map")
		}
	}
}
