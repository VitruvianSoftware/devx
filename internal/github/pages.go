package github

import (
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
