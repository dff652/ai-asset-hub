package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	First        key.Binding
	Last         key.Binding
	Expand       key.Binding
	Collapse     key.Binding
	Filter       key.Binding
	FindingsOnly key.Binding
	Reload       key.Binding
	Select       key.Binding
	Write        key.Binding
	UpdateAsset  key.Binding
	RemoveAsset  key.Binding
	Build        key.Binding
	Publish      key.Binding
	Preflight    key.Binding
	Diff         key.Binding
	Apply        key.Binding
	Doctor       key.Binding
	Rollback     key.Binding
	Version      key.Binding
	CheckUpdate  key.Binding
	Home         key.Binding
	Help         key.Binding
	Quit         key.Binding
	ForceQuit    key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k")),
		Down:         key.NewBinding(key.WithKeys("down", "j")),
		First:        key.NewBinding(key.WithKeys("g", "home")),
		Last:         key.NewBinding(key.WithKeys("G", "end")),
		Expand:       key.NewBinding(key.WithKeys("right", "enter")),
		Collapse:     key.NewBinding(key.WithKeys("left", "esc")),
		Filter:       key.NewBinding(key.WithKeys("/")),
		FindingsOnly: key.NewBinding(key.WithKeys("f")),
		Reload:       key.NewBinding(key.WithKeys("r")),
		Select:       key.NewBinding(key.WithKeys(" ")),
		Write:        key.NewBinding(key.WithKeys("w")),
		UpdateAsset:  key.NewBinding(key.WithKeys("u")),
		RemoveAsset:  key.NewBinding(key.WithKeys("X", "delete")),
		Build:        key.NewBinding(key.WithKeys("b")),
		Publish:      key.NewBinding(key.WithKeys("p")),
		Preflight:    key.NewBinding(key.WithKeys("e")),
		Diff:         key.NewBinding(key.WithKeys("d")),
		Apply:        key.NewBinding(key.WithKeys("a")),
		Doctor:       key.NewBinding(key.WithKeys("h")),
		Rollback:     key.NewBinding(key.WithKeys("x")),
		Version:      key.NewBinding(key.WithKeys("v")),
		CheckUpdate:  key.NewBinding(key.WithKeys("c")),
		Home:         key.NewBinding(key.WithKeys("m")),
		Help:         key.NewBinding(key.WithKeys("?")),
		Quit:         key.NewBinding(key.WithKeys("q")),
		ForceQuit:    key.NewBinding(key.WithKeys("ctrl+c")),
	}
}
