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
)

func TestKubectlLogsTailArgs(t *testing.T) {
	got := kubectlLogsTailArgs("/kc", "kind-dev", "myns", "api-7d9", "app", 5)
	want := []string{
		"--kubeconfig", "/kc", "--context", "kind-dev",
		"logs", "--since=5s", "-f", "api-7d9", "-c", "app", "--namespace", "myns",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParsePodWatchEvent_NewRunningContainers(t *testing.T) {
	line := []byte(`{"type":"ADDED","object":{"metadata":{"name":"api-1","namespace":"myns"},` +
		`"status":{"containerStatuses":[` +
		`{"name":"app","containerID":"docker://abc","state":{"running":{}}},` +
		`{"name":"sidecar","containerID":"","state":{"waiting":{"message":"ImagePullBackOff"}}}]}}}`)
	evt, err := parsePodWatchEvent(line)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Type != "ADDED" || evt.Object.Metadata.Name != "api-1" {
		t.Fatalf("bad event: %+v", evt)
	}
	cs := evt.Object.Status.ContainerStatuses
	if len(cs) != 2 || cs[0].ContainerID != "docker://abc" {
		t.Fatalf("bad container statuses: %+v", cs)
	}
	if cs[1].State.Waiting == nil || cs[1].State.Waiting.Message != "ImagePullBackOff" {
		t.Errorf("expected waiting message, got %+v", cs[1].State)
	}
}
