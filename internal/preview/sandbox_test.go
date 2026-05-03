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

package preview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_DerivedNames(t *testing.T) {
	tests := []struct {
		prNumber       int
		wantProject    string
		wantBranch     string
		wantWorktree   string
	}{
		{42, "pr-42", "devx-pr-42", filepath.Join(os.TempDir(), "devx-preview-42")},
		{1, "pr-1", "devx-pr-1", filepath.Join(os.TempDir(), "devx-preview-1")},
		{9999, "pr-9999", "devx-pr-9999", filepath.Join(os.TempDir(), "devx-preview-9999")},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("PR_%d", tt.prNumber), func(t *testing.T) {
			s := New(tt.prNumber)

			if s.ProjectName != tt.wantProject {
				t.Errorf("ProjectName = %q, want %q", s.ProjectName, tt.wantProject)
			}
			if s.LocalBranch != tt.wantBranch {
				t.Errorf("LocalBranch = %q, want %q", s.LocalBranch, tt.wantBranch)
			}
			if s.WorktreeDir != tt.wantWorktree {
				t.Errorf("WorktreeDir = %q, want %q", s.WorktreeDir, tt.wantWorktree)
			}
			if s.PRNumber != tt.prNumber {
				t.Errorf("PRNumber = %d, want %d", s.PRNumber, tt.prNumber)
			}
		})
	}
}

func TestDryRun_Output(t *testing.T) {
	s := New(42)
	output := s.DryRun()

	mustContain := []string{
		"DRY RUN: devx preview 42",
		"pr-42",
		"devx-pr-42",
		"devx-preview-42",
		"devx-db-pr-42-<engine>",
		"devx-data-pr-42-<engine>",
		"gh pr view 42",
		"git fetch origin pull/42/head:devx-pr-42",
		"git worktree add",
		"DEVX_PROJECT_OVERRIDE=pr-42 devx up",
		"git branch -D devx-pr-42",
	}

	for _, expected := range mustContain {
		if !strings.Contains(output, expected) {
			t.Errorf("DryRun output missing %q\n\nFull output:\n%s", expected, output)
		}
	}
}

func TestJSON_Structure(t *testing.T) {
	s := New(99)
	data, err := s.JSON()
	if err != nil {
		t.Fatalf("JSON() returned error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v", err)
	}

	requiredKeys := []string{
		"pr_number",
		"project_name",
		"local_branch",
		"worktree_dir",
		"db_prefix",
		"volume_prefix",
	}

	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("JSON output missing key %q", key)
		}
	}

	if result["pr_number"].(float64) != 99 {
		t.Errorf("pr_number = %v, want 99", result["pr_number"])
	}
	if result["project_name"].(string) != "pr-99" {
		t.Errorf("project_name = %v, want pr-99", result["project_name"])
	}
}
