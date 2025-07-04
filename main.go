package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
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
