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

// DevxConfigService defines a developer application in devx.yaml.
type DevxConfigService struct {
	Name        string                `yaml:"name"`
	Runtime     string                `yaml:"runtime"`    // "host" (default), "container", "kubernetes", "cloud"
	Command     []string              `yaml:"command"`    // e.g. ["npm", "run", "dev"]
	DependsOn   []DevxConfigDependsOn `yaml:"depends_on"` // services/databases that must be healthy first
	Healthcheck DevxConfigHealthcheck `yaml:"healthcheck"`
	Port        int                   `yaml:"port"`
	Env         map[string]string     `yaml:"env"`            // extra env vars
	Sync        []DevxConfigSync      `yaml:"sync,omitempty"` // file sync mappings into containers
	Dir         string                `yaml:"-"`              // Internal: working directory (set by include resolver)
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

// DevxConfig is the root devx.yaml schema.
type DevxConfig struct {
	Name      string                       `yaml:"name"`      // Project name
	Domain    string                       `yaml:"domain"`    // Custom domain (BYOD)
	Env       []string                     `yaml:"env"`       // Vault sources for secret injection
	Include   []DevxConfigInclude          `yaml:"include"`   // External devx.yaml files to compose (Idea 44)
	Tunnels   []DevxConfigTunnel           `yaml:"tunnels"`   // List of ports to expose
	Databases []DevxConfigDatabase         `yaml:"databases"` // List of databases to provision
	Services  []DevxConfigService          `yaml:"services"`  // List of applications to orchestrate
	Test      DevxConfigTest               `yaml:"test"`      // Test configuration
	Mocks     []DevxConfigMock             `yaml:"mocks"`     // List of OpenAPI mock servers to provision
	Profiles  map[string]DevxConfigProfile `yaml:"profiles"`  // Named environment overlays
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

	return cfg, nil
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
