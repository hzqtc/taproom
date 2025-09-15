package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"taproom/internal/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

var (
	flagShowVersion = pflag.BoolP("version", "v", false, "Show app version")
	flagShowHelp    = pflag.BoolP("help", "h", false, "Show help message")
)

//go:embed .version
var version string

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	pflag.Parse()

	if *flagShowVersion {
		fmt.Print(version)
		os.Exit(0)
	}

	if *flagShowHelp {
		pflag.Usage()
		os.Exit(0)
	}

	logfile := getEnv("TAPROOM_LOG", "/tmp/taproom.log")
	f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to create log file: %v", err)
	}
	defer f.Close()
	// Send log output to the file
	log.SetOutput(f)

	// The WithAltScreen() option provides a full-screen TUI experience.
	p := tea.NewProgram(model.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
