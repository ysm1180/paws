package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Filter     key.Binding
	Connect    key.Binding
	Disconnect key.Binding
	Refresh    key.Binding
	Quit       key.Binding
	Back       key.Binding
	PathBar    key.Binding
}

var Keys = KeyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k")),
	Down:       key.NewBinding(key.WithKeys("down", "j")),
	Tab:        key.NewBinding(key.WithKeys("tab")),
	ShiftTab:   key.NewBinding(key.WithKeys("shift+tab")),
	Enter:      key.NewBinding(key.WithKeys("enter")),
	Escape:     key.NewBinding(key.WithKeys("esc")),
	Filter:     key.NewBinding(key.WithKeys("/")),
	Connect:    key.NewBinding(key.WithKeys("c")),
	Disconnect: key.NewBinding(key.WithKeys("d")),
	Refresh:    key.NewBinding(key.WithKeys("r")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c")),
	Back:       key.NewBinding(key.WithKeys("backspace", "h", "left")),
	PathBar:    key.NewBinding(key.WithKeys(":")),
}
