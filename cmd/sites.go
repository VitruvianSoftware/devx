package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	cfapi "github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/github"
)

var sitesCmd = &cobra.Command{
	Use:   "sites",
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
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────")
		fmt.Println("│  ⚠️  One-Time Domain Verification Required")
		fmt.Println("├─────────────────────────────────────────────────────────────")
		fmt.Println("│")
		fmt.Println("│  GitHub requires org-level domain verification before")
		fmt.Println("│  it will provision an SSL certificate for custom domains.")
		fmt.Println("│")
		fmt.Println("│  Steps:")
		fmt.Printf("│    1. Open: %s\n", verifyURL)
		fmt.Printf("│    2. Click \"Add a domain\" → enter: %s\n", domain)
		fmt.Println("│    3. GitHub will show a TXT record to add")
		fmt.Println("│    4. Add the TXT record in your Cloudflare DNS dashboard")
		fmt.Println("│    5. Click \"Verify\" in GitHub")
		fmt.Println("│    6. Re-run: devx sites init --domain " + domain)
		fmt.Println("│")
		fmt.Println("│  This only needs to be done once per domain per org.")
		fmt.Println("└─────────────────────────────────────────────────────────────")
		fmt.Println()
		return nil
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

func init() {
	sitesInitCmd.Flags().StringVar(&sitesDomainFlag, "domain", "",
		"Root domain for the site (e.g., vitruviansoftware.dev)")
	sitesInitCmd.Flags().StringVar(&sitesNameFlag, "name", "",
		"Subdomain name (defaults to repository name)")

	sitesCmd.AddCommand(sitesInitCmd)
	rootCmd.AddCommand(sitesCmd)
}
