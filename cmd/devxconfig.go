package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ─── Schema Types ─────────────────────────────────────────────────────────────

// DevxConfigInclude defines an external devx.yaml to compose into this project (Idea 44).
type DevxConfigInclude struct {
	Path    string `yaml:"path"`     // Path to devx.yaml file (relative to the parent devx.yaml)
	EnvFile string `yaml:"env_file"` // Optional: override .env for this sub-project
}

// DevxConfigDatabaseSeed defines the seed command for a database.
type DevxConfigDatabaseSeed struct {
	Command string `yaml:"command"` // Command to run on the host to seed the database
}

// DevxConfigDatabasePull defines how to pull production data into the local database.
type DevxConfigDatabasePull struct {
	Command string `yaml:"command"` // Command that writes a dump to stdout
	Format  string `yaml:"format"`  // "custom" for pg_restore binary format (postgres only)
	Jobs    int    `yaml:"jobs"`    // Parallel workers for pg_restore (default: num CPUs)
}

// DevxConfigDatabase defines a local database to provision.
type DevxConfigDatabase struct {
	Engine string                 `yaml:"engine"`
	Port   int                    `yaml:"port"`
	Pull   DevxConfigDatabasePull `yaml:"pull"`
	Seed   DevxConfigDatabaseSeed `yaml:"seed"`
	Dir    string                 `yaml:"-"` // Internal: set by include resolver to the included project's dir
}

// DevxConfigTest holds test configuration.
type DevxConfigTest struct {
	UI DevxConfigTestUI `yaml:"ui"`
}

// DevxConfigTestUI defines end-to-end test orchestration settings.
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

// DevxConfigSync defines a host→container file sync mapping powered by Mutagen.
// This bypasses slow VirtioFS volume mounts for instant hot-reload.
type DevxConfigSync struct {
	Container string   `yaml:"container"` // Target container name (e.g., "my-api")
	Src       string   `yaml:"src"`       // Host source path (relative to devx.yaml)
	Dest      string   `yaml:"dest"`      // Container destination path
	Ignore    []string `yaml:"ignore"`    // Additional ignore patterns (on top of defaults)
}

// DevxConfigTunnel configures a Cloudflare tunnel to expose a local port.
type DevxConfigTunnel struct {
	Name      string `yaml:"name"`       // Subdomain explicitly requested
	Port      int    `yaml:"port"`       // Local port to forward traffic towards
	BasicAuth string `yaml:"basic_auth"` // basic auth literal value 'user:pass'
	Throttle  string `yaml:"throttle"`   // traffic shaping profile e.g. '3g'
}

// DevxConfigServiceBridgeTarget defines an inline bridge connect target on a service (Idea 46.3).
type DevxConfigServiceBridgeTarget struct {
	Service   string `yaml:"service"`    // K8s service name
	Namespace string `yaml:"namespace"` // Override namespace (default: bridge.namespace)
	Port      int    `yaml:"port"`       // Remote service port
	LocalPort int    `yaml:"local_port"` // Local port to bind (0 = auto)
}

// DevxConfigServiceBridgeIntercept defines an inline bridge intercept on a service (Idea 46.3).
type DevxConfigServiceBridgeIntercept struct {
	Service   string `yaml:"service"`    // K8s service to intercept
	Namespace string `yaml:"namespace"` // Override namespace (default: bridge.namespace)
	Port      int    `yaml:"port"`       // Remote service port
	LocalPort int    `yaml:"local_port"` // Local port to route traffic to
	Mode      string `yaml:"mode"`       // "steal" or "mirror" (required)
}

// DevxConfigService defines a developer application in devx.yaml.
type DevxConfigService struct {
	Name            string                            `yaml:"name"`
	Runtime         string                            `yaml:"runtime"`                      // "host" (default), "container", "kubernetes", "cloud", "bridge"
	Command         []string                          `yaml:"command"`                      // e.g. ["npm", "run", "dev"]
	DependsOn       []DevxConfigDependsOn             `yaml:"depends_on"`                   // services/databases that must be healthy first
	Healthcheck     DevxConfigHealthcheck             `yaml:"healthcheck"`
	Port            int                               `yaml:"port"`
	Env             map[string]string                 `yaml:"env"`                          // extra env vars
	Sync            []DevxConfigSync                  `yaml:"sync,omitempty"`               // file sync mappings into containers
	BridgeTarget    *DevxConfigServiceBridgeTarget    `yaml:"bridge_target,omitempty"`       // Idea 46.3: inline outbound bridge
	BridgeIntercept *DevxConfigServiceBridgeIntercept `yaml:"bridge_intercept,omitempty"`    // Idea 46.3: inline intercept
	Dir             string                            `yaml:"-"`                            // Internal: working directory (set by include resolver)
}

