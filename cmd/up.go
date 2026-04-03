package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/VitruvianSoftware/devx/internal/authproxy"
	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/logs"
	"github.com/VitruvianSoftware/devx/internal/network"
	"github.com/VitruvianSoftware/devx/internal/orchestrator"
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

// DevxConfigMock defines a remote OpenAPI-backed mock server entry.
type DevxConfigMock struct {
	Name string `yaml:"name"` // Friendly name (becomes env var MOCK_<NAME>_URL)
	URL  string `yaml:"url"`  // Remote OpenAPI spec URL (must be http:// or https://)
	Port int    `yaml:"port"` // Host port (0 = auto-assign a free port)
}

// DevxConfigDependsOn references a service/database dependency with a gating condition.
type DevxConfigDependsOn struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"` // "service_healthy" or "service_started"
}

// DevxConfigHealthcheck defines how to verify a service is ready.
type DevxConfigHealthcheck struct {
	HTTP     string `yaml:"http"`     // HTTP endpoint to poll (e.g., "http://localhost:8080/health")
	TCP      string `yaml:"tcp"`      // TCP address to probe (e.g., "localhost:5432")
	Interval string `yaml:"interval"` // Duration string (e.g., "1s", "500ms")
	Timeout  string `yaml:"timeout"`  // Duration string (e.g., "30s")
	Retries  int    `yaml:"retries"`  // Number of consecutive successes required
}

// DevxConfigService defines a developer application in devx.yaml.
type DevxConfigService struct {
	Name        string                `yaml:"name"`
	Runtime     string                `yaml:"runtime"`    // "host" (default), "container", "kubernetes", "cloud"
	Command     []string              `yaml:"command"`    // e.g. ["npm", "run", "dev"]
	DependsOn   []DevxConfigDependsOn `yaml:"depends_on"` // services/databases that must be healthy first
	Healthcheck DevxConfigHealthcheck `yaml:"healthcheck"`
	Port        int                   `yaml:"port"`
	Env         map[string]string     `yaml:"env"` // extra env vars
}

// DevxConfigProfile defines a named overlay that merges additively onto the base config.
// Follows Docker Compose override semantics: matching names merge fields (profile wins),
// new entries are appended.
type DevxConfigProfile struct {
	Databases []DevxConfigDatabase `yaml:"databases"`
	Tunnels   []DevxConfigTunnel   `yaml:"tunnels"`
	Services  []DevxConfigService  `yaml:"services"`
	Mocks     []DevxConfigMock     `yaml:"mocks"`
}

type DevxConfig struct {
	Name      string                       `yaml:"name"`      // Project name
	Domain    string                       `yaml:"domain"`    // Custom domain (BYOD)
	Tunnels   []DevxConfigTunnel           `yaml:"tunnels"`   // List of ports to expose
	Databases []DevxConfigDatabase         `yaml:"databases"` // List of databases to provision
	Services  []DevxConfigService          `yaml:"services"`  // List of applications to orchestrate
	Test      DevxConfigTest               `yaml:"test"`      // Test configuration
	Mocks     []DevxConfigMock             `yaml:"mocks"`     // List of OpenAPI mock servers to provision
	Profiles  map[string]DevxConfigProfile `yaml:"profiles"`  // Named environment overlays
}

