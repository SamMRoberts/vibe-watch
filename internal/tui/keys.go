package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up             key.Binding
	Down           key.Binding
	Enter          key.Binding
	Esc            key.Binding
	Tab            key.Binding
	ShiftTab       key.Binding
	Quit           key.Binding
	Refresh        key.Binding
	Filter         key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	Home           key.Binding
	End            key.Binding
	PreviousPrompt key.Binding
	NextPrompt     key.Binding
	Collapse       key.Binding
	CollapseAll    key.Binding
	DetailLevel    key.Binding
	AnalyticsPanel key.Binding
	Timestamps     key.Binding
	Follow         key.Binding
}

type helpKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (h helpKeyMap) ShortHelp() []key.Binding {
	return h.short
}

func (h helpKeyMap) FullHelp() [][]key.Binding {
	return h.full
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "views"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "views"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "b"),
		key.WithHelp("pgup/b", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "top"),
	),
	End: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "bottom"),
	),
	PreviousPrompt: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "prev prompt"),
	),
	NextPrompt: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next prompt"),
	),
	Collapse: key.NewBinding(
		key.WithKeys(" ", "space"),
		key.WithHelp("space", "collapse"),
	),
	CollapseAll: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "toggle all"),
	),
	DetailLevel: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "detail"),
	),
	AnalyticsPanel: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "analytics"),
	),
	Timestamps: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "time"),
	),
	Follow: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "follow"),
	),
}

func dashboardHelp() helpKeyMap {
	return helpKeyMap{
		short: []key.Binding{keys.Quit, keys.Tab, keys.Up, keys.Down, keys.Enter, keys.Refresh, keys.Filter},
		full: [][]key.Binding{
			{keys.Up, keys.Down, keys.Enter},
			{keys.Tab, keys.ShiftTab, keys.Refresh, keys.Filter},
			{keys.Quit},
		},
	}
}

func detailHelp() helpKeyMap {
	return helpKeyMap{
		short: []key.Binding{keys.Esc, keys.Tab, keys.Enter, keys.Follow},
		full: [][]key.Binding{
			{keys.Up, keys.Down, keys.PreviousPrompt, keys.NextPrompt, keys.Enter},
			{keys.Collapse, keys.CollapseAll, keys.DetailLevel, keys.AnalyticsPanel, keys.Timestamps, keys.Follow},
			{keys.PageUp, keys.PageDown, keys.Home, keys.End, keys.Tab, keys.ShiftTab, keys.Esc},
		},
	}
}

func focusedHelp() helpKeyMap {
	return helpKeyMap{
		short: []key.Binding{keys.Esc, keys.Tab, keys.Up, keys.Down, keys.PageUp, keys.PageDown},
		full: [][]key.Binding{
			{keys.Up, keys.Down, keys.PageUp, keys.PageDown},
			{keys.Home, keys.End, keys.Tab, keys.ShiftTab, keys.Esc},
		},
	}
}

func analyticsHelp() helpKeyMap {
	return helpKeyMap{
		short: []key.Binding{keys.Quit, keys.Tab, keys.Refresh},
		full: [][]key.Binding{
			{keys.Tab, keys.ShiftTab, keys.Refresh},
			{keys.Quit},
		},
	}
}

func filterHelp() helpKeyMap {
	accept := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply"))
	backspace := key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "delete"))
	return helpKeyMap{
		short: []key.Binding{accept, keys.Esc, backspace},
		full:  [][]key.Binding{{accept, keys.Esc, backspace}},
	}
}
