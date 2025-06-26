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
	Help key.Binding
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
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		GoToTop: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/home", "go to top"),
		),
		GoToBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "go to bottom"),
		),

		// General
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),

		// Search
		FocusSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ClearSearch: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear search"),
		),
		ExitSearch: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "exit search"),
		),

		// Sorting & Filtering
		ToggleSort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort: name/popularity"),
		),
		FilterAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all packages"),
		),
		FilterFormulae: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "formulae only"),
		),
		FilterCasks: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "casks only"),
		),
		FilterInstalled: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "installed"),
		),
		FilterOutdated: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "outdated"),
		),
		FilterExplicit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "explicitly installed"),
		),
	}
}

// ShortHelp returns a slice of keybindings for the short help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.FocusSearch, k.ToggleSort}
}

// FullHelp returns a multi-slice of keybindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.GoToTop, k.GoToBottom}, // Navigation
		{k.FocusSearch, k.ClearSearch, k.ToggleSort},                  // Search & Sort
		{k.FilterAll, k.FilterFormulae, k.FilterCasks},                // Filters
		{k.FilterInstalled, k.FilterOutdated, k.FilterExplicit},       // More Filters
		{k.Help, k.Quit}, // General
	}
}
