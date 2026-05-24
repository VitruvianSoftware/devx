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
	"strings"
	"testing"
)

func TestLineWriter_PrefixesEachLine_NoColor(t *testing.T) {
	var buf bytes.Buffer
	w := NewLineWriter(&buf, "api", false, nil)
	_, _ = w.Write([]byte("hello\nworld\n"))
	got := buf.String()
	want := "[api] hello\n[api] world\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLineWriter_BuffersPartialLines(t *testing.T) {
	var buf bytes.Buffer
	w := NewLineWriter(&buf, "api", false, nil)
	_, _ = w.Write([]byte("par"))
	_, _ = w.Write([]byte("tial\n"))
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
	_, _ = w.Write([]byte("token=supersecretvalue\n"))
	if strings.Contains(buf.String(), "supersecretvalue") {
		t.Errorf("expected secret to be redacted, got %q", buf.String())
	}
}