var upDomain string
var upProfile string

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

		// --- Idea 37: Apply profile overlay (additive/merge, Docker Compose style) ---
		if upProfile != "" {
			profile, ok := cfgYaml.Profiles[upProfile]
			if !ok {
				var available []string
				for k := range cfgYaml.Profiles {
					available = append(available, k)
				}
				if len(available) == 0 {
					return fmt.Errorf("profile %q not found — no profiles defined in devx.yaml", upProfile)
				}
				return fmt.Errorf("profile %q not found — available profiles: %v", upProfile, available)
			}
			mergeProfile(&cfgYaml, profile)
			fmt.Printf("📦 Applied profile: %s\n", upProfile)
		}

		if len(cfgYaml.Tunnels) == 0 && len(cfgYaml.Databases) == 0 && len(cfgYaml.Services) == 0 {
			return fmt.Errorf("devx.yaml has no 'tunnels', 'databases', or 'services' defined")
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

				// Idea 36: Auto-resolve port conflicts before spawning
				dbPort := db.Port
				if dbPort > 0 {
					actual, shifted, warning := network.ResolvePort(dbPort)
					if shifted {
						fmt.Fprintf(os.Stderr, "\n%s\n\n", warning)
					}
					dbPort = actual
				}

				args := []string{"db", "spawn", db.Engine}
				if dbPort > 0 {
					args = append(args, "--port", fmt.Sprintf("%d", dbPort))
				}
				cmd := exec.Command(devxBin, args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					// Idea 35: Tail crash logs inline on failure
					containerName := fmt.Sprintf("devx-db-%s", db.Engine)
					logs.TailContainerCrashLogs("podman", containerName, 50)
					return fmt.Errorf("failed provisioning %s: %w", db.Engine, err)
				}
			}
		}

		if len(cfgYaml.Tunnels) == 0 && len(cfgYaml.Services) == 0 {
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

		if err := ensureCloudflareLogin(); err != nil {
			return err
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

			// Idea 36: Auto-resolve port conflicts for tunnels
			resolvedPort := tConfig.Port
			actual, shifted, warning := network.ResolvePort(resolvedPort)
			if shifted {
				fmt.Fprintf(os.Stderr, "\n%s\n\n", warning)
				resolvedPort = actual
			}

			fmt.Printf("🌍 Routing %s to port %d...\n", domain, resolvedPort)
			if err := cloudflare.RouteDNS(tunnelName, domain); err != nil {
				return fmt.Errorf("failed routing DNS for %s: %w", domain, err)
			}

			targetPort := fmt.Sprintf("%d", resolvedPort)

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

		// --- Idea 34: Service Orchestration via DAG ---
		var dagCleanup func()
		if len(cfgYaml.Services) > 0 {
			dag := orchestrator.NewDAG()

			// Register database nodes (already started above, but needed for dependency resolution)
			for _, db := range cfgYaml.Databases {
				if db.Engine == "" {
					continue
				}
				hc := orchestrator.HealthcheckConfig{
					TCP:     fmt.Sprintf("localhost:%d", db.Port),
					Timeout: 15 * time.Second,
				}
				_ = dag.AddNode(&orchestrator.Node{
					Name:        db.Engine,
					Type:        orchestrator.NodeDatabase,
					Healthcheck: hc,
					Port:        db.Port,
				})
			}

			// Register service nodes
			for _, svc := range cfgYaml.Services {
				var deps []string
				for _, d := range svc.DependsOn {
					deps = append(deps, d.Name)
				}

				rt := orchestrator.RuntimeHost
				switch svc.Runtime {
				case "container":
					rt = orchestrator.RuntimeContainer
				case "kubernetes":
					rt = orchestrator.RuntimeKubernetes
				case "cloud":
					rt = orchestrator.RuntimeCloud
				}

				hc := orchestrator.HealthcheckConfig{}
				if svc.Healthcheck.HTTP != "" {
					hc.HTTP = svc.Healthcheck.HTTP
				}
				if svc.Healthcheck.TCP != "" {
					hc.TCP = svc.Healthcheck.TCP
				}
				if svc.Healthcheck.Interval != "" {
					hc.Interval, _ = time.ParseDuration(svc.Healthcheck.Interval)
				}
				if svc.Healthcheck.Timeout != "" {
					hc.Timeout, _ = time.ParseDuration(svc.Healthcheck.Timeout)
				}
				hc.Retries = svc.Healthcheck.Retries

				_ = dag.AddNode(&orchestrator.Node{
					Name:        svc.Name,
					Type:        orchestrator.NodeService,
					DependsOn:   deps,
					Healthcheck: hc,
					Port:        svc.Port,
					Runtime:     rt,
					Command:     svc.Command,
					Env:         svc.Env,
				})
			}

			if err := dag.Validate(); err != nil {
				return fmt.Errorf("service dependency error: %w", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var dagErr error
			dagCleanup, dagErr = dag.Execute(ctx)
			if dagErr != nil {
				if dagCleanup != nil {
					dagCleanup()
				}
				return fmt.Errorf("service orchestration failed: %w", dagErr)
			}
			fmt.Printf("\n✅ All services are running and healthy.\n")
		}

		if len(cfgYaml.Tunnels) > 0 {
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
				if dagCleanup != nil {
					dagCleanup()
				}
				return fmt.Errorf("cloudflared crashed: %w", err)
			}
		} else if len(cfgYaml.Services) > 0 {
			// No tunnels, but services are running — block until Ctrl+C
			fmt.Printf("\n🎉 Project '%s' is running! Press Ctrl+C to stop.\n\n", projectName)
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
		}

		if dagCleanup != nil {
			dagCleanup()
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

// mergeProfile applies an additive overlay onto the base config.
// For databases, tunnels, services, and mocks: entries with matching names/engines
// have their fields merged (profile wins). New entries are appended.
func mergeProfile(cfg *DevxConfig, profile DevxConfigProfile) {
	// Merge databases by engine
	for _, pdb := range profile.Databases {
		found := false
		for i, bdb := range cfg.Databases {
			if bdb.Engine == pdb.Engine {
				if pdb.Port != 0 {
					cfg.Databases[i].Port = pdb.Port
				}
				found = true
				break
			}
		}
		if !found {
			cfg.Databases = append(cfg.Databases, pdb)
		}
	}

	// Merge tunnels by name
	for _, pt := range profile.Tunnels {
		found := false
		for i, bt := range cfg.Tunnels {
			if bt.Name == pt.Name {
				if pt.Port != 0 {
					cfg.Tunnels[i].Port = pt.Port
				}
				if pt.BasicAuth != "" {
					cfg.Tunnels[i].BasicAuth = pt.BasicAuth
				}
				if pt.Throttle != "" {
					cfg.Tunnels[i].Throttle = pt.Throttle
				}
				found = true
				break
			}
		}
		if !found {
			cfg.Tunnels = append(cfg.Tunnels, pt)
		}
	}

	// Merge services by name
	for _, ps := range profile.Services {
		found := false
		for i, bs := range cfg.Services {
			if bs.Name == ps.Name {
				if ps.Runtime != "" {
					cfg.Services[i].Runtime = ps.Runtime
				}
				if len(ps.Command) > 0 {
					cfg.Services[i].Command = ps.Command
				}
				if ps.Port != 0 {
					cfg.Services[i].Port = ps.Port
				}
				if len(ps.DependsOn) > 0 {
					cfg.Services[i].DependsOn = ps.DependsOn
				}
				if ps.Healthcheck.HTTP != "" || ps.Healthcheck.TCP != "" {
					cfg.Services[i].Healthcheck = ps.Healthcheck
				}
				if len(ps.Env) > 0 {
					if cfg.Services[i].Env == nil {
						cfg.Services[i].Env = make(map[string]string)
					}
					for k, v := range ps.Env {
						cfg.Services[i].Env[k] = v
					}
				}
				found = true
				break
			}
		}
		if !found {
			cfg.Services = append(cfg.Services, ps)
		}
	}

	// Merge mocks by name
	for _, pm := range profile.Mocks {
		found := false
		for i, bm := range cfg.Mocks {
			if bm.Name == pm.Name {
				if pm.URL != "" {
					cfg.Mocks[i].URL = pm.URL
				}
				if pm.Port != 0 {
					cfg.Mocks[i].Port = pm.Port
				}
				found = true
				break
			}
		}
		if !found {
			cfg.Mocks = append(cfg.Mocks, pm)
		}
	}
}

func init() {
	upCmd.Flags().StringVar(&upDomain, "domain", "", "Custom Cloudflare domain (BYOD) to override setting in devx.yaml")
	upCmd.Flags().StringVar(&upProfile, "profile", "", "Apply a named profile overlay from devx.yaml (additive merge, Docker Compose style)")
	tunnelCmd.AddCommand(upCmd)
	rootCmd.AddCommand(upCmd) // Aliased at the root level as a top-level command
}
