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

import (
	"reflect"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/image"
)

func TestContainerNodeName(t *testing.T) {
	if got := containerNodeName("api"); got != "devx-svc-api" {
		t.Errorf("containerNodeName = %q, want devx-svc-api", got)
	}
}

func TestContainerRunArgs_ImageWithPortEnvCommand(t *testing.T) {
	c := &ContainerNodeConfig{Image: "myorg/api:dev", Args: []string{"--cap-add=NET_ADMIN"}}
	got := containerRunArgs(c, "devx-svc-api", "myorg/api:dev", 18080, 8080,
		map[string]string{"LOG_LEVEL": "debug"}, []string{"./api", "--dev"})
	want := []string{
		"run", "-d",
		"--name", "devx-svc-api",
		"--label", "managed-by=devx",
		"--label", "devx-service=api",
		"--restart", "unless-stopped",
		"-p", "18080:8080",
		"-e", "LOG_LEVEL=debug",
		"--cap-add=NET_ADMIN",
		"myorg/api:dev",
		"./api", "--dev",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("containerRunArgs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestContainerRunArgs_NoPortNoEnvNoCommand(t *testing.T) {
	c := &ContainerNodeConfig{Image: "nginx:1.27"}
	got := containerRunArgs(c, "devx-svc-web", "nginx:1.27", 0, 0, nil, nil)
	want := []string{
		"run", "-d",
		"--name", "devx-svc-web",
		"--label", "managed-by=devx",
		"--label", "devx-service=web",
		"--restart", "unless-stopped",
		"nginx:1.27",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("containerRunArgs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestContainerImageRef_BuildDefaultsTag(t *testing.T) {
	ref := containerImageRef("web", &image.Spec{Name: "web", Context: "."})
	if ref != "devx-svc-web:dev" {
		t.Errorf("ref = %q, want devx-svc-web:dev", ref)
	}
}

func TestContainerImageRef_BuildExplicitTag(t *testing.T) {
	ref := containerImageRef("web", &image.Spec{Name: "web", Tag: "local"})
	if ref != "devx-svc-web:local" {
		t.Errorf("ref = %q, want devx-svc-web:local", ref)
	}
}

func TestContainerLogsTailArgs(t *testing.T) {
	got := containerLogsTailArgs("devx-svc-api")
	want := []string{"logs", "--tail", "50", "-f", "devx-svc-api"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestRemoveContainerNode_NoopWhenNotStarted(t *testing.T) {
	// removeContainerNode must be a safe no-op when the container was never started,
	// so cleanup never resolves a provider for a node that didn't run.
	n := &Node{Name: "api", Container: &ContainerNodeConfig{Image: "x", ProviderName: "definitely-not-a-provider"}}
	// containerStarted is false → must return nil without resolving the runtime.
	if err := removeContainerNode(n); err != nil {
		t.Errorf("expected no-op nil, got %v", err)
	}
}
