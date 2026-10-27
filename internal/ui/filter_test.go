package ui

import (
	"slices"
	"testing"
)

func TestParseFilterUninstalled(t *testing.T) {
	f, err := parseFilter("Uninstalled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != FilterUninstalled {
		t.Errorf("expected %v, got %v", FilterUninstalled, f)
	}
}

func TestFilterUninstalledString(t *testing.T) {
	if got := FilterUninstalled.String(); got != "Uninstalled" {
		t.Errorf("expected %q, got %q", "Uninstalled", got)
	}
}

func TestParseFilterGroupConflicts(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		wantErr bool
	}{
		{
			name:    "formulae and casks conflict",
			filters: []string{"Formulae", "Casks"},
			wantErr: true,
		},
		{
			name:    "installed and uninstalled conflict",
			filters: []string{"Installed", "Uninstalled"},
			wantErr: true,
		},
		{
			name:    "explicitly installed and uninstalled conflict",
			filters: []string{"Expl. Installed", "Uninstalled"},
			wantErr: true,
		},
		{
			name:    "outdated and uninstalled conflict",
			filters: []string{"Outdated", "Uninstalled"},
			wantErr: true,
		},
		{
			name:    "installed and outdated can be combined",
			filters: []string{"Installed", "Outdated"},
		},
		{
			name:    "uninstalled and active can be combined",
			filters: []string{"Uninstalled", "Active"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFilterGroup(tt.filters)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for filters %v, got nil", tt.filters)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for filters %v: %v", tt.filters, err)
			}
		})
	}
}

func TestToggleUninstalledClearsConflictingFilters(t *testing.T) {
	var fg filterGroup
	fg.enableFilter(FilterInstalled)
	fg.enableFilter(FilterOutdated)
	fg.toggleFilter(FilterUninstalled)

	if got, want := fg.split(), []Filter{FilterUninstalled}; !slices.Equal(got, want) {
		t.Errorf("expected filters %v, got %v", want, got)
	}
}

func TestToggleInstalledClearsUninstalled(t *testing.T) {
	var fg filterGroup
	fg.enableFilter(FilterUninstalled)
	fg.toggleFilter(FilterInstalled)

	if got, want := fg.split(), []Filter{FilterInstalled}; !slices.Equal(got, want) {
		t.Errorf("expected filters %v, got %v", want, got)
	}
}

func TestInstalledAndOutdatedCanBeCombined(t *testing.T) {
	var fg filterGroup
	fg.enableFilter(FilterOutdated)
	fg.toggleFilter(FilterInstalled)

	// Enabling Installed must not clear Outdated, since they can be combined.
	if got, want := fg.split(), []Filter{FilterInstalled, FilterOutdated}; !slices.Equal(got, want) {
		t.Errorf("expected filters %v, got %v", want, got)
	}
}

func TestToggleOutdatedClearsUninstalled(t *testing.T) {
	var fg filterGroup
	fg.enableFilter(FilterUninstalled)
	fg.toggleFilter(FilterOutdated)

	if got, want := fg.split(), []Filter{FilterOutdated}; !slices.Equal(got, want) {
		t.Errorf("expected filters %v, got %v", want, got)
	}
}

func TestToggleUninstalledOff(t *testing.T) {
	var fg filterGroup
	fg.enableFilter(FilterUninstalled)
	fg.toggleFilter(FilterUninstalled)

	if got := fg.split(); len(got) != 0 {
		t.Errorf("expected no filters, got %v", got)
	}
}
