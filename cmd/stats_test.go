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

package cmd

import (
	"testing"
)

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		data   []int64
		p      int
		expect int64
	}{
		{"empty", []int64{}, 50, 0},
		{"single", []int64{100}, 50, 100},
		{"single_p99", []int64{100}, 99, 100},
		{"two_p50", []int64{100, 200}, 50, 100},
		{"two_p90", []int64{100, 200}, 90, 200},
		{"ten_p50", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 50, 50},
		{"ten_p90", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 90, 90},
		{"ten_p99", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 99, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.data, tt.p)
			if got != tt.expect {
				t.Errorf("percentile(%v, %d) = %d, want %d", tt.data, tt.p, got, tt.expect)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms     int64
		expect string
	}{
		{50, "50ms"},
		{500, "500ms"},
		{1000, "1.0s"},
		{5200, "5.2s"},
		{62000, "1m2s"},
		{125000, "2m5s"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			got := formatDuration(tt.ms)
			if got != tt.expect {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.ms, got, tt.expect)
			}
		})
	}
}
