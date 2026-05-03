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
	"github.com/VitruvianSoftware/devx/internal/telemetry"
	"github.com/VitruvianSoftware/devx/internal/trafficproxy"
	"github.com/spf13/cobra"
)


var upDomain string
var upProfile string

var upCmd = &cobra.Command{
	Use:   "up",
	GroupID: "orchestration",
	Short: "Provision databases and expose ports defined in devx.yaml.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureVMRunning(); err != nil {
			return err
		}

		yamlPath, err := mustFindDevxConfig()
		if err != nil {
			return err
		}

		// Idea 44: resolveConfig handles include blocks and profile merging in one pass
		cfgYaml, err := resolveConfig(yamlPath, upProfile)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		if len(cfgYaml.Tunnels) == 0 && len(cfgYaml.Databases) == 0 && len(cfgYaml.Services) == 0 {
			return fmt.Errorf("devx.yaml has no 'tunnels', 'databases', or 'services' defined")
		}

		projectName := cfgYaml.Name
		if projectName == "" {
			projectName = filepath.Base(mustGetwd())
		}

		// Idea 55: Allow preview sandboxes to override the project name for
		// database namespace isolation. When DEVX_PROJECT_OVERRIDE is set,
		// database containers are created with a unique prefix (e.g., pr-42)
		// so they don't collide with the developer's active environment.
		projectOverride := os.Getenv("DEVX_PROJECT_OVERRIDE")
		if projectOverride != "" {
			projectName = projectOverride
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
						_, _ = fmt.Fprintf(os.Stderr, "\n%s\n\n", warning)
					}
					dbPort = actual
				}

				args := []string{"db", "spawn", db.Engine}
				if dbPort > 0 {
					args = append(args, "--port", fmt.Sprintf("%d", dbPort))
				}
				if projectOverride != "" {
					args = append(args, "--project", projectOverride)
				}
				cmd := exec.Command(devxBin, args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					// Idea 35: Tail crash logs inline on failure
					containerName := fmt.Sprintf("devx-db-%s", db.Engine)
					if projectOverride != "" {
						containerName = fmt.Sprintf("devx-db-%s-%s", projectOverride, db.Engine)
					}
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
				_, _ = fmt.Fprintf(os.Stderr, "\n%s\n\n", warning)
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

			for _, svc := range cfgYaml.Services {
				var deps []string
				for _, d := range svc.DependsOn {
					deps = append(deps, d.Name)
				}

				rt := orchestrator.RuntimeHost
				var bridgeMode orchestrator.BridgeMode
				var bridgeCfg *orchestrator.BridgeNodeConfig

				switch svc.Runtime {
				case "container":
					rt = orchestrator.RuntimeContainer
				case "kubernetes":
					rt = orchestrator.RuntimeKubernetes
				case "cloud":
					rt = orchestrator.RuntimeCloud
				case "bridge":
					rt = orchestrator.RuntimeBridge
					// Build BridgeNodeConfig from inline fields + top-level bridge config
					if svc.BridgeTarget != nil {
						bridgeMode = orchestrator.BridgeModeConnect
						ns := svc.BridgeTarget.Namespace
						if ns == "" && cfgYaml.Bridge != nil {
							ns = cfgYaml.Bridge.Namespace
						}
						if ns == "" {
							ns = "default"
						}
						bridgeCfg = &orchestrator.BridgeNodeConfig{
							Kubeconfig:    cfgYaml.Bridge.Kubeconfig,
							Context:       cfgYaml.Bridge.Context,
							Namespace:     ns,
							TargetService: svc.BridgeTarget.Service,
							RemotePort:    svc.BridgeTarget.Port,
							LocalPort:     svc.BridgeTarget.LocalPort,
							Mode:          orchestrator.BridgeModeConnect,
						}
					} else if svc.BridgeIntercept != nil {
						bridgeMode = orchestrator.BridgeModeIntercept
						ns := svc.BridgeIntercept.Namespace
						if ns == "" && cfgYaml.Bridge != nil {
							ns = cfgYaml.Bridge.Namespace
						}
						if ns == "" {
							ns = "default"
						}
						agentImg := ""
						if cfgYaml.Bridge != nil {
							agentImg = cfgYaml.Bridge.AgentImage
						}
						bridgeCfg = &orchestrator.BridgeNodeConfig{
							Kubeconfig:    cfgYaml.Bridge.Kubeconfig,
							Context:       cfgYaml.Bridge.Context,
							Namespace:     ns,
							TargetService: svc.BridgeIntercept.Service,
							RemotePort:    svc.BridgeIntercept.Port,
							LocalPort:     svc.BridgeIntercept.LocalPort,
							AgentImage:    agentImg,
							Mode:          orchestrator.BridgeModeIntercept,
							InterceptMode: svc.BridgeIntercept.Mode,
						}
					}
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
					Name:         svc.Name,
					Type:         orchestrator.NodeService,
					DependsOn:    deps,
					Healthcheck:  hc,
					Port:         svc.Port,
					Runtime:      rt,
					Command:      svc.Command,
					Env:          svc.Env,
					Dir:          svc.Dir,
					BridgeMode:   bridgeMode,
					BridgeConfig: bridgeCfg,
				})
			}

			if err := dag.Validate(); err != nil {
				return fmt.Errorf("service dependency error: %w", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var dagErr error
			dagStart := time.Now()
			dagCleanup, dagErr = dag.Execute(ctx)
			telemetry.RecordEvent("up_startup", time.Since(dagStart))
			if dagErr != nil {
				if dagCleanup != nil {
					dagCleanup()
				}
				return fmt.Errorf("service orchestration failed: %w", dagErr)
			}
			fmt.Printf("\n✅ All services are running and healthy.\n")

			// Idea 46.3: Generate bridge.env for devx shell after all bridge services are healthy
			if err := dag.WriteBridgeEnvFile(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "⚠️  Could not write bridge.env: %v\n", err)
			}

			// Idea 43: Hint about file syncing when sync blocks are present
			hasSyncBlocks := false
			for _, svc := range cfgYaml.Services {
				if len(svc.Sync) > 0 {
					hasSyncBlocks = true
					break
				}
			}
			if hasSyncBlocks {
				fmt.Println("💡 Tip: Run 'devx sync up' to enable zero-rebuild hot reloading for container services.")
			}
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


func init() {
	upCmd.Flags().StringVar(&upDomain, "domain", "", "Custom Cloudflare domain (BYOD) to override setting in devx.yaml")
	upCmd.Flags().StringVar(&upProfile, "profile", "", "Apply a named profile overlay from devx.yaml (additive merge, Docker Compose style)")
	tunnelCmd.AddCommand(upCmd)
	rootCmd.AddCommand(upCmd) // Aliased at the root level as a top-level command
}
