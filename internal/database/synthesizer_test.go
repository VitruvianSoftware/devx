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

func TestSynthesizableEngines(t *testing.T) {
	engines := SynthesizableEngines()
	if len(engines) != 2 {
		t.Errorf("expected 2 synthesizable engines, got %d", len(engines))
	}
	expected := map[string]bool{"postgres": true, "mysql": true}
	for _, e := range engines {
		if !expected[e] {
			t.Errorf("unexpected engine %q in SynthesizableEngines()", e)
		}
	}
}

func TestIsSynthesizable(t *testing.T) {
	tests := []struct {
		engine string
		want   bool
	}{
		{"postgres", true},
		{"mysql", true},
		{"redis", false},
		{"mongo", false},
		{"sqlite", false},
	}

	for _, tt := range tests {
		if got := IsSynthesizable(tt.engine); got != tt.want {
			t.Errorf("IsSynthesizable(%q) = %v, want %v", tt.engine, got, tt.want)
		}
	}
}

func TestSanitizeLLMSQL_MarkdownWrapped(t *testing.T) {
	input := "```sql\nINSERT INTO users (name) VALUES ('alice');\nINSERT INTO users (name) VALUES ('bob');\n```"
	result := SanitizeLLMSQL(input)

	if !strings.HasPrefix(result, "BEGIN;") {
		t.Errorf("expected BEGIN; prefix, got: %s", result[:20])
	}
	if !strings.HasSuffix(result, "COMMIT;") {
		t.Errorf("expected COMMIT; suffix, got: %s", result[len(result)-20:])
	}
	if !strings.Contains(result, "INSERT INTO users") {
		t.Error("expected INSERT INTO users in result")
	}
	if strings.Contains(result, "```") {
		t.Error("backticks should have been stripped")
	}
}

func TestSanitizeLLMSQL_RawSQL(t *testing.T) {
	input := "INSERT INTO users (name) VALUES ('alice');"
	result := SanitizeLLMSQL(input)

	if !strings.HasPrefix(result, "BEGIN;") {
		t.Errorf("expected BEGIN; prefix for raw SQL")
	}
	if !strings.Contains(result, "INSERT INTO users") {
		t.Error("expected original SQL preserved")
	}
}

func TestSanitizeLLMSQL_AlreadyTransactional(t *testing.T) {
	input := "BEGIN;\nINSERT INTO users (name) VALUES ('alice');\nCOMMIT;"
	result := SanitizeLLMSQL(input)

	// Should not double-wrap
	if strings.Count(result, "BEGIN;") != 1 {
		t.Errorf("expected exactly one BEGIN;, got %d", strings.Count(result, "BEGIN;"))
	}
}

func TestSanitizeLLMSQL_Empty(t *testing.T) {
	result := SanitizeLLMSQL("")
	if result != "" {
		t.Errorf("expected empty string for empty input, got %q", result)
	}
}

func TestSanitizeLLMSQL_MultipleCodeBlocks(t *testing.T) {
	input := "Here's the SQL:\n```sql\nINSERT INTO users (name) VALUES ('a');\n```\nAnd more:\n```sql\nINSERT INTO orders (user_id) VALUES (1);\n```"
	result := SanitizeLLMSQL(input)

	if !strings.Contains(result, "INSERT INTO users") {
		t.Error("expected first INSERT preserved")
	}
	if !strings.Contains(result, "INSERT INTO orders") {
		t.Error("expected second INSERT preserved")
	}
	if strings.Contains(result, "```") {
		t.Error("backticks should have been stripped")
	}
}

func TestSanitizeLLMSQL_NonSQLMarkdown(t *testing.T) {
	input := "```\nINSERT INTO test (a) VALUES (1);\n```"
	result := SanitizeLLMSQL(input)

	if !strings.Contains(result, "INSERT INTO test") {
		t.Error("expected SQL from generic code block")
	}
}
