package ui

import (
	"fmt"
	"taproom/internal/data"
)

type packageTableColumn int

const (
	colUnknown packageTableColumn = -1
)

const (
	colSymbol      packageTableColumn = iota // Symbol to differentiate formula vs cask
	colName                                  // Name of the formula or token of the cask, unique identifier when combine with IsCask
	colVersion                               // Newest version
	colTap                                   // Homebrew tap
	colDescription                           // Brief description
	colInstalls                              // Number of installs in the last 90 days
	colSize                                  // Size of the package on disk
	colStatus                                // Calculated status such as deprecated, installed, outdated, pinned

	totalNumColumns
)

var colWidthMap = map[packageTableColumn]int{
	colSymbol:      1,
	colName:        15,
	colVersion:     15,
	colTap:         15,
	colDescription: 30,
	colInstalls:    10,
	colSize:        8,
	colStatus:      15,
}

func (c packageTableColumn) String() string {
	switch c {
	case colSymbol:
		return " "
	case colName:
		return "Name"
	case colVersion:
		return "Version"
	case colTap:
		return "Tap"
	case colDescription:
		return "Description"
	case colInstalls:
		return "Installs"
	case colSize:
		return "Size"
	case colStatus:
		return "Status"
	default:
		return "Unknown"
	}
}

func parseColumnName(name string) (packageTableColumn, error) {
	switch name {
	case "Name":
		return colName, nil
	case "Version":
		return colVersion, nil
	case "Tap":
		return colTap, nil
	case "Description":
		return colDescription, nil
	case "Installs":
		return colInstalls, nil
	case "Size":
		return colSize, nil
	case "Status":
		return colStatus, nil
	default:
		return colUnknown, fmt.Errorf("Unknown column: %s", name)
	}
}

func (c packageTableColumn) hideable() bool {
	return c != colSymbol && c != colName
}

func (c packageTableColumn) sortable() bool {
	return c == colName || c == colTap || c == colInstalls || c == colSize || c == colStatus
}

func (c packageTableColumn) reverseSort() bool {
	return c == colInstalls || c == colSize
}

func (c packageTableColumn) rightAligned() bool {
	return c == colInstalls || c == colSize
}

func (c packageTableColumn) width() int {
	return colWidthMap[c]
}

func (c packageTableColumn) nextColumn() packageTableColumn {
	return packageTableColumn((int(c) + 1) % int(totalNumColumns))
}

func (c packageTableColumn) prevColumn() packageTableColumn {
	return packageTableColumn((int(c) - 1 + int(totalNumColumns)) % int(totalNumColumns))
}

func (c packageTableColumn) getColumnData(pkg *data.Package) string {
	switch c {
	case colSymbol:
		return pkg.Symbol()
	case colName:
		return pkg.Name
	case colVersion:
		return pkg.ShortVersion()
	case colTap:
		return pkg.Tap
	case colDescription:
		return pkg.Desc
	case colInstalls:
		return fmt.Sprintf("%d", pkg.Installs90d)
	case colSize:
		if pkg.IsInstalled {
			return pkg.FormattedSize
		} else {
			return "N/A"
		}
	case colStatus:
		return pkg.Status()
	default:
		return ""
	}
}
