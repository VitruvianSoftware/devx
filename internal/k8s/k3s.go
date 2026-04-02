// Package k8s manages the local zero-config Kubernetes clusters using k3s.
package k8s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	k3sImage      = "docker.io/rancher/k3s:v1.30.0-k3s1"
	containerBase = "devx-k8s"
	labelManaged  = "managed-by=devx"
	labelK8s      = "devx-k8s=true"
)

type Cluster struct {
	Name          string `json:"name"`
	ContainerName string `json:"container"`
	Port          int    `json:"port"`
	ConfigPath    string `json:"config_path"`
	Status        string `json:"status"`
}

// Spawn boots a new k3s container and provisions its isolated kubeconfig.
func Spawn(runtime, name string) (*Cluster, error) {
	containerName := fmt.Sprintf("%s-%s", containerBase, name)

	if running, _ := isRunning(runtime, containerName); running {
		port, _ := containerPort(runtime, containerName)
		fmt.Printf("✓ Cluster %q is already running on port %d\n", name, port)
		return &Cluster{
			Name:          name,
			ContainerName: containerName,
			Port:          port,
			ConfigPath:    configPath(name),
			Status:        "Up",
		}, nil
	}

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("could not acquire free port: %w", err)
	}

	fmt.Printf("🚀 Spawning k3s cluster %q on port %d...\n", name, port)

	runArgs := []string{
		"run", "-d", "--privileged",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:6443", port),
		"--label", labelManaged,
		"--label", labelK8s,
		"--label", fmt.Sprintf("devx-k8s-name=%s", name),
		k3sImage,
		"server", "--tls-san", "127.0.0.1",
	}

	var stderrBuf bytes.Buffer
	cmd := exec.Command(runtime, runArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to spawn cluster %q: %w\n%s", name, err, stderrBuf.String())
	}

	fmt.Printf("⏳ Waiting for API server to generate kubeconfig...\n")
	if err := extractKubeconfig(runtime, containerName, name, port); err != nil {
		return nil, err
	}

	return &Cluster{
		Name:          name,
		ContainerName: containerName,
		Port:          port,
		ConfigPath:    configPath(name),
		Status:        "Up",
	}, nil
}

// List returns all active devx k3s clusters.
func List(runtime string) ([]Cluster, error) {
	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "label="+labelK8s,
		"--format", "{{json .}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list k8s clusters: %w", err)
	}

	var clusters []Cluster
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		cname, _ := raw["Names"].(string)
		status, _ := raw["Status"].(string)

		labels, _ := raw["Labels"].(string)
		name := extractLabel(labels, "devx-k8s-name")
		port, _ := containerPort(runtime, cname)

		clusters = append(clusters, Cluster{
			Name:          name,
			ContainerName: cname,
			Port:          port,
			ConfigPath:    configPath(name),
			Status:        status,
		})
	}
	return clusters, nil
}

// Remove stops the cluster and deletes the isolated config.
func Remove(runtime, name string) error {
	containerName := fmt.Sprintf("%s-%s", containerBase, name)
	fmt.Printf("🗑️  Removing k3s cluster %q (%s)...\n", name, containerName)

	if err := exec.Command(runtime, "rm", "-f", containerName).Run(); err != nil {
		return fmt.Errorf("failed to remove cluster %q: %w", name, err)
	}

	cp := configPath(name)
	_ = os.Remove(cp)

	fmt.Printf("✓ Cluster %q and its kubeconfig removed\n", name)
	return nil
}

func extractKubeconfig(runtime, container, name string, port int) error {
	// Poll until the file exists inside the container
	deadline := time.Now().Add(60 * time.Second)
	var rawConfig []byte
	var err error

	for time.Now().Before(deadline) {
		rawConfig, err = exec.Command(runtime, "exec", container, "cat", "/etc/rancher/k3s/k3s.yaml").Output()
		if err == nil && len(rawConfig) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("timed out waiting for k3s.yaml inside cluster: %w", err)
	}

	// Rewrite standard port 6443 to the dynamically acquired host port
	configStr := string(rawConfig)
	configStr = strings.ReplaceAll(configStr, "https://127.0.0.1:6443", fmt.Sprintf("https://127.0.0.1:%d", port))

	cp := configPath(name)
	dir := filepath.Dir(cp)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create ~/.kube directory: %w", err)
	}

	if err := os.WriteFile(cp, []byte(configStr), 0600); err != nil {
		return fmt.Errorf("failed to write host kubeconfig to %s: %w", cp, err)
	}

	return nil
}

func configPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", fmt.Sprintf("devx-%s.yaml", name))
}

// --- helpers ---

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
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &port); err != nil {
		return 0, err
	}
	return port, nil
}

func extractLabel(rawLabels, key string) string {
	for _, part := range strings.Split(rawLabels, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return kv[1]
		}
	}
	return ""
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
