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
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
)

// b64Payloads decodes any `echo '<base64>' | base64 -d` segments in the captured
// commands so tests can assert on the real manifest/patch content rather than
// opaque base64. This mirrors how DeployMetalLB/EnsureClusterDefaults deliver
// payloads to the node without nested-quoting hazards.
var b64Re = regexp.MustCompile(`echo '([A-Za-z0-9+/=]+)' \| base64 -d`)

func b64Payloads(cmds []string) []string {
	var out []string
	for _, c := range cmds {
		for _, m := range b64Re.FindAllStringSubmatch(c, -1) {
			if dec, err := base64.StdEncoding.DecodeString(m[1]); err == nil {
				out = append(out, string(dec))
			}
		}
	}
	return out
}

// EnsureClusterDefaults bakes the homelab resilience defaults into the cluster:
// CoreDNS must run 2 replicas with required hostname anti-affinity (k3s ships a
// single replica → a single-node DNS outage), and exactly one default
// StorageClass must win (k3s ships local-path as default; Longhorn is the real
// default, so local-path must be un-defaulted).
func TestEnsureClusterDefaults_PatchesCoreDNSAndStorageClass(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})

	if err := m.EnsureClusterDefaults(context.Background(), nil); err != nil {
		t.Fatalf("EnsureClusterDefaults: %v", err)
	}

	joined := strings.Join(got, "\n")

	// CoreDNS: a merge patch against the kube-system coredns Deployment.
	if !strings.Contains(joined, "patch deployment coredns") ||
		!strings.Contains(joined, "-n kube-system") {
		t.Errorf("expected a coredns deployment patch in kube-system; got:\n%s", joined)
	}

	// The actual patch content (decoded from base64) must carry 2 replicas and a
	// required hostname pod-anti-affinity keyed on the kube-dns pods.
	patches := strings.Join(b64Payloads(got), "\n")
	for _, want := range []string{
		`"replicas":2`,
		"podAntiAffinity",
		"requiredDuringSchedulingIgnoredDuringExecution",
		"kubernetes.io/hostname",
		"kube-dns",
	} {
		if !strings.Contains(patches, want) {
			t.Errorf("coredns patch missing %q; decoded payloads:\n%s", want, patches)
		}
	}

	// StorageClass: local-path must be explicitly un-defaulted, and idempotently
	// (a no-op if local-path is absent), never deleted.
	if !strings.Contains(joined, "storageclass") ||
		!strings.Contains(joined, "local-path") ||
		!strings.Contains(joined, "storageclass.kubernetes.io/is-default-class=false") {
		t.Errorf("expected local-path to be annotated non-default; got:\n%s", joined)
	}
	if strings.Contains(joined, "delete storageclass") {
		t.Errorf("must not delete local-path, only un-default it; got:\n%s", joined)
	}
}

// DeployMetalLB with a non-empty l2Hostnames list must pin the L2Advertisement to
// those stable nodes via spec.nodeSelectors (one matchLabels per hostname). Without
// a nodeSelector the speaker VIP can flap onto a high-latency Mac node.
func TestDeployMetalLB_PinsL2AdvertisementToHostnames(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})

	if err := m.DeployMetalLB(context.Background(), "10.44.86.210-10.44.86.215", []string{"fedora", "nuc9"}); err != nil {
		t.Fatalf("DeployMetalLB: %v", err)
	}

	manifests := strings.Join(b64Payloads(got), "\n")
	for _, want := range []string{
		"nodeSelectors",
		"kubernetes.io/hostname",
		"fedora",
		"nuc9",
	} {
		if !strings.Contains(manifests, want) {
			t.Errorf("L2Advertisement manifest missing %q; decoded payloads:\n%s", want, manifests)
		}
	}
	// The IP pool must still carry the address range.
	if !strings.Contains(manifests, "10.44.86.210-10.44.86.215") {
		t.Errorf("IPAddressPool missing the IP range; decoded payloads:\n%s", manifests)
	}
}

// With an empty l2Hostnames list, DeployMetalLB must emit the L2Advertisement
// WITHOUT a nodeSelector (current behaviour — backward compatible).
func TestDeployMetalLB_NoHostnamesNoNodeSelector(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})

	if err := m.DeployMetalLB(context.Background(), "10.44.86.210-10.44.86.215", nil); err != nil {
		t.Fatalf("DeployMetalLB: %v", err)
	}

	manifests := strings.Join(b64Payloads(got), "\n")
	if strings.Contains(manifests, "nodeSelectors") {
		t.Errorf("empty l2Hostnames must not emit nodeSelectors; decoded payloads:\n%s", manifests)
	}
	// The L2Advertisement and IP pool must still be present.
	for _, want := range []string{"L2Advertisement", "IPAddressPool", "10.44.86.210-10.44.86.215"} {
		if !strings.Contains(manifests, want) {
			t.Errorf("manifest missing %q; decoded payloads:\n%s", want, manifests)
		}
	}
}

