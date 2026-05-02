// Package nuke provides discovery and deletion logic for the 'devx nuke' command.
// It detects project ecosystems, collects caches/build artifacts/devx resources,
// and presents a pre-flight manifest with disk sizes before performing any deletions.
package nuke

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/provider"
)

// Item represents a single resource that will be deleted by devx nuke.
type Item struct {
	Category    string // e.g. "Node.js", "Go", "devx"
	Label       string // human-readable name ("node_modules", "postgres container")
	Path        string // filesystem path, or empty for container/volume items
	SizeBytes   int64
	SizeDisplay string // pre-formatted (e.g. "1.2 GB", "container")
	Kind        string // "dir", "file", "container", "volume"
}

// Manifest collects all items scheduled for deletion.
type Manifest struct {
	Items     []Item
	TotalSize int64
	Runtime   provider.ContainerRuntime
}

// Collect scans the given project directory and the devx VM for everything
// that would be removed by nuke. It never deletes anything — only reads.
func Collect(projectDir string, runtime provider.ContainerRuntime) (*Manifest, error) {
	m := &Manifest{Runtime: runtime}

	// 1. Language-specific caches and build artefacts
	m.collectLocalFS(projectDir)

	// 2. devx-managed containers and volumes
	m.collectDevxContainers(runtime)

	// 3. Mutagen sync sessions (Idea 43)
	m.collectMutagenSessions()

	// 4. Bridge session files (Idea 46.1)
	m.collectBridgeFiles()

	return m, nil
}

// collectLocalFS walks known directory patterns relative to projectDir.
func (m *Manifest) collectLocalFS(root string) {
	type candidate struct {
		category string
		label    string
		relPath  string
	}

	candidates := []candidate{
		// Node / JS ecosystem
		{"Node.js", "node_modules", "node_modules"},
		{"Node.js", ".next (build cache)", ".next"},
		{"Node.js", ".nuxt (build cache)", ".nuxt"},
		{"Node.js", "dist/", "dist"},
		{"Node.js", "build/", "build"},
		{"Node.js", ".turbo/", ".turbo"},
		{"Node.js", ".parcel-cache/", ".parcel-cache"},
		// Go
		{"Go", "vendor/", "vendor"},
		// Python
		{"Python", ".venv/", ".venv"},
		{"Python", "venv/", "venv"},
		{"Python", ".pytest_cache/", ".pytest_cache"},
		{"Python", "__pycache__/", "__pycache__"},
		// Rust
		{"Rust", "target/", "target"},
		// Java / JVM
		{"Java", "target/ (Maven)", "target"},
		{"Java", "build/ (Gradle)", "build"},
		// General
		{"General", ".cache/", ".cache"},
		{"General", "tmp/", "tmp"},
	}

	seen := map[string]bool{}
	for _, c := range candidates {
		full := filepath.Join(root, c.relPath)
		if seen[full] {
			continue
		}
		seen[full] = true

		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}

		size := dirSize(full)
		m.Items = append(m.Items, Item{
			Category:    c.category,
			Label:       c.label,
			Path:        full,
			SizeBytes:   size,
			SizeDisplay: formatBytes(size),
			Kind:        "dir",
		})
		m.TotalSize += size
	}

	// Go module cache — global, lives in GOPATH/pkg/mod
	if gopath := goPath(); gopath != "" {
		modCache := filepath.Join(gopath, "pkg", "mod", "cache")
		if info, err := os.Stat(modCache); err == nil && info.IsDir() {
			size := dirSize(modCache)
			m.Items = append(m.Items, Item{
				Category:    "Go",
				Label:       "module cache (GOPATH/pkg/mod/cache)",
				Path:        modCache,
				SizeBytes:   size,
				SizeDisplay: formatBytes(size),
				Kind:        "dir",
			})
			m.TotalSize += size
		}
	}

	// Go build cache — go env GOCACHE
	if gocache := goEnv("GOCACHE"); gocache != "" {
		if info, err := os.Stat(gocache); err == nil && info.IsDir() {
			size := dirSize(gocache)
			m.Items = append(m.Items, Item{
				Category:    "Go",
				Label:       "build cache (GOCACHE)",
				Path:        gocache,
				SizeBytes:   size,
				SizeDisplay: formatBytes(size),
				Kind:        "dir",
			})
			m.TotalSize += size
		}
	}
}