// DevxConfigProfile defines a named overlay that merges additively onto the base config.
// Follows Docker Compose override semantics: matching names merge fields (profile wins),
// new entries are appended.
type DevxConfigProfile struct {
	Databases []DevxConfigDatabase `yaml:"databases"`
	Tunnels   []DevxConfigTunnel   `yaml:"tunnels"`
	Services  []DevxConfigService  `yaml:"services"`
	Mocks     []DevxConfigMock     `yaml:"mocks"`
	Bridge    *DevxConfigBridge    `yaml:"bridge"` // Idea 46.1: override bridge config per profile
}

// DevxConfigPipelineStage defines a single pipeline step (test, lint, build, verify).
// The Before/After hooks are scaffolded for Idea 45.3 — parsed and validated but
// not executed in 45.2.
type DevxConfigPipelineStage struct {
	Command  []string   `yaml:"command,omitempty"`  // Single command shorthand
	Commands [][]string `yaml:"commands,omitempty"` // Multi-command sequential execution
	Before   [][]string `yaml:"before,omitempty"`   // Pre-stage hooks (Idea 45.3)
	After    [][]string `yaml:"after,omitempty"`    // Post-stage hooks (Idea 45.3)
}

// Cmds returns the resolved command list, preferring 'commands' over 'command'.
func (ps *DevxConfigPipelineStage) Cmds() [][]string {
	if len(ps.Commands) > 0 {
		return ps.Commands
	}
	if len(ps.Command) > 0 {
		return [][]string{ps.Command}
	}
	return nil
}

// DevxConfigPipeline declares optional explicit pipeline stage overrides.
// When present, auto-detection via DetectStack is bypassed entirely ("Explicit Wins").
type DevxConfigPipeline struct {
	Test   *DevxConfigPipelineStage `yaml:"test,omitempty"`
	Lint   *DevxConfigPipelineStage `yaml:"lint,omitempty"`
	Build  *DevxConfigPipelineStage `yaml:"build,omitempty"`
	Verify *DevxConfigPipelineStage `yaml:"verify,omitempty"`
}

// DevxConfigCustomAction defines a named, on-demand task (scaffolded for Idea 45.3).
type DevxConfigCustomAction struct {
	Command  []string   `yaml:"command,omitempty"`
	Commands [][]string `yaml:"commands,omitempty"`
}

// Cmds returns the resolved command list, preferring 'commands' over 'command'.
func (ca *DevxConfigCustomAction) Cmds() [][]string {
	if len(ca.Commands) > 0 {
		return ca.Commands
	}
	if len(ca.Command) > 0 {
		return [][]string{ca.Command}
	}
	return nil
}

// DevxConfigBridgeTarget defines a remote K8s service to bridge locally (Idea 46.1).
type DevxConfigBridgeTarget struct {
	Service   string `yaml:"service"`    // K8s service name (e.g., "payments-api")
	Namespace string `yaml:"namespace"` // K8s namespace (default: from bridge.namespace)
	Port      int    `yaml:"port"`       // Remote service port to forward
	LocalPort int    `yaml:"local_port"` // Local port to bind (0 = auto)
}

// DevxConfigBridgeIntercept defines an inbound traffic intercept target (Idea 46.2).
type DevxConfigBridgeIntercept struct {
	Service   string `yaml:"service"`    // K8s service to intercept
	Namespace string `yaml:"namespace"`  // Override namespace (default: bridge.namespace)
	Port      int    `yaml:"port"`       // Remote service port to intercept
	LocalPort int    `yaml:"local_port"` // Local port to route traffic to (default: same as port)
	Mode      string `yaml:"mode"`       // "steal" or "mirror" (required)
}

// DevxConfigBridge defines the hybrid edge-to-local routing configuration (Idea 46).
// Enables developers to connect their local environment to remote K8s services
// via kubectl port-forward, following the "Client-Driven Architecture" principle.
type DevxConfigBridge struct {
	Kubeconfig string                      `yaml:"kubeconfig"`   // Path to kubeconfig (default: ~/.kube/config)
	Context    string                      `yaml:"context"`      // Kube context to use
	Namespace  string                      `yaml:"namespace"`    // Default namespace for targets
	AgentImage string                      `yaml:"agent_image"`  // Override agent container image (Idea 46.2)
	Targets    []DevxConfigBridgeTarget    `yaml:"targets"`      // Outbound: remote services to bridge (46.1)
	Intercepts []DevxConfigBridgeIntercept `yaml:"intercepts"`   // Inbound: traffic intercept targets (46.2)
}

