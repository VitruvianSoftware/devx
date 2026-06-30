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

package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// withRecordingRunner swaps the package-level runShellCommand for a stub that
// records every command it is asked to run (returning errFor(cmd) for each) and
// restores the original when the test finishes.
func withRecordingRunner(t *testing.T, errFor func(cmd string) error) *[]string {
	t.Helper()
	var calls []string
	orig := runShellCommand
	runShellCommand = func(cmd string) error {
		calls = append(calls, cmd)
		if errFor != nil {
			return errFor(cmd)
		}
		return nil
	}
	t.Cleanup(func() { runShellCommand = orig })
	return &calls
}

// TestExecuteInstall_TrustsEachTapAfterTapBeforeInstall is the core regression
// test: newer Homebrew refuses to LOAD formulae from untrusted third-party taps,
// so ExecuteInstall must issue `brew trust <tap>` after `brew tap <tap>` and
// before the tool's `brew install` for every tap in the plan.
func TestExecuteInstall_TrustsEachTapAfterTapBeforeInstall(t *testing.T) {
	calls := withRecordingRunner(t, nil)

	plan := &InstallPlan{
		PackageManager: "brew",
		Steps: []InstallStep{
			{Tool: "Cloudflared", Command: "brew install cloudflared", Tap: "cloudflare/cloudflare"},
			{Tool: "Mutagen", Command: "brew install mutagen", Tap: "mutagen-io/mutagen"},
		},
	}

	if err := ExecuteInstall(plan, true); err != nil {
		t.Fatalf("ExecuteInstall returned error: %v", err)
	}

	for _, s := range plan.Steps {
		tapIdx := slices.Index(*calls, "brew tap "+s.Tap)
		trustIdx := slices.Index(*calls, "brew trust "+s.Tap)
		installIdx := slices.Index(*calls, s.Command)

		if tapIdx < 0 {
			t.Errorf("expected a `brew tap %s` command; calls=%v", s.Tap, *calls)
		}
		if trustIdx < 0 {
			t.Errorf("expected a `brew trust %s` command; calls=%v", s.Tap, *calls)
		}
		if installIdx < 0 {
			t.Errorf("expected install command %q; calls=%v", s.Command, *calls)
		}
		if tapIdx >= 0 && trustIdx >= 0 && tapIdx > trustIdx {
			t.Errorf("`brew trust %s` (idx %d) must come after `brew tap %s` (idx %d); calls=%v",
				s.Tap, trustIdx, s.Tap, tapIdx, *calls)
		}
		if trustIdx >= 0 && installIdx >= 0 && trustIdx > installIdx {
			t.Errorf("`brew trust %s` (idx %d) must come before install %q (idx %d); calls=%v",
				s.Tap, trustIdx, s.Command, installIdx, *calls)
		}
	}
}

// TestExecuteInstall_NoTrustWithoutTap makes sure we never emit a stray
// `brew trust` for tools that do not come from a tap (homebrew/core formulae).
func TestExecuteInstall_NoTrustWithoutTap(t *testing.T) {
	calls := withRecordingRunner(t, nil)

	plan := &InstallPlan{
		PackageManager: "brew",
		Steps: []InstallStep{
			{Tool: "GitHub CLI", Command: "brew install gh"}, // no Tap
		},
	}

	if err := ExecuteInstall(plan, true); err != nil {
		t.Fatalf("ExecuteInstall returned error: %v", err)
	}

	for _, c := range *calls {
		if strings.HasPrefix(c, "brew tap ") || strings.HasPrefix(c, "brew trust ") {
			t.Errorf("did not expect a tap/trust command for a core formula, got %q; calls=%v", c, *calls)
		}
	}
}

// TestExecuteInstall_TrustFailureIsNonFatal verifies the guard for older
// Homebrew that lacks the `brew trust` subcommand: a failing trust must NOT
// abort the run, and the install step must still execute.
func TestExecuteInstall_TrustFailureIsNonFatal(t *testing.T) {
	calls := withRecordingRunner(t, func(cmd string) error {
		if strings.HasPrefix(cmd, "brew trust ") {
			// Simulate `Error: Unknown command: trust` on older Homebrew.
			return fmt.Errorf("Unknown command: trust")
		}
		return nil
	})

	plan := &InstallPlan{
		PackageManager: "brew",
		Steps: []InstallStep{
			{Tool: "Cloudflared", Command: "brew install cloudflared", Tap: "cloudflare/cloudflare"},
		},
	}

	if err := ExecuteInstall(plan, true); err != nil {
		t.Fatalf("brew trust failure should be non-fatal, but ExecuteInstall returned: %v", err)
	}

	if !slices.Contains(*calls, "brew install cloudflared") {
		t.Errorf("install must still run after a non-fatal trust failure; calls=%v", *calls)
	}
}

