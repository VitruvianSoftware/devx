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

package cron

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		job     Job
		wantErr string
	}{
		{
			name:    "missing name",
			job:     Job{Command: []string{"echo", "ok"}},
			wantErr: "missing name",
		},
		{
			name:    "whitespace-only name",
			job:     Job{Name: "   ", Command: []string{"echo", "ok"}},
			wantErr: "missing name",
		},
		{
			name:    "missing command",
			job:     Job{Name: "x"},
			wantErr: "has no command",
		},
		{
			name:    "empty command element",
			job:     Job{Name: "x", Command: []string{"echo", "", "ok"}},
			wantErr: "empty command element",
		},
		{
			name: "valid job",
			job:  Job{Name: "x", Command: []string{"echo", "ok"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.job.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestParseTimeout(t *testing.T) {
	cases := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"5m", 5 * time.Minute, false},
		{"30s", 30 * time.Second, false},
		{"1h30m", 90 * time.Minute, false},
		{"-1s", 0, true},
		{"banana", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseTimeout(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseTimeout(%q) expected error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTimeout(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseTimeout(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestFindByNameAndSortedNames(t *testing.T) {
	jobs := []Job{
		{Name: "zebra", Command: []string{"true"}},
		{Name: "apple", Command: []string{"true"}},
		{Name: "mango", Command: []string{"true"}},
	}

	got := SortedNames(jobs)
	want := []string{"apple", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedNames = %v, want %v", got, want)
	}

	if j, ok := FindByName(jobs, "mango"); !ok || j.Name != "mango" {
		t.Errorf("FindByName(mango) = %v %v", j, ok)
	}
	if _, ok := FindByName(jobs, "PEAR"); ok {
		t.Errorf("FindByName should be case-sensitive")
	}
}

func TestExecuteHappyPathReturnsExitZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell binaries")
	}
	out, err := os.CreateTemp(t.TempDir(), "cron-stdout-*")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	res, err := Execute(context.Background(), ExecuteConfig{
		Job: Job{
			Name:    "echo-hello",
			Command: []string{"echo", "hello-from-cron"},
		},
		Stdout: out,
		Stderr: out,
	})
	if err != nil {
		t.Fatalf("Execute returned err: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", res.Duration)
	}
	if res.TimedOut {
		t.Errorf("TimedOut should be false")
	}

	data, _ := os.ReadFile(out.Name())
	if !strings.Contains(string(data), "hello-from-cron") {
		t.Errorf("expected stdout to contain greeting, got %q", string(data))
	}
}

func TestExecuteNonZeroExitIsReported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell binaries")
	}
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()

	res, err := Execute(context.Background(), ExecuteConfig{
		Job: Job{
			Name:    "exit-7",
			Command: []string{"sh", "-c", "exit 7"},
		},
		// Discard output for cleanliness; we only care about the exit code.
		Stdout: devnull,
		Stderr: devnull,
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if res.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7", res.ExitCode)
	}
	if res.TimedOut {
		t.Errorf("TimedOut should be false for plain non-zero exit")
	}
}

func TestExecuteTimeoutMarksResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell binaries")
	}
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()

	res, err := Execute(context.Background(), ExecuteConfig{
		Job: Job{
			Name:    "slow",
			Command: []string{"sh", "-c", "sleep 5"},
			Timeout: 100 * time.Millisecond,
		},
		Stdout: devnull,
		Stderr: devnull,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !res.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
	if res.Duration > time.Second {
		t.Errorf("Duration = %v, expected near 100ms", res.Duration)
	}
}

func TestExecuteValidationFailurePropagates(t *testing.T) {
	res, err := Execute(context.Background(), ExecuteConfig{
		Job: Job{Name: "", Command: []string{"true"}},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 (never ran)", res.ExitCode)
	}
}

func TestMergeEnvJobEnvBeatsSecretsBeatsBase(t *testing.T) {
	// Indirectly tested through Execute, but a focused test catches
	// precedence regressions without spawning subprocesses.
	merged := mergeEnv(
		[]string{"FOO=base", "BAR=base"},
		map[string]string{"FOO": "secret", "QUUX": "secret"},
		map[string]string{"FOO": "job", "BAZ": "job"},
	)
	got := envMap(merged)
	want := map[string]string{
		"FOO":  "job",
		"BAR":  "base",
		"QUUX": "secret",
		"BAZ":  "job",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("env[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}

func TestExecuteHonorsWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell binaries")
	}
	tmp := t.TempDir()
	out, _ := os.CreateTemp(tmp, "stdout-*")
	defer out.Close()

	res, err := Execute(context.Background(), ExecuteConfig{
		Job: Job{
			Name:    "pwd",
			Command: []string{"sh", "-c", "pwd"},
		},
		WorkDir: tmp,
		Stdout:  out,
		Stderr:  out,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d", res.ExitCode)
	}
	data, _ := os.ReadFile(out.Name())
	want, _ := filepath.EvalSymlinks(tmp)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(data)))
	if got != want {
		t.Errorf("pwd = %q, want %q", got, want)
	}
}
