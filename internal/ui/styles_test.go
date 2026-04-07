package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestInitThemeDark(t *testing.T) {
	*flagTheme = "dark"
	InitTheme()
	if !lipgloss.HasDarkBackground() {
		t.Error("expected dark background after InitTheme with 'dark'")
	}
}

func TestInitThemeLight(t *testing.T) {
	*flagTheme = "light"
	InitTheme()
	if lipgloss.HasDarkBackground() {
		t.Error("expected light background after InitTheme with 'light'")
	}
}

func TestInitThemeAuto(t *testing.T) {
	// auto should not panic
	*flagTheme = "auto"
	InitTheme()
}
