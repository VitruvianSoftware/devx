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
	"context"
	"strings"
	"testing"
)

func TestCloudRunDeployArgs(t *testing.T) {
	c := &CloudRunNodeConfig{
		Image:   "gcr.io/p/img:dev",
		Region:  "us-central1",
		Project: "my-proj",
		Env:     map[string]string{"B": "2", "A": "1"},
		Flags:   []string{"--memory", "512Mi"},
	}
	got := cloudRunDeployArgs(c, "my-svc")
	want := []string{
		"run", "deploy", "my-svc",
		"--image", "gcr.io/p/img:dev",
		"--region", "us-central1",
		"--project", "my-proj",
		"--platform", "managed",
		"--quiet",
		"--set-env-vars", "A=1,B=2", // env keys sorted
		"--no-allow-unauthenticated",
		"--memory", "512Mi", // extra flags passed through
	}
	if !equalArgs(got, want) {
		t.Errorf("cloudRunDeployArgs() =\n%v\nwant\n%v", got, want)
	}

	// AllowUnauthenticated true; no env; no extra flags.
	pub := &CloudRunNodeConfig{Image: "img", Region: "r", Project: "p", AllowUnauthenticated: true}
	gotPub := cloudRunDeployArgs(pub, "s")
	wantPub := []string{"run", "deploy", "s", "--image", "img", "--region", "r",
		"--project", "p", "--platform", "managed", "--quiet", "--allow-unauthenticated"}
	if !equalArgs(gotPub, wantPub) {
		t.Errorf("cloudRunDeployArgs(public) =\n%v\nwant\n%v", gotPub, wantPub)
	}
}

func TestSetEnvVarsArg(t *testing.T) {
	if got := setEnvVarsArg(map[string]string{"Z": "26", "A": "1", "M": "13"}); got != "A=1,M=13,Z=26" {
		t.Errorf("setEnvVarsArg() = %q, want A=1,M=13,Z=26 (sorted)", got)
	}
	if got := setEnvVarsArg(nil); got != "" {
		t.Errorf("setEnvVarsArg(nil) = %q, want empty", got)
	}
}

func TestStartCloudRunNode_MissingConfig(t *testing.T) {
	// nil CloudRun → required-fields error, before any gcloud call.
	err := startCloudRunNode(context.Background(), &Node{Name: "svc", Runtime: RuntimeCloud})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected required-fields error, got %v", err)
	}
	// missing project → still an error.
	n := &Node{Name: "svc", Runtime: RuntimeCloud, CloudRun: &CloudRunNodeConfig{Image: "i", Region: "r"}}
	if err := startCloudRunNode(context.Background(), n); err == nil {
		t.Error("expected error when project is missing")
	}
}
