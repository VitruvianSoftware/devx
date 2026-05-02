package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/audit"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var auditRuntime string
var auditOnlySecrets bool
var auditOnlyVulns bool

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Scan for leaked secrets and dependency vulnerabilities before pushing",
	Long: `Runs two security scans against the current project directory:

  secrets  — Gitleaks: detect hardcoded API keys, tokens, credentials in source
  vulns    — Trivy: scan Go/Node/Python/Rust dependencies for known CVEs

Both tools are run natively if installed on your machine, or automatically
via an ephemeral read-only container if not — no installation required.

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
	for _, cmd := range []*cobra.Command{auditCmd, auditSecretsCmd, auditVulnsCmd} {
		cmd.Flags().StringVar(&auditRuntime, "runtime", "",
			"Container runtime override (podman or docker — auto-detected if empty)")
	}
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

func runAudit(_ *cobra.Command, _ []string) error {
	runSecrets := !auditOnlyVulns
	runVulns := !auditOnlySecrets

	cwd, _ := os.Getwd()

	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit"))

	var totalIssues int

	if runSecrets {
		found, err := execScan(audit.Gitleaks, cwd, auditRuntime)
		if err != nil {
			return err
		}
		if found {
			totalIssues++
		}
	}

	if runVulns {
		found, err := execScan(audit.Trivy, cwd, auditRuntime)
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
	cwd, _ := os.Getwd()
	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit secrets"))
	found, err := execScan(audit.Gitleaks, cwd, auditRuntime)
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
	cwd, _ := os.Getwd()
	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("devx audit vulns"))
	found, err := execScan(audit.Trivy, cwd, auditRuntime)
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
func execScan(tool audit.Tool, cwd, runtime string) (foundIssues bool, err error) {
	mode, rt := audit.Detect(tool)
	if runtime != "" {
		rt = runtime
	}

	modeLabel := auditStyleMode.Render("native")
	if mode == audit.ModeContainer {
		modeLabel = auditStyleMode.Render("container (" + rt + ")")
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
	out, found, runErr := audit.Run(tool, cwd, runtime, format)

	// ── VM not running: offer to start it and retry ────────────────────────
	if runErr == audit.ErrVMNotRunning {
		prov, pErr := getVMProvider()
		if pErr != nil {
			return false, pErr
		}
		vmName := prov.Name()

		fmt.Printf("  %s  %s VM is not running.\n", auditStyleFail.Render("!"), capitalizeProvider(vmName))
		if NonInteractive {
			return false, fmt.Errorf("%s VM is sleeping — please start it first", vmName)
		}

		var startVM bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Start %s VM?", capitalizeProvider(vmName))).
					Description(fmt.Sprintf("devx audit needs the %s VM to run the scanner container. Start it now?", capitalizeProvider(vmName))).
					Affirmative("Yes, start it").
					Negative("Skip this scan").
					Value(&startVM),
			),
		).WithTheme(huh.ThemeCatppuccin())
		_ = form.Run()

		if !startVM {
			fmt.Printf("  %s Skipped (VM not started)\n\n", auditStyleMuted.Render("—"))
			return false, nil
		}

		fmt.Printf("  %s Starting %s VM...\n", auditStyleMuted.Render("→"), strings.Title(vmName))
		
		devName := os.Getenv("USER")
		if devName == "" {
			devName = "developer"
		}
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil && s.DevHostname != "" {
			cfg.DevHostname = s.DevHostname
		}
		if cfg.DevHostname == "" {
			cfg.DevHostname = "devx"
		}

		if err := prov.Start(cfg.DevHostname); err != nil {
			return false, fmt.Errorf("failed to start %s VM: %w", vmName, err)
		}
		fmt.Println()

		// Retry the scan now that the VM is up
		start = time.Now()
		out, found, runErr = audit.Run(tool, cwd, runtime, format)
	}

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

func capitalizeProvider(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
