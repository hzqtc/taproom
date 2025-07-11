package main

import "github.com/charmbracelet/bubbles/key"

// keyMap defines the keybindings for the application.
type keyMap struct {
	// General
	SwitchFocus key.Binding
	FocusSearch key.Binding
	Enter       key.Binding
	Esc         key.Binding
	Refresh     key.Binding
	Quit        key.Binding

	// Filtering & Sorting
	SortByNext      key.Binding
	SortByPrev      key.Binding
	FilterAll       key.Binding
	FilterFormulae  key.Binding
	FilterCasks     key.Binding
	FilterInstalled key.Binding
	FilterOutdated  key.Binding
	FilterExplicit  key.Binding
	FilterDisabled  key.Binding

	// Package Commands
	OpenHomePage key.Binding
	Upgrade      key.Binding
	UpgradeAll   key.Binding
	Install      key.Binding
	Remove       key.Binding
	Pin          key.Binding
	Unpin        key.Binding
}

// defaultKeyMap returns a map of default keybindings.
func defaultKeyMap() keyMap {
	return keyMap{
		// General
		SwitchFocus: key.NewBinding(key.WithKeys("tab")),
		FocusSearch: key.NewBinding(key.WithKeys("/")),
		Enter:       key.NewBinding(key.WithKeys("enter")),
		Esc:         key.NewBinding(key.WithKeys("esc")),
		Refresh:     key.NewBinding(key.WithKeys("r")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c")),

		// Sorting & Filtering
		SortByNext:      key.NewBinding(key.WithKeys("s")),
		SortByPrev:      key.NewBinding(key.WithKeys("S")),
		FilterAll:       key.NewBinding(key.WithKeys("a")),
		FilterFormulae:  key.NewBinding(key.WithKeys("f")),
		FilterCasks:     key.NewBinding(key.WithKeys("c")),
		FilterInstalled: key.NewBinding(key.WithKeys("i")),
		FilterOutdated:  key.NewBinding(key.WithKeys("o")),
		FilterExplicit:  key.NewBinding(key.WithKeys("e")),
		FilterDisabled:  key.NewBinding(key.WithKeys("d")),

		// Package Commands
		OpenHomePage: key.NewBinding(key.WithKeys("b")),
		Upgrade:      key.NewBinding(key.WithKeys("u")),
		UpgradeAll:   key.NewBinding(key.WithKeys("U")),
		Install:      key.NewBinding(key.WithKeys("t")),
		Remove:       key.NewBinding(key.WithKeys("x")),
		Pin:          key.NewBinding(key.WithKeys("p")),
		Unpin:        key.NewBinding(key.WithKeys("P")),
	}
}
