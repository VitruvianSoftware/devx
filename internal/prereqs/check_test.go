package prereqs_test

import (
	"fmt"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/prereqs"
)

func TestAllPassed_AllFound(t *testing.T) {
	results := []prereqs.Result{
		{Name: "podman", Found: true},
		{Name: "cloudflared", Found: true},
		{Name: "butane", Found: true},
	}
	if !prereqs.AllPassed(results) {
		t.Error("expected AllPassed = true when all tools found")
	}
}

func TestAllPassed_OneMissing(t *testing.T) {
	results := []prereqs.Result{
		{Name: "podman", Found: true},
		{Name: "cloudflared", Found: false},
		{Name: "butane", Found: true},
	}
	if prereqs.AllPassed(results) {
		t.Error("expected AllPassed = false when a tool is missing")
	}
}

func TestSummary_OnlyFoundTools(t *testing.T) {
	results := []prereqs.Result{
		{Name: "podman", Found: true},
		{Name: "cloudflared", Found: false},
		{Name: "butane", Found: true},
	}
	got := prereqs.Summary(results)
	want := "podman, butane"
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestMissingList_ReturnsMissing(t *testing.T) {
	results := []prereqs.Result{
		{Name: "podman", Found: true},
		{Name: "cloudflared", Found: false, Error: fmt.Errorf("cloudflared not found in PATH")},
	}
	missing := prereqs.MissingList(results)
	if len(missing) != 1 {
		t.Errorf("MissingList() returned %d items, want 1", len(missing))
	}
}

// TestCheck_Real verifies Check() runs and returns the right number of results.
// It doesn't assert specific tool presence since CI may not have all tools.
func TestCheck_ReturnsThreeResults(t *testing.T) {
	results := prereqs.Check()
	if len(results) != 3 {
		t.Errorf("Check() returned %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Name == "" {
			t.Error("result has empty Name")
		}
	}
}
