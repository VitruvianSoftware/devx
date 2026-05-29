# Unified Log Streaming for `devx up` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Opt-in, skaffold-style container log streaming in `devx up` across all runtimes — logs stream inline in the `up` terminal (prefixed + colored) **and** land in `~/.devx/logs/<service>.log` so a separately-running `devx logs` TUI sees them too.

**Architecture:** Generalize the host tee pattern (`io.MultiWriter(os.Stdout, file)` at [dag.go:461](../../internal/orchestrator/dag.go)) to every runtime. The **file is the cross-process fan-out point**. A per-node `LogMode` (Off/Raw/Prefixed), resolved from a `--logs`/`--no-logs` flag and a `logs:` config field (flag > per-service > top-level > built-in default; host defaults on/raw, others off), gates each producer. Producers shell out (`<rt> logs -f`, `kubectl logs -f`, `gcloud … logs tail`) and tee through a shared line-buffered `LineWriter` (prefix + color + secret redaction).

**Tech Stack:** Go 1.25.5, charmbracelet/lipgloss (colors), gopkg.in/yaml.v3, `internal/logs` (Streamer/SecretRedactor/TUI), `internal/provider` (`ContainerRuntime`), `internal/orchestrator`. Tests: stdlib `testing`.

**Dependency:** The container producer (Task 5) requires `implementation_plan_container_runtime.md` (it tails `devx-svc-<name>`, the name that plan establishes). Tasks 1–4, 6, 7, 8 are independent of it.

**Prerequisite context for the engineer:**
- `internal/logs/streamer.go` already multiplexes two source types into one `LogLine` channel: `watchContainers` (live, via `<rt> logs -f`) and `watchHostLogs` (files in `~/.devx/logs/*.log`, via `tail -f`). `LogLine{Timestamp, Service, Message, Type}`. `SecretRedactor` has `.Redact(string) string`.
- The TUI colors are package globals today: `colorMap`, `colors` (7 hex codes), `colorIdx`, `getColor(service) lipgloss.Color` at [tui.go:35-58](../../internal/logs/tui.go).
- Host logs are written at [dag.go:448-462](../../internal/orchestrator/dag.go) (`O_APPEND` today). `devx up` blocks until Ctrl+C at [up.go:433-458](../../cmd/up.go); producers started during deploy stream into that window and are cancelled by `dagCleanup` / context.
- Run tests with `go test ./...`.

---

### Task 1: Thread-safe `ColorPicker` (extracted from TUI globals)

**Files:**
- Create: `internal/logs/color.go`
- Modify: `internal/logs/tui.go` (replace the `colorMap`/`colors`/`colorIdx`/`getColor` globals at line 35-58 with delegation to a package-default `ColorPicker`)
- Test: `internal/logs/color_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/logs/color_test.go`:

```go
package logs

import (
	"sync"
	"testing"
)

func TestColorPicker_StablePerService(t *testing.T) {
	p := NewColorPicker()
	a1 := p.Color("api")
	a2 := p.Color("api")
	if a1 != a2 {
		t.Errorf("same service got different colors: %v vs %v", a1, a2)
	}
	if p.Color("web") == a1 {
		// Not guaranteed unique forever (palette wraps), but the first two differ.
		t.Errorf("distinct early services should get distinct colors")
	}
}

func TestColorPicker_ConcurrentSafe(t *testing.T) {
	p := NewColorPicker()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = p.Color("svc") }()
	}
	wg.Wait() // must not race (run with -race)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run TestColorPicker -v`
Expected: FAIL — `undefined: NewColorPicker`.

- [ ] **Step 3: Implement `ColorPicker` and refactor the TUI globals**

Create `internal/logs/color.go`:

```go
package logs

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// paletteColors is the fixed per-service color palette (preserved from the TUI).
var paletteColors = []string{"#FF5F87", "#FFF700", "#00FF00", "#00FFFF", "#FF00FF", "#8A2BE2", "#FFA500"}

// ColorPicker assigns a stable lipgloss color to each service name, cycling
// through the palette. Safe for concurrent use.
type ColorPicker struct {
	mu     sync.Mutex
	colors map[string]lipgloss.Color
	idx    int
}

func NewColorPicker() *ColorPicker {
	return &ColorPicker{colors: map[string]lipgloss.Color{}}
}

// Color returns the (stable) color for a service, assigning one on first use.
func (p *ColorPicker) Color(service string) lipgloss.Color {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.colors[service]; ok {
		return c
	}
	c := lipgloss.Color(paletteColors[p.idx%len(paletteColors)])
	p.colors[service] = c
	p.idx++
	return c
}

// defaultColorPicker is the process-wide picker shared by the inline writer and
// the TUI so colors agree across both views.
var defaultColorPicker = NewColorPicker()
```

