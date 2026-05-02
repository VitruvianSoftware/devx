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

package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	cfapi "github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/github"
)

var sitesCmd = &cobra.Command{
	Use:   "sites",
	GroupID: "ci",
	Short: "Manage documentation and static site hosting",
	Long: `Commands for initializing and managing GitHub Pages deployments
with automatic Cloudflare DNS configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var (
	sitesDomainFlag string
	sitesNameFlag   string
)

var sitesInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Enable GitHub Pages and configure Cloudflare DNS for this repository",
	Long: `Enables GitHub Pages (workflow-based) on the current repository and creates
a CNAME record in Cloudflare DNS pointing <name>.<domain> to <org>.github.io.

By default, <name> is derived from the repository name. Use --name to override.

Prerequisites:
  - gh CLI authenticated (gh auth login)
  - CLOUDFLARE_API_TOKEN env var set with DNS edit permissions
  - Domain zone must exist in your Cloudflare account

Examples:
  devx sites init --domain vitruviansoftware.dev
  # → Creates CNAME: devx.vitruviansoftware.dev (derived from repo name)

  devx sites init --domain vitruviansoftware.dev --name platform
  # → Creates CNAME: platform.vitruviansoftware.dev (explicit override)`,
	RunE: runSitesInit,
}

func runSitesInit(cmd *cobra.Command, _ []string) error {
	// Load .env so tokens like CLOUDFLARE_API_TOKEN are available
	_ = godotenv.Load(envFile)

	// ── Step 1: Detect the current repository ─────────────────────────────
	owner, repo, err := github.RepoInfo()
	if err != nil {
		return err
	}
	fmt.Printf("📦 Detected repository: %s/%s\n", owner, repo)

	// ── Step 2: Resolve the domain ────────────────────────────────────────
	domain := sitesDomainFlag
	if domain == "" {
		if NonInteractive {
			return fmt.Errorf("--domain is required in non-interactive mode")
		}
		err := huh.NewInput().
			Title("What domain should this site live under?").
			Description("e.g., vitruviansoftware.dev → creates <name>.vitruviansoftware.dev").
			Placeholder("example.dev").
			Value(&domain).
			Run()
		if err != nil {
			return fmt.Errorf("domain prompt: %w", err)
		}
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// ── Step 3: Resolve the subdomain name ────────────────────────────────
	siteName := sitesNameFlag
	if siteName == "" {
		siteName = strings.ToLower(repo)
	}

	subdomain := fmt.Sprintf("%s.%s", siteName, domain)
	pagesTarget := github.PagesEndpoint(owner)

	// ── Step 4: Explicit impact summary ───────────────────────────────────
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────")
	fmt.Println("│  ⚠️  DNS Change Summary")
	fmt.Println("├─────────────────────────────────────────────────────────────")
	fmt.Printf("│  Repository:    %s/%s\n", owner, repo)
	fmt.Printf("│  GitHub Pages:  ENABLED (workflow deployment mode)\n")
	fmt.Printf("│\n")
	fmt.Printf("│  DNS Record:    CNAME (DNS-only, not proxied)\n")
	fmt.Printf("│    Name:        %s\n", subdomain)
	fmt.Printf("│    Target:      %s\n", pagesTarget)
	fmt.Printf("│\n")
	fmt.Printf("│  SSL:           Let's Encrypt certificate (auto-provisioned)\n")
	fmt.Printf("│\n")
	fmt.Printf("│  Live URL:      https://%s\n", subdomain)
	if sitesNameFlag == "" {
		fmt.Printf("│\n")
		fmt.Printf("│  ℹ️  Subdomain \"%s\" was derived from the repo name.\n", siteName)
		fmt.Printf("│     Use --name <custom> to override.\n")
	}
	fmt.Println("└─────────────────────────────────────────────────────────────")

	if DryRun {
		fmt.Println("\n🏁 Dry run — no changes were made.")
		return nil
	}

	// ── Step 5: Confirmation gate ─────────────────────────────────────────
	if !NonInteractive {
		var confirmed bool
		err := huh.NewConfirm().
			Title("Proceed with these DNS and GitHub changes?").
			Affirmative("Yes, apply changes").
			Negative("No, abort").
			Value(&confirmed).
			Run()
		if err != nil {
			return fmt.Errorf("confirmation prompt: %w", err)
		}
		if !confirmed {
			fmt.Println("\n❌ Aborted. No changes were made.")
			return nil
		}
	}

	// ── Step 6: Enable GitHub Pages ───────────────────────────────────────
	fmt.Print("\n⏳ Enabling GitHub Pages... ")
	if err := github.EnablePages(owner, repo); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// ── Step 7: Create Cloudflare CNAME ───────────────────────────────────
	fmt.Print("⏳ Looking up Cloudflare zone... ")
	zoneID, err := cfapi.LookupZoneID(domain)
	if err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Printf("✓ (%s)\n", zoneID[:8]+"...")

	fmt.Print("⏳ Creating DNS CNAME record... ")
	if err := cfapi.CreateCNAME(zoneID, subdomain, pagesTarget, false); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// ── Step 8: Configure custom domain + SSL ─────────────────────────────
	needsVerification := false
	fmt.Print("⏳ Configuring custom domain & SSL certificate... ")
	if err := github.SetCustomDomain(owner, repo, subdomain); err != nil {
		if _, ok := err.(*github.DomainNotVerifiedError); ok {
			fmt.Println("⚠ (needs verification)")
			needsVerification = true
		} else {
			fmt.Println("✗")
			return err
		}
	} else {
		fmt.Println("✓")
	}

	// ── Step 9: Write the CNAME file locally ──────────────────────────────
	cnamePath := "docs/public/CNAME"
	if _, statErr := os.Stat("docs/public"); os.IsNotExist(statErr) {
		cnamePath = "CNAME"
	}
	if err := os.MkdirAll(strings.TrimSuffix(cnamePath, "/CNAME"), 0755); err == nil {
		if writeErr := os.WriteFile(cnamePath, []byte(subdomain+"\n"), 0644); writeErr == nil {
			fmt.Printf("⏳ Wrote local CNAME file: %s ✓\n", cnamePath)
		}
	}

	// ── Domain verification instructions ──────────────────────────────────
	if needsVerification {
		verifyURL := fmt.Sprintf("https://github.com/organizations/%s/settings/pages", owner)

		if NonInteractive {
			// Non-interactive: just print instructions and exit
			fmt.Println()
			fmt.Println("┌─────────────────────────────────────────────────────────────")
			fmt.Println("│  ⚠️  One-Time Domain Verification Required")
			fmt.Println("├─────────────────────────────────────────────────────────────")
			fmt.Println("│")
			fmt.Printf("│  Verify your domain at:\n")
			fmt.Printf("│    %s\n", verifyURL)
			fmt.Println("│")
			fmt.Println("│  Then run: devx sites verify --domain " + domain)
			fmt.Println("│  followed by: devx sites init --domain " + domain)
			fmt.Println("└─────────────────────────────────────────────────────────────")
			fmt.Println()
			return nil
		}

		// Interactive: walk the developer through it
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────")
		fmt.Println("│  🔐 One-Time Domain Verification (interactive)")
		fmt.Println("├─────────────────────────────────────────────────────────────")
		fmt.Println("│")
		fmt.Println("│  GitHub requires domain verification before issuing SSL.")
		fmt.Println("│  devx will handle the Cloudflare DNS side for you.")
		fmt.Println("│")
		fmt.Printf("│  1. Open: %s\n", verifyURL)
		fmt.Printf("│  2. Click \"Add a domain\" → enter: %s\n", domain)
		fmt.Println("│  3. GitHub will show a TXT record name and value")
		fmt.Println("│  4. Paste them below — devx will create the DNS record")
		fmt.Println("└─────────────────────────────────────────────────────────────")
		fmt.Println()

		if err := runVerificationWizard(domain, zoneID); err != nil {
			return err
		}

		// Retry the custom domain setup
		fmt.Println()
		fmt.Println("⏳ Waiting 10 seconds for DNS propagation...")
		time.Sleep(10 * time.Second)

		fmt.Print("⏳ Retrying custom domain & SSL setup... ")
		if err := github.SetCustomDomain(owner, repo, subdomain); err != nil {
			fmt.Println("⚠")
			fmt.Println()
			fmt.Println("  Domain may still be verifying. Click \"Verify\" in GitHub,")
			fmt.Println("  then re-run: devx sites init --domain " + domain)
			fmt.Println()
			return nil
		}
		fmt.Println("✓")
	}

	// ── Summary ──────────────────────────────────────────────────────────
	fmt.Printf("\n🎉 Site infrastructure is ready!\n\n")
	fmt.Printf("  Live URL:       https://%s\n", subdomain)
	fmt.Printf("  Pages target:   %s\n", pagesTarget)
	fmt.Printf("\n")
	fmt.Printf("  Next steps:\n")
	fmt.Printf("    1. Add a deploy-docs.yml GitHub Action (if not already present)\n")
	fmt.Printf("    2. Push to main to trigger the first deployment\n")
	fmt.Printf("    3. SSL certificate may take up to 15 minutes to provision\n")
	fmt.Println()

	return nil
}

// runVerificationWizard interactively collects TXT record details and creates
// the record in Cloudflare.
func runVerificationWizard(domain, zoneID string) error {
	var txtHost, txtValue string

	err := huh.NewInput().
		Title("TXT Record Name").
		Description("Copy the record name from GitHub (e.g., _github-pages-challenge-YourOrg)").
		Placeholder("_github-pages-challenge-YourOrg").
		Value(&txtHost).
		Run()
	if err != nil {
		return fmt.Errorf("TXT host prompt: %w", err)
	}

	err = huh.NewInput().
		Title("TXT Record Value").
		Description("Copy the verification code from GitHub").
		Placeholder("abc123def456...").
		Value(&txtValue).
		Run()
	if err != nil {
		return fmt.Errorf("TXT value prompt: %w", err)
	}

	txtHost = strings.TrimSpace(txtHost)
	txtValue = strings.TrimSpace(txtValue)

	if txtHost == "" || txtValue == "" {
		return fmt.Errorf("both TXT record name and value are required")
	}

	// If they didn't include the full domain, append it
	if !strings.Contains(txtHost, ".") {
		txtHost = txtHost + "." + domain
	}

	fmt.Println()
	fmt.Printf("⏳ Creating TXT record: %s → %s ... ", txtHost, txtValue)
	if err := cfapi.CreateTXT(zoneID, txtHost, txtValue); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")
	fmt.Println()
	fmt.Println("  ✅ TXT record created in Cloudflare!")
	fmt.Println("  👉 Go back to GitHub and click \"Verify\"")

	// Wait for user to verify
	fmt.Println()
	fmt.Println("  Press Enter after you've clicked Verify in GitHub...")
	_, _ = fmt.Scanln()

	return nil
}

// ── devx sites verify ────────────────────────────────────────────────────────

var sitesVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Create a DNS TXT record for GitHub Pages domain verification",
	Long: `Interactive wizard to create the TXT record required by GitHub for
domain verification. Use this when setting up a custom domain for the
first time.

devx handles the Cloudflare DNS side — you just paste the TXT record
details from GitHub's verification page.

Example:
  devx sites verify --domain vitruviansoftware.dev`,
	RunE: runSitesVerify,
}

