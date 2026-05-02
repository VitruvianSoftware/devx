// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorCyan   = lipgloss.Color("#79C0FF")
	colorGreen  = lipgloss.Color("#3FB950")
	colorRed    = lipgloss.Color("#FF7B72")
	colorYellow = lipgloss.Color("#E3B341")
	colorMuted  = lipgloss.Color("#8B949E")
	colorWhite  = lipgloss.Color("#E6EDF3")
	colorBorder = lipgloss.Color("#30363D")

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