In `internal/logs/tui.go`, delete the `colorMap`, `colors`, `colorIdx` vars and the `getColor` function (lines 46-58), and replace `getColor` with a thin delegate (keep the name so existing TUI call sites are untouched). Where lines 46-58 were, leave the `titleStyle`/`infoStyle` vars (lines 36-45) intact and add:

```go
func getColor(service string) lipgloss.Color {
	return defaultColorPicker.Color(service)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/logs/ -run TestColorPicker -race -v`
Expected: PASS, no race.
Then `go build ./...` — Expected: clean (TUI still compiles).

- [ ] **Step 5: Commit**

```bash
git add internal/logs/color.go internal/logs/tui.go internal/logs/color_test.go
git commit -m "refactor(logs): extract thread-safe ColorPicker from TUI globals"
```

---

### Task 2: `LineWriter` — prefix + color + redaction, line-buffered

**Files:**
- Create: `internal/logs/linewriter.go`
- Test: `internal/logs/linewriter_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/logs/linewriter_test.go`:

```go
package logs

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineWriter_PrefixesEachLine_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewLineWriter(&buf, "api", false, nil)
	w.Write([]byte("hello\nworld\n"))
	got := buf.String()
	want := "[api] hello\n[api] world\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLineWriter_BuffersPartialLines(t *testing.T) {
	var buf bytes.Buffer
	w := NewLineWriter(&buf, "api", false, nil)
	w.Write([]byte("par"))
	w.Write([]byte("tial\n"))
	if got := buf.String(); got != "[api] partial\n" {
		t.Errorf("partial-line buffering failed: %q", got)
	}
}

func TestLineWriter_Redacts(t *testing.T) {
	var buf bytes.Buffer
	// SecretRedactor (internal/logs/redactor.go) is built from KEY=VALUE pairs;
	// values <= 3 chars or non-sensitive keys are skipped, so use a long value.
	r := NewSecretRedactorFromPairs([]string{"MY_SECRET=supersecretvalue"})
	w := NewLineWriter(&buf, "api", false, r)
	w.Write([]byte("token=supersecretvalue\n"))
	if strings.Contains(buf.String(), "supersecretvalue") {
		t.Errorf("expected secret to be redacted, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run TestLineWriter -v`
Expected: FAIL — `undefined: NewLineWriter`.

- [ ] **Step 3: Implement `LineWriter`**

Create `internal/logs/linewriter.go`:

```go
package logs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// LineWriter wraps a destination writer and, on each complete '\n'-terminated
// line, prepends a "[service] " prefix (colored when color==true), applies
// secret redaction, and writes the result. Partial lines are buffered until the
// next newline. Safe for concurrent Writes from multiple goroutines.
type LineWriter struct {
	dst      io.Writer
	prefix   string // pre-rendered "[service]" (with color if enabled)
	redactor *SecretRedactor

	mu  sync.Mutex
	buf bytes.Buffer
}

// NewLineWriter builds a LineWriter for a service. color toggles ANSI coloring
// (callers pass ColorEnabled()). redactor may be nil.
func NewLineWriter(dst io.Writer, service string, color bool, redactor *SecretRedactor) *LineWriter {
	prefix := "[" + service + "]"
	if color {
		prefix = lipgloss.NewStyle().Foreground(defaultColorPicker.Color(service)).Render(prefix)
	}
	return &LineWriter{dst: dst, prefix: prefix, redactor: redactor}
}

func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil { // no full line yet; keep the remainder buffered
			w.buf.Reset()
			w.buf.WriteString(line)
			break
		}
		msg := line[:len(line)-1] // strip '\n'
		if w.redactor != nil {
			msg = w.redactor.Redact(msg)
		}
		if _, err := fmt.Fprintf(w.dst, "%s %s\n", w.prefix, msg); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

// ColorEnabled reports whether inline log prefixes should be colored. Disabled
// when NO_COLOR is set (https://no-color.org). Callers pass the result to
// NewLineWriter.
func ColorEnabled() bool {
	return os.Getenv("NO_COLOR") == ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/logs/ -run TestLineWriter -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/logs/linewriter.go internal/logs/linewriter_test.go
git commit -m "feat(logs): add line-buffered LineWriter (prefix + color + redaction)"
```