// TestExecuteInstall_TapFailureStillFatal guards the existing contract: a failing
// `brew tap` is fatal (we cannot install from a tap that did not register), so
// adding trust must not have softened that into a continue.
func TestExecuteInstall_TapFailureStillFatal(t *testing.T) {
	withRecordingRunner(t, func(cmd string) error {
		if strings.HasPrefix(cmd, "brew tap ") {
			return fmt.Errorf("boom")
		}
		return nil
	})

	plan := &InstallPlan{
		PackageManager: "brew",
		Steps: []InstallStep{
			{Tool: "Cloudflared", Command: "brew install cloudflared", Tap: "cloudflare/cloudflare"},
		},
	}

	if err := ExecuteInstall(plan, true); err == nil {
		t.Fatal("expected ExecuteInstall to fail when `brew tap` fails")
	}
}

// fakeBrewScript simulates the newer-Homebrew "untrusted tap" load gate: an
// install from a third-party tap is refused unless the tap was first trusted.
// State is tracked as marker files under $FAKE_BREW_STATE so the test can assert
// what actually happened.
const fakeBrewScript = `#!/bin/sh
state="$FAKE_BREW_STATE"
mkdir -p "$state"
echo "$@" >> "$state/log"
sub="$1"
shift
case "$sub" in
  tap)
    echo "$1" >> "$state/tapped"
    exit 0
    ;;
  trust)
    # Idempotent: trusting an already-trusted tap is a no-op success.
    echo "$1" >> "$state/trusted"
    exit 0
    ;;
  install)
    formula="$1"
    tap="cloudflare/cloudflare" # the tap that owns the 'cloudflared' formula
    if grep -qx "$tap" "$state/trusted" 2>/dev/null; then
      echo "$formula" >> "$state/installed"
      exit 0
    fi
    echo "Error: Refusing to load formula $tap/$formula from untrusted tap $tap. Run 'brew trust $tap' to trust it." >&2
    exit 1
    ;;
  *)
    echo "fake brew: unsupported subcommand: $sub" >&2
    exit 64
    ;;
esac
`

// TestExecuteInstall_RealRunner_UntrustedTapGate exercises the REAL exec-backed
// runShellCommand (not a mock) against a fake `brew` that enforces the exact
// untrusted-tap gate that froze brew-installed devx (see #434). It proves the
// auto-installer can actually install a tool from a third-party tap on a machine
// where that tap starts out untrusted — the trust step is what unblocks it.
func TestExecuteInstall_RealRunner_UntrustedTapGate(t *testing.T) {
	binDir := t.TempDir()
	fakeBrew := filepath.Join(binDir, "brew")
	if err := os.WriteFile(fakeBrew, []byte(fakeBrewScript), 0o755); err != nil {
		t.Fatalf("write fake brew: %v", err)
	}

	// Probe: if this sandbox cannot exec a script from a temp dir, skip rather
	// than fail — the mock-based tests already lock down the command ordering.
	// Point the probe at its own state dir so it never writes outside TempDir.
	probe := exec.Command(fakeBrew, "tap", "probe/probe")
	probe.Env = append(os.Environ(), "FAKE_BREW_STATE="+t.TempDir())
	if err := probe.Run(); err != nil {
		t.Skipf("fake brew not executable in this environment: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// 1. Control: prove the gate is real. A direct install before any trust must
	//    be refused with the untrusted-tap error.
	t.Run("gate_refuses_untrusted_install", func(t *testing.T) {
		state := t.TempDir()
		t.Setenv("FAKE_BREW_STATE", state)
		if err := runShellCommand("brew install cloudflared"); err == nil {
			t.Fatal("expected fake brew to refuse installing from an untrusted tap")
		}
		if _, err := os.Stat(filepath.Join(state, "installed")); err == nil {
			t.Fatal("formula must not be installed while its tap is untrusted")
		}
	})

	// 2. The real thing: ExecuteInstall taps, trusts, then installs. With the
	//    trust step the previously-refused install now succeeds.
	t.Run("execute_install_trusts_then_installs", func(t *testing.T) {
		state := t.TempDir()
		t.Setenv("FAKE_BREW_STATE", state)

		plan := &InstallPlan{
			PackageManager: "brew",
			Steps: []InstallStep{
				{Tool: "Cloudflared", Command: "brew install cloudflared", Tap: "cloudflare/cloudflare"},
			},
		}
		if err := ExecuteInstall(plan, true); err != nil {
			t.Fatalf("ExecuteInstall failed against untrusted tap: %v", err)
		}

		installed, err := os.ReadFile(filepath.Join(state, "installed"))
		if err != nil || !strings.Contains(string(installed), "cloudflared") {
			t.Fatalf("cloudflared was not installed (trust did not unblock it): err=%v installed=%q", err, installed)
		}

		// The recorded brew log must show trust before install.
		log, err := os.ReadFile(filepath.Join(state, "log"))
		if err != nil {
			t.Fatalf("read brew log: %v", err)
		}
		trustAt := strings.Index(string(log), "trust cloudflare/cloudflare")
		installAt := strings.Index(string(log), "install cloudflared")
		if trustAt < 0 || installAt < 0 || trustAt > installAt {
			t.Errorf("expected trust before install in brew log:\n%s", log)
		}
	})
}
