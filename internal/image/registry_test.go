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

// The in-cluster registry must persist pushed images across pod restarts
// (emptyDir loses them on every restart). The manifest should back the registry
// with a PersistentVolumeClaim on local-path (node-local; images are re-pullable,
// so node-pinning is acceptable — dev-local removed Longhorn).
func TestRegistryManifest_UsesDurablePVC(t *testing.T) {
	m := registryManifest("local-path", "registry-data")

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

// registryManifest renders the StorageClass + PVC name it is given, with no
// leftover placeholders. registryStorage picks `nfs` (off-node, NAS-backed) when
// that class exists, else the k3s built-in `local-path`; the PVC name differs by
// class because a PVC's storageClassName is immutable.
func TestRegistryManifest_RendersStorageClassAndPVC(t *testing.T) {
	cases := []struct{ sc, pvc string }{
		{"local-path", "registry-data"},
		{"nfs", "registry-data-nfs"},
	}
	for _, c := range cases {
		m := registryManifest(c.sc, c.pvc)
		for _, want := range []string{
			"storageClassName: " + c.sc,
			"name: " + c.pvc,
			"claimName: " + c.pvc,
		} {
			if !strings.Contains(m, want) {
				t.Errorf("registryManifest(%q,%q) missing %q", c.sc, c.pvc, want)
			}
		}
		for _, ph := range []string{"__STORAGE_CLASS__", "__PVC_NAME__"} {
			if strings.Contains(m, ph) {
				t.Errorf("registryManifest(%q,%q) left placeholder %q unrendered", c.sc, c.pvc, ph)
			}
		}
	}
}
