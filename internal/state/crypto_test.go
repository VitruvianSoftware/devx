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

package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCryptoRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	original := filepath.Join(tmp, "original.txt")
	encrypted := filepath.Join(tmp, "encrypted.bin")
	decrypted := filepath.Join(tmp, "decrypted.txt")

	data := []byte("hello world state replication")
	if err := os.WriteFile(original, data, 0644); err != nil {
		t.Fatal(err)
	}

	passphrase := GeneratePassphrase()
	if len(strings.Split(passphrase, "-")) != 4 {
		t.Errorf("expected 4 words in passphrase, got %s", passphrase)
	}

	if err := EncryptFile(original, encrypted, passphrase); err != nil {
		t.Fatal(err)
	}

	if err := DecryptFile(encrypted, decrypted, passphrase); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(decrypted)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != string(data) {
		t.Errorf("expected %s, got %s", string(data), string(out))
	}
}

func TestCryptoBadPassphrase(t *testing.T) {
	tmp := t.TempDir()
	original := filepath.Join(tmp, "original.txt")
	encrypted := filepath.Join(tmp, "encrypted.bin")
	decrypted := filepath.Join(tmp, "decrypted.txt")

	_ = os.WriteFile(original, []byte("secret"), 0644)
	_ = EncryptFile(original, encrypted, "correct-passphrase")

	err := DecryptFile(encrypted, decrypted, "wrong-passphrase")
	if err == nil {
		t.Error("expected error with wrong passphrase")
	}
}
