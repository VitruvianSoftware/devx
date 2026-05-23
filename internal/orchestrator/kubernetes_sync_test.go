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
	"os"
	"path/filepath"
	"testing"
)

func TestDirSignature(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	sig1, err := dirSignature(dir)
	if err != nil {
		t.Fatalf("dirSignature: %v", err)
	}

	// Adding a file changes the signature.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sig2, _ := dirSignature(dir)
	if sig1 == sig2 {
		t.Error("signature should change when a file is added")
	}

	// Growing a file changes the signature (size component).
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world!!"), 0o600); err != nil {
		t.Fatal(err)
	}
	sig3, _ := dirSignature(dir)
	if sig3 == sig2 {
		t.Error("signature should change when a file grows")
	}

	// Changes under ignored dirs (node_modules) must NOT change the signature.
	nm := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nm, "huge.js"), []byte("ignored vendored code"), 0o600); err != nil {
		t.Fatal(err)
	}
	sig4, _ := dirSignature(dir)
	if sig4 != sig3 {
		t.Errorf("node_modules changes must be ignored: %q != %q", sig4, sig3)
	}
}
