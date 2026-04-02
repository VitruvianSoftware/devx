// Package mock provides core lifecycle management for devx OpenAPI mock servers.
// Mock servers are powered by Stoplight Prism (stoplight/prism:5) and run as
// long-lived background containers, similar to devx-managed databases.
//
// Each mock is identified by a user-defined name (e.g. "stripe") and responds
// to HTTP requests according to the remote OpenAPI spec it is initialized with.
//
// Environment variable injected by `devx mock list`:
//
//	MOCK_<NAME>_URL=http://localhost:<port>
package mock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	prismImage    = "docker.io/stoplight/prism:5"
	containerBase = "devx-mock"
	labelManaged  = "managed-by=devx"
	labelMock     = "devx-mock=true"
)

// MockServer holds metadata about a running (or desired) mock server.
type MockServer struct {
	Name          string
	SpecURL       string
	ContainerName string
	HostPort      int
}

// Up starts a Prism mock container for the given name and remote spec URL.
// If port is 0 a free port is acquired automatically.
func Up(runtime, name, specURL string, port int) (*MockServer, error) {
	if specURL == "" {
		return nil, fmt.Errorf("spec URL is required for mock %q", name)
	}
	if !strings.HasPrefix(specURL, "http://") && !strings.HasPrefix(specURL, "https://") {
		return nil, fmt.Errorf("spec URL must start with http:// or https:// (local file support is a future enhancement)")
	}

	containerName := fmt.Sprintf("%s-%s", containerBase, name)

	// If already running, report success idempotently
	if running, _ := isRunning(runtime, containerName); running {
		port, _ := containerPort(runtime, containerName)
		fmt.Printf("✓ Mock %q is already running on port %d\n", name, port)
		return &MockServer{Name: name, SpecURL: specURL, ContainerName: containerName, HostPort: port}, nil
	}

	// Acquire free port if not specified
	if port == 0 {
		var err error
		port, err = freePort()
		if err != nil {
			return nil, fmt.Errorf("could not acquire free port for mock %q: %w", name, err)
		}
	}

	fmt.Printf("🚀 Starting mock %q (Prism → %s) on port %d...\n", name, specURL, port)

	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:4010", port),
		"--label", labelManaged,
		"--label", labelMock,
		"--label", fmt.Sprintf("devx-mock-name=%s", name),
		"--label", fmt.Sprintf("devx-mock-url=%s", specURL),
		"--restart", "unless-stopped",
		prismImage,
		"mock",
		specURL,             // positional document arg must come first
		"--host", "0.0.0.0",
		"--multiprocess", "false", // workaround for Prism isPrimary crash in v4/v5
	}

	var stderrBuf bytes.Buffer
	cmd := exec.Command(runtime, runArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to start mock %q: %w\n%s", name, err, stderrBuf.String())
	}

	return &MockServer{
		Name:          name,
		SpecURL:       specURL,
		ContainerName: containerName,
		HostPort:      port,
	}, nil
}

// MockInfo describes a running devx mock container for the list command.
type MockInfo struct {
	Name          string `json:"name"`
	ContainerName string `json:"container"`
	SpecURL       string `json:"spec_url"`
	Port          int    `json:"port"`
	EnvVar        string `json:"env_var"`
	Status        string `json:"status"`
}

// List returns all running devx-managed mock containers.
func List(runtime string) ([]MockInfo, error) {
	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "label="+labelMock,
		"--format", "{{json .}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list mock containers: %w", err)
	}

	var infos []MockInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		// Inspect for labels
		cname, _ := raw["Names"].(string)
		status, _ := raw["Status"].(string)

		labels, _ := raw["Labels"].(string)
		name, specURL := parseMockLabels(labels)
		port, _ := containerPort(runtime, cname)

		key := "MOCK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_URL"
		infos = append(infos, MockInfo{
			Name:          name,
			ContainerName: cname,
			SpecURL:       specURL,
			Port:          port,
			EnvVar:        fmt.Sprintf("%s=http://localhost:%d", key, port),
			Status:        status,
		})
	}
	return infos, nil
}

// Restart stops and restarts a named mock container without removing it.
func Restart(runtime, name string) error {
	containerName := fmt.Sprintf("%s-%s", containerBase, name)
	fmt.Printf("🔄 Restarting mock %q...\n", name)
	if err := exec.Command(runtime, "restart", containerName).Run(); err != nil {
		return fmt.Errorf("failed to restart mock %q: %w", name, err)
	}
	fmt.Printf("✓ Mock %q restarted\n", name)
	return nil
}

// Remove stops and removes a named mock container.
func Remove(runtime, name string) error {
	containerName := fmt.Sprintf("%s-%s", containerBase, name)
	fmt.Printf("🗑️  Removing mock %q (%s)...\n", name, containerName)
	if err := exec.Command(runtime, "rm", "-f", containerName).Run(); err != nil {
		return fmt.Errorf("failed to remove mock %q: %w", name, err)
	}
	fmt.Printf("✓ Mock %q removed\n", name)
	return nil
}

// WaitForReady polls until Prism is actually serving HTTP responses (not just accepting TCP).
func WaitForReady(port int, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil // any HTTP response means Prism is up
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timed out waiting for mock server on port %d", port)
}

// EnvKey returns the environment variable key for the mock URL injection.
func EnvKey(name string) string {
	return "MOCK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_URL"
}

// --- internal helpers ---

func isRunning(runtime, containerName string) (bool, error) {
	out, err := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func containerPort(runtime, containerName string) (int, error) {
	out, err := exec.Command(runtime, "inspect", containerName,
		"--format", "{{range $p, $conf := .NetworkSettings.Ports}}{{(index $conf 0).HostPort}}{{end}}",
	).Output()
	if err != nil {
		return 0, err
	}
	var port int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &port)
	return port, nil
}

func parseMockLabels(rawLabels string) (name, url string) {
	for _, part := range strings.Split(rawLabels, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "devx-mock-name":
			name = kv[1]
		case "devx-mock-url":
			url = kv[1]
		}
	}
	return
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
