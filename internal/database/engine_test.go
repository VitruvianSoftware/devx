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

package database

import "testing"

func TestConnStringPostgres(t *testing.T) {
	e := Registry["postgres"]
	got := e.ConnString(5432)
	expected := "postgresql://devx:devx@localhost:5432/devx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringRedis(t *testing.T) {
	e := Registry["redis"]
	got := e.ConnString(6379)
	expected := "redis://localhost:6379"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringMySQL(t *testing.T) {
	e := Registry["mysql"]
	got := e.ConnString(3306)
	expected := "mysql://devx:devx@localhost:3306/devx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringMongo(t *testing.T) {
	e := Registry["mongo"]
	got := e.ConnString(27017)
	expected := "mongodb://devx:devx@localhost:27017"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSupportedEngines(t *testing.T) {
	engines := SupportedEngines()
	if len(engines) != 4 {
		t.Errorf("expected 4 engines, got %d", len(engines))
	}
}

func TestRegistryHasAllEngines(t *testing.T) {
	for _, name := range SupportedEngines() {
		if _, ok := Registry[name]; !ok {
			t.Errorf("engine %q not found in registry", name)
		}
	}
}
