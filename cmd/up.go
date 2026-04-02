package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/VitruvianSoftware/devx/internal/authproxy"
	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/trafficproxy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type DevxConfigTunnel struct {
	Name      string `yaml:"name"`       // Subdomain explicitly requested
	Port      int    `yaml:"port"`       // Local port to forward traffic towards
	BasicAuth string `yaml:"basic_auth"` // basic auth literal value 'user:pass'
	Throttle  string `yaml:"throttle"`   // traffic shaping profile e.g. '3g'
}

type DevxConfigDatabase struct {
	Engine string `yaml:"engine"`
	Port   int    `yaml:"port"`
}

type DevxConfigTest struct {
	UI DevxConfigTestUI `yaml:"ui"`
}

type DevxConfigTestUI struct {
	Setup   string `yaml:"setup"`   // Pre-processing steps (e.g., migrations) to run before tests
	Command string `yaml:"command"` // The actual test command to execute
}

type DevxConfig struct {
	Name      string               `yaml:"name"`      // Project name
	Domain    string               `yaml:"domain"`    // Custom domain (BYOD)
	Tunnels   []DevxConfigTunnel   `yaml:"tunnels"`   // List of ports to expose
	Databases []DevxConfigDatabase `yaml:"databases"` // List of databases to provision
	Test      DevxConfigTest       `yaml:"test"`      // Test configuration
}

var upDomain string

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Provision databases and expose ports defined in devx.yaml.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureVMRunning(); err != nil {
			return err
		}

		yamlPath := "devx.yaml"
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return fmt.Errorf("could not find devx.yaml in the current directory. Please create one to use 'devx tunnel up'")
		}

		b, err := os.ReadFile(yamlPath)
		if err != nil {
			return fmt.Errorf("failed creating reading devx.yaml: %w", err)
		}

		var cfgYaml DevxConfig
		if err = yaml.Unmarshal(b, &cfgYaml); err != nil {
			return fmt.Errorf("failed parsing YAML file block: %w", err)
		}

		if len(cfgYaml.Tunnels) == 0 && len(cfgYaml.Databases) == 0 {
			return fmt.Errorf("devx.yaml has no 'tunnels' or 'databases' defined")
		}

		projectName := cfgYaml.Name
		if projectName == "" {
			projectName = filepath.Base(mustGetwd())
		}
		
		if len(cfgYaml.Databases) > 0 {
			fmt.Printf("🏗️ Bootstrapping Project '%s' Databases...\n", projectName)
			devxBin, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolving devx binary: %w", err)
			}
			for _, db := range cfgYaml.Databases {
				if db.Engine == "" {
					continue
				}
				args := []string{"db", "spawn", db.Engine}
				if db.Port > 0 {
					args = append(args, "--port", fmt.Sprintf("%d", db.Port))
				}
				cmd := exec.Command(devxBin, args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed provisioning %s: %w", db.Engine, err)
				}
			}
		}

		if len(cfgYaml.Tunnels) == 0 {
			fmt.Printf("\n🎉 Project '%s' databases are up!\n\n", projectName)
			return nil
		}

		devName := os.Getenv("USER")
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil && s.DevHostname != "" {
			cfg.DevHostname = s.DevHostname
		}

		baseDomain := cfg.CFDomain
		if upDomain != "" {
			baseDomain = upDomain
		} else if cfgYaml.Domain != "" {
			baseDomain = cfgYaml.Domain
		}

		if baseDomain == "" {
			return fmt.Errorf("CFDomain is not configured. Run `devx init` or `devx config secrets` first")
		}

		if err := cloudflare.CheckLogin(); err != nil {
			return fmt.Errorf("cloudflared missing credentials: %w", err)
		}

		// Generate a single tunnel for the whole project topology
		tunnelName := fmt.Sprintf("devx-expose-%s-prj-%s-%x", devName, projectName, int(rand.Int31()&0xfff))

		fmt.Printf("🏗️ Bootstrapping Project '%s' via tunnel %s...\n", projectName, tunnelName)

		tunnel, err := cloudflare.EnsureTunnel(tunnelName)
		if err != nil {
			return fmt.Errorf("failed creating project aggregate tunnel: %w", err)
		}

		var ingresses []cloudflare.IngressEntry

		for _, tConfig := range cfgYaml.Tunnels {
			// e.g. 'api' -> 'api-myproject' mapping
			exposeID := tConfig.Name
			if exposeID == "" {
				exposeID = fmt.Sprintf("port-%d", tConfig.Port) // fallbacks
			}
			composedDomainID := fmt.Sprintf("%s-%s", exposeID, projectName)
			domain := exposure.GenerateDomain(composedDomainID, baseDomain)
			
			fmt.Printf("🌍 Routing %s to port %d...\n", domain, tConfig.Port)
			if err := cloudflare.RouteDNS(tunnelName, domain); err != nil {
				return fmt.Errorf("failed routing DNS for %s: %w", domain, err)
			}

			targetPort := fmt.Sprintf("%d", tConfig.Port)

			if tConfig.Throttle != "" {
				proxyPort, cleanupTraffic, err := trafficproxy.Start(targetPort, tConfig.Throttle)
				if err != nil {
					return fmt.Errorf("failed starting traffic proxy for %s: %w", domain, err)
				}
				defer cleanupTraffic()
				targetPort = fmt.Sprintf("%d", proxyPort)
				fmt.Printf("  🐢 Traffic shaping active (%s) on %s\n", tConfig.Throttle, domain)
			}

			if tConfig.BasicAuth != "" {
				proxyPort, cleanupAuth, err := authproxy.Start(targetPort, tConfig.BasicAuth)
				if err != nil {
					return fmt.Errorf("failed starting auth proxy for %s: %w", domain, err)
				}
				// we technically leak these cleanups when this function defers out unless we catch it
				defer cleanupAuth()
				targetPort = fmt.Sprintf("%d", proxyPort)
				fmt.Printf("  🔒 Proxy Auth active on %s\n", domain)
			}
			
			// register to our ingress routing pipeline
			ingresses = append(ingresses, cloudflare.IngressEntry{
				Hostname:   domain,
				TargetPort: targetPort,
			})
			
			// persist exposure state
			_ = exposure.Save(exposure.Entry{
				TunnelName: tunnelName,
				TunnelID:   tunnel.ID,
				Port:       fmt.Sprintf("%d", tConfig.Port),
				Domain:     domain,
			})
		}
		
		fmt.Printf("\n🎉 All services are now explicitly available worldwide! Press Ctrl+C to stop exposing your environment.\n\n")

		configFile, err := cloudflare.WriteMultiIngressConfig(tunnel.ID, ingresses)
		if err != nil {
			return fmt.Errorf("failed to create ingress config payload: %w", err)
		}

		pCmd := exec.Command("cloudflared", "tunnel", "--config", configFile, "run")
		
		pCmd.Stdout = nil
		pCmd.Stderr = nil 

		err = pCmd.Run()
		if err != nil && err.Error() != "signal: interrupt" {
			return fmt.Errorf("cloudflared crashed: %w", err)
		}

		return nil
	},
}

func mustGetwd() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}

func init() {
	upCmd.Flags().StringVar(&upDomain, "domain", "", "Custom Cloudflare domain (BYOD) to override setting in devx.yaml")
	tunnelCmd.AddCommand(upCmd)
	rootCmd.AddCommand(upCmd) // Aliased at the root level as a top-level command
}
