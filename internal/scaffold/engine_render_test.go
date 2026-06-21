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

package scaffold

import "testing"

// TestRegistryTemplatesRender renders every registered template into a temp dir
// and fails if any .tmpl cannot be parsed/executed. This guards a real class of
// bug: an unescaped "{{ }}" in a template — e.g. a GitHub Actions ${{ ... }}
// expression that collides with text/template's delimiters — is a PARSE error
// that silently broke `devx scaffold go-cli` (the workflow ${{ secrets.* }} was
// not escaped) until it was found by hand. There were previously no render tests.
func TestRegistryTemplatesRender(t *testing.T) {
	vars := Vars{
		ProjectName: "sample-service",
		ProjectSlug: "sample_service",
		ModulePath:  "github.com/acme/sample-service",
		Author:      "Test Author",
		Year:        "2026",
		Domain:      "example.dev",
		GoVersion:   "1.25",
		Description: "A sample service.",
		HasDatabase: true,
		DBEngine:    "postgres",
	}
	for _, tmpl := range Registry {
		t.Run(tmpl.ID, func(t *testing.T) {
			if _, err := Scaffold(tmpl.ID, t.TempDir(), vars, true); err != nil {
				t.Fatalf("Scaffold(%q) failed to render: %v", tmpl.ID, err)
			}
		})
	}
}
