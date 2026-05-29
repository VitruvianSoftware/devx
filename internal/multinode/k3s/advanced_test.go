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

package k3s

import (
	"strings"
	"testing"
)

// k3s `etcd-snapshot save --name <base>` does not create a file literally named
// <base>; it appends "-<node-name>-<unix-timestamp>" (and ".zip" when
// compression is enabled). findSnapshotFile must resolve the real on-disk name
// from a directory listing by matching the base name as a prefix.
func TestFindSnapshotFile(t *testing.T) {
	tests := []struct {
		name     string
		lsOutput string
		prefix   string
		want     string
	}{
		{
			name:     "k3s appends node name and unix timestamp to the --name base",
			lsOutput: "cluster-snapshot-20260529-150405-james-dev-machine-1748539445\n",
			prefix:   "cluster-snapshot-20260529-150405",
			want:     "cluster-snapshot-20260529-150405-james-dev-machine-1748539445",
		},
		{
			name:     "compressed snapshots carry a .zip extension",
			lsOutput: "cluster-snapshot-20260529-150405-node1-1748539445.zip\n",
			prefix:   "cluster-snapshot-20260529-150405",
			want:     "cluster-snapshot-20260529-150405-node1-1748539445.zip",
		},
		{
			name: "ignores unrelated snapshots and picks the matching prefix",
			lsOutput: strings.Join([]string{
				"on-demand-server-0-1753178523",
				"cluster-snapshot-20260101-000000-node1-1700000000",
				"cluster-snapshot-20260529-150405-node1-1748539445",
			}, "\n"),
			prefix: "cluster-snapshot-20260529-150405",
			want:   "cluster-snapshot-20260529-150405-node1-1748539445",
		},
		{
			name: "multiple matches returns the most recent (greatest timestamp)",
			lsOutput: strings.Join([]string{
				"cluster-snapshot-20260529-150405-node1-1748539445",
				"cluster-snapshot-20260529-150405-node1-1748539999",
			}, "\n"),
			prefix: "cluster-snapshot-20260529-150405",
			want:   "cluster-snapshot-20260529-150405-node1-1748539999",
		},
		{
			name:     "skips blank lines and trims surrounding whitespace",
			lsOutput: "\n  cluster-snapshot-20260529-150405-node1-1748539445  \n\n",
			prefix:   "cluster-snapshot-20260529-150405",
			want:     "cluster-snapshot-20260529-150405-node1-1748539445",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findSnapshotFile(tt.lsOutput, tt.prefix)
			if err != nil {
				t.Fatalf("findSnapshotFile() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("findSnapshotFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindSnapshotFile_NoMatch(t *testing.T) {
	lsOutput := "on-demand-server-0-1753178523\netcd-snapshot-node1-1700000000\n"
	if _, err := findSnapshotFile(lsOutput, "cluster-snapshot-20260529-150405"); err == nil {
		t.Fatal("findSnapshotFile() expected an error when no file matches the prefix, got nil")
	}
}
