package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

var (
	showHelp        = pflag.BoolP("help", "h", false, "Show help message")
	invalidateCache = pflag.BoolP("invalidate-cache", "i", false, "Invalidate cache and force re-downloading data")
	hiddenCols      = pflag.StringSlice(
		"hide-columns",
		[]string{},
		"Hide specific columns seprated by comma (options: Version, Tap, Description, Installs, Size, Status)",
	)
)

func main() {
	pflag.Parse()

	if *showHelp {
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

	// The WithAltScreen() option provides a full-screen TUI experience.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
