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

package mcpserver

import (
	"reflect"
	"strings"
	"testing"
)

func TestEveryToolSpecHasNameAndBuilder(t *testing.T) {
	for _, spec := range toolSpecs {
		if spec.name == "" {
			t.Errorf("toolSpec with empty name: %+v", spec)
		}
		if !strings.HasPrefix(spec.name, "devx_") {
			t.Errorf("tool %q should be prefixed with devx_ to avoid collisions", spec.name)
		}
		if spec.description == "" {
			t.Errorf("tool %q missing description", spec.name)
		}
		if spec.build == nil {
			t.Errorf("tool %q has nil builder", spec.name)
		}
	}
}

func TestToolNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range toolSpecs {
		if seen[s.name] {
			t.Errorf("duplicate tool name: %s", s.name)
		}
		seen[s.name] = true
	}
}

func TestReadOnlyAndDestructiveAreMutuallyExclusive(t *testing.T) {
	for _, s := range toolSpecs {
		if s.readOnly && s.destructive {
			t.Errorf("tool %q cannot be both read-only and destructive", s.name)
		}
	}
}

func TestDestructiveToolsAreFlagged(t *testing.T) {
	// Spot-check: anything ending in _rm, _teardown, _restore, or named
	// state_attach / ship must carry the destructive flag.
	mustBeDestructive := []string{
		"devx_db_rm",
		"devx_cloud_rm",
		"devx_vm_teardown",
		"devx_db_snapshot_restore",
		"devx_state_attach",
		"devx_ship",
	}
	flagged := map[string]bool{}
	for _, s := range toolSpecs {
		if s.destructive {
			flagged[s.name] = true
		}
	}
	for _, name := range mustBeDestructive {
		if !flagged[name] {
			t.Errorf("expected %q to be marked destructive", name)
		}
	}
}

func TestBuildDevxArgs(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want []string
	}{
		{"devx_doctor", nil, []string{"doctor", "--json"}},
		{"devx_db_list", nil, []string{"db", "list", "--json"}},
		{"devx_db_spawn", map[string]any{"engine": "postgres"}, []string{"db", "spawn", "postgres", "-y", "--json"}},
		{"devx_db_spawn", map[string]any{"engine": "postgres", "port": 5433.0}, []string{"db", "spawn", "postgres", "-y", "--json", "-p", "5433"}},
		{"devx_cloud_spawn", map[string]any{"service": "s3"}, []string{"cloud", "spawn", "s3"}},
		{"devx_cloud_spawn", map[string]any{"service": "gcs", "port": 4443.0}, []string{"cloud", "spawn", "gcs", "-p", "4443"}},
		{"devx_db_snapshot_create", map[string]any{"engine": "postgres", "name": "pre-mig"}, []string{"db", "snapshot", "create", "postgres", "pre-mig", "-y", "--json"}},
		{"devx_state_share", map[string]any{"db_only": true}, []string{"state", "share", "--json", "--db-only"}},
		{"devx_state_attach", map[string]any{"id": "s3://x:fast-blue-rabbit-dawn"}, []string{"state", "attach", "s3://x:fast-blue-rabbit-dawn", "-y", "--json"}},
		{"devx_ci_run", nil, []string{"ci", "run", "-y", "--json"}},
		{"devx_ci_run", map[string]any{"workflow": "ci.yml", "job": "test"}, []string{"ci", "run", "-y", "--json", "ci.yml", "--job", "test"}},
		{"devx_ci_run", map[string]any{"dry_run": true}, []string{"ci", "run", "-y", "--json", "--dry-run"}},
		{"devx_ship", map[string]any{"message": "feat: X"}, []string{"agent", "ship", "-m", "feat: X"}},
		{"devx_ship", map[string]any{"message": "feat: X", "base": "develop"}, []string{"agent", "ship", "-m", "feat: X", "--base", "develop"}},
		{"devx_db_rm", map[string]any{"engine": "postgres", "purge": true}, []string{"db", "rm", "postgres", "-y", "--purge"}},
		{"devx_cloud_rm", map[string]any{"service": "s3"}, []string{"cloud", "rm", "s3", "-y"}},
		{"devx_vm_teardown", nil, []string{"vm", "teardown", "-y"}},
	}
	for _, tc := range cases {
		spec := findSpec(t, tc.tool)
		got, err := spec.build(tc.args)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.tool, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.tool, got, tc.want)
		}
	}
}

func TestBuildDevxArgsRequiresMandatoryFields(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"devx_db_spawn", nil},
		{"devx_db_spawn", map[string]any{"engine": ""}},
		{"devx_cloud_spawn", nil},
		{"devx_db_snapshot_create", map[string]any{"engine": "postgres"}}, // missing name
		{"devx_state_attach", nil},
		{"devx_ship", nil},
		{"devx_db_rm", nil},
	}
	for _, tc := range cases {
		spec := findSpec(t, tc.tool)
		if _, err := spec.build(tc.args); err == nil {
			t.Errorf("%s with args %v: expected error, got nil", tc.tool, tc.args)
		}
	}
}

func TestAllToolNamesIsStableSet(t *testing.T) {
	names := AllToolNames()
	if len(names) != len(toolSpecs) {
		t.Errorf("AllToolNames returned %d names, want %d", len(names), len(toolSpecs))
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate name: %s", n)
		}
		seen[n] = true
	}
}

func findSpec(t *testing.T, name string) toolSpec {
	t.Helper()
	for _, s := range toolSpecs {
		if s.name == name {
			return s
		}
	}
	t.Fatalf("no tool spec named %q", name)
	return toolSpec{}
}
