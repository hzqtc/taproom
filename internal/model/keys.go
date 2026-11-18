package model

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

	// Package Commands
	OpenHomePage key.Binding
	OpenBrewUrl  key.Binding
	OpenRelease  key.Binding
	Upgrade      key.Binding
	UpgradeAll   key.Binding
	Install      key.Binding
	Remove       key.Binding
	Pin          key.Binding
	Unpin        key.Binding
	CleanUp      key.Binding
}

// defaultKeyMap returns a map of default keybindings.
func defaultKeyMap() keyMap {
	return keyMap{
		// General
		SwitchFocus: key.NewBinding(key.WithKeys("tab")),
		FocusSearch: key.NewBinding(key.WithKeys("/")),
		Enter:       key.NewBinding(key.WithKeys("enter")),
		Esc:         key.NewBinding(key.WithKeys("esc")),
		Refresh:     key.NewBinding(key.WithKeys("R")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c")),

		// Package Commands
		OpenHomePage: key.NewBinding(key.WithKeys("h")),
		OpenBrewUrl:  key.NewBinding(key.WithKeys("b")),
		OpenRelease:  key.NewBinding(key.WithKeys("r")),
		Upgrade:      key.NewBinding(key.WithKeys("u")),
		UpgradeAll:   key.NewBinding(key.WithKeys("U")),
		Install:      key.NewBinding(key.WithKeys("t")),
		Remove:       key.NewBinding(key.WithKeys("x")),
		Pin:          key.NewBinding(key.WithKeys("p")),
		Unpin:        key.NewBinding(key.WithKeys("P")),
		CleanUp:      key.NewBinding(key.WithKeys("L")),
	}
}