---

### Task 3: `LogMode` + precedence resolver + `--logs`/`--no-logs` flag + `logs:` config

**Files:**
- Create: `internal/orchestrator/logmode.go`
- Modify: `internal/orchestrator/dag.go` (add `LogMode LogMode` field to `Node` at 145-180)
- Modify: `cmd/devxconfig.go` (`Logs *bool` on `DevxConfig` line 312-329 and `DevxConfigService` line 183-197)
- Modify: `cmd/up.go` (flag vars at 48-49; flag registration at 476-479; per-service `LogMode` wiring in the node loop)
- Test: `internal/orchestrator/logmode_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/orchestrator/logmode_test.go`:

```go
package orchestrator

import "testing"

func b(v bool) *bool { return &v }

func TestResolveLogMode(t *testing.T) {
	cases := []struct {
		name                     string
		flag, perSvc, topLevel   *bool
		runtime                  string
		want                     LogMode
	}{
		{"host default → raw", nil, nil, nil, "host", LogRaw},
		{"k8s default → off", nil, nil, nil, "kubernetes", LogOff},
		{"container default → off", nil, nil, nil, "container", LogOff},
		{"flag on → prefixed (k8s)", b(true), nil, nil, "kubernetes", LogPrefixed},
		{"flag on → prefixed (host)", b(true), nil, nil, "host", LogPrefixed},
		{"flag off → off (host)", b(false), nil, nil, "host", LogOff},
		{"flag beats per-service", b(false), b(true), nil, "kubernetes", LogOff},
		{"per-service beats top-level", nil, b(false), b(true), "kubernetes", LogOff},
		{"top-level on → prefixed", nil, nil, b(true), "cloud", LogPrefixed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveLogMode(c.flag, c.perSvc, c.topLevel, c.runtime); got != c.want {
				t.Errorf("ResolveLogMode = %v, want %v", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestResolveLogMode -v`
Expected: FAIL — `undefined: LogMode`, `undefined: ResolveLogMode`.

- [ ] **Step 3a: Implement `LogMode` + resolver**

Create `internal/orchestrator/logmode.go`:

```go
package orchestrator

// LogMode controls how a service node's logs are surfaced during `devx up`.
type LogMode int

const (
	LogOff      LogMode = iota // no inline output; for non-host, no file either (producer not started)
	LogRaw                     // inline raw (no prefix) + file — preserves today's host default
	LogPrefixed                // inline "[service]" prefixed+colored + file
)

// ResolveLogMode applies the opt-in precedence: flag > per-service > top-level >
// built-in default. flag/perSvc/topLevel are nil when unset. Built-in default:
// host = LogRaw (today's behavior), all other runtimes = LogOff. Any explicit
// "on" yields LogPrefixed (the multi-service case where prefixes disambiguate);
// any explicit "off" yields LogOff.
func ResolveLogMode(flag, perSvc, topLevel *bool, runtime string) LogMode {
	for _, p := range []*bool{flag, perSvc, topLevel} {
		if p != nil {
			if *p {
				return LogPrefixed
			}
			return LogOff
		}
	}
	if runtime == string(RuntimeHost) {
		return LogRaw
	}
	return LogOff
}
```

Add to the `Node` struct in `dag.go` (near the other config fields):

```go
	LogMode LogMode // how this node's logs are surfaced (resolved in the cmd layer)
```

- [ ] **Step 3b: Add the `Logs` config fields**

In `cmd/devxconfig.go`, add to `DevxConfig` (line 312-329):

```go
	Logs          *bool                             `yaml:"logs,omitempty"`          // default log-streaming opt-in for all services
```

and to `DevxConfigService` (line 183-197):

```go
	Logs            *bool                             `yaml:"logs,omitempty"`             // per-service log-streaming opt-in (overrides top-level)
```

- [ ] **Step 3c: Add the flag and wire `LogMode` in `up.go`**

In `cmd/up.go`, add flag vars near line 48:

```go
var upLogs bool
var upNoLogs bool
```

Register them in `init()` near line 476-479:

```go
	upCmd.Flags().BoolVar(&upLogs, "logs", false, "Stream logs from all deployed services inline (skaffold-style); also written to ~/.devx/logs/")
	upCmd.Flags().BoolVar(&upNoLogs, "no-logs", false, "Disable all inline log streaming, including host services")
```

