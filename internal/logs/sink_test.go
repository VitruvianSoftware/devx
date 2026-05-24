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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHostSink_RawKeepsStdoutStderrSeparate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	var out, errb bytes.Buffer
	outW, errW, closeFn, err := BuildHostSink("api", LogRawMode, &out, &errb, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = outW.Write([]byte("to-stdout\n"))
	_, _ = errW.Write([]byte("to-stderr\n"))
	_ = closeFn()
	if out.String() != "to-stdout\n" {
		t.Errorf("stdout = %q, want \"to-stdout\\n\"", out.String())
	}
	if errb.String() != "to-stderr\n" {
		t.Errorf("stderr = %q, want \"to-stderr\\n\"", errb.String())
	}
	fileBytes, _ := os.ReadFile(ServiceLogPath("api"))
	if got := string(fileBytes); !strings.Contains(got, "to-stdout") || !strings.Contains(got, "to-stderr") {
		t.Errorf("file should contain both streams, got %q", got)
	}
}

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
	defer func() { _ = f.Close() }()
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
	_, _ = w.Write([]byte("hi\n"))
	_ = closeFn()
	if inline.String() != "[api] hi\n" {
		t.Errorf("inline = %q", inline.String())
	}
	fileBytes, _ := os.ReadFile(ServiceLogPath("api"))
	if string(fileBytes) != "hi\n" {
		t.Errorf("file = %q", string(fileBytes))
	}
}
