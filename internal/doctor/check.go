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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ToolStatus represents the state of a single prerequisite tool.
type ToolStatus struct {
	Name        string `json:"name"`
	Binary      string `json:"binary"`
	FeatureArea string `json:"feature_area"`
	Required    bool   `json:"required"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version,omitempty"`
	Path        string `json:"path,omitempty"`
	InstallCmd  string `json:"install_cmd,omitempty"`
	InstallTap  string `json:"install_tap,omitempty"`
	Note        string `json:"note,omitempty"`
}

// CredentialStatus represents the state of a credential or auth session.
type CredentialStatus struct {
	Name       string `json:"name"`
	RequiredBy string `json:"required_by"`
	Configured bool   `json:"configured"`
	Detail     string `json:"detail,omitempty"`
	HowToFix   string `json:"how_to_fix,omitempty"`
}

// SystemInfo holds detected system information.
type SystemInfo struct {
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	PackageManager string `json:"package_manager"`
	PMVersion      string `json:"pm_version,omitempty"`
}

// AIProviderStatus represents the state of a single AI provider.
type AIProviderStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`      // "local_llm", "cloud_api", or "coding_agent"
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
	Note      string `json:"note,omitempty"`
}

// AILandscape holds the detected AI providers and coding agents.
type AILandscape struct {
	Providers    []AIProviderStatus `json:"providers"`
	LocalReady   bool               `json:"local_ready"`   // At least one local LLM is running
	CloudReady   bool               `json:"cloud_ready"`   // At least one cloud API key is set
	AgentsFound  int                `json:"agents_found"`  // Number of coding agents detected
}

// Report is the full doctor audit result.
type Report struct {
	System      SystemInfo         `json:"system"`
	Tools       []ToolStatus       `json:"tools"`
	Credentials []CredentialStatus `json:"credentials"`
	AI          AILandscape        `json:"ai"`
}

// toolDef defines a tool to check.
type toolDef struct {
	Name        string
	Binary      string
	FeatureArea string
	Required    bool
	VersionFlag string // flag to get version (e.g., "--version")
	InstallBrew string // brew package name
	InstallTap  string // brew tap (if needed)
	Note        string
}

// allTools returns the complete list of tools that devx depends on.
func allTools() []toolDef {
	return []toolDef{
		{
			Name:        "Podman",
			Binary:      "podman",
			FeatureArea: "Core VM",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "podman",
			Note:        "VM backend (--provider=podman)",
		},
		{
			Name:        "Cloudflared",
			Binary:      "cloudflared",
			FeatureArea: "Tunnels",
			Required:    true,
			VersionFlag: "--version",
			InstallBrew: "cloudflared",
			InstallTap:  "cloudflare/cloudflare",
		},
		{
			Name:        "Butane",
			Binary:      "butane",
			FeatureArea: "VM Init",
			Required:    true,
			VersionFlag: "--version",
			InstallBrew: "butane",
		},
		{
			Name:        "GitHub CLI",
			Binary:      "gh",
			FeatureArea: "Sites, Preview",
			Required:    true,
			VersionFlag: "--version",
			InstallBrew: "gh",
		},
		{
			Name:        "Docker",
			Binary:      "docker",
			FeatureArea: "Core VM",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "docker",
			Note:        "VM backend (--provider=docker)",
		},
		{
			Name:        "AWS CLI",
			Binary:      "aws",
			FeatureArea: "State Replication",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "awscli",
			Note:        "for devx state share with s3://",
		},
		{
			Name:        "OrbStack",
			Binary:      "orb",
			FeatureArea: "Core VM",
			Required:    false,
			VersionFlag: "version",
			InstallBrew: "orbstack",
			Note:        "VM backend (--provider=orbstack)",
		},
		{
			Name:        "Lima",
			Binary:      "limactl",
			FeatureArea: "Core VM",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "lima",
			Note:        "VM backend (--provider=lima)",
		},
		{
			Name:        "Colima",
			Binary:      "colima",
			FeatureArea: "Core VM",
			Required:    false,
			VersionFlag: "version",
			InstallBrew: "colima",
			Note:        "VM backend (--provider=colima)",
		},
		{
			Name:        "nerdctl",
			Binary:      "nerdctl",
			FeatureArea: "Container Runtime",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "nerdctl",
			Note:        "container CLI for Lima/Colima VMs",
		},
		{
			Name:        "1Password CLI",
			Binary:      "op",
			FeatureArea: "Vault",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "1password-cli",
			Note:        "for 1Password secret integration",
		},
		{
			Name:        "Bitwarden CLI",
			Binary:      "bw",
			FeatureArea: "Vault",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "bitwarden-cli",
			Note:        "for Bitwarden secret integration",
		},
		{
			Name:        "Google Cloud SDK",
			Binary:      "gcloud",
			FeatureArea: "Vault, State Replication",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "google-cloud-sdk",
			Note:        "for GCP secrets and devx state share with gs://",
		},
		{
			Name:        "Mutagen",
			Binary:      "mutagen",
			FeatureArea: "File Sync",
			Required:    false,
			VersionFlag: "version",
			InstallBrew: "mutagen",
			InstallTap:  "mutagen-io/mutagen",
			Note:        "for zero-rebuild hot reloading (devx sync)",
		},
		{
			Name:        "Kubectl",
			Binary:      "kubectl",
			FeatureArea: "Bridge",
			Required:    false,
			VersionFlag: "version --client --short",
			InstallBrew: "kubectl",
			Note:        "for hybrid bridge to remote K8s clusters (devx bridge)",
		},
	}
}

// DetectSystem detects the OS, architecture, and package manager.
func DetectSystem() SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// Detect package manager
	switch runtime.GOOS {
	case "darwin":
		if path, err := exec.LookPath("brew"); err == nil {
			info.PackageManager = "brew"
			if out, err := exec.Command(path, "--version").Output(); err == nil {
				lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
				if len(lines) > 0 {
					info.PMVersion = strings.TrimPrefix(lines[0], "Homebrew ")
				}
			}
		}
	case "linux":
		for _, pm := range []string{"apt", "dnf", "pacman", "yum", "apk", "nix"} {
			if _, err := exec.LookPath(pm); err == nil {
				info.PackageManager = pm
				break
			}
		}
	}

	return info
}

// CheckTools audits all prerequisite CLI tools.
func CheckTools() []ToolStatus {
	defs := allTools()
	results := make([]ToolStatus, 0, len(defs))

	for _, d := range defs {
		ts := ToolStatus{
			Name:        d.Name,
			Binary:      d.Binary,
			FeatureArea: d.FeatureArea,
			Required:    d.Required,
			Note:        d.Note,
		}

		// Build install command
		if d.InstallBrew != "" {
			ts.InstallCmd = "brew install " + d.InstallBrew
			ts.InstallTap = d.InstallTap
		}

		// Check if installed
		path, err := exec.LookPath(d.Binary)
		if err != nil {
			ts.Installed = false
			results = append(results, ts)
			continue
		}
		ts.Installed = true
		ts.Path = path

		// Get version
		if d.VersionFlag != "" {
			if out, err := exec.Command(d.Binary, d.VersionFlag).CombinedOutput(); err == nil {
				ts.Version = parseVersion(d.Binary, string(out))
			}
		}

		results = append(results, ts)
	}

	return results
}

// CheckCredentials audits all credentials and auth sessions.
func CheckCredentials(envFile string) []CredentialStatus {
	// Load .env if it exists
	envVars := loadEnvFile(envFile)

	results := []CredentialStatus{
		checkEnvVar("Cloudflare API Token", "sites, DNS operations",
			"CLOUDFLARE_API_TOKEN", "CF_API_TOKEN", envVars,
			"Add CLOUDFLARE_API_TOKEN to .env or export it"),

		checkFileExists("cloudflared login", "vm init, tunnel creation",
			filepath.Join(homeDir(), ".cloudflared", "cert.pem"),
			"Run: cloudflared login"),

		checkGHAuth(),

		checkTailscale(),

		checkEnvVarSingle("CF Tunnel Token", "tunnel expose",
			"CF_TUNNEL_TOKEN", envVars,
			"Generated automatically by: devx vm init"),
	}

	// Optional vault credentials
	if _, err := exec.LookPath("op"); err == nil {
		results = append(results, checkCommandSuccess("1Password session", "config pull (1Password)",
			"op", []string{"account", "list"},
			"Run: eval $(op signin)"))
	}

	if _, err := exec.LookPath("bw"); err == nil {
		results = append(results, checkCommandSuccess("Bitwarden session", "config pull (Bitwarden)",
			"bw", []string{"status"},
			"Run: bw login"))
	}

	if _, err := exec.LookPath("gcloud"); err == nil {
		results = append(results, checkGCloudAuth())
	}

	return results
}

// RunFullAudit runs the complete doctor check.
func RunFullAudit(envFile string) Report {
	return Report{
		System:      DetectSystem(),
		Tools:       CheckTools(),
		Credentials: CheckCredentials(envFile),
		AI:          CheckAILandscape(),
	}
}

// CheckAILandscape probes for available AI providers and coding agents.
func CheckAILandscape() AILandscape {
	var providers []AIProviderStatus
	var localReady, cloudReady bool
	agentsFound := 0

	// ── Local LLM Servers ──────────────────────────────────────────

	// Ollama (port 11434)
	if isPortOpen("11434") {
		model := ""
		if out, err := exec.Command("ollama", "list").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 1 {
				model = fmt.Sprintf("%d model(s) loaded", len(lines)-1)
			}
		}
		detail := "running on :11434"
		if model != "" {
			detail += ", " + model
		}
		providers = append(providers, AIProviderStatus{
			Name: "Ollama", Type: "local_llm", Available: true,
			Detail: detail, Note: "Preferred local provider",
		})
		localReady = true
	} else {
		// Check if ollama binary exists but server isn't running
		note := "Install: brew install ollama"
		if _, err := exec.LookPath("ollama"); err == nil {
			note = "Installed but not running — start with: ollama serve"
		}
		providers = append(providers, AIProviderStatus{
			Name: "Ollama", Type: "local_llm", Available: false,
			Detail: "not running", Note: note,
		})
	}

	// LM Studio (port 1234)
	if isPortOpen("1234") {
		providers = append(providers, AIProviderStatus{
			Name: "LM Studio", Type: "local_llm", Available: true,
			Detail: "running on :1234",
		})
		localReady = true
	} else {
		providers = append(providers, AIProviderStatus{
			Name: "LM Studio", Type: "local_llm", Available: false,
			Detail: "not running", Note: "Download: https://lmstudio.ai",
		})
	}

	// ── Cloud API Keys ──────────────────────────────────────────────

	// OpenAI
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		detail := "OPENAI_API_KEY set"
		if base := os.Getenv("OPENAI_API_BASE"); base != "" {
			detail += fmt.Sprintf(" (custom base: %s)", base)
		}
		providers = append(providers, AIProviderStatus{
			Name: "OpenAI API", Type: "cloud_api", Available: true,
			Detail: detail, Note: "Cloud fallback for devx db synthesize",
		})
		cloudReady = true
	} else {
		providers = append(providers, AIProviderStatus{
			Name: "OpenAI API", Type: "cloud_api", Available: false,
			Detail: "OPENAI_API_KEY not set",
			Note:  "Export OPENAI_API_KEY for cloud AI features",
		})
	}

	// Anthropic
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		providers = append(providers, AIProviderStatus{
			Name: "Anthropic API", Type: "cloud_api", Available: true,
			Detail: "ANTHROPIC_API_KEY set",
		})
		cloudReady = true
	} else {
		providers = append(providers, AIProviderStatus{
			Name: "Anthropic API", Type: "cloud_api", Available: false,
			Detail: "ANTHROPIC_API_KEY not set",
		})
	}

	// Google Gemini
	geminiAvail := false
	geminiDetail := "not configured"
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		geminiAvail = true
		geminiDetail = "GEMINI_API_KEY set"
	} else if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		geminiAvail = true
		geminiDetail = "GOOGLE_API_KEY set"
	}
	providers = append(providers, AIProviderStatus{
		Name: "Google Gemini API", Type: "cloud_api", Available: geminiAvail,
		Detail: geminiDetail,
	})
	if geminiAvail {
		cloudReady = true
	}

	// ── AI Coding Agents ───────────────────────────────────────────

	agentChecks := []struct {
		name   string
		binary string
		note   string
	}{
		{"Gemini CLI", "gemini", "Google's AI coding agent"},
		{"Claude Code", "claude", "Anthropic's AI coding agent"},
		{"Codex CLI", "codex", "OpenAI's AI coding agent"},
		{"OpenCode", "opencode", "Open-source AI coding agent"},
		{"Cursor", "cursor", "AI-powered IDE"},
		{"Aider", "aider", "AI pair programming in terminal"},
	}

	for _, ac := range agentChecks {
		if path, err := exec.LookPath(ac.binary); err == nil {
			providers = append(providers, AIProviderStatus{
				Name: ac.name, Type: "coding_agent", Available: true,
				Detail: path, Note: ac.note,
			})
			agentsFound++
		}
	}

	if agentsFound == 0 {
		providers = append(providers, AIProviderStatus{
			Name: "AI Coding Agents", Type: "coding_agent", Available: false,
			Detail: "none detected",
			Note:  "Install one and run: devx agent init",
		})
	}

	return AILandscape{
		Providers:   providers,
		LocalReady:  localReady,
		CloudReady:  cloudReady,
		AgentsFound: agentsFound,
	}
}

// isPortOpen checks if a TCP port is open on localhost with a short timeout.
func isPortOpen(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 150*time.Millisecond)
	if err != nil {
		return false
	}
	if conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}

// --- helpers ---

func parseVersion(binary, raw string) string {
	raw = strings.TrimSpace(raw)
	lines := strings.SplitN(raw, "\n", 2)
	line := lines[0]

	switch binary {
	case "podman":
		// "podman version 4.9.4"
		return strings.TrimPrefix(line, "podman version ")
	case "cloudflared":
		// "cloudflared version 2024.12.1 (built 2024-12-18-...)"
		parts := strings.Fields(line)
		for i, p := range parts {
			if p == "version" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	case "butane":
		// "Butane 0.22.0"
		return strings.TrimPrefix(line, "Butane ")
	case "gh":
		// "gh version 2.65.0 (2024-12-...)"
		parts := strings.Fields(line)
		for i, p := range parts {
			if p == "version" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	case "docker":
		// "Docker version 27.4.0, ..."
		parts := strings.Fields(line)
		for i, p := range parts {
			if p == "version" && i+1 < len(parts) {
				return strings.TrimSuffix(parts[i+1], ",")
			}
		}
	case "orb":
		return line
	case "limactl":
		// "limactl version 1.0.2"
		return strings.TrimPrefix(line, "limactl version ")
	case "colima":
		// "colima version 0.8.1"
		return strings.TrimPrefix(line, "colima version ")
	case "nerdctl":
		// "nerdctl version 2.0.3"
		return strings.TrimPrefix(line, "nerdctl version ")
	case "op":
		return strings.TrimSpace(line)
	case "bw":
		return strings.TrimSpace(line)
	case "gcloud":
		// multi-line, find "Google Cloud SDK X.Y.Z"
		for _, l := range strings.Split(raw, "\n") {
			if strings.Contains(l, "Google Cloud SDK") {
				return strings.TrimSpace(strings.TrimPrefix(l, "Google Cloud SDK"))
			}
		}
	case "kubectl":
		// "Client Version: v1.31.0" or "Client Version: vX.Y.Z"
		return strings.TrimPrefix(strings.TrimSpace(line), "Client Version: ")
	}

	// Fallback: return first line, trimmed
	if len(line) > 40 {
		return line[:40] + "..."
	}
	return line
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func loadEnvFile(path string) map[string]string {
	env := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			env[key] = val
		}
	}
	return env
}

func checkEnvVar(name, requiredBy, key1, key2 string, envVars map[string]string, howToFix string) CredentialStatus {
	cs := CredentialStatus{
		Name:       name,
		RequiredBy: requiredBy,
		HowToFix:   howToFix,
	}

	// Check .env first, then OS env
	if v, ok := envVars[key1]; ok && v != "" {
		cs.Configured = true
		cs.Detail = fmt.Sprintf("configured (.env, %s)", key1)
		return cs
	}
	if key2 != "" {
		if v, ok := envVars[key2]; ok && v != "" {
			cs.Configured = true
			cs.Detail = fmt.Sprintf("configured (.env, %s)", key2)
			return cs
		}
	}
	if v := os.Getenv(key1); v != "" {
		cs.Configured = true
		cs.Detail = fmt.Sprintf("configured (env var %s)", key1)
		return cs
	}
	if key2 != "" {
		if v := os.Getenv(key2); v != "" {
			cs.Configured = true
			cs.Detail = fmt.Sprintf("configured (env var %s)", key2)
			return cs
		}
	}

	cs.Detail = "not found"
	return cs
}

func checkEnvVarSingle(name, requiredBy, key string, envVars map[string]string, howToFix string) CredentialStatus {
	return checkEnvVar(name, requiredBy, key, "", envVars, howToFix)
}

func checkFileExists(name, requiredBy, path, howToFix string) CredentialStatus {
	cs := CredentialStatus{
		Name:       name,
		RequiredBy: requiredBy,
		HowToFix:   howToFix,
	}
	if _, err := os.Stat(path); err == nil {
		cs.Configured = true
		cs.Detail = path
	} else {
		cs.Detail = "not found"
	}
	return cs
}

func checkGHAuth() CredentialStatus {
	cs := CredentialStatus{
		Name:       "GitHub CLI auth",
		RequiredBy: "sites init/status",
		HowToFix:   "Run: gh auth login --scopes admin:org",
	}

	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		cs.Detail = "not authenticated"
		return cs
	}

	outStr := string(out)
	cs.Configured = true

	if strings.Contains(outStr, "admin:org") {
		cs.Detail = "authenticated (admin:org ✓)"
	} else {
		cs.Detail = "authenticated (missing admin:org scope)"
		cs.HowToFix = "Run: gh auth refresh --scopes admin:org"
	}
	return cs
}

func checkCommandSuccess(name, requiredBy, binary string, args []string, howToFix string) CredentialStatus {
	cs := CredentialStatus{
		Name:       name,
		RequiredBy: requiredBy,
		HowToFix:   howToFix,
	}
	if err := exec.Command(binary, args...).Run(); err != nil {
		cs.Detail = "not authenticated"
	} else {
		cs.Configured = true
		cs.Detail = "authenticated"
	}
	return cs
}

// checkTailscale checks for Tailscale authentication.
// Tailscale uses interactive browser auth via `tailscale up`, not an env var.
// We check if a devx VM already exists (Tailscale is baked into Ignition).
func checkTailscale() CredentialStatus {
	cs := CredentialStatus{
		Name:       "Tailscale auth",
		RequiredBy: "vm init (Tailnet join)",
		HowToFix:   "Handled interactively during: devx vm init",
	}

	// Check for VMs from any supported provider
	vmFound := false

	// Check Podman machines
	if out, err := exec.Command("podman", "machine", "list", "--format", "{{.Name}}").Output(); err == nil {
		if strings.TrimSpace(string(out)) != "" {
			vmFound = true
		}
	}

	// Check Lima instances
	if !vmFound {
		if out, err := exec.Command("limactl", "list", "--format", "{{.Name}}").Output(); err == nil {
			if strings.TrimSpace(string(out)) != "" {
				vmFound = true
			}
		}
	}

	// Check Colima profiles
	if !vmFound {
		if err := exec.Command("colima", "status").Run(); err == nil {
			vmFound = true
		}
	}

	if vmFound {
		cs.Configured = true
		cs.Detail = "configured via VM (interactive browser auth)"
		return cs
	}

	cs.Detail = "will authenticate during vm init (browser flow)"
	cs.Configured = true // Not a blocker — auth happens interactively at init time
	return cs
}

// checkGCloudAuth checks for any credentialed GCP accounts.
// Supports multiple accounts — reports the active one.
func checkGCloudAuth() CredentialStatus {
	cs := CredentialStatus{
		Name:       "GCP auth",
		RequiredBy: "config pull (GCP)",
		HowToFix:   "Run: gcloud auth login",
	}

	out, err := exec.Command("gcloud", "auth", "list", "--format=value(account,status)").Output()
	if err != nil {
		cs.Detail = "not authenticated"
		return cs
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var activeAccount string
	var totalAccounts int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		totalAccounts++
		// Format: "account@example.com\t*" (active) or "account@example.com\t" (inactive)
		parts := strings.Split(line, "\t")
		account := strings.TrimSpace(parts[0])
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "*" {
			activeAccount = account
		}
	}

	if totalAccounts == 0 {
		cs.Detail = "no accounts configured"
		return cs
	}

	cs.Configured = true
	if activeAccount != "" {
		cs.Detail = fmt.Sprintf("%s (%d account(s))", activeAccount, totalAccounts)
	} else {
		cs.Detail = fmt.Sprintf("%d account(s), no active default", totalAccounts)
	}
	return cs
}