Add a helper (near `toContainerNodeConfig`):

```go
// logFlagPointer turns the mutually-exclusive --logs/--no-logs flags into the
// tri-state *bool the resolver expects (nil = neither flag set).
func logFlagPointer() *bool {
	switch {
	case upNoLogs:
		v := false
		return &v
	case upLogs:
		v := true
		return &v
	default:
		return nil
	}
}
```

In the service loop, set `LogMode` on each node. Add to the `dag.AddNode(&orchestrator.Node{...})` literal:

```go
				LogMode:      orchestrator.ResolveLogMode(logFlagPointer(), svc.Logs, cfgYaml.Logs, svc.Runtime),
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestrator/ -run TestResolveLogMode -v && go build ./...`
Expected: PASS (all subtests), clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/logmode.go internal/orchestrator/dag.go cmd/devxconfig.go cmd/up.go
git commit -m "feat(up): add --logs/--no-logs flag, logs: config, and LogMode resolver"
```

---

### Task 4: Service sink helper + host producer honoring `LogMode`

**Files:**
- Create: `internal/logs/sink.go`
- Modify: `internal/orchestrator/dag.go` (`startHostProcess` at 436-478; add a `logCloser` field to `Node`; call it in `cleanupFn`)
- Test: `internal/logs/sink_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/logs/sink_test.go`:

```go
package logs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenServiceLog_Truncates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Pre-existing stale content must be truncated on open.
	p := ServiceLogPath("api")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte("STALE"), 0644)

	f, err := OpenServiceLog("api")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString("fresh\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "fresh\n" {
		t.Errorf("expected truncate-then-write, got %q", string(got))
	}
}

