package state

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/doctor"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/provider"
)

// DumpReport represents the full topological and contextual state of the devx environment.
type DumpReport struct {
	DoctorReport *doctor.Report     `json:"doctor_report"`
	VM           VMStatus           `json:"vm_status"`
	Topology     TopologyStatus     `json:"topology"`
	Tunnels      []TunnelDiagnostic `json:"tunnels"`
	Config       RedactedConfig     `json:"redacted_config"`
}

type VMStatus struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	State     string `json:"state"`
	Tailscale string `json:"tailscale"`
}

type ContainerDiagnostic struct {
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
	State string `json:"state"`
	Logs  string `json:"logs,omitempty"`
}

type TopologyStatus struct {
	DevxManagedContainers []ContainerDiagnostic `json:"devx_managed_containers"`
}

type TunnelDiagnostic struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type RedactedConfig struct {
	YamlContent string `json:"yaml_content,omitempty"`
	EnvContent  string `json:"env_content,omitempty"`
}

// GenerateDump orchestrates the collection of state for all sub-systems.
func GenerateDump(cfg *config.Config, prov *provider.Provider, vmState string, tsStatus string) (*DumpReport, error) {
	report := &DumpReport{}

	// System Health (Doctor Audit)
	audit := doctor.RunFullAudit(".env")
	report.DoctorReport = &audit

	// VM
	report.VM = VMStatus{
		Name:      cfg.DevHostname,
		Provider:  prov.VM.Name(),
		State:     vmState,
		Tailscale: tsStatus,
	}

	// Tunnels
	if tunnels, err := cloudflare.ListExposedTunnels(cfg.DevName); err == nil {
		prefix := fmt.Sprintf("devx-expose-%s-", cfg.DevName)
		for _, t := range tunnels {
			exposeID := strings.TrimPrefix(t.Name, prefix)
			report.Tunnels = append(report.Tunnels, TunnelDiagnostic{
				Name:   t.Name,
				Domain: exposure.GenerateDomain(exposeID, cfg.CFDomain),
			})
		}
	}

	// Topology Containers (using provider Runtime)
	// For simplicity in the dump, we list all containers.

	out, err := prov.Runtime.Exec("ps", "-a", "--filter", "name=devx-", "--format", "{{.Names}}|{{.Image}}|{{.State}}")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 3)
			if len(parts) == 3 {
				cd := ContainerDiagnostic{
					Name:  parts[0],
					Image: parts[1],
					State: parts[2],
				}
				// If not running, grab recent logs
				if strings.ToLower(parts[2]) != "running" && strings.ToLower(parts[2]) != "up" {
					logOut, _ := prov.Runtime.Exec("logs", "--tail", "25", cd.Name)
					cd.Logs = string(logOut)
				}
				report.Topology.DevxManagedContainers = append(report.Topology.DevxManagedContainers, cd)
			}
		}
	}

	// Config
	buf, err := os.ReadFile("devx.yaml")
	if err == nil {
		report.Config.YamlContent = redactYamlEnv(string(buf))
	}
	envBuf, err := os.ReadFile(".env")
	if err == nil {
		report.Config.EnvContent = redactEnvFile(string(envBuf))
	}

	return report, nil
}

// GenerateMarkdown converts the diagnostic dump into a readable markdown string.
func GenerateMarkdown(r *DumpReport) string {
	var sb strings.Builder

	sb.WriteString("# DevX Diagnostic Report\n\n")

	sb.WriteString("## 1. System & Doctor Audit\n")
	sb.WriteString(fmt.Sprintf("- OS: %s\n", r.DoctorReport.System.OS))
	sb.WriteString(fmt.Sprintf("- Arch: %s\n", r.DoctorReport.System.Arch))
	sb.WriteString(fmt.Sprintf("- Package Manager: %s\n\n", r.DoctorReport.System.PackageManager))

	for _, tool := range r.DoctorReport.Tools {
		status := "✅ Installed"
		if !tool.Installed {
			status = "❌ Missing"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s (v: %s)\n", status, tool.Name, tool.Version))
	}
	sb.WriteString("\n## 2. Credentials\n")
	for _, cred := range r.DoctorReport.Credentials {
		status := "✅ Configured"
		if !cred.Configured {
			status = "❌ Missing"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", status, cred.Name))
	}

	sb.WriteString("\n## 3. Virtual Machine (Infrastructure)\n")
	sb.WriteString(fmt.Sprintf("- Name: %s\n", r.VM.Name))
	sb.WriteString(fmt.Sprintf("- Provider: %s\n", r.VM.Provider))
	sb.WriteString(fmt.Sprintf("- State: %s\n", r.VM.State))
	sb.WriteString(fmt.Sprintf("- Tailscale: %s\n", r.VM.Tailscale))

	sb.WriteString("\n## 4. Tunnels (Exposed Ports)\n")
	if len(r.Tunnels) == 0 {
		sb.WriteString("- No tunnels currently exposed.\n")
	}
	for _, t := range r.Tunnels {
		sb.WriteString(fmt.Sprintf("- %s: https://%s\n", t.Name, t.Domain))
	}

	sb.WriteString("\n## 5. Topology (Containers)\n")
	if len(r.Topology.DevxManagedContainers) == 0 {
		sb.WriteString("- No managed containers detected.\n")
	}
	for _, c := range r.Topology.DevxManagedContainers {
		sb.WriteString(fmt.Sprintf("### %s\n", c.Name))
		sb.WriteString(fmt.Sprintf("- Image: %s\n", c.Image))
		sb.WriteString(fmt.Sprintf("- State: %s\n", c.State))
		if c.Logs != "" {
			sb.WriteString("\n<details><summary><b>Crash Logs (Last 25 lines)</b></summary>\n\n```\n")
			sb.WriteString(c.Logs)
			sb.WriteString("\n```\n</details>\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 6. Configurations (Redacted)\n")
	if r.Config.YamlContent != "" {
		sb.WriteString("### devx.yaml\n```yaml\n" + r.Config.YamlContent + "\n```\n")
	}
	if r.Config.EnvContent != "" {
		sb.WriteString("### .env\n```bash\n" + r.Config.EnvContent + "\n```\n")
	}

	return sb.String()
}

// GenerateJSON stringifies the report.
func GenerateJSON(r *DumpReport) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func redactYamlEnv(content string) string {
	// A simple heuristic to redact values in the env: block.
	// Matches `KEY: value` where it is indented properly under an env node.
	lines := strings.Split(content, "\n")
	inEnv := false
	envIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Determine current indentation
		indent := len(line) - len(strings.TrimLeft(line, " "))

		// If we are currently parsing env block and indent un-indented, stop
		if inEnv && indent <= envIndent {
			inEnv = false
		}

		if strings.HasPrefix(trimmed, "env:") {
			inEnv = true
			envIndent = indent
			continue
		}

		if inEnv {
			// Replace string literals after colon
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				k := parts[0]
				lines[i] = strings.Repeat(" ", indent) + k + ": \"<REDACTED>\""
			}
		}
	}
	return strings.Join(lines, "\n")
}

func redactEnvFile(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			lines[i] = parts[0] + "=\"<REDACTED>\""
		}
	}
	return strings.Join(lines, "\n")
}
