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
// Requires the domain to be verified at the org level first.
func SetCustomDomain(owner, repo, domain string) error {
	out, err := exec.Command("gh", "api", "-X", "PUT",
		fmt.Sprintf("/repos/%s/%s/pages", owner, repo),
		"-f", fmt.Sprintf("cname=%s", domain),
		"-F", "https_enforced=true",
	).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "verify your domain") {
			return fmt.Errorf("domain not verified: run 'devx sites verify --domain %s' first", domain)
		}
		return fmt.Errorf("set custom domain: %w\n%s", err, outStr)
	}
	return nil
}

// DomainVerification holds the TXT record details returned by GitHub.
type DomainVerification struct {
	Host  string // e.g. _github-pages-challenge-VitruvianSoftware.vitruviansoftware.dev
	Value string // The TXT record value
}

// RequestDomainVerification initiates domain verification at the org level.
// Returns the TXT record details that must be set in DNS before calling ConfirmDomainVerification.
func RequestDomainVerification(owner, domain string) (*DomainVerification, error) {
	out, err := exec.Command("gh", "api", "-X", "POST",
		fmt.Sprintf("/orgs/%s/pages/domains", owner),
		"-f", fmt.Sprintf("domain=%s", domain),
	).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "already") || strings.Contains(outStr, "409") {
			return nil, nil // Already verified
		}
		return nil, fmt.Errorf("request domain verification: %w\n%s", err, outStr)
	}

	// GitHub responds with the challenge details
	var resp struct {
		Host  string `json:"host"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse verification response: %w\nraw: %s", err, string(out))
	}

	if resp.Host == "" || resp.Value == "" {
		return nil, fmt.Errorf("GitHub did not return verification TXT record details.\nRaw response: %s", string(out))
	}

	return &DomainVerification{Host: resp.Host, Value: resp.Value}, nil
}

// ConfirmDomainVerification tells GitHub to check the TXT record and complete verification.
func ConfirmDomainVerification(owner, domain string) error {
	// GitHub doesn't have a specific "confirm" endpoint — the verification happens
	// automatically once GitHub detects the TXT record. We can check status instead.
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("/orgs/%s/pages/domains", owner),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("check verification status: %w\n%s", err, string(out))
	}

	// Check if our domain is in the verified list
	if strings.Contains(string(out), domain) {
		return nil
	}
	return fmt.Errorf("domain %s is not yet verified — DNS TXT record may still be propagating", domain)
}
