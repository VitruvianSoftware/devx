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

package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/audit"
	"github.com/VitruvianSoftware/devx/internal/provider"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var auditOnlySecrets bool
var auditOnlyVulns bool

var auditCmd = &cobra.Command{
	Use:   "audit",
	GroupID: "telemetry",
	Short: "Scan for leaked secrets and dependency vulnerabilities before pushing",
	Long: `Runs two security scans against the current project directory:

  secrets  — Gitleaks: detect hardcoded API keys, tokens, credentials in source
  vulns    — Trivy: scan Go/Node/Python/Rust dependencies for known CVEs

Both tools are run natively if installed on your machine, or automatically
via an ephemeral read-only container if not — no installation required.

The container runtime is determined by the active --provider setting
(podman, lima, colima, docker, orbstack).

Tip: Run 'devx audit install-hooks' once to wire this into git pre-push
so it runs automatically before every push.`,
	RunE: runAudit,
}

var auditSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Scan for hardcoded secrets and credentials (Gitleaks)",
	RunE:  runAuditSecrets,
}

var auditVulnsCmd = &cobra.Command{
	Use:   "vulns",
	Short: "Scan dependencies for known CVEs (Trivy)",
	RunE:  runAuditVulns,
}

var auditInstallHooksCmd = &cobra.Command{
	Use:   "install-hooks",
	Short: "Install a git pre-push hook that runs devx audit automatically",
	RunE:  runAuditInstallHooks,
}

func init() {
	auditCmd.Flags().BoolVar(&auditOnlySecrets, "secrets", false, "Run only secrets scan")
	auditCmd.Flags().BoolVar(&auditOnlyVulns, "vulns", false, "Run only vulnerability scan")

	auditCmd.AddCommand(auditSecretsCmd)
	auditCmd.AddCommand(auditVulnsCmd)
	auditCmd.AddCommand(auditInstallHooksCmd)
	rootCmd.AddCommand(auditCmd)
}

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	auditStylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")).Bold(true)
	auditStyleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
	auditStyleSection = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true)
	auditStyleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	auditStyleMode    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341"))
)

// resolveAuditRuntime returns the ContainerRuntime from the provider cascade.
// This ensures audit uses the same backend as devx up / devx shell.
func resolveAuditRuntime() provider.ContainerRuntime {
	prov, err := getFullProvider()
	if err != nil {
		return nil
	}
	return prov.Runtime
}

func runAudit(_ *cobra.Command, _ []string) error {
	if err := ensureVMRunning(); err != nil {
		return err
	}

	runSecrets := !auditOnlyVulns
	runVulns := !auditOnlySecrets

	cwd, _ := os.Getwd()
	rt := resolveAuditRuntime()

	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit"))

	var totalIssues int

	if runSecrets {
		found, err := execScan(audit.Gitleaks, cwd, rt)
		if err != nil {
			return err
		}
		if found {
			totalIssues++
		}
	}

	if runVulns {
		found, err := execScan(audit.Trivy, cwd, rt)
		if err != nil {
			return err
		}
		if found {
			totalIssues++
		}
	}

	fmt.Println()
	if totalIssues == 0 {
		fmt.Printf("%s All scans passed — safe to push.\n\n", auditStylePass.Render("✓"))
		return nil
	}
	fmt.Printf("%s %d scan(s) found issues — review output above before pushing.\n\n",
		auditStyleFail.Render("✗"), totalIssues)
	// Exit non-zero so pre-push hooks abort the push
	os.Exit(1)
	return nil
}

func runAuditSecrets(_ *cobra.Command, _ []string) error {
	if err := ensureVMRunning(); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	rt := resolveAuditRuntime()
	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit secrets"))
	found, err := execScan(audit.Gitleaks, cwd, rt)
	if err != nil {
		return err
	}
	fmt.Println()
	if found {
		os.Exit(1)
	}
	return nil
}

func runAuditVulns(_ *cobra.Command, _ []string) error {
	if err := ensureVMRunning(); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	rt := resolveAuditRuntime()
	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit vulns"))
	found, err := execScan(audit.Trivy, cwd, rt)
	if err != nil {
		return err
	}
	fmt.Println()
	if found {
		os.Exit(1)
	}
	return nil
}

func runAuditInstallHooks(_ *cobra.Command, _ []string) error {
	cwd, _ := os.Getwd()
	if err := audit.InstallPrePushHook(cwd); err != nil {
		if strings.Contains(err.Error(), "already installed") {
			fmt.Printf("%s pre-push hook already installed.\n", tui.IconDone)
			return nil
		}
		return err
	}
	fmt.Printf("%s Installed git pre-push hook at .git/hooks/pre-push\n", tui.IconDone)
	fmt.Printf("  %s\n", auditStyleMuted.Render("devx audit will now run automatically before every git push."))
	return nil
}

// execScan runs a single tool and prints a rich status header + output.
func execScan(tool audit.Tool, cwd string, rt provider.ContainerRuntime) (foundIssues bool, err error) {
	mode := audit.Detect(tool, rt)

	modeLabel := auditStyleMode.Render("native")
	if mode == audit.ModeContainer {
		rtName := "container"
		if rt != nil {
			rtName = rt.Name()
		}
		modeLabel = auditStyleMode.Render("container (" + rtName + ")")
	}

	fmt.Printf("  %s %s  %s\n",
		auditStyleSection.Render("▸ "+tool.Name),
		auditStyleMuted.Render("—"),
		modeLabel,
	)

	format := "table"
	if outputJSON {
		format = "json"
	}

	start := time.Now()
	out, found, runErr := audit.Run(tool, cwd, rt, format)

	if runErr != nil {
		fmt.Printf("  %s  %s\n\n", auditStyleFail.Render("ERROR"), runErr.Error())
		return false, runErr
	}

	elapsed := time.Since(start)

	// Print the tool output, indented nicely
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}

	if found {
		fmt.Printf("\n  %s  %s\n\n",
			auditStyleFail.Render("✗ FAIL"),
			auditStyleMuted.Render(fmt.Sprintf("(%s — issues found)", elapsed.Round(time.Millisecond))),
		)
	} else {
		fmt.Printf("\n  %s  %s\n\n",
			auditStylePass.Render("✓ PASS"),
			auditStyleMuted.Render(fmt.Sprintf("(%s)", elapsed.Round(time.Millisecond))),
		)
	}

	return found, nil
}
