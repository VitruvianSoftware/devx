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
