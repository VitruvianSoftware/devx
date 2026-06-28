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
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeCordoner struct {
	mu                  sync.Mutex
	drained, uncordoned int
}

func (f *fakeCordoner) DrainNode(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drained++
	return nil
}

func (f *fakeCordoner) UncordonNode(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uncordoned++
	return nil
}

// #366: a failure anywhere in the VM mutate/restart cycle must NOT leave the
// node cordoned — applyNodeCycle guarantees an uncordon on the way out.
func TestApplyNodeCycle_UncordonsEvenWhenBodyFails(t *testing.T) {
	f := &fakeCordoner{}
	boom := errors.New("limactl start failed")
	err := applyNodeCycle(context.Background(), f, "mbp", func() error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap %v", err, boom)
	}
	if f.drained != 1 {
		t.Errorf("drained = %d, want 1", f.drained)
	}
	if f.uncordoned != 1 {
		t.Errorf("uncordoned = %d, want 1 (must uncordon even on failure)", f.uncordoned)
	}
}

func TestApplyNodeCycle_UncordonsOnSuccess(t *testing.T) {
	f := &fakeCordoner{}
	if err := applyNodeCycle(context.Background(), f, "mbp", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if f.uncordoned != 1 {
		t.Errorf("uncordoned = %d, want 1", f.uncordoned)
	}
}

// #366: readiness must be polled, not a fixed sleep — waitNodeReady returns as
// soon as the node reports Ready, and errors if it never does.
func TestWaitNodeReady_ReturnsWhenReady(t *testing.T) {
	calls := 0
	err := waitNodeReady(context.Background(), func(context.Context) (bool, error) {
		calls++
		return calls >= 3, nil
	}, time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls < 3 {
		t.Errorf("calls = %d, want >= 3", calls)
	}
}

func TestWaitNodeReady_TimesOut(t *testing.T) {
	err := waitNodeReady(context.Background(), func(context.Context) (bool, error) {
		return false, nil
	}, 20*time.Millisecond, 5*time.Millisecond)
	if err == nil {
		t.Error("want a timeout error when the node never becomes Ready")
	}
}
