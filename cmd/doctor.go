package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/doctor"
	"github.com/VitruvianSoftware/devx/internal/ship"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that all prerequisites are installed and configured",
	Long: `Audits your development environment for required CLI tools, credentials,
and authentication sessions. Reports what's installed, what's missing,
and how to fix any gaps.

Run without arguments for a full health check:
  devx doctor

Machine-readable output for AI agents:
  devx doctor --json`,
	RunE: runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	report := doctor.RunFullAudit(envFile)

	if outputJSON {
		enc, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	// ── System ──────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(tui.StyleTitle.Render("🩺 devx doctor — Environment Health Check"))

	osName := friendlyOS(report.System.OS)
	pmInfo := report.System.PackageManager
	if pmInfo == "" {
		pmInfo = tui.StyleDetailError.Render("none detected")
	} else if report.System.PMVersion != "" {
		pmInfo = pmInfo + " " + report.System.PMVersion
	}
	fmt.Printf("  %s  %s (%s) • %s\n\n",
		tui.StyleLabel.Render("System:"),
		osName,
		report.System.Arch,
		pmInfo,
	)

	// ── CLI Tools ───────────────────────────────────────────────────
	fmt.Println(tui.StyleTitle.Render("  CLI Tools"))

	// Group: required first, then optional
	requiredTools := make([]doctor.ToolStatus, 0)
	optionalTools := make([]doctor.ToolStatus, 0)
	for _, t := range report.Tools {
		if t.Required {
			requiredTools = append(requiredTools, t)
		} else {
			optionalTools = append(optionalTools, t)
		}
	}

	missingRequired := 0
	for _, t := range requiredTools {
		printToolRow(t)
		if !t.Installed {
			missingRequired++
		}
	}

	if len(optionalTools) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleMuted.Render("  Optional:"))
		for _, t := range optionalTools {
			printToolRow(t)
		}
	}
	fmt.Println()

	// ── Credentials ─────────────────────────────────────────────────
	fmt.Println(tui.StyleTitle.Render("  Credentials"))

	missingCreds := 0
	for _, c := range report.Credentials {
		printCredRow(c)
		if !c.Configured {
			missingCreds++
		}
	}
	fmt.Println()

	// ── Feature Readiness ───────────────────────────────────────────
	fmt.Println(tui.StyleTitle.Render("  Feature Readiness"))

	features := computeFeatureReadiness(report)
	for _, f := range features {
		icon := tui.IconDone
		detail := tui.StyleDetailDone.Render("ready")
		if !f.ready {
			icon = tui.StyleDetailError.Render("✗")
			detail = tui.StyleDetailError.Render("needs: " + f.missing)
		}
		fmt.Printf("    %s  %s  %s\n", icon, tui.StyleStepName.Render(f.command), detail)
	}
	fmt.Println()

	// ── Agentic Guardrails ─────────────────────────────────────────
	fmt.Println(tui.StyleTitle.Render("  Agentic Guardrails"))

	cwd, _ := os.Getwd()
	hookInstalled := ship.IsPrePushHookInstalled(cwd)
	if hookInstalled {
		fmt.Printf("    %s  %-24s %s\n",
			tui.IconDone,
			"pre-push hook",
			tui.StyleDetailDone.Render("installed (devx agent ship enforced)"),
		)
	} else {
		fmt.Printf("    %s  %-24s %s\n",
			tui.IconFailed,
			"pre-push hook",
			tui.StyleDetailError.Render("not installed — run: devx agent ship --install-hook"),
		)
	}
	fmt.Println()

	// ── Summary ─────────────────────────────────────────────────────
	if missingRequired > 0 {
		fmt.Println(tui.StyleErrorBox.Render(
			fmt.Sprintf("⚠️  %d required tool(s) missing. Install them with:\n   devx doctor install", missingRequired)))
		fmt.Println()
	} else if missingCreds > 0 {
		fmt.Println(tui.StyleBox.Render(
			fmt.Sprintf("ℹ️  All tools installed. %d credential(s) need attention.\n   Run: devx doctor auth", missingCreds)))
		fmt.Println()
	} else {
		fmt.Println(tui.StyleSuccessBox.Render("✅ All prerequisites installed and configured. You're good to go!"))
		fmt.Println()
	}

	return nil
}

