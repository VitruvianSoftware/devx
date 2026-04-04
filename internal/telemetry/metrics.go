package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// maxEntries is the FIFO cap for the local metrics file.
const maxEntries = 1000

// MetricEntry is a single recorded event.
type MetricEntry struct {
	Event      string `json:"event"`
	DurationMs int64  `json:"duration_ms"`
	Timestamp  string `json:"timestamp"`
}

// metricsPath returns the absolute path to the local metrics file.
func metricsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx", "metrics.json")
}

// RecordEvent appends a timestamped duration entry to ~/.devx/metrics.json
// and opportunistically exports an OTel span to localhost:4318 if a backend is running.
// Safe for concurrent use (file-level flock). Silently no-ops on any I/O error.
func RecordEvent(event string, duration time.Duration, attrs ...Attribute) {
	p := metricsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)

	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	var entries []MetricEntry
	data, _ := os.ReadFile(p)
	if len(data) > 0 {
		// Tolerate corrupted files — start fresh if unmarshal fails
		_ = json.Unmarshal(data, &entries)
	}

	entries = append(entries, MetricEntry{
		Event:      event,
		DurationMs: duration.Milliseconds(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})

	// FIFO rotation: trim oldest entries if over cap
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}

	// Truncate and rewrite
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.Write(out)

	// Dual-write: opportunistically export to local OTel backend (fire-and-forget)
	exportWg.Add(1)
	go func() {
		defer exportWg.Done()
		ExportSpan(event, duration, attrs...)
	}()
}

var exportWg sync.WaitGroup

// Flush waits up to 100ms for pending OTLP exports to complete.
// Call this before exiting the CLI to ensure telemetry spans aren't dropped.
func Flush() {
	c := make(chan struct{})
	go func() {
		exportWg.Wait()
		close(c)
	}()
	select {
	case <-c:
	case <-time.After(100 * time.Millisecond):
	}
}

// NudgeIfSlow prints an actionable tip to stderr if duration exceeds threshold.
// Suppressed when jsonMode is true (for --json flag compliance).
func NudgeIfSlow(event string, duration, threshold time.Duration, jsonMode bool) {
	if jsonMode || duration < threshold {
		return
	}
	fmt.Fprintf(os.Stderr, "\n💡 Tip: Your %s took %s. Enable 'predictive_build: true' on container\n", event, duration.Round(time.Second))
	fmt.Fprintf(os.Stderr, "   services in devx.yaml to have devx silently pre-build heavy dependency\n")
	fmt.Fprintf(os.Stderr, "   layers in the background. See: https://devx.vitruviansoftware.dev/guide/caching\n\n")
}

// LoadMetrics reads all metric entries from the local metrics file.
// Returns an empty slice (not nil) if the file is missing or corrupted.
func LoadMetrics() []MetricEntry {
	data, err := os.ReadFile(metricsPath())
	if err != nil {
		return []MetricEntry{}
	}
	var entries []MetricEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return []MetricEntry{}
	}
	return entries
}

// ClearMetrics truncates the local metrics file.
func ClearMetrics() error {
	return os.Truncate(metricsPath(), 0)
}
