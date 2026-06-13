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

package image

import (
	"strings"
	"testing"
)

// The in-cluster registry must persist pushed images across pod/node restarts
// (emptyDir loses them on every restart). The manifest should back the registry
// with a PersistentVolumeClaim on the cluster's default StorageClass (Longhorn on
// dev-local — durable + relocatable on node loss).
func TestRegistryManifest_UsesDurablePVC(t *testing.T) {
	m := registryManifest

	if strings.Contains(m, "emptyDir") {
		t.Errorf("registry manifest still uses emptyDir; images are lost on pod restart")
	}

	// A PVC named registry-data must be declared and mounted as the data volume.
	if !strings.Contains(m, "kind: PersistentVolumeClaim") {
		t.Errorf("registry manifest declares no PersistentVolumeClaim")
	}
	for _, want := range []string{
		"name: registry-data",
		"claimName: registry-data",
		"ReadWriteOnce",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("registry manifest missing %q", want)
		}
	}

	// A single-replica Deployment backed by an RWO PVC must use the Recreate
	// strategy, or a rolling update deadlocks (new pod can't mount the volume the
	// old pod still holds).
	if !strings.Contains(m, "type: Recreate") {
		t.Errorf("registry Deployment must use strategy type: Recreate with an RWO PVC")
	}

	// The data volume mount must still point at the registry's storage dir.
	if !strings.Contains(m, "/var/lib/registry") {
		t.Errorf("registry data mountPath /var/lib/registry missing")
	}
}

// The PVC must not pin a specific StorageClass: omitting storageClassName uses the
// cluster default (Longhorn on dev-local; falls back gracefully elsewhere), so the
// registry never blocks on a class that may not exist yet.
func TestRegistryManifest_UsesDefaultStorageClass(t *testing.T) {
	if strings.Contains(registryManifest, "storageClassName:") {
		t.Errorf("registry PVC should omit storageClassName (use cluster default), got an explicit class")
	}
}
