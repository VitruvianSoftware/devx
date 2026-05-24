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

	"github.com/charmbracelet/lipgloss"
)

// paletteColors is the fixed per-service color palette (preserved from the TUI).
var paletteColors = []string{"#FF5F87", "#FFF700", "#00FF00", "#00FFFF", "#FF00FF", "#8A2BE2", "#FFA500"}

// ColorPicker assigns a stable lipgloss color to each service name, cycling
// through the palette. Safe for concurrent use.
type ColorPicker struct {
	mu     sync.Mutex
	colors map[string]lipgloss.Color
	idx    int
}

func NewColorPicker() *ColorPicker {
	return &ColorPicker{colors: map[string]lipgloss.Color{}}
}

// Color returns the (stable) color for a service, assigning one on first use.
func (p *ColorPicker) Color(service string) lipgloss.Color {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.colors[service]; ok {
		return c
	}
	c := lipgloss.Color(paletteColors[p.idx%len(paletteColors)])
	p.colors[service] = c
	p.idx++
	return c
}

// defaultColorPicker is the process-wide picker shared by the inline writer and
// the TUI so colors agree across both views.
var defaultColorPicker = NewColorPicker()
