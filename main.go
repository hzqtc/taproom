package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

var (
	flagShowVersion     = pflag.BoolP("version", "v", false, "Show app version")
	flagShowHelp        = pflag.BoolP("help", "h", false, "Show help message")
	flagInvalidateCache = pflag.BoolP("invalidate-cache", "i", false, "Invalidate cache and force re-downloading data")
	flagShowLoadTimer   = pflag.BoolP("load-timer", "t", false, "Show a timer in the loading screen")
	flagHideCols        = pflag.StringSlice(
		"hide-columns",
		[]string{},
		"Hide specific columns seprated by comma (no spaces): Version, Tap, Description, Installs, Size, Status",
	)
	flagSortColumn = pflag.StringP(
		"sort-column",
		"s",
		"Name",
		"Choose which column (Name, Tap, Installs, Size, Status) to sort by initially",
	)
	flagFilters = pflag.StringSliceP(
		"filters",
		"f",
		[]string{},
		"Filters to enable (comma separated no space).\n"+
			"Pick 0 or 1 filter from each group: (Formulae, Casks), (Installed, Outdated, Expl. Installed, Active)",
	)
	flagHideHelp = pflag.Bool("hide-help", false, "Hide the help text")
)

//go:embed .version
var version string

func main() {
	pflag.Parse()

	if *flagShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *flagShowHelp {
		pflag.Usage()
		os.Exit(0)
	}

	f, err := os.OpenFile("/tmp/taproom.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to create log file: %v", err)
	}
	defer f.Close()
	// Send log output to the file
	log.SetOutput(f)

	flags := Flags{
		noCache:       *flagInvalidateCache,
		showLoadTimer: *flagShowLoadTimer,
		hiddenColumns: *flagHideCols,
		sortColumn:    *flagSortColumn,
		filters:       *flagFilters,
		hideHelp:      *flagHideHelp,
	}

	// The WithAltScreen() option provides a full-screen TUI experience.
	p := tea.NewProgram(initialModel(flags), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