func runSitesVerify(cmd *cobra.Command, _ []string) error {
	_ = godotenv.Load(envFile)

	domain := sitesDomainFlag
	if domain == "" {
		if NonInteractive {
			return fmt.Errorf("--domain is required in non-interactive mode")
		}
		err := huh.NewInput().
			Title("What domain are you verifying?").
			Placeholder("example.dev").
			Value(&domain).
			Run()
		if err != nil {
			return fmt.Errorf("domain prompt: %w", err)
		}
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	fmt.Print("⏳ Looking up Cloudflare zone... ")
	zoneID, err := cfapi.LookupZoneID(domain)
	if err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Printf("✓ (%s)\n", zoneID[:8]+"...")

	owner, _, _ := github.RepoInfo()
	if owner == "" {
		owner = "YourOrg"
	}
	verifyURL := fmt.Sprintf("https://github.com/organizations/%s/settings/pages", owner)

	fmt.Println()
	fmt.Printf("  Open: %s\n", verifyURL)
	fmt.Printf("  Add domain: %s\n", domain)
	fmt.Println()

	return runVerificationWizard(domain, zoneID)
}

// ── devx sites status ────────────────────────────────────────────────────────

var sitesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check GitHub Pages deployment and SSL certificate status",
	Long: `Shows the current state of GitHub Pages for this repository,
including custom domain configuration, SSL certificate provisioning
status, and HTTPS enforcement.

Example:
  devx sites status`,
	RunE: runSitesStatus,
}

