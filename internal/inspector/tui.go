package inspector

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	cyan   = lipgloss.Color("#79C0FF")
	green  = lipgloss.Color("#3FB950")
	red    = lipgloss.Color("#FF7B72")
	yellow = lipgloss.Color("#E3B341")
	muted  = lipgloss.Color("#8B949E")
	white  = lipgloss.Color("#E6EDF3")

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	urlStyle    = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	mutedStyle  = lipgloss.NewStyle().Foreground(muted)
	rowStyle    = lipgloss.NewStyle().Foreground(white)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	replayTag   = lipgloss.NewStyle().Foreground(muted).Italic(true)
	flashOK     = lipgloss.NewStyle().Foreground(green)
	flashErr    = lipgloss.NewStyle().Foreground(red)
	sectionHead = lipgloss.NewStyle().Bold(true).Foreground(cyan).MarginTop(1)
)

func statusStyle(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return lipgloss.NewStyle().Foreground(red)
	case code >= 400:
		return lipgloss.NewStyle().Foreground(yellow)
	case code >= 300:
		return lipgloss.NewStyle().Foreground(cyan)
	default:
		return lipgloss.NewStyle().Foreground(green)
	}
}

// ── Messages ────────────────────────────────────────────────────────────────

// CaptureMsg is sent to the TUI when a new exchange is captured.
type CaptureMsg CapturedExchange

type replayDoneMsg struct{ err error }

// ── View States ─────────────────────────────────────────────────────────────

type viewState int

const (
	viewList viewState = iota
	viewDetail
)

// ── Model ───────────────────────────────────────────────────────────────────

// TUIModel is the Bubble Tea model for the inspector interface.
type TUIModel struct {
	exchanges []CapturedExchange
	cursor    int
	scroll    int
	view      viewState
	detailScroll int

	width  int
	height int

	tunnelURL  string
	proxyPort  int
	targetPort string
	proxy      *Proxy

	captureCh chan CapturedExchange
	flash     string
}

// NewTUIModel creates a new inspector TUI.
func NewTUIModel(proxy *Proxy, ch chan CapturedExchange, tunnelURL, targetPort string, proxyPort int) TUIModel {
	return TUIModel{
		proxy:      proxy,
		captureCh:  ch,
		tunnelURL:  tunnelURL,
		targetPort: targetPort,
		proxyPort:  proxyPort,
	}
}

func waitForCapture(ch chan CapturedExchange) tea.Cmd {
	return func() tea.Msg {
		ex, ok := <-ch
		if !ok {
			return nil
		}
		return CaptureMsg(ex)
	}
}

func (m TUIModel) Init() tea.Cmd {
	return waitForCapture(m.captureCh)
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case CaptureMsg:
		m.exchanges = append(m.exchanges, CapturedExchange(msg))
		m.flash = ""
		// Auto-scroll to newest if cursor is already at/near the bottom
		if m.cursor >= len(m.exchanges)-2 {
			m.cursor = len(m.exchanges) - 1
		}
		m.clampScroll()
		return m, waitForCapture(m.captureCh)

	case replayDoneMsg:
		if msg.err != nil {
			m.flash = flashErr.Render("✗ Replay failed: " + msg.err.Error())
		} else {
			m.flash = flashOK.Render("✓ Replayed")
		}

	case tea.KeyMsg:
		switch m.view {
		case viewList:
			return m.updateList(msg)
		case viewDetail:
			return m.updateDetail(msg)
		}
	}
	return m, nil
}

// ── List View Update ────────────────────────────────────────────────────────

