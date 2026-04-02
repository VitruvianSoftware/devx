package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// EnablePages enables GitHub Pages on a repository with workflow-based deployment.
// Uses the `gh` CLI which handles authentication via existing user session.
func EnablePages(owner, repo string) error {
	out, err := exec.Command("gh", "api", "-X", "POST",
		fmt.Sprintf("/repos/%s/%s/pages", owner, repo),
		"-f", "build_type=workflow",
	).CombinedOutput()
	if err != nil {
		outStr := string(out)
		// "409 Conflict" means Pages is already enabled — treat as success
		if strings.Contains(outStr, "409") || strings.Contains(outStr, "already exists") {
			return nil
		}
		return fmt.Errorf("enable GitHub Pages: %w\n%s", err, outStr)
	}
	return nil
}

// RepoInfo extracts the owner and repo name from the current git working directory.
func RepoInfo() (owner, repo string, err error) {
	out, err := exec.Command("gh", "repo", "view", "--json", "owner,name", "-q", ".owner.login + \"/\" + .name").Output()
	if err != nil {
		return "", "", fmt.Errorf("could not detect GitHub repo (is `gh` authenticated?): %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected repo format: %s", string(out))
	}
	return parts[0], parts[1], nil
}

// PagesEndpoint returns the canonical GitHub Pages hostname for an organization.
// e.g., "vitruviansoftware" → "vitruviansoftware.github.io"
func PagesEndpoint(owner string) string {
	return strings.ToLower(owner) + ".github.io"
}

// SetCustomDomain configures the custom domain on GitHub Pages and triggers
// SSL certificate provisioning via Let's Encrypt.
// Requires the domain to be verified at the org level first (done via GitHub UI).
func SetCustomDomain(owner, repo, domain string) error {
	// Step 1: Set the custom domain (without HTTPS — cert needs time to provision)
	out, err := exec.Command("gh", "api", "-X", "PUT",
		fmt.Sprintf("/repos/%s/%s/pages", owner, repo),
		"-f", fmt.Sprintf("cname=%s", domain),
	).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "verify your domain") || strings.Contains(outStr, "Invalid cname") {
			return &DomainNotVerifiedError{Domain: domain, Owner: owner}
		}
		return fmt.Errorf("set custom domain: %w\n%s", err, outStr)
	}

	// Step 2: Try to enable HTTPS enforcement.
	// This may fail with "certificate does not exist yet" — that's OK,
	// GitHub will auto-provision the cert and enforce HTTPS once ready.
	//nolint:errcheck // cert provisioning is async — this may fail and that's OK
	_, _ = exec.Command("gh", "api", "-X", "PUT",
		fmt.Sprintf("/repos/%s/%s/pages", owner, repo),
		"-F", "https_enforced=true",
	).CombinedOutput()

	return nil
}

// DomainNotVerifiedError indicates the domain needs org-level verification in GitHub UI.
type DomainNotVerifiedError struct {
	Domain string
	Owner  string
}

func (e *DomainNotVerifiedError) Error() string {
	return fmt.Sprintf("domain %q is not verified for the %s organization", e.Domain, e.Owner)
}

// PagesStatus holds the current state of GitHub Pages for a repository.
type PagesStatus struct {
	CNAME         string     `json:"cname"`
	Status        string     `json:"status"` // "built", "building", etc.
	HTMLURL       string     `json:"html_url"`
	HTTPSEnforced bool       `json:"https_enforced"`
	DomainState   string     `json:"protected_domain_state"` // "verified", "unverified", "pending"
	HTTPSCert     *HTTPSCert `json:"https_certificate"`
}

// HTTPSCert holds the SSL certificate details.
type HTTPSCert struct {
	State       string   `json:"state"`       // "approved", "pending", "new", "errored"
	Description string   `json:"description"` // Human-readable status
	Domains     []string `json:"domains"`
	ExpiresAt   string   `json:"expires_at"`
}

// GetPagesStatus retrieves the current GitHub Pages configuration and SSL status.
func GetPagesStatus(owner, repo string) (*PagesStatus, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("/repos/%s/%s/pages", owner, repo),
	).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "404") || strings.Contains(outStr, "Not Found") {
			return nil, fmt.Errorf("GitHub Pages is not enabled on %s/%s", owner, repo)
		}
		return nil, fmt.Errorf("get pages status: %w\n%s", err, outStr)
	}

	var status PagesStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse pages status: %w", err)
	}
	return &status, nil
}