// collectDevxContainers lists all devx-managed containers and volumes.
func (m *Manifest) collectDevxContainers(runtime provider.ContainerRuntime) {
	// Containers
	out, err := runtime.Exec("ps", "-a",
		"--filter", "label=managed-by=devx",
		"--format", "{{.Names}}\t{{.Status}}",
	)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) < 1 || parts[0] == "" {
				continue
			}
			m.Items = append(m.Items, Item{
				Category:    "devx",
				Label:       parts[0],
				SizeDisplay: "container",
				Kind:        "container",
			})
		}
	}

	// Volumes
	out, err = runtime.Exec("volume", "ls",
		"--filter", "label=managed-by=devx",
		"--format", "{{.Name}}",
	)
	if err == nil {
		for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			m.Items = append(m.Items, Item{
				Category:    "devx",
				Label:       name,
				SizeDisplay: "volume",
				Kind:        "volume",
			})
		}
	}
}

// collectMutagenSessions discovers active devx-managed Mutagen sync sessions.
func (m *Manifest) collectMutagenSessions() {
	if _, err := exec.LookPath("mutagen"); err != nil {
		return // mutagen not installed — nothing to collect
	}

	out, err := exec.Command("mutagen", "sync", "list", "--label-selector", "managed-by=devx").CombinedOutput()
	if err != nil {
		return // no sessions or mutagen error
	}

	// Parse session names from output
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name: ") {
			name := strings.TrimPrefix(line, "Name: ")
			m.Items = append(m.Items, Item{
				Category:    "sync",
				Label:       name,
				SizeDisplay: "mutagen session",
				Kind:        "sync",
			})
		}
	}
}

// collectBridgeFiles discovers bridge session state files (Idea 46.1).
func (m *Manifest) collectBridgeFiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	bridgeFiles := []struct {
		name string
		path string
	}{
		{"bridge session (bridge.json)", filepath.Join(home, ".devx", "bridge.json")},
		{"bridge env vars (bridge.env)", filepath.Join(home, ".devx", "bridge.env")},
	}

	for _, bf := range bridgeFiles {
		info, err := os.Stat(bf.path)
		if err != nil {
			continue
		}
		m.Items = append(m.Items, Item{
			Category:    "bridge",
			Label:       bf.name,
			Path:        bf.path,
			SizeBytes:   info.Size(),
			SizeDisplay: formatBytes(info.Size()),
			Kind:        "file",
		})
		m.TotalSize += info.Size()
	}
}

// Execute performs the actual deletion of all items in the manifest.
// Each deletion is reported via the progress callback.
func (m *Manifest) Execute(progress func(item Item, err error)) {
	for _, item := range m.Items {
		var err error
		switch item.Kind {
		case "dir", "file":
			err = os.RemoveAll(item.Path)
		case "container":
			_, _ = m.Runtime.Exec("stop", item.Label)
			_, err = m.Runtime.Exec("rm", "-f", item.Label)
		case "volume":
			_, err = m.Runtime.Exec("volume", "rm", "-f", item.Label)
		case "sync":
			err = exec.Command("mutagen", "sync", "terminate", item.Label).Run()
		}
		progress(item, err)
	}
}

// dirSize returns the total size in bytes of a directory tree.
// Returns 0 if the path is not accessible.
func dirSize(path string) int64 {
	var size int64
	// Use du for speed — os.Walk on node_modules with 200k files is very slow
	out, err := exec.Command("du", "-sk", path).Output()
	if err == nil {
		var kb int64
		fmt.Sscanf(string(out), "%d", &kb) //nolint:errcheck
		return kb * 1024
	}
	// Fallback: filepath.Walk
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatBytes(b int64) string {
	return FormatBytes(b)
}

// FormatBytes formats a byte count as a human-readable string (exported for cmd use).
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func goPath() string {
	gp := os.Getenv("GOPATH")
	if gp != "" {
		return gp
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go")
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