func (m TUIModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampScroll()
		}
	case "down", "j":
		if m.cursor < len(m.exchanges)-1 {
			m.cursor++
			m.clampScroll()
		}
	case "G":
		if len(m.exchanges) > 0 {
			m.cursor = len(m.exchanges) - 1
			m.clampScroll()
		}
	case "g":
		m.cursor = 0
		m.clampScroll()
	case "enter":
		if len(m.exchanges) > 0 {
			m.view = viewDetail
			m.detailScroll = 0
		}
	case "r":
		if len(m.exchanges) > 0 {
			ex := m.exchanges[m.cursor]
			return m, func() tea.Msg {
				err := m.proxy.Replay(ex.ID)
				return replayDoneMsg{err: err}
			}
		}
	case "c":
		m.exchanges = nil
		m.cursor = 0
		m.scroll = 0
		m.proxy.Clear()
	}
	return m, nil
}

// ── Detail View Update ──────────────────────────────────────────────────────

func (m TUIModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "backspace":
		m.view = viewList
	case "up", "k":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "down", "j":
		m.detailScroll++
	case "r":
		if len(m.exchanges) > 0 {
			ex := m.exchanges[m.cursor]
			return m, func() tea.Msg {
				err := m.proxy.Replay(ex.ID)
				return replayDoneMsg{err: err}
			}
		}
	}
	return m, nil
}

func (m *TUIModel) clampScroll() {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}

func (m TUIModel) visibleRows() int {
	// header (4 lines) + column header (2) + footer (2) = 8 chrome lines
	rows := m.height - 8
	if rows < 1 {
		rows = 1
	}
	return rows
}

// ── View Rendering ──────────────────────────────────────────────────────────

func (m TUIModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	switch m.view {
	case viewDetail:
		return m.renderDetailView()
	default:
		return m.renderListView()
	}
}

