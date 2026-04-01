package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/VitruvianSoftware/devx/internal/webhook"
)

var webhookPort    int
var webhookExpose  bool
var webhookRuntime string

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Local webhook tools",
}

var webhookCatchCmd = &cobra.Command{
	Use:   "catch",
	Short: "Capture and inspect outbound webhook payloads in the terminal",
	Long: `Starts a local HTTP catch-all server that accepts any request and
displays it in a live terminal UI with per-method colours, header inspection,
and pretty-printed JSON payloads.

Set WEBHOOK_URL (or STRIPE_WEBHOOK_URL, GITHUB_WEBHOOK_URL, etc.) in your
app to point at the local address printed on startup.

Use --expose to wrap the server in a Cloudflare tunnel and get a public
HTTPS URL — ideal when a 3rd party needs to reach your local handler.`,
	RunE: runWebhookCatch,
}

func init() {
	webhookCatchCmd.Flags().IntVarP(&webhookPort, "port", "p", 9999,
		"Local port to listen on")
	webhookCatchCmd.Flags().BoolVar(&webhookExpose, "expose", false,
		"Expose via Cloudflare tunnel to get a public HTTPS URL")
	webhookCatchCmd.Flags().StringVar(&webhookRuntime, "runtime", "podman",
		"Container runtime (only used with --expose to check for cloudflared)")
	webhookCmd.AddCommand(webhookCatchCmd)
	rootCmd.AddCommand(webhookCmd)
}

func runWebhookCatch(_ *cobra.Command, _ []string) error {
	srv := webhook.New(webhookPort)

	addr, err := srv.Start()
	if err != nil {
		return err
	}

	// Extract just the port from the bound address (handles :0 OS assignment)
	localURL := "http://localhost:" + portFromAddr(addr)

	// ── Cloudflare tunnel (optional) ────────────────────────────────────────
	publicURL := ""
	if webhookExpose {
		fmt.Fprintf(os.Stderr, "%s Starting Cloudflare tunnel for %s...\n",
			tui.IconRunning, localURL)
		publicURL = startWebhookTunnel(addr)
		if publicURL == "" {
			fmt.Fprintf(os.Stderr, "  ⚠ Tunnel failed — continuing with local URL only\n")
		}
	}

	// ── Print env var hints before entering the TUI ──────────────────────────
	fmt.Fprintf(os.Stderr, "\n%s\n\n", tui.StyleTitle.Render("devx webhook catch"))
	fmt.Fprintf(os.Stderr, "  Set in your app's .env:\n\n")
	fmt.Fprintf(os.Stderr, "    WEBHOOK_URL=%s\n", localURL)
	if publicURL != "" {
		fmt.Fprintf(os.Stderr, "    WEBHOOK_URL=%s   (public)\n", publicURL)
	}
	fmt.Fprintln(os.Stderr)

	// If we have a real TTY (interactive terminal), launch the Bubble Tea TUI.
	// Otherwise (CI, piped output, --json) fall back to streaming JSON lines.
	isTTY := isTerminal()
	if isTTY && !outputJSON {
		return runWebhookTUI(srv, localURL, publicURL)
	}
	return runWebhookJSONStream(srv)
}

// runWebhookTUI runs the full Bubble Tea interactive TUI.
func runWebhookTUI(srv *webhook.Server, localURL, publicURL string) error {
	fmt.Fprintf(os.Stderr, "  Starting TUI... (q to quit)\n\n")
	time.Sleep(200 * time.Millisecond)

	model := webhook.NewModel(localURL, publicURL)
	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		for req := range srv.Requests() {
			webhook.SendRequest(p, req)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		srv.Shutdown()
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	srv.Shutdown()
	return nil
}

// runWebhookJSONStream logs each request as a JSON line — useful in CI or
// when piping to jq.
func runWebhookJSONStream(srv *webhook.Server) error {
	fmt.Fprintf(os.Stderr, "  Streaming requests as JSON (Ctrl+C to stop)\n\n")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	enc := newRequestEncoder(os.Stdout)
	count := 0
	for {
		select {
		case req, ok := <-srv.Requests():
			if !ok {
				return nil
			}
			enc.encode(req)
			count++
		case <-sigCh:
			srv.Shutdown()
			fmt.Fprintf(os.Stderr, "\n%s Webhook catcher stopped. %d request(s) captured.\n",
				tui.IconDone, count)
			return nil
		}
	}
}

// isTerminal returns true if os.Stdout is connected to a real TTY.
func isTerminal() bool {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// portFromAddr extracts "9999" from "0.0.0.0:9999" or "[::]:9999".
func portFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[idx+1:]
	}
	return addr
}

// startWebhookTunnel attempts to start a cloudflared quick tunnel for the
// given local address. Returns the public URL, or empty string on failure.
// It starts cloudflared in the background and parses its stderr for the URL.
func startWebhookTunnel(addr string) string {
	port := portFromAddr(addr)

	// cloudflared tunnel --url http://localhost:<port>
	cmd := exec.Command("cloudflared", "tunnel", "--url",
		"http://localhost:"+port, "--no-autoupdate")

	urlCh := make(chan string, 1)

	// cloudflared prints the tunnel URL to stderr
	cmd.Stderr = &tunnelURLExtractor{ch: urlCh}

	if err := cmd.Start(); err != nil {
		// cloudflared not installed
		return ""
	}

	// Wait up to 8s for the tunnel URL
	select {
	case u := <-urlCh:
		return u
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		return ""
	}
}

// tunnelURLExtractor scans cloudflared stderr for "https://*.trycloudflare.com"
type tunnelURLExtractor struct {
	ch   chan string
	sent bool
}

func (t *tunnelURLExtractor) Write(p []byte) (int, error) {
	if t.sent {
		return len(p), nil
	}
	s := string(p)
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.Index(line, "https://"); idx >= 0 {
			candidate := line[idx:]
			// Trim trailing whitespace/control chars
			if end := strings.IndexAny(candidate, " \t\r\n"); end > 0 {
				candidate = candidate[:end]
			}
			if strings.Contains(candidate, "trycloudflare.com") ||
				strings.Contains(candidate, "cloudflare.com") {
				t.ch <- candidate
				t.sent = true
				return len(p), nil
			}
		}
	}
	return len(p), nil
}

// requestEncoder emits one JSON object per request to the given writer.
type requestEncoder struct {
	enc *json.Encoder
}

func newRequestEncoder(w io.Writer) *requestEncoder {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return &requestEncoder{enc: e}
}

func (r *requestEncoder) encode(req webhook.Request) {
	type wireRequest struct {
		Time     string            `json:"time"`
		Method   string            `json:"method"`
		Path     string            `json:"path"`
		Headers  map[string]string `json:"headers"`
		Body     string            `json:"body"`
		Duration string            `json:"duration_ms"`
	}
	_ = r.enc.Encode(wireRequest{
		Time:     req.Time.Format("15:04:05.000"),
		Method:   req.Method,
		Path:     req.Path,
		Headers:  req.Headers,
		Body:     req.Body,
		Duration: fmt.Sprintf("%d", req.Duration.Milliseconds()),
	})
}