func printToolRow(t doctor.ToolStatus) {
	if t.Installed {
		ver := t.Version
		if ver == "" {
			ver = "✓"
		}
		note := ""
		if t.Note != "" {
			note = tui.StyleMuted.Render(" " + t.Note)
		}
		fmt.Printf("    %s  %-14s %-12s %s%s\n",
			tui.IconDone,
			t.Binary,
			tui.StyleDetailDone.Render(ver),
			tui.StyleMuted.Render("("+t.FeatureArea+")"),
			note,
		)
	} else {
		label := "missing"
		if !t.Required {
			label = "not installed"
		}
		note := ""
		if t.Note != "" {
			note = tui.StyleMuted.Render(" " + t.Note)
		}
		icon := tui.IconFailed
		style := tui.StyleDetailError
		if !t.Required {
			icon = tui.StyleMuted.Render("—")
			style = tui.StyleMuted
		}
		fmt.Printf("    %s  %-14s %-12s %s%s\n",
			icon,
			t.Binary,
			style.Render(label),
			tui.StyleMuted.Render("("+t.FeatureArea+")"),
			note,
		)
	}
}

func printCredRow(c doctor.CredentialStatus) {
	if c.Configured {
		fmt.Printf("    %s  %-24s %s\n",
			tui.IconDone,
			c.Name,
			tui.StyleDetailDone.Render(c.Detail),
		)
	} else {
		fmt.Printf("    %s  %-24s %s\n",
			tui.IconFailed,
			c.Name,
			tui.StyleDetailError.Render(c.Detail),
		)
	}
}

type featureReadiness struct {
	command string
	ready   bool
	missing string
}

func computeFeatureReadiness(r doctor.Report) []featureReadiness {
	tools := make(map[string]bool)
	for _, t := range r.Tools {
		tools[t.Binary] = t.Installed
	}

	creds := make(map[string]bool)
	for _, c := range r.Credentials {
		creds[c.Name] = c.Configured
	}

	return []featureReadiness{
		{
			command: "devx vm init",
			ready:   tools["podman"] && tools["butane"] && tools["cloudflared"],
			missing: missingList(tools, "podman", "butane", "cloudflared"),
		},
		{
			command: "devx tunnel expose",
			ready:   tools["cloudflared"],
			missing: missingList(tools, "cloudflared"),
		},
		{
			command: "devx sites init",
			ready:   tools["gh"] && creds["Cloudflare API Token"],
			missing: joinMissing(missingList(tools, "gh"), missingCredList(creds, "Cloudflare API Token")),
		},
		{
			command: "devx db spawn",
			ready:   tools["podman"] || tools["docker"],
			missing: "podman or docker",
		},
		{
			command: "devx config pull",
			ready:   tools["op"] || tools["bw"] || tools["gcloud"],
			missing: "op, bw, or gcloud",
		},
		{
			command: "devx agent ship",
			ready:   tools["gh"],
			missing: missingList(tools, "gh"),
		},
	}
}

func missingList(tools map[string]bool, names ...string) string {
	var missing []string
	for _, n := range names {
		if !tools[n] {
			missing = append(missing, n)
		}
	}
	return strings.Join(missing, ", ")
}

func missingCredList(creds map[string]bool, names ...string) string {
	var missing []string
	for _, n := range names {
		if !creds[n] {
			missing = append(missing, n)
		}
	}
	return strings.Join(missing, ", ")
}

func joinMissing(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, ", ")
}

