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

package cluster

import (
	"path/filepath"
	"testing"
)

// #366: mutating cluster ops must be mutually exclusive so two concurrent
// `devx cluster …` runs (the checkout is commonly shared across sessions) can't
// interleave and diverge the cluster.
func TestClusterLock_MutualExclusion(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "cluster.lock")
	orig := lockPath
	lockPath = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { lockPath = orig })

	l1, err := acquireClusterLock()
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	if l2, err := acquireClusterLock(); err == nil {
		l2.release()
		t.Fatal("second acquire should fail while the lock is held")
	}

	// After release, the lock is re-acquirable.
	l1.release()
	l3, err := acquireClusterLock()
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	l3.release()
}
