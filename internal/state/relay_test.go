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

import "testing"

func TestParseRelay(t *testing.T) {
	tests := []struct {
		input       string
		wantBackend string
		wantURI     string
		wantErr     bool
	}{
		{"s3://my-bucket/path", "s3", "s3://my-bucket/path", false},
		{"gs://my-bucket/path", "gcs", "gs://my-bucket/path", false},
		{"http://my-bucket", "", "", true},
		{"invalid", "", "", true},
		{"s3://", "s3", "s3://", false}, // Technically valid prefix
	}

	for _, tt := range tests {
		backend, uri, err := ParseRelay(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRelay(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if backend != tt.wantBackend {
			t.Errorf("ParseRelay(%q) backend = %v, want %v", tt.input, backend, tt.wantBackend)
		}
		if uri != tt.wantURI {
			t.Errorf("ParseRelay(%q) uri = %v, want %v", tt.input, uri, tt.wantURI)
		}
	}
}
