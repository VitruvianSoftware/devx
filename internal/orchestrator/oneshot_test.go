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
	"testing"
	"time"
)

// A one-shot task that exits 0 must complete and let its dependents start, so
// the whole DAG comes up successfully.
func TestExecute_OneShotCompletesThenDependentRuns(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddNode(&Node{
		Name:    "seed",
		Type:    NodeService,
		Runtime: RuntimeHost,
		Command: []string{"sh", "-c", "exit 0"},
		OneShot: true,
	})
	_ = dag.AddNode(&Node{
		Name:      "app",
		Type:      NodeService,
		Runtime:   RuntimeHost,
		Command:   []string{"sh", "-c", "exit 0"},
		DependsOn: []string{"seed"},
	})

	cleanup, err := dag.Execute(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("Execute() with a successful one-shot should succeed, got: %v", err)
	}
}

// A one-shot task that exits non-zero must fail the DAG (and thereby block its
// dependents), with an error that names the task.
func TestExecute_OneShotFailureBlocksDAG(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddNode(&Node{
		Name:    "seed",
		Type:    NodeService,
		Runtime: RuntimeHost,
		Command: []string{"sh", "-c", "exit 3"},
		OneShot: true,
	})
	_ = dag.AddNode(&Node{
		Name:      "app",
		Type:      NodeService,
		Runtime:   RuntimeHost,
		Command:   []string{"sh", "-c", "exit 0"},
		DependsOn: []string{"seed"},
	})

	cleanup, err := dag.Execute(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Fatal("Execute() with a failing one-shot should return an error, got nil")
	}
	if !contains(err.Error(), "one-shot task") || !contains(err.Error(), "seed") {
		t.Errorf("error should name the failed one-shot task, got: %v", err)
	}
}

func TestWaitForCompletion_ExitZero(t *testing.T) {
	n := &Node{Name: "ok", Type: NodeService, Runtime: RuntimeHost, Command: []string{"sh", "-c", "exit 0"}}
	if err := startHostProcess(context.Background(), n); err != nil {
		t.Fatalf("startHostProcess: %v", err)
	}
	if err := waitForCompletion(context.Background(), n); err != nil {
		t.Errorf("waitForCompletion on a 0-exit task should succeed, got: %v", err)
	}
}

func TestWaitForCompletion_ExitNonZero(t *testing.T) {
	n := &Node{Name: "bad", Type: NodeService, Runtime: RuntimeHost, Command: []string{"sh", "-c", "exit 5"}}
	if err := startHostProcess(context.Background(), n); err != nil {
		t.Fatalf("startHostProcess: %v", err)
	}
	if err := waitForCompletion(context.Background(), n); err == nil {
		t.Error("waitForCompletion on a non-zero-exit task should return an error, got nil")
	}
}

func TestWaitForCompletion_Timeout(t *testing.T) {
	n := &Node{
		Name:        "slow",
		Type:        NodeService,
		Runtime:     RuntimeHost,
		Command:     []string{"sh", "-c", "sleep 5"},
		Healthcheck: HealthcheckConfig{Timeout: 100 * time.Millisecond},
	}
	if err := startHostProcess(context.Background(), n); err != nil {
		t.Fatalf("startHostProcess: %v", err)
	}
	defer func() {
		if n.cancel != nil {
			n.cancel()
		}
		if n.process != nil && n.process.Process != nil {
			_ = n.process.Process.Kill()
		}
	}()

	start := time.Now()
	err := waitForCompletion(context.Background(), n)
	if err == nil {
		t.Fatal("waitForCompletion should time out on a long-running task, got nil")
	}
	if !contains(err.Error(), "timed out") {
		t.Errorf("expected a timeout error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waitForCompletion honored its own timeout too slowly: %s", elapsed)
	}
}

func TestWaitForCompletion_NotStarted(t *testing.T) {
	n := &Node{Name: "never", Type: NodeService, OneShot: true}
	if err := waitForCompletion(context.Background(), n); err == nil {
		t.Error("waitForCompletion on an unstarted task should error, got nil")
	}
}
