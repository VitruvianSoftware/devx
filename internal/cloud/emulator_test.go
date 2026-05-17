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

package cloud

import (
	"reflect"
	"sort"
	"testing"
)

func TestRegistryEntriesAreWellFormed(t *testing.T) {
	for key, e := range Registry {
		if e.Service != key {
			t.Errorf("registry[%q].Service = %q, want %q", key, e.Service, key)
		}
		if e.Name == "" {
			t.Errorf("registry[%q].Name is empty", key)
		}
		if e.Image == "" {
			t.Errorf("registry[%q].Image is empty", key)
		}
		if e.DefaultPort <= 0 {
			t.Errorf("registry[%q].DefaultPort = %d, want > 0", key, e.DefaultPort)
		}
		if e.InternalPort <= 0 {
			t.Errorf("registry[%q].InternalPort = %d, want > 0", key, e.InternalPort)
		}
		if e.ReadyLog == "" {
			t.Errorf("registry[%q].ReadyLog is empty", key)
		}
		if len(e.EnvVars) == 0 {
			t.Errorf("registry[%q].EnvVars is empty", key)
		}
	}
}

func TestSupportedServicesIsSortedAndDerivedFromRegistry(t *testing.T) {
	got := SupportedServices()

	want := make([]string, 0, len(Registry))
	for k := range Registry {
		want = append(want, k)
	}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SupportedServices() = %v, want %v", got, want)
	}

	if !sort.StringsAreSorted(got) {
		t.Errorf("SupportedServices() not sorted: %v", got)
	}
}

func TestSupportedServicesIncludesS3(t *testing.T) {
	got := SupportedServices()
	for _, s := range got {
		if s == "s3" {
			return
		}
	}
	t.Errorf("SupportedServices() = %v, missing \"s3\"", got)
}

func TestEnvVarValuesAppliesHostPortToTemplates(t *testing.T) {
	gcs := Registry["gcs"]
	got := gcs.EnvVarValues("localhost:4443")
	if got["STORAGE_EMULATOR_HOST"] != "http://localhost:4443" {
		t.Errorf("STORAGE_EMULATOR_HOST = %q, want %q",
			got["STORAGE_EMULATOR_HOST"], "http://localhost:4443")
	}
}

func TestEnvVarValuesPassesLiteralsUnchanged(t *testing.T) {
	s3 := Registry["s3"]
	got := s3.EnvVarValues("localhost:9000")

	cases := map[string]string{
		"AWS_ENDPOINT_URL_S3":   "http://localhost:9000",
		"AWS_ENDPOINT_URL":      "http://localhost:9000",
		"AWS_ACCESS_KEY_ID":     "minioadmin",
		"AWS_SECRET_ACCESS_KEY": "minioadmin",
		"AWS_REGION":            "us-east-1",
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}
