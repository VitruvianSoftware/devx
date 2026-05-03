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

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepStatus represents the lifecycle state of a setup step.
type StepStatus int

const (
	Pending StepStatus = iota
	Running
	Done
	Failed
)

// Step holds the display state for a single setup stage.
type Step struct {
	Name   string
	Status StepStatus
	Detail string
}

// ProgressMsg is sent from the setup goroutine to advance TUI state.
type ProgressMsg struct {
	Index  int
	Status StepStatus
	Detail string
}

// SetupResult is sent when all steps have completed successfully.
type SetupResult struct {
	Domain   string
	Hostname string
	AuthURL  string // non-empty if Tailscale needs manual auth
}

// SetupCompleteMsg signals that setup finished (success or fail).
type SetupCompleteMsg struct {
	Result *SetupResult
	Err    error
}

// AuthURLMsg is sent mid-setup when Tailscale emits a login URL.
type AuthURLMsg struct{ URL string }

// SetupModel is the bubbletea model for the `init` command TUI.
type SetupModel struct {
	steps   []Step
	stepFns []func() (string, error) // one closure per step, built externally
	current int
	spinner spinner.Model
	done    bool
	failed  bool
	result  *SetupResult
	authURL string
	width   int
}

// NewSetupModel creates the model. stepNames and stepFns must be the same length.
func NewSetupModel(stepNames []string, stepFns []func() (string, error)) SetupModel {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = SpinnerStyle

	steps := make([]Step, len(stepNames))
	for i, name := range stepNames {
		steps[i] = Step{Name: name, Status: Pending}
	}
	steps[0].Status = Running

	return SetupModel{
		steps:   steps,
		stepFns: stepFns,
		spinner: s,
	}
}

// runStep executes stepFns[index] as a tea.Cmd in a goroutine.
func runStep(index int, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		detail, err := fn()
		if err != nil {
			return ProgressMsg{Index: index, Status: Failed, Detail: err.Error()}
		}
		return ProgressMsg{Index: index, Status: Done, Detail: detail}
	}
}

func (m SetupModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runStep(0, m.stepFns[0]))
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case AuthURLMsg:
		m.authURL = msg.URL
		// Keep running, waiting for tailscale step to finish
		return m, nil

	case ProgressMsg:
		if msg.Index < len(m.steps) {
			m.steps[msg.Index].Status = msg.Status
			m.steps[msg.Index].Detail = msg.Detail
		}

		if msg.Status == Failed {
			m.failed = true
			m.done = true
			return m, tea.Quit
		}

		next := msg.Index + 1
		if next < len(m.stepFns) {
			m.steps[next].Status = Running
			m.current = next
			return m, tea.Batch(m.spinner.Tick, runStep(next, m.stepFns[next]))
		}

		// All steps complete
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleTitle.Render("🚀  devx — Environment Setup"))
	b.WriteString("\n\n")

	for i, step := range m.steps {
		icon := m.iconFor(step.Status, i)
		name := StyleStepName.Render(step.Name)

		var detail string
		switch step.Status {
		case Running:
			detail = StyleDetailRunning.Render(m.spinner.View() + " " + step.Detail)
		case Done:
			detail = StyleDetailDone.Render(step.Detail)
		case Failed:
			detail = StyleDetailError.Render(step.Detail)
		default:
			detail = StyleMuted.Render("—")
		}

		_, _ = fmt.Fprintf(&b, "  %s  %s  %s\n", icon, name, detail)
	}

	// Auth URL callout
	if m.authURL != "" {
		b.WriteString("\n")
		box := StyleBox.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				StyleDetailRunning.Render("Tailscale Authentication Required"),
				"",
				StyleMuted.Render("Open this URL in your browser:"),
				StyleURL.Render(m.authURL),
			),
		)
		b.WriteString(box)
		b.WriteString("\n")
	}

	if m.done && !m.failed && m.result != nil {
		b.WriteString("\n")
		summary := lipgloss.JoinVertical(lipgloss.Left,
			StyleDetailDone.Render("✓  Setup complete!"),
			"",
			StyleLabel.Render("Public endpoint")+" "+StyleValue.Render("https://"+m.result.Domain),
			StyleLabel.Render("VM hostname")+"   "+StyleValue.Render(m.result.Hostname),
			"",
			StyleMuted.Render("Run any container on port 8080 — it's instantly public."),
			StyleMuted.Render("Example: podman run -d -p 8080:80 docker.io/nginx"),
		)
		b.WriteString(StyleSuccessBox.Render(summary))
		b.WriteString("\n")
	}

	if m.failed {
		b.WriteString("\n")
		b.WriteString(StyleErrorBox.Render(StyleDetailError.Render("Setup failed. See error above.")))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(StyleMuted.Render("  ctrl+c to quit"))
	b.WriteString("\n")
	return b.String()
}

func (m SetupModel) iconFor(status StepStatus, index int) string {
	switch status {
	case Running:
		return IconRunning
	case Done:
		return IconDone
	case Failed:
		return IconFailed
	default:
		return IconPending
	}
}

// SetResult injects the final result into the model (called from cmd layer).
func (m *SetupModel) SetResult(r *SetupResult) {
	m.result = r
}

// IsDone reports whether the setup flow has completed (success or failure).
func (m SetupModel) IsDone() bool { return m.done }

// IsFailed reports whether the setup flow ended in an error.
func (m SetupModel) IsFailed() bool { return m.failed }
