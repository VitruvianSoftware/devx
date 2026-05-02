package prereqs

import (
	"fmt"
	"os/exec"
	"strings"
)

// Result represents the check result for a single tool.
type Result struct {
	Name    string
	Found   bool
	Version string
	Error   error
}

// Check verifies all required tools are present on PATH.
func Check(vmBinary string) []Result {
	tools := []string{vmBinary, "cloudflared", "butane"}
	results := make([]Result, len(tools))
	for i, tool := range tools {
		out, err := exec.Command(tool, "--version").Output()
		if err != nil {
			// Some tools use "version" without dashes
			out, err = exec.Command(tool, "version").Output()
		}
		r := Result{Name: tool, Found: err == nil}
		if err == nil {
			// Take just the first line of version output
			lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
			r.Version = lines[0]
		} else {
			r.Error = fmt.Errorf("%s not found in PATH: install with 'brew install %s'", tool, tool)
		}
		results[i] = r
	}
	return results
}

// AllPassed returns true if every tool was found.
func AllPassed(results []Result) bool {
	for _, r := range results {
		if !r.Found {
			return false
		}
	}
	return true
}

// Summary formats the version string for display.
func Summary(results []Result) string {
	names := make([]string, 0, len(results))
	for _, r := range results {
		if r.Found {
			names = append(names, r.Name)
		}
	}
	return strings.Join(names, ", ")
}

// MissingList returns error strings for all missing tools.
func MissingList(results []Result) []string {
	var missing []string
	for _, r := range results {
		if !r.Found {
			missing = append(missing, r.Error.Error())
		}
	}
	return missing
}
