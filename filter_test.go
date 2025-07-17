package main

import (
	"testing"
)

func TestEnableFilter(t *testing.T) {
	var fg filterGroup
	fg = fg.enableFilter(filterFormulae)

	if !fg.isFilterEnabled(filterFormulae) {
		t.Error("Expected filterFormulae to be enabled")
	}
}

func TestEnableMutuallyExclusiveFilters(t *testing.T) {
	var fg filterGroup
	fg = fg.enableFilter(filterFormulae)
	fg = fg.enableFilter(filterCasks)

	if fg.isFilterEnabled(filterFormulae) {
		t.Error("Expected filterFormulae to be disabled after enabling filterCasks")
	}
	if !fg.isFilterEnabled(filterCasks) {
		t.Error("Expected filterCasks to be enabled")
	}
}

func TestEnableIndependentFilters(t *testing.T) {
	var fg filterGroup
	fg = fg.enableFilter(filterFormulae)
	fg = fg.enableFilter(filterInstalled)

	if !fg.isFilterEnabled(filterFormulae) {
		t.Error("Expected filterFormulae to be enabled")
	}
	if !fg.isFilterEnabled(filterInstalled) {
		t.Error("Expected filterInstalled to be enabled")
	}
}

func TestDisableFilter(t *testing.T) {
	var fg filterGroup
	fg = fg.enableFilter(filterOutdated)
	fg = fg.disableFilter(filterOutdated)

	if fg.isFilterEnabled(filterOutdated) {
		t.Error("Expected filterOutdated to be disabled")
	}
}

func TestToggleFilter(t *testing.T) {
	var fg filterGroup

	fg = fg.toggleFilter(filterActive)
	if !fg.isFilterEnabled(filterActive) {
		t.Error("Expected filterActive to be enabled")
	}

	fg = fg.toggleFilter(filterActive)
	if fg.isFilterEnabled(filterActive) {
		t.Error("Expected filterActive to be disabled after toggle")
	}
}

func TestSplit(t *testing.T) {
	var fg filterGroup
	fg = fg.enableFilter(filterFormulae)
	fg = fg.enableFilter(filterInstalled)

	split := fg.split()
	if len(split) != 2 {
		t.Errorf("Expected 2 filters in split, got %d", len(split))
	}
	if split[0] != filterFormulae || split[1] != filterInstalled {
		t.Errorf("Expected split to contain filterFormulae and filterInstalled, got %v", split)
	}
}

func TestStringFilter(t *testing.T) {
	if filterFormulae.String() != "Formulae" {
		t.Errorf("Expected 'Formulae', got %s", filterFormulae.String())
	}
	if filter(0).String() != "Unknown" {
		t.Errorf("Expected 'Unknown', got %s", filter(0).String())
	}
}

func TestStringFilterGroup(t *testing.T) {
	var fg filterGroup
	if fg.String() != "None" {
		t.Errorf("Expected 'None', got %s", fg.String())
	}
	fg = fg.enableFilter(filterCasks)
	fg = fg.enableFilter(filterInstalled)

	str := fg.String()
	if str != "Casks & Installed" {
		t.Errorf("Expected 'Casks & Installed', got %s", str)
	}
}
