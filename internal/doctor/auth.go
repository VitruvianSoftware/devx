package doctor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AuthStep represents a single authentication step.
type AuthStep struct {
	Name       string `json:"name"`
	RequiredBy string `json:"required_by"`
	Configured bool   `json:"configured"`
	Detail     string `json:"detail"`
	Action     string `json:"action,omitempty"` // human-readable action to take
}

// AuthPlan returns the list of authentication steps with their current status.
func AuthPlan(envFile string) []AuthStep {
	envVars := loadEnvFile(envFile)

	steps := []AuthStep{
		planCloudflaredLogin(),
		planGHAuth(),
		planCloudflareToken(envVars, envFile),
	}

	return steps
}

// RunAuthStep executes a single authentication step interactively.
// Returns true if the step was completed successfully.
func RunAuthStep(step AuthStep, envFile string) bool {
	switch step.Name {
	case "cloudflared login":
		return runCloudflaredLogin()
	case "GitHub CLI":
		return runGHAuth()
	case "Cloudflare API Token":
		return runCloudflareTokenSetup(envFile)
	}
	return false
}

// --- individual auth planners ---

func planCloudflaredLogin() AuthStep {
	s := AuthStep{
		Name:       "cloudflared login",
		RequiredBy: "vm init, tunnel creation",
		Action:     "Opens browser to authenticate with Cloudflare",
	}

	certPath := filepath.Join(homeDir(), ".cloudflared", "cert.pem")
	if _, err := os.Stat(certPath); err == nil {
		s.Configured = true
		s.Detail = certPath
	} else {
		s.Detail = "not authenticated"
	}
	return s
}

func planGHAuth() AuthStep {
	s := AuthStep{
		Name:       "GitHub CLI",
		RequiredBy: "sites init/status",
		Action:     "Authenticates gh CLI with admin:org scope",
	}

	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		s.Detail = "not authenticated"
		return s
	}

	outStr := string(out)
	s.Configured = true

	if strings.Contains(outStr, "admin:org") {
		s.Detail = "authenticated (admin:org ✓)"
	} else {
		s.Detail = "authenticated (missing admin:org scope)"
		s.Configured = false // needs refresh
		s.Action = "Refreshes gh auth to add admin:org scope"
	}
	return s
}

func planCloudflareToken(envVars map[string]string, envFile string) AuthStep {
	s := AuthStep{
		Name:       "Cloudflare API Token",
		RequiredBy: "sites init, DNS operations",
		Action:     "Prompts for token and saves to .env",
	}

	if v, ok := envVars["CLOUDFLARE_API_TOKEN"]; ok && v != "" {
		s.Configured = true
		s.Detail = "configured (.env, CLOUDFLARE_API_TOKEN)"
		return s
	}
	if v, ok := envVars["CF_API_TOKEN"]; ok && v != "" {
		s.Configured = true
		s.Detail = "configured (.env, CF_API_TOKEN)"
		return s
	}
	if v := os.Getenv("CLOUDFLARE_API_TOKEN"); v != "" {
		s.Configured = true
		s.Detail = "configured (env var)"
		return s
	}

	s.Detail = "not found"
	return s
}

// --- runners ---

func runCloudflaredLogin() bool {
	fmt.Println("    Opening browser for Cloudflare authentication...")
	fmt.Println("    This will create ~/.cloudflared/cert.pem")
	fmt.Println()

	cmd := exec.Command("cloudflared", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("    ✗ cloudflared login failed: %v\n", err)
		return false
	}
	return true
}

func runGHAuth() bool {
	// Check if already authed but missing scope
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err == nil && !strings.Contains(string(out), "admin:org") {
		// Already authenticated, just need to refresh scopes
		fmt.Println("    Refreshing GitHub CLI to add admin:org scope...")
		cmd := exec.Command("gh", "auth", "refresh", "--scopes", "admin:org")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Printf("    ✗ gh auth refresh failed: %v\n", err)
			return false
		}
		return true
	}

	if err != nil {
		// Not authenticated at all
		fmt.Println("    Authenticating GitHub CLI with admin:org scope...")
		cmd := exec.Command("gh", "auth", "login", "--scopes", "admin:org")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Printf("    ✗ gh auth login failed: %v\n", err)
			return false
		}
		return true
	}

	// Already fully configured
	return true
}

func runCloudflareTokenSetup(envFile string) bool {
	fmt.Println("    Create an API token at: https://dash.cloudflare.com/profile/api-tokens")
	fmt.Println("    Required permissions: Zone:DNS:Edit, Zone:Zone:Read")
	fmt.Println()
	fmt.Print("    Paste your Cloudflare API Token: ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("    ✗ Failed to read input: %v\n", err)
		return false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Println("    ✗ No token provided")
		return false
	}

	// Append to .env file
	f, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Printf("    ✗ Failed to open %s: %v\n", envFile, err)
		return false
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\nCLOUDFLARE_API_TOKEN=%s\n", token); err != nil {
		fmt.Printf("    ✗ Failed to write to %s: %v\n", envFile, err)
		return false
	}

	fmt.Printf("    ✓ Token saved to %s\n", envFile)
	return true
}
