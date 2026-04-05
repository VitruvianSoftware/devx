package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// Report is the full doctor audit result.
type Report struct {
	System      SystemInfo         `json:"system"`
	Tools       []ToolStatus       `json:"tools"`
	Credentials []CredentialStatus `json:"credentials"`
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
			Required:    true,
			VersionFlag: "--version",
			InstallBrew: "podman",
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
			FeatureArea: "Sites",
			Required:    true,
			VersionFlag: "--version",
			InstallBrew: "gh",
		},
		{
			Name:        "Docker",
			Binary:      "docker",
			FeatureArea: "Core VM (alt)",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "docker",
			Note:        "alternative to Podman (--provider=docker)",
		},
		{
			Name:        "OrbStack",
			Binary:      "orb",
			FeatureArea: "Core VM (alt)",
			Required:    false,
			VersionFlag: "version",
			InstallBrew: "orbstack",
			Note:        "alternative to Podman (--provider=orbstack)",
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
			FeatureArea: "Vault",
			Required:    false,
			VersionFlag: "--version",
			InstallBrew: "google-cloud-sdk",
			Note:        "for GCP Secret Manager integration",
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
	}
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
// We check if the VM is already provisioned (Tailscale is baked into Ignition).
func checkTailscale() CredentialStatus {
	cs := CredentialStatus{
		Name:       "Tailscale auth",
		RequiredBy: "vm init (Tailnet join)",
		HowToFix:   "Handled interactively during: devx vm init",
	}

	// Check if a devx VM already exists (Tailscale is configured inside it)
	out, err := exec.Command("podman", "machine", "list", "--format", "{{.Name}}").Output()
	if err == nil {
		names := strings.TrimSpace(string(out))
		if names != "" {
			cs.Configured = true
			cs.Detail = "configured via VM (interactive browser auth)"
			return cs
		}
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
