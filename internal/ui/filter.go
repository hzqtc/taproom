package ui

import (
	"fmt"
	"math/bits"
	"strings"
)

// Filter defines which subset of packages is currently being viewed.
type Filter uint

// filterGroup is a bitmask collection of multiple 'filter'
type filterGroup uint

const (
	emptyFilterGroup filterGroup = 0
)

// Filters are individual bit flags
const (
	FilterFormulae            Filter = 1 << iota // 0000 0001
	FilterCasks                                  // 0000 0010
	FilterInstalled                              // 0000 0100
	FilterUninstalled                            // 0000 1000
	FilterOutdated                               // 0001 0000
	FilterExplicitlyInstalled                    // 0010 0000
	FilterActive                                 // 0100 0000

	filterMax
	filterUnknown
)

// Mutually exclusive filter groups
// Filters from different groups can co-exist
// A filter can belong to more than one group; all groups it belongs to are
// considered conflicting. This lets Outdated conflict with Uninstalled without
// conflicting with Installed, since the two can meaningfully be combined.
var conflictFilters = []filterGroup{
	filterGroup(FilterFormulae | FilterCasks),
	filterGroup(FilterInstalled | FilterExplicitlyInstalled | FilterUninstalled),
	// Outdated packages are installed by definition
	filterGroup(FilterOutdated | FilterUninstalled),
}

func (f Filter) getConflictFilters() filterGroup {
	conflicts := emptyFilterGroup
	for _, fg := range conflictFilters {
		if fg.isFilterEnabled(f) {
			conflicts |= fg
		}
	}
	return conflicts
}

func (fg filterGroup) isFilterEnabled(f Filter) bool {
	return fg&filterGroup(f) != emptyFilterGroup
}

func (fg *filterGroup) reset() filterGroup {
	*fg = emptyFilterGroup
	return *fg
}

func (fg *filterGroup) enableFilter(f Filter) filterGroup {
	// Clear all conflicting filters
	*fg &= ^f.getConflictFilters()
	// Enable the filter
	*fg |= filterGroup(f)
	return *fg
}

func (fg *filterGroup) disableFilter(f Filter) filterGroup {
	*fg &= ^filterGroup(f)
	return *fg
}

func (fg *filterGroup) toggleFilter(f Filter) filterGroup {
	filterEnabled := fg.isFilterEnabled(f)
	if filterEnabled {
		return fg.disableFilter(f)
	} else {
		return fg.enableFilter(f)
	}
}

func (fg filterGroup) split() []Filter {
	filters := []Filter{}
	for i := uint(1); i < uint(filterMax); i <<= 1 {
		if fg.isFilterEnabled(Filter(i)) {
			filters = append(filters, Filter(i))
		}
	}
	return filters
}

func (f Filter) String() string {
	switch f {
	case FilterFormulae:
		return "Formulae"
	case FilterCasks:
		return "Casks"
	case FilterInstalled:
		return "Installed"
	case FilterUninstalled:
		return "Uninstalled"
	case FilterOutdated:
		return "Outdated"
	case FilterExplicitlyInstalled:
		return "Expl. Installed"
	case FilterActive:
		return "Active"
	default:
		return "Unknown"
	}
}

func parseFilter(s string) (Filter, error) {
	switch s {
	case "Formulae":
		return FilterFormulae, nil
	case "Casks":
		return FilterCasks, nil
	case "Installed":
		return FilterInstalled, nil
	case "Uninstalled":
		return FilterUninstalled, nil
	case "Outdated":
		return FilterOutdated, nil
	case "Expl. Installed":
		return FilterExplicitlyInstalled, nil
	case "Active":
		return FilterActive, nil
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
