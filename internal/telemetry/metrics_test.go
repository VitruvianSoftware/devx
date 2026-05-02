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

package telemetry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setupTestMetrics(t *testing.T) (cleanup func()) {
	t.Helper()
	tmp := t.TempDir()
	// Override HOME so metricsPath() resolves to temp dir
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	return func() {
		os.Setenv("HOME", orig)
	}
}

func TestRecordEvent_WritesEntry(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	RecordEvent("test_build", 5*time.Second)

	entries := LoadMetrics()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Event != "test_build" {
		t.Errorf("expected event 'test_build', got %q", entries[0].Event)
	}
	if entries[0].DurationMs != 5000 {
		t.Errorf("expected 5000ms, got %d", entries[0].DurationMs)
	}
}

func TestRecordEvent_FIFORotation(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	// Fill beyond the cap
	for i := 0; i < maxEntries+50; i++ {
		RecordEvent("fill", time.Duration(i)*time.Millisecond)
	}

	entries := LoadMetrics()
	if len(entries) != maxEntries {
		t.Fatalf("expected %d entries after rotation, got %d", maxEntries, len(entries))
	}

	// Oldest entries should have been trimmed — first entry should be #50 (0-indexed)
	if entries[0].DurationMs != 50 {
		t.Errorf("expected first entry duration 50ms after rotation, got %d", entries[0].DurationMs)
	}
}

func TestRecordEvent_CorruptedFileRecovery(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	p := metricsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte("{{{{not json!!!!"), 0644)

	// Should not panic — silently recovers
	RecordEvent("after_corrupt", 1*time.Second)

	entries := LoadMetrics()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after corrupt recovery, got %d", len(entries))
	}
	if entries[0].Event != "after_corrupt" {
		t.Errorf("expected event 'after_corrupt', got %q", entries[0].Event)
	}
}

func TestNudgeIfSlow_BelowThreshold(t *testing.T) {
	// Capture stderr
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	NudgeIfSlow("build", 30*time.Second, 60*time.Second, false)

	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	os.Stderr = old

	if buf.Len() != 0 {
		t.Errorf("expected no nudge output below threshold, got: %s", buf.String())
	}
}

func TestNudgeIfSlow_AboveThreshold(t *testing.T) {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	NudgeIfSlow("build", 65*time.Second, 60*time.Second, false)

	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	os.Stderr = old

	if buf.Len() == 0 {
		t.Error("expected nudge output above threshold, got nothing")
	}
	if !bytes.Contains(buf.Bytes(), []byte("predictive_build")) {
		t.Errorf("nudge should mention predictive_build, got: %s", buf.String())
	}
}

func TestNudgeIfSlow_SuppressedInJSONMode(t *testing.T) {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	NudgeIfSlow("build", 120*time.Second, 60*time.Second, true)

	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	os.Stderr = old

	if buf.Len() != 0 {
		t.Errorf("expected no nudge in JSON mode, got: %s", buf.String())
	}
}

func TestLoadMetrics_MissingFile(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	entries := LoadMetrics()
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestClearMetrics(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	RecordEvent("to_clear", 1*time.Second)
	entries := LoadMetrics()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry before clear, got %d", len(entries))
	}

	if err := ClearMetrics(); err != nil {
		t.Fatalf("ClearMetrics failed: %v", err)
	}

	entries = LoadMetrics()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestRecordEvent_ConcurrentSafety(t *testing.T) {
	cleanup := setupTestMetrics(t)
	defer cleanup()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			RecordEvent("concurrent", time.Duration(n)*time.Millisecond)
		}(i)
	}
	wg.Wait()

	entries := LoadMetrics()
	if len(entries) != 20 {
		t.Errorf("expected 20 entries from concurrent writes, got %d", len(entries))
	}

	// Verify all entries are valid JSON
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("corrupted JSON after concurrent writes: %v", err)
	}
	var check []MetricEntry
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("round-trip JSON failed: %v", err)
	}
}
