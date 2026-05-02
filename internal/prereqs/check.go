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
