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

package logs

import (
	"context"
	"fmt"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/provider"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()
	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
	colorMap = map[string]lipgloss.Color{}
	colors   = []string{"#FF5F87", "#FFF700", "#00FF00", "#00FFFF", "#FF00FF", "#8A2BE2", "#FFA500"}
	colorIdx = 0
)

func getColor(service string) lipgloss.Color {
	if c, ok := colorMap[service]; ok {
		return c
	}
	colorMap[service] = lipgloss.Color(colors[colorIdx%len(colors)])
	colorIdx++
	return colorMap[service]
}

type Model struct {
	streamer *Streamer
	ctx      context.Context
	cancel   context.CancelFunc

	lines    []LogLine
	viewport viewport.Model
	ready    bool
	width    int
	height   int
}

func InitialModel(rt provider.ContainerRuntime) Model {
	ctx, cancel := context.WithCancel(context.Background())
	st := NewStreamer(rt)
	st.Start(ctx)

	return Model{
		streamer: st,
		ctx:      ctx,
		cancel:   cancel,
		lines:    make([]LogLine, 0, 1000),
	}
}

// InitialModelWithRedactor creates the TUI model with secret redaction enabled.
// All log lines will be scrubbed for known secret values before display.
func InitialModelWithRedactor(r *SecretRedactor, rt provider.ContainerRuntime) Model {
	ctx, cancel := context.WithCancel(context.Background())
	st := NewStreamer(rt)
	st.Redactor = r
	st.Start(ctx)

	return Model{
		streamer: st,
		ctx:      ctx,
		cancel:   cancel,
		lines:    make([]LogLine, 0, 1000),
	}
}

type lineMsg LogLine

func waitForLine(sub chan LogLine) tea.Cmd {
	return func() tea.Msg {
		return lineMsg(<-sub)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForLine(m.streamer.Lines))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			m.cancel() // Stop streamer
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := 3 // title + borders
		footerHeight := 3 // status + borders
		verticalMarginHeight := headerHeight + footerHeight

		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			m.viewport.SetContent(m.renderedLines())
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case lineMsg:
		m.lines = append(m.lines, LogLine(msg))
		if len(m.lines) > 1000 {
			m.lines = m.lines[len(m.lines)-1000:]
		}

		if m.ready {
			wasAtBottom := m.viewport.AtBottom()
			m.viewport.SetContent(m.renderedLines())
			if wasAtBottom {
				m.viewport.GotoBottom()
			}
		}

		cmds = append(cmds, waitForLine(m.streamer.Lines))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) renderedLines() string {
	var sb strings.Builder
	for _, l := range m.lines {
		c := getColor(l.Service)
		style := lipgloss.NewStyle().Foreground(c).Bold(true)
		prefix := style.Render(fmt.Sprintf("[%s]", l.Service))

		// Wrap lines nicely or just truncate. For now just standard format.
		sb.WriteString(fmt.Sprintf("%s %s %s\n", l.Timestamp.Format("15:04:05"), prefix, l.Message))
	}
	return sb.String()
}

func (m Model) headerView() string {
	title := titleStyle.Render("devx Unified Log Multiplexer")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("Lines: %d", len(m.lines)))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing...\n"
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}
