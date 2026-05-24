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

package logs

import (
	"sync"
	"testing"
)

func TestColorPicker_StablePerService(t *testing.T) {
	p := NewColorPicker()
	a1 := p.Color("api")
	a2 := p.Color("api")
	if a1 != a2 {
		t.Errorf("same service got different colors: %v vs %v", a1, a2)
	}
	if p.Color("web") == a1 {
		// Not guaranteed unique forever (palette wraps), but the first two differ.
		t.Errorf("distinct early services should get distinct colors")
	}
}

func TestColorPicker_ConcurrentSafe(t *testing.T) {
	p := NewColorPicker()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = p.Color("svc") }()
	}
	wg.Wait() // must not race (run with -race)
}
