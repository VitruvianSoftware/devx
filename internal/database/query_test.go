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

package database

import (
	"strings"
	"testing"
)

func TestParseQueryOutput_Postgres(t *testing.T) {
	output := "table\ttotal_size\testimated_rows\nusers\t16 kB\t42\norders\t8192 bytes\t10\n(2 rows)"
	result := parseQueryOutput("postgres", output, "SELECT ...")

	if len(result.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %d: %v", len(result.Headers), result.Headers)
	}
	if result.Headers[0] != "table" {
		t.Errorf("expected first header 'table', got %q", result.Headers[0])
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if result.Rows[0][0] != "users" {
		t.Errorf("expected first row first col 'users', got %q", result.Rows[0][0])
	}
}

func TestParseQueryOutput_Empty(t *testing.T) {
	result := parseQueryOutput("postgres", "", "SELECT ...")
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows for empty output, got %d", len(result.Rows))
	}
}

func TestParseQueryOutput_FilterNoise(t *testing.T) {
	output := "SET\nBEGIN\ntable\ttotal_size\nusers\t16 kB\nCOMMIT\n(1 row)"
	result := parseQueryOutput("postgres", output, "SELECT ...")

	if len(result.Headers) != 2 {
		t.Fatalf("expected 2 headers after filtering, got %d: %v", len(result.Headers), result.Headers)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row after filtering, got %d", len(result.Rows))
	}
}

func TestRenderTable_EmptyResult(t *testing.T) {
	result := &QueryResult{Headers: []string{"a"}, Rows: nil}
	output := RenderTable(result)
	if !strings.Contains(output, "0 rows") {
		t.Errorf("expected '0 rows' message, got: %s", output)
	}
}

func TestRenderTable_NilResult(t *testing.T) {
	output := RenderTable(nil)
	if !strings.Contains(output, "0 rows") {
		t.Errorf("expected '0 rows' message for nil, got: %s", output)
	}
}

func TestRenderTable_WithData(t *testing.T) {
	result := &QueryResult{
		Headers: []string{"name", "age"},
		Rows: [][]string{
			{"Alice", "30"},
			{"Bob", "25"},
		},
	}
	output := RenderTable(result)
	if !strings.Contains(output, "2 rows") {
		t.Errorf("expected '2 rows' in output, got: %s", output)
	}
}

func TestCannedQueries_AllEnginesHaveSQL(t *testing.T) {
	for name, canned := range CannedQueries {
		for _, engine := range []string{"postgres", "mysql"} {
			if _, ok := canned.SQL[engine]; !ok {
				t.Errorf("canned query %q missing SQL for engine %s", name, engine)
			}
		}
	}
}
