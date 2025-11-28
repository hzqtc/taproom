package brew

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetFormulaInstallInfoWithMissingReceipt(t *testing.T) {
	// Create a temporary directory structure that simulates a formula
	// installation without INSTALL_RECEIPT.json
	tmpDir := t.TempDir()
	formulaName := "test-formula"
	version := "1.0.0"

	formulaDir := filepath.Join(tmpDir, formulaName)
	versionDir := filepath.Join(formulaDir, version)

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Call getFormulaInstallInfo without INSTALL_RECEIPT.json
	// This should not panic and should return basic info
	info := getFormulaInstallInfo(false, formulaDir)

	if info == nil {
		t.Fatal("expected non-nil installInfo, got nil")
	}

	if info.name != formulaName {
		t.Errorf("expected name %q, got %q", formulaName, info.name)
	}

	if info.version != version {
		t.Errorf("expected version %q, got %q", version, info.version)
	}

	// These should be empty/zero since there's no receipt
	if info.tap != "" {
		t.Errorf("expected empty tap, got %q", info.tap)
	}

	if info.revision != 0 {
		t.Errorf("expected revision 0, got %d", info.revision)
	}
}

func TestGetFormulaInstallInfoWithRevision(t *testing.T) {
	// Test parsing revision from version string
	tmpDir := t.TempDir()
	formulaName := "test-formula"
	version := "1.0.0_2" // version with revision

	formulaDir := filepath.Join(tmpDir, formulaName)
	versionDir := filepath.Join(formulaDir, version)
	receiptDir := versionDir

	if err := os.MkdirAll(receiptDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create a valid INSTALL_RECEIPT.json
	receipt := `{
		"installed_as_dependency": false,
		"time": 1234567890,
		"source": {
			"versions": {
				"stable": "1.0.0"
			},
			"tap": "homebrew/core",
			"path": "/path/to/formula.rb"
		}
	}`

	receiptPath := filepath.Join(receiptDir, "INSTALL_RECEIPT.json")
	if err := os.WriteFile(receiptPath, []byte(receipt), 0644); err != nil {
		t.Fatalf("failed to write receipt: %v", err)
	}

	info := getFormulaInstallInfo(false, formulaDir)

	if info == nil {
		t.Fatal("expected non-nil installInfo, got nil")
	}

	if info.revision != 2 {
		t.Errorf("expected revision 2, got %d", info.revision)
	}

	if info.version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", info.version)
	}

	if info.tap != "homebrew/core" {
		t.Errorf("expected tap %q, got %q", "homebrew/core", info.tap)
	}
}