func runSitesStatus(_ *cobra.Command, _ []string) error {
	owner, repo, err := github.RepoInfo()
	if err != nil {
		return err
	}
	fmt.Printf("📦 Repository: %s/%s\n\n", owner, repo)

	status, err := github.GetPagesStatus(owner, repo)
	if err != nil {
		return err
	}

	// Domain
	domainDisplay := "(none)"
	if status.CNAME != "" {
		domainDisplay = status.CNAME
	}

	// Domain verification
	domainState := "—"
	if status.DomainState != "" {
		switch status.DomainState {
		case "verified":
			domainState = "✅ verified"
		case "pending":
			domainState = "⏳ pending"
		default:
			domainState = "⚠ " + status.DomainState
		}
	}

	// HTTPS enforcement
	httpsStatus := "❌ disabled"
	if status.HTTPSEnforced {
		httpsStatus = "✅ enforced"
	}

	// SSL Certificate
	certState := "—"
	certExpiry := "—"
	certDomains := "—"
	if status.HTTPSCert != nil {
		switch status.HTTPSCert.State {
		case "approved":
			certState = "✅ " + status.HTTPSCert.Description
		case "pending", "new":
			certState = "⏳ " + status.HTTPSCert.Description
		case "errored":
			certState = "❌ " + status.HTTPSCert.Description
		default:
			certState = status.HTTPSCert.State + ": " + status.HTTPSCert.Description
		}
		if status.HTTPSCert.ExpiresAt != "" {
			certExpiry = status.HTTPSCert.ExpiresAt
		}
		if len(status.HTTPSCert.Domains) > 0 {
			certDomains = strings.Join(status.HTTPSCert.Domains, ", ")
		}
	}

	// Build status
	buildStatus := "—"
	if status.Status != "" {
		switch status.Status {
		case "built":
			buildStatus = "✅ built"
		case "building":
			buildStatus = "⏳ building"
		default:
			buildStatus = status.Status
		}
	}

	fmt.Println("┌─────────────────────────────────────────────────────────────")
	fmt.Println("│  📊 GitHub Pages Status")
	fmt.Println("├─────────────────────────────────────────────────────────────")
	fmt.Printf("│  Custom Domain:   %s\n", domainDisplay)
	fmt.Printf("│  Domain State:    %s\n", domainState)
	fmt.Printf("│  Build Status:    %s\n", buildStatus)
	fmt.Println("│")
	fmt.Printf("│  HTTPS:           %s\n", httpsStatus)
	fmt.Printf("│  SSL Certificate: %s\n", certState)
	fmt.Printf("│  SSL Domains:     %s\n", certDomains)
	fmt.Printf("│  SSL Expiry:      %s\n", certExpiry)
	if status.HTMLURL != "" {
		fmt.Println("│")
		fmt.Printf("│  Live URL:        %s\n", status.HTMLURL)
	}
	fmt.Println("└─────────────────────────────────────────────────────────────")
	fmt.Println()

	return nil
}

func init() {
	sitesInitCmd.Flags().StringVar(&sitesDomainFlag, "domain", "",
		"Root domain for the site (e.g., vitruviansoftware.dev)")
	sitesInitCmd.Flags().StringVar(&sitesNameFlag, "name", "",
		"Subdomain name (defaults to repository name)")

	sitesVerifyCmd.Flags().StringVar(&sitesDomainFlag, "domain", "",
		"Domain to verify (e.g., vitruviansoftware.dev)")

	sitesCmd.AddCommand(sitesInitCmd)
	sitesCmd.AddCommand(sitesVerifyCmd)
	sitesCmd.AddCommand(sitesStatusCmd)
	rootCmd.AddCommand(sitesCmd)
}
