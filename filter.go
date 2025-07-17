package main

import "strings"

// filter defines which subset of packages is currently being viewed.
type filter uint

// filterGroup is a bitmask collection of multiple 'filter'
type filterGroup uint

const (
	emptyFilterGroup filterGroup = 0
)

// Filters are individual bit flags
const (
	filterFormulae            filter = 1 << iota // 0000 0001
	filterCasks                                  // 0000 0010
	filterInstalled                              // 0000 0100
	filterOutdated                               // 0000 1000
	filterExplicitlyInstalled                    // 0001 0000
	filterActive                                 // 0010 0000

	filterMax
)

// Mutually exclusive filter groups
// Filters from different groups can co-exist
var conflictFilters = []filterGroup{
	filterGroup(filterFormulae | filterCasks),
	filterGroup(filterInstalled | filterOutdated | filterExplicitlyInstalled | filterActive),
}

func (f filter) getConflictFilters() filterGroup {
	for _, fg := range conflictFilters {
		if fg.isFilterEnabled(f) {
			return fg
		}
	}
	return emptyFilterGroup
}

func (fg filterGroup) isFilterEnabled(f filter) bool {
	return fg&filterGroup(f) != emptyFilterGroup
}

func (fg *filterGroup) reset() filterGroup {
	*fg = emptyFilterGroup
	return *fg
}

func (fg *filterGroup) enableFilter(f filter) filterGroup {
	// Clear all conflicting filters
	*fg &= ^f.getConflictFilters()
	// Enable the filter
	*fg |= filterGroup(f)
	return *fg
}

func (fg *filterGroup) disableFilter(f filter) filterGroup {
	*fg &= ^filterGroup(f)
	return *fg
}

func (fg *filterGroup) toggleFilter(f filter) filterGroup {
	filterEnabled := fg.isFilterEnabled(f)
	if filterEnabled {
		return fg.disableFilter(f)
	} else {
		return fg.enableFilter(f)
	}
}

func (fg filterGroup) split() []filter {
	filters := []filter{}
	for i := uint(1); i < uint(filterMax); i <<= 1 {
		if fg.isFilterEnabled(filter(i)) {
			filters = append(filters, filter(i))
		}
	}
	return filters
}

func (f filter) String() string {
	switch f {
	case filterFormulae:
		return "Formulae"
	case filterCasks:
		return "Casks"
	case filterInstalled:
		return "Installed"
	case filterOutdated:
		return "Outdated"
	case filterExplicitlyInstalled:
		return "Expl. Installed"
	case filterActive:
		return "Active"
	default:
		return "Unknown"
	}
}

func (fg filterGroup) String() string {
	if fg == emptyFilterGroup {
		return "None"
	} else {
		filters := fg.split()
		fStrs := make([]string, len(filters))
		for i, f := range filters {
			fStrs[i] = f.String()
		}
		return strings.Join(fStrs, " & ")
	}
}