// EnsureClusterDefaults must surface a real failure (e.g. the API server being
// unreachable) so callers can fail the phase rather than silently skip HA setup.
func TestEnsureClusterDefaults_PropagatesError(t *testing.T) {
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		if strings.Contains(cmd, "patch deployment coredns") {
			return "", context.DeadlineExceeded
		}
		return "", nil
	})
	if err := m.EnsureClusterDefaults(context.Background(), nil); err == nil {
		t.Fatal("expected error when the coredns patch fails, got nil")
	}
}

// buildLocalPathConfigJSON renders the k3s built-in local-path provisioner's
// nodePathMap: nodes with a custom dataDir get a per-node path (<dataDir>/storage,
// so their local-path PVs land on the same big volume as the rest of k3s's data,
// since the provisioner's path is independent of k3s --data-dir); all others use
// the k3s default. Output is deterministic (sorted) and always includes DEFAULT.
func TestBuildLocalPathConfigJSON(t *testing.T) {
	got := buildLocalPathConfigJSON(map[string]string{"nuc9i5": "/home/k3s", "boxA": "/data"})
	for _, want := range []string{
		`"DEFAULT_PATH_FOR_NON_LISTED_NODES"`,
		"/var/lib/rancher/k3s/storage",
		`"nuc9i5"`, "/home/k3s/storage",
		`"boxA"`, "/data/storage",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("buildLocalPathConfigJSON missing %q in:\n%s", want, got)
		}
	}
	// Deterministic ordering: boxA before nuc9i5 (sorted).
	if strings.Index(got, "boxA") > strings.Index(got, "nuc9i5") {
		t.Errorf("node entries must be sorted; got:\n%s", got)
	}
	// Empty dataDirs → DEFAULT only, no per-node entries.
	if empty := buildLocalPathConfigJSON(nil); !strings.Contains(empty, "DEFAULT_PATH_FOR_NON_LISTED_NODES") || strings.Contains(empty, "nuc9i5") {
		t.Errorf("empty dataDirs should yield only DEFAULT; got:\n%s", empty)
	}
}

// With dataDir nodes, EnsureClusterDefaults must patch the local-path-config
// nodePathMap (per-node <dataDir>/storage) and restart the provisioner so it picks
// up the new path.
func TestEnsureClusterDefaults_SteersLocalPathForDataDir(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil // empty get → not converged → must patch + restart
	})
	if err := m.EnsureClusterDefaults(context.Background(), map[string]string{"nuc9i5": "/home/k3s"}); err != nil {
		t.Fatalf("EnsureClusterDefaults: %v", err)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "patch configmap local-path-config") {
		t.Errorf("expected a local-path-config patch; got:\n%s", joined)
	}
	if !strings.Contains(joined, "rollout restart deployment local-path-provisioner") {
		t.Errorf("expected the provisioner to be restarted; got:\n%s", joined)
	}
	// config.json is a JSON string nested in the patch, so its quotes arrive escaped
	// (\"nuc9i5\") — assert on the bare tokens; the exact structure is covered by
	// TestBuildLocalPathConfigJSON.
	payloads := strings.Join(b64Payloads(got), "\n")
	for _, want := range []string{"nuc9i5", "/home/k3s/storage", "DEFAULT_PATH_FOR_NON_LISTED_NODES"} {
		if !strings.Contains(payloads, want) {
			t.Errorf("local-path patch missing %q; decoded:\n%s", want, payloads)
		}
	}
}

// Without dataDir nodes, EnsureClusterDefaults must NOT touch local-path-config.
func TestEnsureClusterDefaults_NoDataDirSkipsLocalPath(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})
	if err := m.EnsureClusterDefaults(context.Background(), nil); err != nil {
		t.Fatalf("EnsureClusterDefaults: %v", err)
	}
	if strings.Contains(strings.Join(got, "\n"), "local-path-config") {
		t.Errorf("no dataDir nodes → must not touch local-path-config; got:\n%s", strings.Join(got, "\n"))
	}
}
