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

package cmd

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractToolNamesFromStreamFindsToolsList(t *testing.T) {
	// Simulate a stdio response stream: an initialize result followed by
	// a tools/list result. Each line is a separate JSON-RPC message.
	stream := []byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"devx","version":"0.0.0"}}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"devx_doctor"},{"name":"devx_db_list"}]}}
`)
	names, err := extractToolNamesFromStream(stream)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	sort.Strings(names)
	want := []string{"devx_db_list", "devx_doctor"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestExtractToolNamesFromStreamErrorsOnNoMatch(t *testing.T) {
	stream := []byte(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}` + "\n")
	if _, err := extractToolNamesFromStream(stream); err == nil {
		t.Errorf("expected error when no tools/list response present")
	}
}

func TestExtractToolNamesSkipsMalformedLines(t *testing.T) {
	// Tolerate stderr leaks or partial buffers mixed into the stream.
	stream := []byte(`garbage that should be ignored
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"devx_doctor"}]}}
`)
	names, err := extractToolNamesFromStream(stream)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(names) != 1 || names[0] != "devx_doctor" {
		t.Errorf("got %v, want [devx_doctor]", names)
	}
}

func TestDisplayPathHandlesEmptyPath(t *testing.T) {
	if got := displayPath("", ""); got != "(manual — see message)" {
		t.Errorf("got %q", got)
	}
	if got := displayPath("/x/y.json", "project"); got != "/x/y.json [project]" {
		t.Errorf("got %q", got)
	}
	if got := displayPath("/x/y.json", ""); got != "/x/y.json" {
		t.Errorf("got %q", got)
	}
}
