package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/doctor"
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

func init() {
	rootCmd.AddCommand(doctorCmd)
}
