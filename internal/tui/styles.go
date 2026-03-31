package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorCyan    = lipgloss.Color("#79C0FF")
	colorGreen   = lipgloss.Color("#3FB950")
	colorRed     = lipgloss.Color("#FF7B72")
	colorYellow  = lipgloss.Color("#E3B341")
	colorMuted   = lipgloss.Color("#8B949E")
	colorWhite   = lipgloss.Color("#E6EDF3")
	colorBg      = lipgloss.Color("#0D1117")
	colorBorder  = lipgloss.Color("#30363D")

	// Status icons
	IconPending = lipgloss.NewStyle().Foreground(colorMuted).Render("◌")
	IconRunning = lipgloss.NewStyle().Foreground(colorCyan).Render("⣾")
	IconDone    = lipgloss.NewStyle().Foreground(colorGreen).Render("✓")
	IconFailed  = lipgloss.NewStyle().Foreground(colorRed).Render("✗")

	// Text styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			PaddingBottom(1)

	StyleStepName = lipgloss.NewStyle().
			Foreground(colorWhite).
			Width(28)

	StyleDetail = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleDetailRunning = lipgloss.NewStyle().
				Foreground(colorCyan)

	StyleDetailDone = lipgloss.NewStyle().
			Foreground(colorGreen)

	StyleDetailError = lipgloss.NewStyle().
				Foreground(colorRed)

	StyleURL = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Width(60)

	StyleSuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGreen).
			Padding(1, 2).
			Width(60)

	StyleErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 2).
			Width(60)

	StyleLabel = lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(20)

	StyleValue = lipgloss.NewStyle().
			Foreground(colorWhite)

	StyleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	SpinnerStyle = lipgloss.NewStyle().Foreground(colorCyan)
)
