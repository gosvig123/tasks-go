package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e5c07b"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61afef")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#abb2bf"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Faint(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Faint(true)

	// Panel styles for split-screen view
	panelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5c6370")).
				Padding(0, 1)

	focusedPanelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#61afef")).
				Padding(0, 1)

	addPanelTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#98c379"))
)
