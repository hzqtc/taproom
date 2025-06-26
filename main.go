package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// The WithAltScreen() option provides a full-screen TUI experience.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	// Run the program.
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
