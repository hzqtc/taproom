package model

import (
	"slices"
	"taproom/internal/data"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterPackagesByInstallStatus(t *testing.T) {
	tests := []struct {
		name string
		key  rune
		want []string
	}{
		{name: "installed", key: 'i', want: []string{"dependency", "explicit"}},
		{name: "uninstalled", key: 'n', want: []string{"uninstalled"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitialModel()
			m.allPackages = []*data.Package{
				{Name: "dependency", InstallStatus: data.InstalledDep},
				{Name: "explicit", InstallStatus: data.ExplicitlyInstalled},
				{Name: "uninstalled", InstallStatus: data.Uninstalled},
			}
			m.filterView, _ = m.filterView.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
			m.filterPackages()

			var got []string
			for _, pkg := range m.table.Packages() {
				got = append(got, pkg.Name)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("filtered packages = %v, want %v", got, tt.want)
			}
		})
	}
}
