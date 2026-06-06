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
	"strings"
	"testing"
	"time"
)

func TestSelectStaleEphemeral(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	node := func(name, ready string, ephemeral bool, tt time.Time) string {
		labels := ""
		if ephemeral {
			labels = `"devx.io/ephemeral":"true"`
		}
		return fmt.Sprintf(
			`{"metadata":{"name":%q,"labels":{%s}},"status":{"conditions":[{"type":"MemoryPressure","status":"False","lastTransitionTime":%q},{"type":"Ready","status":%q,"lastTransitionTime":%q}]}}`,
			name, labels, now.Format(time.RFC3339), ready, tt.Format(time.RFC3339),
		)
	}

	nodes := []string{
		node("ephemeral-stale", "False", true, now.Add(-1*time.Hour)),    // selected
		node("ephemeral-recent", "False", true, now.Add(-5*time.Minute)), // too recent
		node("ephemeral-ready", "True", true, now.Add(-2*time.Hour)),     // healthy
		node("permanent-down", "False", false, now.Add(-2*time.Hour)),    // not ephemeral
	}
	payload := `{"items":[` + strings.Join(nodes, ",") + `]}`

	got, err := selectStaleEphemeral([]byte(payload), 30*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "ephemeral-stale" {
		t.Errorf("selectStaleEphemeral = %v, want [ephemeral-stale]", got)
	}
}

func TestSelectStaleEphemeral_Empty(t *testing.T) {
	got, err := selectStaleEphemeral([]byte(`{"items":[]}`), time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want no stale nodes, got %v", got)
	}
}

func TestSelectStaleEphemeral_BadJSON(t *testing.T) {
	if _, err := selectStaleEphemeral([]byte(`not json`), time.Minute, time.Now()); err == nil {
		t.Error("want error on malformed JSON")
	}
}
