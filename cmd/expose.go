package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"

	"github.com/VitruvianSoftware/devx/internal/authproxy"
	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/trafficproxy"
	"github.com/spf13/cobra"
)

var exposeName string
var exposeDomain string
var basicAuth string
var trafficProfile string

var exposeCmd = &cobra.Command{
	Use:   "expose [port]",
	Short: "Expose a local port to the internet via your *.ipv1337.dev domain.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port := args[0]
		devName := os.Getenv("USER")

		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			if s.DevHostname != "" {
				cfg.DevHostname = s.DevHostname
			}
		}

		baseDomain := cfg.CFDomain
		if exposeDomain != "" {
			baseDomain = exposeDomain
		} else if baseDomain == "" {
			return fmt.Errorf("CFDomain is not configured. Run `devx init` or `devx secrets` first")
		}

		// Ensure cloudflared is logged in
		if err := cloudflare.CheckLogin(); err != nil {
			return fmt.Errorf("cloudflared missing credentials: %w", err)
		}

		// Generate random sub-domain name if one wasn't provided
		exposeID := exposeName
		if exposeID == "" {
			exposeID = fmt.Sprintf("app-%x", int(rand.Int31()&0xfffff))
		}

		fullDomain := exposure.GenerateDomain(exposeID, baseDomain)
		tunnelName := fmt.Sprintf("devx-expose-%s-%s", devName, exposeID)

		fmt.Printf("🚀 Organizing explicit tunnel '%s'...\n", tunnelName)

		tunnel, err := cloudflare.EnsureTunnel(tunnelName)
		if err != nil {
			return fmt.Errorf("failed creating tunnel: %w", err)
		}

		fmt.Printf("🌍 Routing DNS for %s...\n", fullDomain)
		if err := cloudflare.RouteDNS(tunnelName, fullDomain); err != nil {
			return fmt.Errorf("failed routing dns: %w", err)
		}

		// Persist exposure metadata so 'devx tunnel list' can show the port
		_ = exposure.Save(exposure.Entry{
			TunnelName: tunnelName,
			TunnelID:   tunnel.ID,
			Port:       port,
			Domain:     fullDomain,
		})

		fmt.Printf("\n🎉 Your local app is explicitly available worldwide at:\n\n")
		fmt.Printf("   🌐 https://%s\n\n", fullDomain)
		fmt.Printf("Traffic is being securely routed to localhost:%s.\n", port)
		fmt.Printf("Press Ctrl+C to stop exposing your app.\n\n")

		targetPort := port
		if trafficProfile != "" {
			proxyPort, cleanupTraffic, err := trafficproxy.Start(targetPort, trafficProfile)
			if err != nil {
				return fmt.Errorf("failed starting traffic proxy: %w", err)
			}
			defer cleanupTraffic()
			targetPort = fmt.Sprintf("%d", proxyPort)
			fmt.Printf("🐢 Traffic shaping enabled (profile: %s).\n\n", trafficProfile)
		}

		if basicAuth != "" {
			proxyPort, cleanupAuth, err := authproxy.Start(port, basicAuth)
			if err != nil {
				return fmt.Errorf("failed starting auth proxy: %w", err)
			}
			defer cleanupAuth()
			targetPort = fmt.Sprintf("%d", proxyPort)
			fmt.Printf("🔒 Basic Auth enabled for this tunnel.\n\n")
		}

		configFile, err := cloudflare.WriteIngressConfig(tunnel.ID, fullDomain, targetPort)
		if err != nil {
			return fmt.Errorf("failed to create ingress config: %w", err)
		}

		pCmd := exec.Command("cloudflared", "tunnel", "--config", configFile, "run")

		// Hide noisy Cloudflared standard output, let it run silently just blocking
		pCmd.Stdout = nil
		pCmd.Stderr = nil

		err = pCmd.Run()
		if err != nil && err.Error() != "signal: interrupt" {
			return fmt.Errorf("cloudflared crashed: %w", err)
		}

		// Clean exit
		return nil
	},
}

func init() {
	exposeCmd.Flags().StringVarP(&exposeName, "name", "n", "", "Static sub-domain name to use (e.g. 'api' -> api.james.ipv1337.dev)")
	exposeCmd.Flags().StringVar(&exposeDomain, "domain", "", "Custom Cloudflare domain (BYOD) (e.g. 'mycompany.dev')")
	exposeCmd.Flags().StringVar(&basicAuth, "basic-auth", "", "Protect the exposed tunnel with basic auth (e.g. 'user:pass')")
	exposeCmd.Flags().StringVar(&trafficProfile, "throttle", "", "Simulate network conditions (profiles: 3g, edge, slow)")
	tunnelCmd.AddCommand(exposeCmd)
}
