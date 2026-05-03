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

package ci

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// colors for prefixed output — each parallel job gets a unique color.
var prefixColors = []string{
	"#FF5F87", "#FFF700", "#00FF00", "#00FFFF",
	"#FF00FF", "#8A2BE2", "#FFA500", "#00FF7F",
}

// PrefixedWriter is a thread-safe, line-buffered writer that prepends a
// color-coded job name prefix to each line. It ensures parallel goroutine
// output doesn't interleave mid-line — identical to Docker Compose's
// multiplexed output strategy.
type PrefixedWriter struct {
	prefix string
	style  lipgloss.Style
	dest   io.Writer
	mu     *sync.Mutex // shared across all writers for atomicity
	buf    []byte
}

// writerRegistry tracks color assignment for consistent prefix coloring.
var (
	writerMu     sync.Mutex
	writerColors = map[string]int{}
	colorCounter int
)

// NewPrefixedWriter creates a writer that prepends a styled prefix to every line.
// The mutex should be shared across all parallel writers to prevent interleaving.
func NewPrefixedWriter(jobName string, dest io.Writer, mu *sync.Mutex) *PrefixedWriter {
	writerMu.Lock()
	idx, ok := writerColors[jobName]
	if !ok {
		idx = colorCounter % len(prefixColors)
		writerColors[jobName] = idx
		colorCounter++
	}
	writerMu.Unlock()

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(prefixColors[idx])).
		Bold(true)

	return &PrefixedWriter{
		prefix: jobName,
		style:  style,
		dest:   dest,
		mu:     mu,
	}
}

// Write implements io.Writer. It buffers input and flushes complete lines
// with the prefix prepended.
func (pw *PrefixedWriter) Write(p []byte) (n int, err error) {
	pw.buf = append(pw.buf, p...)

	for {
		idx := indexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}

		line := string(pw.buf[:idx])
		pw.buf = pw.buf[idx+1:]

		pw.mu.Lock()
		styledPrefix := pw.style.Render(fmt.Sprintf("%-20s", pw.prefix))
		_, _ = fmt.Fprintf(pw.dest, "%s │ %s\n", styledPrefix, line)
		pw.mu.Unlock()
	}

	return len(p), nil
}

// Flush writes any remaining buffered content.
func (pw *PrefixedWriter) Flush() {
	if len(pw.buf) > 0 {
		pw.mu.Lock()
		styledPrefix := pw.style.Render(fmt.Sprintf("%-20s", pw.prefix))
		_, _ = fmt.Fprintf(pw.dest, "%s │ %s\n", styledPrefix, string(pw.buf))
		pw.mu.Unlock()
		pw.buf = nil
	}
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

// ResetWriterRegistry clears the color assignment state. Useful for testing.
func ResetWriterRegistry() {
	writerMu.Lock()
	defer writerMu.Unlock()
	writerColors = map[string]int{}
	colorCounter = 0
}

// CondensedMatrixName generates a short display name for matrix jobs
// suitable for the prefix column. E.g., "build (darwin, arm64)" → "build·dar·a64"
func CondensedMatrixName(jobKey string, matrix map[string]string) string {
	if len(matrix) == 0 {
		return jobKey
	}

	var parts []string
	// Sort keys for determinism
	keys := make([]string, 0, len(matrix))
	for k := range matrix {
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, k := range keys {
		v := matrix[k]
		// Abbreviate to 3 chars
		if len(v) > 3 {
			v = v[:3]
		}
		parts = append(parts, v)
	}

	return jobKey + "·" + strings.Join(parts, "·")
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
