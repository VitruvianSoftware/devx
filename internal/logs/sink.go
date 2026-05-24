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
	"io"
	"os"
	"path/filepath"
)

// SinkMode mirrors orchestrator.LogMode without importing it (logs must not
// depend on orchestrator). The orchestrator maps its LogMode to these.
type SinkMode int

const (
	LogOffMode      SinkMode = iota // file only (no inline)
	LogRawMode                      // inline raw + file
	LogPrefixedMode                 // inline prefixed+colored + file
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

// BuildHostSink is like BuildSink but returns separate stdout and stderr writers.
// A host child process is the only producer with distinct stdout/stderr streams;
// in raw mode (the host default) its stderr must stay on os.Stderr to preserve
// today's behavior (e.g. `devx up 2>err.log`). In prefixed mode both streams share
// one mutex-guarded LineWriter (the unified inline view); in off mode both are
// file-only. The same log file backs both writers.
func BuildHostSink(service string, mode SinkMode, stdout, stderr io.Writer, color bool, redactor *SecretRedactor) (outW, errW io.Writer, closeFn func() error, err error) {
	f, err := OpenServiceLog(service)
	if err != nil {
		return nil, nil, nil, err
	}
	closeFn = func() error { return f.Close() }
	switch mode {
	case LogRawMode:
		return io.MultiWriter(stdout, f), io.MultiWriter(stderr, f), closeFn, nil
	case LogPrefixedMode:
		lw := NewLineWriter(stdout, service, color, redactor)
		return io.MultiWriter(lw, f), io.MultiWriter(lw, f), closeFn, nil
	default: // LogOffMode
		return f, f, closeFn, nil
	}
}
