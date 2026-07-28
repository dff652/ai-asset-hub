package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	header   lipgloss.Style
	selected lipgloss.Style
	muted    lipgloss.Style
	warning  lipgloss.Style
	error    lipgloss.Style
	border   lipgloss.Style
}

func newStyles(plain bool) styles {
	if plain {
		return styles{}
	}
	return styles{
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		selected: lipgloss.NewStyle().Bold(true).Reverse(true),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		border:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}
