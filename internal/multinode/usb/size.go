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

package usb

import (
	"fmt"
	"strconv"
	"strings"
)

// parseSizeMB parses a size like "16G", "16GiB", "512M" into mebibytes
// (G = 1024 MiB). It requires an explicit M or G unit.
func parseSizeMB(s string) (int, error) {
	u := strings.ToUpper(strings.TrimSpace(s))
	u = strings.TrimSuffix(u, "IB")
	u = strings.TrimSuffix(u, "B")
	var mult int
	switch {
	case strings.HasSuffix(u, "G"):
		mult, u = 1024, strings.TrimSuffix(u, "G")
	case strings.HasSuffix(u, "M"):
		mult, u = 1, strings.TrimSuffix(u, "M")
	default:
		return 0, fmt.Errorf("size %q must end in M or G", s)
	}
	n, err := strconv.Atoi(strings.TrimSpace(u))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}
