package main

import "github.com/charmbracelet/bubbles/key"

// keyMap defines the keybindings for the application.
type keyMap struct {
	// Navigation
	Up         key.Binding
	Down       key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	GoToTop    key.Binding
	GoToBottom key.Binding

	// General
	Quit key.Binding

	// Search
	FocusSearch key.Binding
	ClearSearch key.Binding
	ExitSearch  key.Binding

	// Filtering & Sorting
	ToggleSort      key.Binding
	FilterAll       key.Binding
	FilterFormulae  key.Binding
	FilterCasks     key.Binding
	FilterInstalled key.Binding
	FilterOutdated  key.Binding
	FilterExplicit  key.Binding
}

// defaultKeyMap returns a map of default keybindings.
func defaultKeyMap() keyMap {
	return keyMap{
		// Navigation
		Up:         key.NewBinding(key.WithKeys("up", "k")),
		Down:       key.NewBinding(key.WithKeys("down", "j")),
		PageUp:     key.NewBinding(key.WithKeys("pgup")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown")),
		GoToTop:    key.NewBinding(key.WithKeys("g", "home")),
		GoToBottom: key.NewBinding(key.WithKeys("G", "end")),

		// General
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c")),

		// Search
		FocusSearch: key.NewBinding(key.WithKeys("/")),
		ClearSearch: key.NewBinding(key.WithKeys("esc")),
		ExitSearch:  key.NewBinding(key.WithKeys("enter")),

		// Sorting & Filtering
		ToggleSort:      key.NewBinding(key.WithKeys("s")),
		FilterAll:       key.NewBinding(key.WithKeys("a")),
		FilterFormulae:  key.NewBinding(key.WithKeys("f")),
		FilterCasks:     key.NewBinding(key.WithKeys("c")),
		FilterInstalled: key.NewBinding(key.WithKeys("i")),
		FilterOutdated:  key.NewBinding(key.WithKeys("o")),
		FilterExplicit:  key.NewBinding(key.WithKeys("e")),
	}
}