// DevxConfig is the root devx.yaml schema.
type DevxConfig struct {
	Name          string                              `yaml:"name"`            // Project name
	Domain        string                              `yaml:"domain"`          // Custom domain (BYOD)
	Env           []string                            `yaml:"env"`             // Vault sources for secret injection
	Include       []DevxConfigInclude                 `yaml:"include"`         // External devx.yaml files to compose (Idea 44)
	Tunnels       []DevxConfigTunnel                  `yaml:"tunnels"`         // List of ports to expose
	Databases     []DevxConfigDatabase                `yaml:"databases"`       // List of databases to provision
	Services      []DevxConfigService                 `yaml:"services"`        // List of applications to orchestrate
	Test          DevxConfigTest                      `yaml:"test"`            // Test configuration
	Mocks         []DevxConfigMock                    `yaml:"mocks"`           // List of OpenAPI mock servers to provision
	Profiles      map[string]DevxConfigProfile        `yaml:"profiles"`        // Named environment overlays
	Pipeline      *DevxConfigPipeline                 `yaml:"pipeline"`        // Explicit pipeline stages (Idea 45.2)
	CustomActions map[string]DevxConfigCustomAction   `yaml:"customActions"`   // Named tasks (scaffolded for Idea 45.3)
	Bridge        *DevxConfigBridge                   `yaml:"bridge"`          // Hybrid edge-to-local routing (Idea 46.1)
}

// ─── Config Resolution ────────────────────────────────────────────────────────

const maxIncludeDepth = 5

