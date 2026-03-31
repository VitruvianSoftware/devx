package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
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

var sitesDomainFlag string

var sitesInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Enable GitHub Pages and configure Cloudflare DNS for this repository",
	Long: `Enables GitHub Pages (workflow-based) on the current repository and creates
a CNAME record in Cloudflare DNS pointing <repo>.<domain> to <org>.github.io.

Prerequisites:
  - gh CLI authenticated (gh auth login)
  - CLOUDFLARE_API_TOKEN env var set with DNS edit permissions
  - Domain zone must exist in your Cloudflare account

Example:
  devx sites init --domain vitruviansoftware.dev
  # → Enables Pages on current repo
  # → Creates CNAME: <repo>.vitruviansoftware.dev → <org>.github.io`,
	RunE: runSitesInit,
}

func runSitesInit(cmd *cobra.Command, _ []string) error {
	// ── Step 1: Detect the current repository ─────────────────────────────
	owner, repo, err := github.RepoInfo()
	if err != nil {
		return err
	}
	fmt.Printf("📦 Detected repository: %s/%s\n", owner, repo)

	// ── Step 2: Resolve the domain ────────────────────────────────────────
	domain := sitesDomainFlag
	if domain == "" {
		// Interactive prompt if no flag was provided
		if NonInteractive {
			return fmt.Errorf("--domain is required in non-interactive mode")
		}
		err := huh.NewInput().
			Title("What domain should this site live under?").
			Description("e.g., vitruviansoftware.dev → creates <repo>.vitruviansoftware.dev").
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

	subdomain := fmt.Sprintf("%s.%s", strings.ToLower(repo), domain)
	pagesTarget := github.PagesEndpoint(owner)

	fmt.Printf("\n🔧 Plan:\n")
	fmt.Printf("   GitHub Pages:  enabled (workflow mode) on %s/%s\n", owner, repo)
	fmt.Printf("   DNS CNAME:     %s → %s\n", subdomain, pagesTarget)

	if DryRun {
		fmt.Println("\n🏁 Dry run — no changes made.")
		return nil
	}

	// ── Step 3: Enable GitHub Pages ───────────────────────────────────────
	fmt.Print("\n⏳ Enabling GitHub Pages... ")
	if err := github.EnablePages(owner, repo); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// ── Step 4: Create Cloudflare CNAME ───────────────────────────────────
	fmt.Print("⏳ Looking up Cloudflare zone... ")
	zoneID, err := cfapi.LookupZoneID(domain)
	if err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Printf("✓ (%s)\n", zoneID[:8]+"...")

	fmt.Print("⏳ Creating DNS CNAME record... ")
	// GitHub Pages requires the CNAME to NOT be proxied (DNS only)
	if err := cfapi.CreateCNAME(zoneID, subdomain, pagesTarget, false); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// ── Step 5: Write the CNAME file locally ──────────────────────────────
	cnamePath := "docs/public/CNAME"
	if _, statErr := os.Stat("docs/public"); os.IsNotExist(statErr) {
		cnamePath = "CNAME"
	}
	if err := os.MkdirAll(strings.TrimSuffix(cnamePath, "/CNAME"), 0755); err == nil {
		if writeErr := os.WriteFile(cnamePath, []byte(subdomain+"\n"), 0644); writeErr == nil {
			fmt.Printf("⏳ Wrote local CNAME file: %s ✓\n", cnamePath)
		}
	}

	// ── Summary ──────────────────────────────────────────────────────────
	fmt.Printf("\n🎉 Site infrastructure is ready!\n\n")
	fmt.Printf("  Live URL:       https://%s\n", subdomain)
	fmt.Printf("  Pages target:   %s\n", pagesTarget)
	fmt.Printf("\n")
	fmt.Printf("  Next steps:\n")
	fmt.Printf("    1. Add a deploy-docs.yml GitHub Action (if not already present)\n")
	fmt.Printf("    2. Push to main to trigger the first deployment\n")
	fmt.Printf("    3. DNS propagation may take up to 5 minutes\n\n")

	return nil
}

func init() {
	sitesInitCmd.Flags().StringVar(&sitesDomainFlag, "domain", "",
		"Root domain for the site (e.g., vitruviansoftware.dev)")

	sitesCmd.AddCommand(sitesInitCmd)
	rootCmd.AddCommand(sitesCmd)
}
