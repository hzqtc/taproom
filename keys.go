package main

import "github.com/charmbracelet/bubbles/key"

// keyMap defines the keybindings for the application.
type keyMap struct {
	// General
	SwitchFocus key.Binding
	Refresh     key.Binding
	Quit        key.Binding

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

	// Brew Commands
	Upgrade    key.Binding
	UpgradeAll key.Binding
	Install    key.Binding
	Remove     key.Binding
	Pin        key.Binding
	Unpin      key.Binding
}

// defaultKeyMap returns a map of default keybindings.
func defaultKeyMap() keyMap {
	return keyMap{
		// General
		SwitchFocus: key.NewBinding(key.WithKeys("tab")),
		Refresh:     key.NewBinding(key.WithKeys("r")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c")),

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

		// Brew Commands
		Upgrade:    key.NewBinding(key.WithKeys("u")),
		UpgradeAll: key.NewBinding(key.WithKeys("U")),
		Install:    key.NewBinding(key.WithKeys("t")),
		Remove:     key.NewBinding(key.WithKeys("x")),
		Pin:        key.NewBinding(key.WithKeys("p")),
		Unpin:      key.NewBinding(key.WithKeys("P")),
	}
}
