package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPurple     = lipgloss.Color("#7D56F4")
	ColorGreen      = lipgloss.Color("#73F59F")
	ColorRed        = lipgloss.Color("#FF6B6B")
	ColorYellow     = lipgloss.Color("#FFDD57")
	ColorBlue       = lipgloss.Color("#7DC4E4")
	ColorDimmed     = lipgloss.Color("#626262")
	ColorBorder     = lipgloss.Color("#3C3C3C")
	ColorBorderFocus = lipgloss.Color("#7D56F4")
	ColorBg         = lipgloss.Color("#1E1E2E")
	ColorFg         = lipgloss.Color("#CDD6F4")
	ColorSelected   = lipgloss.Color("#313244")

	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)

	PanelFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderFocus)

	PanelTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorFg).
		Background(ColorPurple).
		Padding(0, 1)

	PanelTitleFocusedStyle = PanelTitleStyle.Copy().
		Background(ColorGreen).
		Foreground(lipgloss.Color("#1E1E2E"))

	ListItemStyle = lipgloss.NewStyle().
		Foreground(ColorFg).
		PaddingLeft(2)

	ListItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorFg).
		Background(ColorSelected).
		Bold(true).
		PaddingLeft(1)

	ListItemActiveStyle = lipgloss.NewStyle().
		Foreground(ColorGreen).
		Bold(true).
		PaddingLeft(2)

	TabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGreen).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ColorGreen).
		Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Padding(0, 2)

	LabelStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed)

	ValueStyle = lipgloss.NewStyle().
		Foreground(ColorFg)

	ValueHighlightStyle = lipgloss.NewStyle().
		Foreground(ColorGreen).
		Bold(true)

	InputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	InputFocusedStyle = InputStyle.Copy().
		BorderForeground(ColorGreen)

	StatusConnectedStyle = lipgloss.NewStyle().
		Foreground(ColorGreen).
		Bold(true)

	StatusDisconnectedStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed)

	StatusErrorStyle = lipgloss.NewStyle().
		Foreground(ColorRed).
		Bold(true)

	StatusLoadingStyle = lipgloss.NewStyle().
		Foreground(ColorYellow)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed)

	HelpBarStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Background(lipgloss.Color("#181825")).
		Padding(0, 1)

	LogTimeStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed)

	LogInfoStyle = lipgloss.NewStyle().
		Foreground(ColorBlue)

	LogErrorStyle = lipgloss.NewStyle().
		Foreground(ColorRed)

	LogSuccessStyle = lipgloss.NewStyle().
		Foreground(ColorGreen)

	PopupStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGreen).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 2)

	PopupTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorGreen)

	PopupItemStyle = lipgloss.NewStyle().
		Foreground(ColorFg)

	PopupItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorGreen).
		Bold(true)

	PopupHintStyle = lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Italic(true)
)

const (
	IconConnected    = "●"
	IconDisconnected = "○"
	IconLoading      = "◐"
	IconError        = "✗"
	IconSuccess      = "✓"
	IconArrowRight   = "→"
	IconArrowDown    = "▼"
	IconDatabase     = "◆"
	IconCache        = "◈"
	IconServer       = "■"
)
