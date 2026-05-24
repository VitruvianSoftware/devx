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

package orchestrator

import "testing"

func b(v bool) *bool { return &v }

func TestResolveLogMode(t *testing.T) {
	cases := []struct {
		name                   string
		flag, perSvc, topLevel *bool
		runtime                string
		want                   LogMode
	}{
		{"host default → raw", nil, nil, nil, "host", LogRaw},
		{"k8s default → off", nil, nil, nil, "kubernetes", LogOff},
		{"container default → off", nil, nil, nil, "container", LogOff},
		{"flag on → prefixed (k8s)", b(true), nil, nil, "kubernetes", LogPrefixed},
		{"flag on → prefixed (host)", b(true), nil, nil, "host", LogPrefixed},
		{"flag off → off (host)", b(false), nil, nil, "host", LogOff},
		{"flag beats per-service", b(false), b(true), nil, "kubernetes", LogOff},
		{"per-service beats top-level", nil, b(false), b(true), "kubernetes", LogOff},
		{"top-level on → prefixed", nil, nil, b(true), "cloud", LogPrefixed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveLogMode(c.flag, c.perSvc, c.topLevel, c.runtime); got != c.want {
				t.Errorf("ResolveLogMode = %v, want %v", got, c.want)
			}
		})
	}
}
