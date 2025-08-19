package model

import (
	"fmt"
	"math/bits"
	"strings"
)

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
	filterUnknown
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

func parseFilter(s string) (filter, error) {
	switch s {
	case "Formulae":
		return filterFormulae, nil
	case "Casks":
		return filterCasks, nil
	case "Installed":
		return filterInstalled, nil
	case "Outdated":
		return filterOutdated, nil
	case "Expl. Installed":
		return filterExplicitlyInstalled, nil
	case "Active":
		return filterActive, nil
	default:
		return filterUnknown, fmt.Errorf("Unknown filter: %s", s)
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

func parseFilterGroup(strs []string) (filterGroup, error) {
	filters := uint(0)
	for _, s := range strs {
		f, err := parseFilter(s)
		if err != nil {
			return filterGroup(filterUnknown), err
		} else {
			// Enable all filters; will check for conflict later
			filters |= uint(f)
		}
	}

	if conflict, fg := hasConflict(filters); conflict {
		return filterGroup(filterUnknown), fmt.Errorf("Conflicting filters: %s", fg.String())
	} else {
		return filterGroup(filters), nil
	}
}

func hasConflict(filters uint) (bool, filterGroup) {
	// A filter bitmask has conflict if it contains more than 1 filter in conflicting filter groups
	for _, fg := range conflictFilters {
		filtersInGroup := filters & uint(fg)
		if bits.OnesCount(filtersInGroup) > 1 {
			return true, filterGroup(filtersInGroup)
		}
	}
	return false, 0
}
