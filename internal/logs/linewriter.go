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
