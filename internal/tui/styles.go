package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	title    lipgloss.Style
	heading  lipgloss.Style
	muted    lipgloss.Style
	success  lipgloss.Style
	failure  lipgloss.Style
	selected lipgloss.Style
}

func newStyles(color bool) styles {
	if !color {
		return styles{}
	}
	return styles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		heading:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		failure:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
	}
}