func (m TUIModel) renderListView() string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("  devx tunnel inspect"))
	b.WriteString("\n")
	if m.tunnelURL != "" {
		b.WriteString(fmt.Sprintf("  🌐 %s → localhost:%s", urlStyle.Render(m.tunnelURL), m.targetPort))
	} else {
		b.WriteString(fmt.Sprintf("  Proxy → localhost:%s", m.targetPort))
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  localhost:%d | %d requests captured", m.proxyPort, len(m.exchanges))))
	b.WriteString("\n\n")

	// Column header
	colHeader := fmt.Sprintf("  %-4s %-9s %-7s %-*s %-6s %s",
		"#", "TIME", "METHOD", m.pathWidth(), "PATH", "STATUS", "DURATION")
	b.WriteString(mutedStyle.Render(colHeader))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  " + strings.Repeat("─", m.width-4)))
	b.WriteString("\n")

	// Rows
	visible := m.visibleRows()
	end := m.scroll + visible
	if end > len(m.exchanges) {
		end = len(m.exchanges)
	}
	for i := m.scroll; i < end; i++ {
		ex := m.exchanges[i]
		line := m.renderRow(ex, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Fill remaining rows
	for i := end - m.scroll; i < visible; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	if m.flash != "" {
		b.WriteString("  " + m.flash)
	} else {
		help := mutedStyle.Render("  ↑↓ navigate • enter detail • r replay • c clear • q quit")
		b.WriteString(help)
	}

	return b.String()
}

func (m TUIModel) renderRow(ex CapturedExchange, selected bool) string {
	ts := ex.Timestamp.Format("15:04:05")
	status := statusStyle(ex.StatusCode).Render(fmt.Sprintf("%d", ex.StatusCode))
	dur := formatDuration(ex.Duration)

	path := ex.Path
	pw := m.pathWidth()
	if len(path) > pw {
		path = path[:pw-1] + "…"
	}

	tag := ""
	if ex.IsReplay {
		tag = replayTag.Render(" ↻")
	}

	prefix := "  "
	style := rowStyle
	if selected {
		prefix = cursorStyle.Render("▸ ")
		style = cursorStyle
	}

	return fmt.Sprintf("%s%-4s %s %-7s %-*s %s  %s%s",
		prefix,
		style.Render(fmt.Sprintf("%d", ex.ID)),
		mutedStyle.Render(ts),
		methodStyle(ex.Method),
		pw, style.Render(path),
		status,
		mutedStyle.Render(dur),
		tag,
	)
}

func (m TUIModel) renderDetailView() string {
	if m.cursor >= len(m.exchanges) {
		return ""
	}
	ex := m.exchanges[m.cursor]

	var b strings.Builder

	// Title bar
	title := fmt.Sprintf("  Request #%d", ex.ID)
	if ex.IsReplay {
		title += " (replay)"
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s → %s (%s)",
		methodStyle(ex.Method),
		rowStyle.Render(ex.Path),
		statusStyle(ex.StatusCode).Render(fmt.Sprintf("%d", ex.StatusCode)),
		mutedStyle.Render(formatDuration(ex.Duration)),
	))
	b.WriteString("\n")

	// Build detail content
	var detail strings.Builder

	detail.WriteString(sectionHead.Render("── Request Headers ──"))
	detail.WriteString("\n")
	detail.WriteString(renderHeaders(ex.ReqHeaders))

	if len(ex.ReqBody) > 0 {
		detail.WriteString(sectionHead.Render("── Request Body ──"))
		detail.WriteString("\n")
		detail.WriteString(truncateBody(ex.ReqBody))
		detail.WriteString("\n")
	}

	detail.WriteString(sectionHead.Render("── Response Headers ──"))
	detail.WriteString("\n")
	detail.WriteString(renderHeaders(ex.RespHeaders))

	if len(ex.RespBody) > 0 {
		detail.WriteString(sectionHead.Render("── Response Body ──"))
		detail.WriteString("\n")
		detail.WriteString(truncateBody(ex.RespBody))
		detail.WriteString("\n")
	}

	// Apply scroll
	lines := strings.Split(detail.String(), "\n")
	visibleLines := m.height - 5 // header(3) + footer(2)
	if visibleLines < 1 {
		visibleLines = 1
	}
	if m.detailScroll >= len(lines) {
		m.detailScroll = len(lines) - 1
	}
	end := m.detailScroll + visibleLines
	if end > len(lines) {
		end = len(lines)
	}
	b.WriteString("\n")
	for _, line := range lines[m.detailScroll:end] {
		b.WriteString("  " + line + "\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  esc back • r replay • ↑↓ scroll • q quit"))

	return b.String()
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (m TUIModel) pathWidth() int {
	// #(4) + time(9) + method(7) + status(6) + dur(10) + spaces(12) = 48
	w := m.width - 48
	if w < 10 {
		w = 10
	}
	if w > 60 {
		w = 60
	}
	return w
}

func methodStyle(method string) string {
	switch method {
	case "GET":
		return lipgloss.NewStyle().Foreground(green).Render(method)
	case "POST":
		return lipgloss.NewStyle().Foreground(yellow).Render(method + " ")
	case "PUT":
		return lipgloss.NewStyle().Foreground(yellow).Render(method + "  ")
	case "DELETE":
		return lipgloss.NewStyle().Foreground(red).Render(method)
	case "PATCH":
		return lipgloss.NewStyle().Foreground(cyan).Render(method)
	default:
		return lipgloss.NewStyle().Foreground(white).Render(fmt.Sprintf("%-6s", method))
	}
}

func formatDuration(d interface{ Milliseconds() int64 }) string {
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func renderHeaders(h http.Header) string {
	if len(h) == 0 {
		return mutedStyle.Render("(none)") + "\n"
	}
	var b strings.Builder
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			b.WriteString(fmt.Sprintf("%s: %s\n",
				lipgloss.NewStyle().Foreground(cyan).Render(k),
				mutedStyle.Render(v),
			))
		}
	}
	return b.String()
}

func truncateBody(body []byte) string {
	s := string(body)
	const maxDisplay = 4096
	if len(s) > maxDisplay {
		s = s[:maxDisplay] + "\n" + mutedStyle.Render(fmt.Sprintf("... (%d bytes total, showing first %d)", len(body), maxDisplay))
	}
	return s
}
