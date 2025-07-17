package main

import (
	"fmt"
)

// columnName is a type for identifying table columns.
type columnName int

const (
	colUnknown columnName = -1
)

const (
	colSymbol      columnName = iota // Symbol to differentiate formula vs cask
	colName                          // Name of the formula or token of the cask, unique identifier when combine with IsCask
	colVersion                       // Newest version
	colTap                           // Homebrew tap
	colDescription                   // Brief description
	colInstalls                      // Number of installs in the last 90 days
	colSize                          // Size of the package on disk
	colStatus                        // Calculated status such as deprecated, installed, outdated, pinned

	totalNumColumns
)

func (c columnName) String() string {
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

func parseColumnName(name string) (columnName, error) {
	switch name {
	// Name and Symbol columns can not be customized
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

func (c columnName) Sortable() bool {
	return c == colName || c == colTap || c == colInstalls || c == colSize || c == colStatus
}

func (c columnName) ReverseSort() bool {
	return c == colInstalls || c == colSize
}

func (c columnName) RightAligned() bool {
	return c == colInstalls || c == colSize
}
