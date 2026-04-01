package webhook

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// requestMsg is sent to the Bubble Tea program when a new request arrives.
type requestMsg Request


// Model is the Bubble Tea model for the webhook catch TUI.
type Model struct {
	requests  []Request
	viewport  viewport.Model
	width     int
	height    int
	webhookURL string
	publicURL  string // non-empty if Cloudflare tunnel is active
	ready      bool
}

// NewModel creates a new TUI model.
func NewModel(webhookURL, publicURL string) Model {
	return Model{
		webhookURL: webhookURL,
		publicURL:  publicURL,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 6
		footerH := 2
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerH-footerH)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerH - footerH
		}
		m.viewport.SetContent(m.renderRequests())
		m.viewport.GotoBottom()
	case requestMsg:
		m.requests = append(m.requests, Request(msg))
		if m.ready {
			m.viewport.SetContent(m.renderRequests())
			m.viewport.GotoBottom()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.header(), m.viewport.View(), m.footer())
}

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	stylePanelTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79C0FF"))
	styleURL        = lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")).Bold(true)
	styleMuted      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	styleBorder     = lipgloss.NewStyle().Foreground(lipgloss.Color("#30363D"))
	styleMethod     = map[string]lipgloss.Style{
		"GET":    lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")).Bold(true),
		"POST":   lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true),
		"PUT":    lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")).Bold(true),
		"PATCH":  lipgloss.NewStyle().Foreground(lipgloss.Color("#D2A8FF")).Bold(true),
		"DELETE": lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true),
	}
	styleJSON    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A5D6FF"))
	styleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	styleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("#21262D"))
)

func (m Model) header() string {
	title := stylePanelTitle.Render("devx webhook catch")
	local := styleURL.Render(m.webhookURL)
	count := styleMuted.Render(fmt.Sprintf("%d request(s) captured", len(m.requests)))

	lines := []string{
		title + "  " + styleMuted.Render("│  q to quit"),
		"",
		"  Local:   " + local,
	}
	if m.publicURL != "" {
		lines = append(lines, "  Public:  "+styleURL.Render(m.publicURL))
	}
	lines = append(lines, "")
	lines = append(lines, "  "+count)
	return strings.Join(lines, "\n")
}

func (m Model) footer() string {
	return styleBorder.Render(strings.Repeat("─", m.width))
}

func (m Model) renderRequests() string {
	if len(m.requests) == 0 {
		return styleMuted.Render("\n  Waiting for requests...\n  Point your app at " + m.webhookURL)
	}

	var sb strings.Builder
	for i, req := range m.requests {
		sb.WriteString(m.renderRequest(i+1, req))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) renderRequest(n int, req Request) string {
	var sb strings.Builder

	// ── Header row ──────────────────────────────────────────────────────────
	methodStyle, ok := styleMethod[req.Method]
	if !ok {
		methodStyle = lipgloss.NewStyle().Bold(true)
	}
	method := methodStyle.Render(fmt.Sprintf("%-7s", req.Method))
	path := lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3")).Render(req.Path)
	ts := styleMuted.Render(req.Time.Format("15:04:05.000"))
	dur := styleMuted.Render(fmt.Sprintf("%dms", req.Duration.Milliseconds()))

	sb.WriteString(fmt.Sprintf("\n  %s  %s  %s  %s  %s\n",
		styleMuted.Render(fmt.Sprintf("#%-3d", n)),
		ts,
		method,
		path,
		dur,
	))

	// ── Headers ─────────────────────────────────────────────────────────────
	importantHeaders := []string{
		"Content-Type", "Content-Length", "Authorization",
		"X-Signature", "X-Signature-256", "Stripe-Signature",
		"X-Hub-Signature", "X-Hook", "X-Event-Type",
	}
	for _, h := range importantHeaders {
		if v, ok := req.Headers[h]; ok {
			// Redact auth tokens
			if h == "Authorization" && len(v) > 20 {
				v = v[:12] + "..." + styleMuted.Render(" (redacted)")
			}
			sb.WriteString(fmt.Sprintf("       %s %s\n",
				styleHeader.Render(fmt.Sprintf("%-25s", h+":")),
				styleMuted.Render(v),
			))
		}
	}

	// ── Body ────────────────────────────────────────────────────────────────
	if req.BodyJSON != "" {
		// Syntax-highlight JSON by colorising keys
		highlighted := highlightJSON(req.BodyJSON)
		// Indent each line
		for _, line := range strings.Split(highlighted, "\n") {
			sb.WriteString("       " + line + "\n")
		}
	} else if req.Body != "" {
		truncated := req.Body
		if len(truncated) > 500 {
			truncated = truncated[:500] + "…"
		}
		sb.WriteString("       " + styleMuted.Render(truncated) + "\n")
	} else {
		sb.WriteString("       " + styleMuted.Render("(empty body)") + "\n")
	}

	// ── Divider ─────────────────────────────────────────────────────────────
	sb.WriteString("  " + styleDivider.Render(strings.Repeat("╌", 70)) + "\n")

	return sb.String()
}

// highlightJSON does minimal syntax colouring of a JSON string:
// - string keys in cyan
// - string values in green
// - numbers/booleans/null in yellow
func highlightJSON(s string) string {
	colorKey := lipgloss.Color("#79C0FF")   // cyan
	colorStr := lipgloss.Color("#A5D6FF")   // light blue
	colorVal := lipgloss.Color("#E3B341")   // yellow

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// A very lightweight approach: colour the key part and the value part
		// This avoids pulling in a full JSON lexer dependency.
		colonIdx := strings.Index(line, `": `)
		if colonIdx > 0 && strings.Contains(line[:colonIdx], `"`) {
			// Has a key
			keyPart := line[:colonIdx+1]
			valPart := line[colonIdx+2:]
			keyStyled := lipgloss.NewStyle().Foreground(colorKey).Render(keyPart)
			var valStyled string
			vt := strings.TrimSpace(valPart)
			if strings.HasPrefix(vt, `"`) {
				valStyled = " " + lipgloss.NewStyle().Foreground(colorStr).Render(valPart)
			} else {
				valStyled = " " + lipgloss.NewStyle().Foreground(colorVal).Render(valPart)
			}
			out = append(out, keyStyled+valStyled)
		} else {
			// Structural line (brace/bracket) — dim
			out = append(out, styleJSON.Render(line))
		}
	}
	return strings.Join(out, "\n")
}

// SendRequest is a helper for goroutines that need to push a request into the
// Bubble Tea program from outside the model.
func SendRequest(p *tea.Program, r Request) {
	p.Send(requestMsg(r))
}