func friendlyOS(os string) string {
	switch os {
	case "darwin":
		// Try to get macOS version
		if out, err := runSilent("sw_vers", "-productVersion"); err == nil {
			return "macOS " + strings.TrimSpace(out)
		}
		return "macOS"
	case "linux":
		// Try to get distro name
		if out, err := runSilent("lsb_release", "-ds"); err == nil {
			return strings.TrimSpace(out)
		}
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func runSilent(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

// ── devx doctor install ──────────────────────────────────────────────────────

var doctorInstallAll bool

var doctorInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install missing prerequisite CLI tools",
	Long: `Detects missing CLI tools and installs them using your system's
package manager (Homebrew on macOS, apt/dnf on Linux).

By default, only required tools are installed. Use --all to include optional ones.

Examples:
  devx doctor install          # install missing required tools
  devx doctor install --all    # install all missing tools (including optional)
  devx doctor install -y       # auto-confirm, no prompts`,
	RunE: runDoctorInstall,
}

func runDoctorInstall(_ *cobra.Command, _ []string) error {
	plan, err := doctor.PlanInstall(!doctorInstallAll)
	if err != nil {
		return err
	}

	if outputJSON {
		enc, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	if len(plan.Steps) == 0 {
		fmt.Println(tui.StyleSuccessBox.Render("✅ Nothing to install — all tools are present!"))
		fmt.Println()
		return nil
	}

	// ── Show Plan (Transparency) ────────────────────────────────────
	fmt.Println()
	fmt.Println(tui.StyleTitle.Render("📦 Install Plan"))
	fmt.Printf("  %s  %s\n\n", tui.StyleLabel.Render("Package Manager:"), plan.PackageManager)

	for _, s := range plan.Steps {
		reqLabel := tui.StyleDetailDone.Render("required")
		if !s.IsRequired {
			reqLabel = tui.StyleMuted.Render("optional")
		}
		fmt.Printf("    %s  %-20s %s\n",
			tui.StyleDetailRunning.Render("→"),
			s.Tool,
			reqLabel,
		)
		if s.Tap != "" {
			fmt.Printf("       %s\n", tui.StyleMuted.Render("brew tap "+s.Tap))
		}
		fmt.Printf("       %s\n", tui.StyleMuted.Render(s.Command))
	}
	fmt.Println()

	// ── Confirm ─────────────────────────────────────────────────────
	if !NonInteractive {
		fmt.Printf("  Install %d tool(s)? [y/N] ", len(plan.Steps))
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil || (answer != "y" && answer != "Y" && answer != "yes") {
			fmt.Println(tui.StyleMuted.Render("  Cancelled."))
			return nil
		}
	}

	fmt.Println()
	fmt.Println(tui.StyleTitle.Render("  Installing..."))
	fmt.Println()

	if err := doctor.ExecuteInstall(plan, NonInteractive); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(tui.StyleSuccessBox.Render("✅ Installation complete! Run 'devx doctor' to verify."))
	fmt.Println()

	return nil
}

// ── devx doctor auth ─────────────────────────────────────────────────────────

var doctorAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Walk through authenticating required tools and credentials",
	Long: `Guides you through authenticating each tool that devx depends on.
Steps that are already configured are automatically skipped.

Examples:
  devx doctor auth`,
	RunE: runDoctorAuth,
}

func runDoctorAuth(_ *cobra.Command, _ []string) error {
	steps := doctor.AuthPlan(envFile)

	if outputJSON {
		enc, _ := json.MarshalIndent(steps, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Println()
	fmt.Println(tui.StyleTitle.Render("🔑 devx doctor auth — Credential Setup"))
	fmt.Println()

	total := len(steps)
	skipped := 0
	completed := 0

	for i, step := range steps {
		stepNum := fmt.Sprintf("[%d/%d]", i+1, total)

		if step.Configured {
			fmt.Printf("  %s  %s  %s\n",
				tui.StyleMuted.Render(stepNum),
				step.Name,
				tui.StyleDetailDone.Render("✅ "+step.Detail),
			)
			skipped++
			continue
		}

		fmt.Printf("  %s  %s  %s\n",
			tui.StyleDetailRunning.Render(stepNum),
			step.Name,
			tui.StyleDetailError.Render(step.Detail),
		)
		fmt.Printf("         %s\n\n",
			tui.StyleMuted.Render(step.Action),
		)

		if doctor.RunAuthStep(step, envFile) {
			completed++
			fmt.Println()
		} else {
			fmt.Println()
		}
	}

	fmt.Println()
	if completed > 0 || skipped == total {
		fmt.Println(tui.StyleSuccessBox.Render(
			fmt.Sprintf("✅ Auth complete! %d configured, %d skipped.\n   Run 'devx doctor' to verify.", completed, skipped)))
	} else {
		fmt.Println(tui.StyleBox.Render(
			fmt.Sprintf("ℹ️  %d step(s) skipped, %d configured.\n   Run 'devx doctor' to see remaining gaps.", skipped, completed)))
	}
	fmt.Println()

	return nil
}

func init() {
	doctorInstallCmd.Flags().BoolVar(&doctorInstallAll, "all", false,
		"Install all missing tools, including optional ones")

	doctorCmd.AddCommand(doctorInstallCmd)
	doctorCmd.AddCommand(doctorAuthCmd)
	rootCmd.AddCommand(doctorCmd)
}
