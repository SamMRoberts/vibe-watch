package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name       string
	IsDark     bool
	Shell      lipgloss.Style
	Header     lipgloss.Style
	Tab        lipgloss.Style
	ActiveTab  lipgloss.Style
	Panel      lipgloss.Style
	Title      lipgloss.Style
	Muted      lipgloss.Style
	Selected   lipgloss.Style
	User       lipgloss.Style
	Assistant  lipgloss.Style
	Tool       lipgloss.Style
	Diagnostic lipgloss.Style
	Help       lipgloss.Style
}

func Names() []string {
	return []string{"auto", "dark", "light", "mono"}
}

func Resolve(name string) Theme {
	switch strings.ToLower(name) {
	case "light":
		return light()
	case "mono":
		return mono()
	case "dark", "auto", "":
		return dark(name)
	default:
		return dark("auto")
	}
}

func Next(name string) string {
	names := Names()
	for i, candidate := range names {
		if candidate == name {
			return names[(i+1)%len(names)]
		}
	}
	return "dark"
}

func dark(name string) Theme {
	if name == "" {
		name = "dark"
	}
	return build(name, true, "#D7E7FF", "#78DCE8", "#A9DC76", "#FFD866", "#FF6188", "#72798A", "#1C1F26", "#2A2F3A")
}

func light() Theme {
	return build("light", false, "#102033", "#006D77", "#2A7F3F", "#8A5A00", "#B00020", "#5A6472", "#F7F8FA", "#E6EEF6")
}

func mono() Theme {
	return build("mono", false, "#FFFFFF", "#FFFFFF", "#DDDDDD", "#CCCCCC", "#FFFFFF", "#AAAAAA", "#000000", "#222222")
}

func build(name string, isDark bool, fg, accent, good, warn, bad, muted, bg, panel string) Theme {
	return Theme{
		Name:   name,
		IsDark: isDark,
		Shell: lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg)).
			Background(lipgloss.Color(bg)),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(fg)).
			Background(lipgloss.Color(panel)).
			Padding(0, 1),
		Tab: lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)).
			Padding(0, 1),
		ActiveTab: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(bg)).
			Background(lipgloss.Color(accent)).
			Padding(0, 1),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accent)).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accent)),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(good)),
		User: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)).
			Bold(true),
		Assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color(good)).
			Bold(true),
		Tool: lipgloss.NewStyle().
			Foreground(lipgloss.Color(warn)).
			Bold(true),
		Diagnostic: lipgloss.NewStyle().
			Foreground(lipgloss.Color(bad)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)),
	}
}
