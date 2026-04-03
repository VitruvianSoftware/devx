package ci

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRealWorkflow(t *testing.T) {
	// Find the real devx CI workflow
	wd, _ := os.Getwd()
	// Navigate from internal/ci to project root
	projectRoot := filepath.Join(wd, "..", "..")
	path := filepath.Join(projectRoot, ".github", "workflows", "ci.yml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("ci.yml not found at %s — skipping real workflow test", path)
	}

	wf, err := ParseWorkflow(path)
	if err != nil {
		t.Fatalf("ParseWorkflow failed: %v", err)
	}

	if wf.Name != "CI" {
		t.Errorf("expected workflow name 'CI', got %q", wf.Name)
	}

	// Verify expected jobs exist
	expectedJobs := []string{"lint", "test", "build", "validate-template"}
	for _, name := range expectedJobs {
		if _, ok := wf.Jobs[name]; !ok {
			t.Errorf("expected job %q not found in workflow", name)
		}
	}

	// Verify build job has matrix
	buildJob := wf.Jobs["build"]
	if buildJob == nil {
		t.Fatal("build job not found")
	}
	if len(buildJob.Strategy.Matrix.Values) == 0 {
		t.Error("build job should have matrix values")
	}
	if goos, ok := buildJob.Strategy.Matrix.Values["goos"]; !ok || len(goos) != 2 {
		t.Errorf("expected goos axis with 2 values, got %v", goos)
	}
	if goarch, ok := buildJob.Strategy.Matrix.Values["goarch"]; !ok || len(goarch) != 2 {
		t.Errorf("expected goarch axis with 2 values, got %v", goarch)
	}

	// Verify build job has needs
	if len(buildJob.Needs) != 2 {
		t.Errorf("expected build to need 2 jobs, got %d: %v", len(buildJob.Needs), buildJob.Needs)
	}
}

func TestMatrixExpansion(t *testing.T) {
	job := &Job{
		Strategy: Strategy{
			Matrix: MatrixDef{
				Values: map[string][]string{
					"goos":   {"darwin", "linux"},
					"goarch": {"amd64", "arm64"},
				},
			},
		},
	}

	expanded := ExpandMatrix("build", job)

	if len(expanded) != 4 {
		t.Fatalf("expected 4 matrix expansions, got %d", len(expanded))
	}

	// Verify all combinations exist
	seen := map[string]bool{}
	for _, ej := range expanded {
		key := ej.MatrixValues["goos"] + "/" + ej.MatrixValues["goarch"]
		seen[key] = true
	}

	expected := []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
	for _, e := range expected {
		if !seen[e] {
			t.Errorf("missing matrix combination: %s", e)
		}
	}
}

func TestMatrixExpansionWithExclude(t *testing.T) {
	job := &Job{
		Strategy: Strategy{
			Matrix: MatrixDef{
				Values: map[string][]string{
					"goos":   {"darwin", "linux"},
					"goarch": {"amd64", "arm64"},
				},
				Exclude: []map[string]string{
					{"goos": "linux", "goarch": "arm64"},
				},
			},
		},
	}

	expanded := ExpandMatrix("build", job)

	if len(expanded) != 3 {
		t.Fatalf("expected 3 matrix expansions (1 excluded), got %d", len(expanded))
	}

	for _, ej := range expanded {
		if ej.MatrixValues["goos"] == "linux" && ej.MatrixValues["goarch"] == "arm64" {
			t.Error("linux/arm64 should have been excluded")
		}
	}
}

func TestNoMatrix(t *testing.T) {
	job := &Job{}
	expanded := ExpandMatrix("test", job)

	if len(expanded) != 1 {
		t.Fatalf("expected 1 expansion for no-matrix job, got %d", len(expanded))
	}
	if expanded[0].DisplayName != "test" {
		t.Errorf("expected display name 'test', got %q", expanded[0].DisplayName)
	}
}

func TestJobDAG(t *testing.T) {
	jobs := map[string]*Job{
		"lint":              {Needs: nil},
		"test":              {Needs: nil},
		"build":             {Needs: StringOrSlice{"lint", "test"}},
		"validate-template": {Needs: nil},
	}

	tiers, err := ResolveJobDAG(jobs)
	if err != nil {
		t.Fatalf("ResolveJobDAG failed: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}

	// Tier 0 should contain lint, test, validate-template (no dependencies)
	if len(tiers[0]) != 3 {
		t.Errorf("expected 3 jobs in tier 0, got %d: %v", len(tiers[0]), tiers[0])
	}

	// Tier 1 should contain build
	if len(tiers[1]) != 1 || tiers[1][0] != "build" {
		t.Errorf("expected tier 1 = [build], got %v", tiers[1])
	}
}

func TestJobDAGCycleDetection(t *testing.T) {
	jobs := map[string]*Job{
		"a": {Needs: StringOrSlice{"b"}},
		"b": {Needs: StringOrSlice{"a"}},
	}

	_, err := ResolveJobDAG(jobs)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	if !contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error message, got: %s", err.Error())
	}
}

func TestTemplateSubstitution(t *testing.T) {
	tc := NewTemplateContext(
		map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
		map[string]string{"API_KEY": "secret123"},
		map[string]string{"goos": "darwin", "goarch": "arm64"},
	)

	tests := []struct {
		input    string
		expected string
	}{
		{"${{ env.CGO_ENABLED }}", "0"},
		{"${{ secrets.API_KEY }}", "secret123"},
		{"${{ matrix.goos }}", "darwin"},
		{"${{ matrix.goarch }}", "arm64"},
		{"GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }}", "GOOS=darwin GOARCH=arm64"},
		{"${{ github.event_name }}", "push"},
		{"${{ runner.os }}", "Linux"},
		{"no expressions here", "no expressions here"},
	}

	for _, tt := range tests {
		result := tc.Substitute(tt.input)
		if result != tt.expected {
			t.Errorf("Substitute(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConditionEvaluation(t *testing.T) {
	tc := NewTemplateContext(
		map[string]string{"DEPLOY": "true"},
		nil,
		map[string]string{"goos": "darwin"},
	)

	tests := []struct {
		condition string
		expected  bool
	}{
		{"", true},                                  // empty = always run
		{"${{ matrix.goos == 'darwin' }}", true},    // equality match
		{"${{ matrix.goos == 'linux' }}", false},    // equality mismatch
		{"${{ matrix.goos != 'linux' }}", true},     // inequality match
		{"${{ matrix.goos != 'darwin' }}", false},   // inequality mismatch
		{"contains(github.ref, 'main')", true},      // complex → fail-open
	}

	for _, tt := range tests {
		result := tc.EvaluateCondition(tt.condition)
		if result != tt.expected {
			t.Errorf("EvaluateCondition(%q) = %v, want %v", tt.condition, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