func TestBuildSink_Prefixed_WritesInlineAndFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	var inline bytes.Buffer
	w, closeFn, err := BuildSink("api", LogPrefixedMode, &inline, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("hi\n"))
	_ = closeFn()
	if inline.String() != "[api] hi\n" {
		t.Errorf("inline = %q", inline.String())
	}
	fileBytes, _ := os.ReadFile(ServiceLogPath("api"))
	if string(fileBytes) != "hi\n" {
		t.Errorf("file = %q", string(fileBytes))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run 'TestOpenServiceLog|TestBuildSink' -v`
Expected: FAIL — `undefined: ServiceLogPath`, `OpenServiceLog`, `BuildSink`, `LogPrefixedMode`.

- [ ] **Step 3: Implement the sink helper**

Create `internal/logs/sink.go`:

```go
package logs

import (
	"io"
	"os"
	"path/filepath"
)

// SinkMode mirrors orchestrator.LogMode without importing it (logs must not
// depend on orchestrator). The orchestrator maps its LogMode to these.
type SinkMode int

const (
	LogOffMode SinkMode = iota // file only (no inline)
	LogRawMode                 // inline raw + file
	LogPrefixedMode            // inline prefixed+colored + file
)

// ServiceLogPath is the per-service log file path consumed by `devx logs`.
func ServiceLogPath(service string) string {
	return filepath.Join(os.Getenv("HOME"), ".devx", "logs", service+".log")
}

// OpenServiceLog creates/truncates the per-service log file (fresh per `up` run).
func OpenServiceLog(service string) (*os.File, error) {
	p := ServiceLogPath(service)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

// BuildSink returns the writer a producer should write log bytes to, plus a
// closer. It always writes the service's log file (the devx-logs fan-out) and,
// per mode, also writes inline (raw or prefixed) to the provided inline writer
// (normally os.Stdout). color toggles ANSI for the prefixed mode; redactor may
// be nil. For LogOffMode the inline writer is omitted (file only).
func BuildSink(service string, mode SinkMode, inline io.Writer, color bool, redactor *SecretRedactor) (io.Writer, func() error, error) {
	f, err := OpenServiceLog(service)
	if err != nil {
		return nil, nil, err
	}
	closeFn := func() error { return f.Close() }
	switch mode {
	case LogRawMode:
		return io.MultiWriter(inline, f), closeFn, nil
	case LogPrefixedMode:
		return io.MultiWriter(NewLineWriter(inline, service, color, redactor), f), closeFn, nil
	default: // LogOffMode
		return f, closeFn, nil
	}
}
```

In `internal/orchestrator/dag.go`, add a field to `Node`:

```go
	logCloser func() error // closes the log sink on cleanup
```

Add a mapping helper in `dag.go` (or `logmode.go`):

```go
func sinkMode(m LogMode) logs.SinkMode {
	switch m {
	case LogRaw:
		return logs.LogRawMode
	case LogPrefixed:
		return logs.LogPrefixedMode
	default:
		return logs.LogOffMode
	}
}
```

Modify `startHostProcess` (line 448-462): replace the unconditional log-file + `MultiWriter` block with a `BuildSink` call. Replace lines 448-462 (`// Setup logging …` through the two `cmd.Stdout`/`cmd.Stderr` MultiWriter assignments) with:

```go
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		cancel()
		return fmt.Errorf("opening log sink: %w", err)
	}
	n.logCloser = closeFn
	cmd.Stdout = w
	cmd.Stderr = w
```

In `cleanupFn` (line 290-320), add (near the `n.cancel()` handling):

```go
			if n.logCloser != nil {
				_ = n.logCloser()
			}
```

Add the import `"github.com/VitruvianSoftware/devx/internal/logs"` to `dag.go` (it already imports `internal/logs` for crash-log tailing — confirm; if not, add it).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/logs/ -run 'TestOpenServiceLog|TestBuildSink' -v && go build ./...`
Expected: PASS, clean build. Host services now stream raw-by-default (unchanged) and prefixed under `--logs`.

- [ ] **Step 5: Commit**

```bash
git add internal/logs/sink.go internal/logs/sink_test.go internal/orchestrator/dag.go
git commit -m "feat(logs): add service-log sink; route host output through LogMode"
```

---

### Task 5: Container log producer (depends on Plan 1)

**Files:**
- Modify: `internal/orchestrator/container_node.go` (start a tail after the container runs)
- Test: `internal/orchestrator/container_node_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrator/container_node_test.go`:

```go
func TestContainerLogsTailArgs(t *testing.T) {
	got := containerLogsTailArgs("devx-svc-api")
	want := []string{"logs", "--tail", "50", "-f", "devx-svc-api"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestContainerLogsTailArgs -v`
Expected: FAIL — `undefined: containerLogsTailArgs`.

- [ ] **Step 3: Implement the container tail**

In `internal/orchestrator/container_node.go`, add (and call it from `startContainerNode` after `n.containerStarted = true`):

```go
// containerLogsTailArgs builds the `<runtime> logs -f` args for a container.
func containerLogsTailArgs(name string) []string {
	return []string{"logs", "--tail", "50", "-f", name}
}

// streamContainerLogs tails the container's logs into the service sink (inline +
// file) when LogMode is on. Runs until ctx is cancelled. No-op for LogOff.
func streamContainerLogs(ctx context.Context, rt provider.ContainerRuntime, n *Node) {
	if n.LogMode == LogOff {
		return
	}
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		return
	}
	n.logCloser = closeFn
	cmd := rt.CommandContext(ctx, containerLogsTailArgs(containerNodeName(n.Name))...)
	cmd.Stdout = w
	cmd.Stderr = w
	go func() { _ = cmd.Run() }() // ends when ctx is cancelled on shutdown
}
```

Add imports `"os"` and `"github.com/VitruvianSoftware/devx/internal/logs"` to `container_node.go`. In `startContainerNode`, after `n.containerStarted = true`, insert:

```go
	streamContainerLogs(ctx, rt, n)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ -run TestContainerLogsTailArgs -v && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/container_node.go internal/orchestrator/container_node_test.go
git commit -m "feat(orchestrator): stream runtime: container logs (inline + file)"
```

---

### Task 6: Kubernetes log producer (pod watch + per-container tail)

**Files:**
- Create: `internal/orchestrator/kubernetes_logs.go`
- Modify: `internal/orchestrator/kubernetes_node.go` (start the log watcher near the port-forward block at line 190-208; store its cancel on the `Node`)
- Modify: `internal/orchestrator/dag.go` (add `logWatchCancel context.CancelFunc` to `Node`; cancel it in `cleanupFn`)
- Test: `internal/orchestrator/kubernetes_logs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/orchestrator/kubernetes_logs_test.go`:

```go
package orchestrator

import (
	"reflect"
	"testing"
)

func TestKubectlLogsTailArgs(t *testing.T) {
	got := kubectlLogsTailArgs("/kc", "kind-dev", "myns", "api-7d9", "app", 5)
	want := []string{
		"--kubeconfig", "/kc", "--context", "kind-dev",
		"logs", "--since=5s", "-f", "api-7d9", "-c", "app", "--namespace", "myns",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParsePodWatchEvent_NewRunningContainers(t *testing.T) {
	line := []byte(`{"type":"ADDED","object":{"metadata":{"name":"api-1","namespace":"myns"},` +
		`"status":{"containerStatuses":[` +
		`{"name":"app","containerID":"docker://abc","state":{"running":{}}},` +
		`{"name":"sidecar","containerID":"","state":{"waiting":{"message":"ImagePullBackOff"}}}]}}}`)
	evt, err := parsePodWatchEvent(line)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Type != "ADDED" || evt.Object.Metadata.Name != "api-1" {
		t.Fatalf("bad event: %+v", evt)
	}
	cs := evt.Object.Status.ContainerStatuses
	if len(cs) != 2 || cs[0].ContainerID != "docker://abc" {
		t.Fatalf("bad container statuses: %+v", cs)
	}
	if cs[1].State.Waiting == nil || cs[1].State.Waiting.Message != "ImagePullBackOff" {
		t.Errorf("expected waiting message, got %+v", cs[1].State)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run 'TestKubectlLogsTailArgs|TestParsePodWatchEvent' -v`
Expected: FAIL — `undefined: kubectlLogsTailArgs`, `parsePodWatchEvent`.

- [ ] **Step 3: Implement the k8s log producer**

Create `internal/orchestrator/kubernetes_logs.go`:

```go
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/logs"
)

// --- watch-event decoding (pure, unit-tested) ---

type podWatchEvent struct {
	Type   string `json:"type"`
	Object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
			ContainerStatuses     []containerStatus `json:"containerStatuses"`
		} `json:"status"`
	} `json:"object"`
}

type containerStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"containerID"`
	State       struct {
		Waiting *struct {
			Message string `json:"message"`
		} `json:"waiting"`
	} `json:"state"`
}

func parsePodWatchEvent(line []byte) (podWatchEvent, error) {
	var e podWatchEvent
	err := json.Unmarshal(line, &e)
	return e, err
}

// kubectlLogsTailArgs builds the per-container `kubectl logs -f` args.
func kubectlLogsTailArgs(kubeconfig, kctx, ns, pod, container string, sinceSeconds int) []string {
	return kubectlArgs(kubeconfig, kctx,
		"logs", fmt.Sprintf("--since=%ds", sinceSeconds), "-f", pod, "-c", container, "--namespace", ns)
}

// --- runtime watcher ---

type trackedContainers struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (t *trackedContainers) addNew(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ids[id] {
		return false
	}
	t.ids[id] = true
	return true
}

// startKubernetesLogs watches pods in ns and tails each new container's logs
// into the service sink (inline + file). Returns a cancel func. No-op for LogOff.
func startKubernetesLogs(parent context.Context, n *Node, kubeconfig, kctx, ns string) (context.CancelFunc, error) {
	if n.LogMode == LogOff {
		return func() {}, nil
	}
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		return nil, err
	}
	n.logCloser = closeFn

	ctx, cancel := context.WithCancel(parent)
	start := time.Now()
	tracked := &trackedContainers{ids: map[string]bool{}}

	go func() {
		// `kubectl get pods -w --output-watch-events -o json` streams one JSON
		// object per line: {"type": "...", "object": {...}}.
		cmd := exec.CommandContext(ctx, "kubectl",
			kubectlArgs(kubeconfig, kctx, "get", "pods", "-n", ns, "-w", "--output-watch-events", "-o", "json")...)
		stdout, err := cmd.StdoutPipe()
		if err != nil || cmd.Start() != nil {
			return
		}
		dec := json.NewDecoder(stdout)
		for {
			var evt podWatchEvent
			if err := dec.Decode(&evt); err != nil {
				return // stream closed or ctx cancelled
			}
			if evt.Type == "DELETED" {
				continue
			}
			all := append(evt.Object.Status.InitContainerStatuses, evt.Object.Status.ContainerStatuses...)
			for _, c := range all {
				if c.ContainerID == "" {
					if c.State.Waiting != nil && c.State.Waiting.Message != "" {
						fmt.Fprintf(w, "%s/%s: %s\n", evt.Object.Metadata.Name, c.Name, c.State.Waiting.Message)
					}
					continue
				}
				if tracked.addNew(c.ContainerID) {
					since := int(time.Since(start).Seconds()) + 1
					go tailPodContainer(ctx, w, kubeconfig, kctx, ns, evt.Object.Metadata.Name, c.Name, since)
				}
			}
		}
	}()
	return cancel, nil
}

func tailPodContainer(ctx context.Context, w io.Writer, kubeconfig, kctx, ns, pod, container string, since int) {
	cmd := exec.CommandContext(ctx, "kubectl", kubectlLogsTailArgs(kubeconfig, kctx, ns, pod, container, since)...)
	cmd.Stdout = w
	cmd.Stderr = w
	_ = cmd.Run() // returns when the pod dies or ctx is cancelled
}
```

In `kubernetes_node.go`, after the port-forward block (line 190-208, before `return nil`), add:

```go
	// Stream pod logs (inline + ~/.devx/logs/) when opted in.
	cancelLogs, err := startKubernetesLogs(ctx, n, kubeconfig, k.Context, ns)
	if err != nil {
		return fmt.Errorf("service %q: starting log stream: %w", n.Name, err)
	}
	n.logWatchCancel = cancelLogs
```

In `dag.go`, add `logWatchCancel context.CancelFunc` to `Node`, and in `cleanupFn` add:

```go
			if n.logWatchCancel != nil {
				n.logWatchCancel()
			}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestrator/ -run 'TestKubectlLogsTailArgs|TestParsePodWatchEvent' -v && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/kubernetes_logs.go internal/orchestrator/kubernetes_node.go internal/orchestrator/dag.go
git commit -m "feat(orchestrator): stream kubernetes pod logs (watch + per-container tail)"
```

---

### Task 7: Cloud Run log producer

**Files:**
- Modify: `internal/orchestrator/cloudrun_node.go` (start a gcloud log tail after deploy at line 31-63; reuse `n.logWatchCancel`)
- Test: `internal/orchestrator/cloudrun_node_test.go`

- [ ] **Step 1: Write the failing test**

Add to (or create) `internal/orchestrator/cloudrun_node_test.go`:

```go
package orchestrator

import (
	"reflect"
	"testing"
)

func TestCloudRunLogsTailArgs(t *testing.T) {
	got := cloudRunLogsTailArgs("my-proj", "us-central1", "api")
	want := []string{
		"beta", "run", "services", "logs", "tail", "api",
		"--project", "my-proj", "--region", "us-central1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestCloudRunLogsTailArgs -v`
Expected: FAIL — `undefined: cloudRunLogsTailArgs`.

- [ ] **Step 3: Implement the Cloud Run tail**

In `internal/orchestrator/cloudrun_node.go`, add:

```go
// cloudRunLogsTailArgs builds the gcloud command to stream a Cloud Run service's
// logs. NOTE: `gcloud beta run services logs tail` is a beta surface — verify it
// against the installed gcloud during integration; if unavailable, fall back to
// `gcloud logging tail` with a `resource.type=cloud_run_revision` filter.
func cloudRunLogsTailArgs(project, region, service string) []string {
	return []string{"beta", "run", "services", "logs", "tail", service,
		"--project", project, "--region", region}
}

// streamCloudRunLogs tails a deployed Cloud Run service's logs into the sink.
func streamCloudRunLogs(ctx context.Context, n *Node, c *CloudRunNodeConfig, service string) {
	if n.LogMode == LogOff {
		return
	}
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		return
	}
	n.logCloser = closeFn
	logCtx, cancel := context.WithCancel(ctx)
	n.logWatchCancel = cancel
	cmd := exec.CommandContext(logCtx, "gcloud", cloudRunLogsTailArgs(c.Project, c.Region, service)...)
	cmd.Stdout = w
	cmd.Stderr = w
	go func() { _ = cmd.Run() }()
}
```

Add imports `"context"`, `"os"`, `"os/exec"`, and `"github.com/VitruvianSoftware/devx/internal/logs"` to `cloudrun_node.go` as needed. In `startCloudRunNode`, after `fmt.Printf("  ✅ %s deployed\n", n.Name)` (line ~61), insert:

```go
	streamCloudRunLogs(ctx, n, c, service)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ -run TestCloudRunLogsTailArgs -v && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/cloudrun_node.go internal/orchestrator/cloudrun_node_test.go
git commit -m "feat(orchestrator): stream Cloud Run service logs via gcloud tail"
```

---

### Task 8: `devx logs` single-source dedupe + relabel

**Files:**
- Modify: `internal/logs/streamer.go` (`watchHostLogs`/`tailFile` relabel at 149-203; `watchContainers` dedupe at 66-97)
- Test: `internal/logs/streamer_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/logs/streamer_test.go`:

```go
package logs

import "testing"

func TestServiceNameFromContainer(t *testing.T) {
	// A devx-svc-<name> container maps to the same service identity as <name>.log,
	// so the two sources dedupe to one.
	if got := serviceNameFromContainer("devx-svc-api"); got != "api" {
		t.Errorf("got %q want api", got)
	}
	if got := serviceNameFromContainer("some-other-container"); got != "some-other-container" {
		t.Errorf("non-devx container should be unchanged, got %q", got)
	}
}

func TestServiceNameFromLogFile(t *testing.T) {
	if got := serviceNameFromLogFile("api.log"); got != "api" {
		t.Errorf("got %q want api", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run 'TestServiceNameFrom' -v`
Expected: FAIL — `undefined: serviceNameFromContainer`, `serviceNameFromLogFile`.

- [ ] **Step 3: Implement dedupe + relabel**

In `internal/logs/streamer.go`, add helpers:

```go
func serviceNameFromContainer(name string) string {
	return strings.TrimPrefix(name, "devx-svc-")
}

func serviceNameFromLogFile(filename string) string {
	return strings.TrimSuffix(filename, ".log")
}
```

Relabel file lines: in `watchHostLogs` (line 164-165), change the service naming from `"host:" + name` to the plain service name, and in `tailFile` (line 197) set `Type: "service"` instead of `"host"`:

```go
					name := serviceNameFromLogFile(f.Name())
					serviceName := name
```

```go
		s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: msg, Type: "service"}
```

Dedupe in `watchContainers` (line 80-93): when a container is `devx-svc-<svc>` and a `<svc>.log` file exists (i.e. `up` is producing it), skip the direct container tail so the service isn't shown twice:

```go
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				svc := serviceNameFromContainer(name)
				if svc != name { // devx-svc-* container
					if _, err := os.Stat(ServiceLogPath(svc)); err == nil {
						continue // file source is authoritative; avoid double display
					}
				}
```

(`tailFile` already passes the service name as the `Service` field; ensure that param is the deduped `svc`/file name so colors match across sources.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/logs/ -run 'TestServiceNameFrom' -v && go test ./internal/logs/ && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/logs/streamer.go internal/logs/streamer_test.go
git commit -m "feat(logs): single-source dedupe + relabel for devx logs"
```

---

## Manual verification (after all tasks)

1. **Default unchanged:** with a host-only `devx.yaml`, `devx up` (no flag) shows host logs exactly as before (raw, no `[service]` prefix). Confirm `~/.devx/logs/<svc>.log` is truncated fresh each run.
2. **Opt-in inline:** `devx up --logs` on a project with host + container + kubernetes services shows interleaved, color-coded `[service]` prefixed logs in the terminal.
3. **Two-terminal fan-out:** while `devx up --logs` runs, open a second terminal and run `devx logs` — confirm the same services appear there, **exactly once** each (no double display for container services), with correct colors/labels (no `host:` prefix).
4. **k8s lifecycle:** kill a pod (`kubectl delete pod …`); confirm the replacement pod's logs re-attach automatically, and an `ImagePullBackOff` surfaces its waiting message inline.
5. **Mute:** `devx up --no-logs` produces a quiet terminal (no inline host logs).

## Self-review notes / known follow-ups

- The k8s producer's pod watch is namespace-scoped. Narrowing with `-l <selector>` (from the applied workloads' label selectors) to avoid tailing unrelated namespace pods is a reasonable follow-up; not required for first cut.
- Cloud Run's `gcloud beta run services logs tail` subcommand must be verified against the installed gcloud (Task 7 note); the `gcloud logging tail` fallback is the contingency.
- TTY-based color auto-disable is reduced to the `NO_COLOR` convention (Task 2) to avoid a new terminal-detection dependency; full TTY detection can be added later.
- The k8s tail attaches once per `containerID`; a restart (new ID) is re-tailed automatically, but a stream that drops while the *same* container keeps running is not auto-reconnected. Per-stream reconnect with a fresh `--since` (the spec's error-handling item) is a follow-up — `kubectl logs -f` is generally stable for local dev.