// resolveConfig reads a devx.yaml, processes all include directives recursively,
// and applies the specified profile overlay. This is the single entry point for
// config resolution — used by ALL commands that read devx.yaml.
//
// profile may be empty string to skip profile merging.
func resolveConfig(yamlPath, profile string) (*DevxConfig, error) {
	absPath, err := filepath.Abs(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path %q: %w", yamlPath, err)
	}

	seen := map[string]bool{}
	cfg, err := loadAndResolve(absPath, 0, seen)
	if err != nil {
		return nil, err
	}

	if profile != "" {
		p, ok := cfg.Profiles[profile]
		if !ok {
			var available []string
			for k := range cfg.Profiles {
				available = append(available, k)
			}
			if len(available) == 0 {
				return nil, fmt.Errorf("profile %q not found — no profiles defined in devx.yaml", profile)
			}
			return nil, fmt.Errorf("profile %q not found — available profiles: %v", profile, available)
		}
		mergeProfile(cfg, p)
		fmt.Printf("📦 Applied profile: %s\n", profile)
	}

	if err := validateBridgeServices(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateBridgeServices checks that services with runtime: bridge are correctly configured.
func validateBridgeServices(cfg *DevxConfig) error {
	for _, svc := range cfg.Services {
		if svc.Runtime != "bridge" {
			continue
		}

		hasTarget := svc.BridgeTarget != nil
		hasIntercept := svc.BridgeIntercept != nil

		if !hasTarget && !hasIntercept {
			return fmt.Errorf("service %q has runtime: bridge but neither bridge_target nor bridge_intercept is defined", svc.Name)
		}
		if hasTarget && hasIntercept {
			return fmt.Errorf("service %q cannot have both bridge_target and bridge_intercept — use separate services", svc.Name)
		}

		if cfg.Bridge == nil {
			return fmt.Errorf("service %q uses runtime: bridge but no top-level 'bridge:' section is defined in devx.yaml", svc.Name)
		}

		if hasTarget {
			if svc.BridgeTarget.Port <= 0 {
				return fmt.Errorf("service %q: bridge_target.port must be > 0", svc.Name)
			}
			if svc.BridgeTarget.Service == "" {
				return fmt.Errorf("service %q: bridge_target.service is required", svc.Name)
			}
		}

		if hasIntercept {
			if svc.BridgeIntercept.Port <= 0 {
				return fmt.Errorf("service %q: bridge_intercept.port must be > 0", svc.Name)
			}
			if svc.BridgeIntercept.Service == "" {
				return fmt.Errorf("service %q: bridge_intercept.service is required", svc.Name)
			}
			if svc.BridgeIntercept.Mode != "steal" {
				return fmt.Errorf("service %q: bridge_intercept.mode must be 'steal' (mirror not yet implemented)", svc.Name)
			}
		}
	}
	return nil
}

// loadAndResolve performs the recursive include resolution for a single devx.yaml file.
func loadAndResolve(absPath string, depth int, seen map[string]bool) (*DevxConfig, error) {
	if depth > maxIncludeDepth {
		return nil, fmt.Errorf("include depth exceeded maximum (%d) — check for circular includes", maxIncludeDepth)
	}

	if seen[absPath] {
		// Already incorporated — silently deduplicate (handles mutual includes)
		return &DevxConfig{}, nil
	}
	seen[absPath] = true

	b, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", absPath, err)
	}

	var cfg DevxConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	for _, inc := range cfg.Include {
		if inc.Path == "" {
			continue
		}
		incPath := inc.Path
		if !filepath.IsAbs(incPath) {
			incPath = filepath.Join(baseDir, incPath)
		}
		incAbsPath, err := filepath.Abs(incPath)
		if err != nil {
			return nil, fmt.Errorf("resolving include path %q from %s: %w", inc.Path, absPath, err)
		}

		if inc.EnvFile != "" {
			envAbs := inc.EnvFile
			if !filepath.IsAbs(envAbs) {
				envAbs = filepath.Join(baseDir, envAbs)
			}
			cfg.Env = append(cfg.Env, "file://"+envAbs)
		}

		if _, err := os.Stat(incAbsPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("include resolution failed: cannot read %s: no such file", incAbsPath)
		}

		incCfg, err := loadAndResolve(incAbsPath, depth+1, seen)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", inc.Path, err)
		}

		incDir := filepath.Dir(incAbsPath)

		// Merge: databases (fail-fast on engine collision)
		for _, db := range incCfg.Databases {
			for _, existing := range cfg.Databases {
				if existing.Engine == db.Engine {
					return nil, fmt.Errorf(
						"conflict: database engine %q defined in both %s and %s\n\n"+
							"Rename one or use a 'profiles:' overlay in the parent devx.yaml to resolve.",
						db.Engine, absPath, incAbsPath,
					)
				}
			}
			db.Dir = incDir
			cfg.Databases = append(cfg.Databases, db)
		}

		// Merge: services (fail-fast on name collision)
		for _, svc := range incCfg.Services {
			for _, existing := range cfg.Services {
				if existing.Name == svc.Name {
					return nil, fmt.Errorf(
						"conflict: service %q defined in both %s and %s\n\n"+
							"Rename one service or use a 'profiles:' overlay in the parent devx.yaml to resolve.\n"+
							"Tip: use descriptive names like 'payments-api' instead of just 'api'.",
						svc.Name, absPath, incAbsPath,
					)
				}
			}
			svc.Dir = incDir
			// Resolve sync src paths relative to the included project's directory
			for i := range svc.Sync {
				if svc.Sync[i].Src != "" && !filepath.IsAbs(svc.Sync[i].Src) {
					svc.Sync[i].Src = filepath.Join(incDir, svc.Sync[i].Src)
				}
			}
			cfg.Services = append(cfg.Services, svc)
		}

		// Merge: tunnels (fail-fast on name collision)
		for _, t := range incCfg.Tunnels {
			for _, existing := range cfg.Tunnels {
				if existing.Name == t.Name {
					return nil, fmt.Errorf(
						"conflict: tunnel %q defined in both %s and %s\n\n"+
							"Rename one or use a 'profiles:' overlay in the parent devx.yaml to resolve.",
						t.Name, absPath, incAbsPath,
					)
				}
			}
			cfg.Tunnels = append(cfg.Tunnels, t)
		}

		// Merge: mocks (fail-fast on name collision)
		for _, m := range incCfg.Mocks {
			for _, existing := range cfg.Mocks {
				if existing.Name == m.Name {
					return nil, fmt.Errorf(
						"conflict: mock %q defined in both %s and %s\n\n"+
							"Rename one or use a 'profiles:' overlay in the parent devx.yaml to resolve.",
						m.Name, absPath, incAbsPath,
					)
				}
			}
			cfg.Mocks = append(cfg.Mocks, m)
		}

		// Merge: bridge (first-defined wins — parent takes precedence)
		if cfg.Bridge == nil && incCfg.Bridge != nil {
			cfg.Bridge = incCfg.Bridge
		}

		// Merge env (vault) sources — deduplicate
		for _, envSrc := range incCfg.Env {
			found := false
			for _, existing := range cfg.Env {
				if existing == envSrc {
					found = true
					break
				}
			}
			if !found {
				cfg.Env = append(cfg.Env, envSrc)
			}
		}
	}

	return &cfg, nil
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
				if ps.BridgeTarget != nil {
					cfg.Services[i].BridgeTarget = ps.BridgeTarget
				}
				if ps.BridgeIntercept != nil {
					cfg.Services[i].BridgeIntercept = ps.BridgeIntercept
				}
				found = true
				break
			}
		}
		if !found {
			cfg.Services = append(cfg.Services, ps)
		}
	}

	// Merge bridge (profile wins entirely if present)
	if profile.Bridge != nil {
		cfg.Bridge = profile.Bridge
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
