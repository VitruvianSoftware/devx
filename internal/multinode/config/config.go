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

// Package config handles loading, parsing, and validating the multi-node cluster
// configuration from YAML files.
package config

import (
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/config"
	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for a multi-node cluster.
type Config struct {
	Cluster ClusterConfig `yaml:"cluster"`
	Nodes   []NodeConfig  `yaml:"nodes"`
}

// ClusterConfig holds cluster-wide settings.
type ClusterConfig struct {
	Name       string          `yaml:"name"`
	K3sVersion string          `yaml:"k3sVersion"`
	Kubeconfig string          `yaml:"kubeconfig"`
	MetalLB    MetalLBConfig   `yaml:"metallb"`
	Cilium     CiliumConfig    `yaml:"cilium"`
	Tailscale  TailscaleConfig `yaml:"tailscale"`
	Docker     DockerConfig    `yaml:"docker"`
	Mounts     []MountConfig   `yaml:"mounts"`
	USB        USBConfig       `yaml:"usb"`
}

// USBConfig holds defaults for `devx cluster usb`, which builds a GPT Fedora
// CoreOS image (via coreos-installer) that boots a bare-metal laptop into a
// self-joining node. All fields are optional; command-line flags override them.
type USBConfig struct {
	Renderers        []string   `yaml:"renderers,omitempty"`        // subset of fcos|baked; empty = all
	NodeNamePrefix   string     `yaml:"nodeNamePrefix,omitempty"`   // default "usb"
	Pool             string     `yaml:"pool,omitempty"`             // node pool label; default "usb"
	AgentToken       string     `yaml:"agentToken,omitempty"`       // dedicated least-privilege k3s agent token
	TailnetServerURL string     `yaml:"tailnetServerURL,omitempty"` // explicit tailnet API URL override
	Scratch          string     `yaml:"scratch,omitempty"`          // none|existing:<src>|free-space
	NodeTTL          string     `yaml:"nodeTTL,omitempty"`          // e.g. "30m"; prune NotReady ephemeral nodes older than this
	WiFi             WiFiConfig `yaml:"wifi,omitempty"`             // optional Wi-Fi creds baked into the boot config
}

// WiFiConfig carries optional Wi-Fi credentials baked into the join script, so a
// laptop with no Ethernet can still reach the cluster.
type WiFiConfig struct {
	SSID string `yaml:"ssid,omitempty"`
	PSK  string `yaml:"psk,omitempty"`
}

// TailscaleConfig holds configuration for Tailscale networking.
type TailscaleConfig struct {
	Enabled bool   `yaml:"enabled"`
	AuthKey string `yaml:"authKey"`
}

// MetalLBConfig holds configuration for the MetalLB load balancer.
type MetalLBConfig struct {
	Enabled bool   `yaml:"enabled"`
	IPRange string `yaml:"ipRange"`
	// L2Hostnames pins the L2Advertisement to a stable subset of nodes via a
	// per-hostname nodeSelector (kubernetes.io/hostname). Empty = advertise from
	// all nodes (default MetalLB behaviour). Pinning stops the speaker VIP from
	// flapping onto high-latency Mac nodes.
	L2Hostnames []string `yaml:"l2Hostnames"`
}

// DockerConfig holds configuration for Docker runtime integration.
type DockerConfig struct {
	Enabled bool `yaml:"enabled"`
	// DockerVersion and ContainerdVersion pin the node container runtime to an
	// exact, mutually-compatible apt version pair (held via apt-mark) so a node
	// restart or unattended upgrade can never pair a mismatched
	// containerd<->shim — the cause of the "unsupported protocol" pod-sandbox
	// creation failures. Empty falls back to lima.DefaultDockerVersion /
	// lima.DefaultContainerdVersion (the pin tracked in git, since cluster.yaml
	// is local). Bump the two together.
	DockerVersion     string `yaml:"dockerVersion,omitempty"`
	ContainerdVersion string `yaml:"containerdVersion,omitempty"`
}

// CiliumConfig selects the Cilium CNI (Flannel+kube-proxy+servicelb disabled in k3s).
type CiliumConfig struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version,omitempty"` // informational; helm pin lives in GitOps
}

// MountConfig describes a host directory shared into the cluster VMs so that
// containers launched by the in-VM Docker daemon (e.g. devcontainers) can see
// host source files at the same path.
//
// Default-vs-opt-out is decided by presence: an omitted `mounts:` key
// (nil slice) defaults to the host home dir; an explicit `mounts: []` opts out.
// See lima.ResolveMounts.
type MountConfig struct {
	Location   string `yaml:"location"`             // host path; "~" is allowed and expanded on the target host
	MountPoint string `yaml:"mountPoint,omitempty"` // guest path; defaults to the same path as Location
	Writable   bool   `yaml:"writable,omitempty"`   // default false; the injected home-default sets this true
}

// NodeConfig describes a single node in the cluster.
type NodeConfig struct {
	Host       string   `yaml:"host"`
	Role       string   `yaml:"role"`             // "server" or "agent"
	Pool       string   `yaml:"pool"`             //
	Kind       string   `yaml:"kind,omitempty"`   // "lima" (default, macOS+VM) | "native" (Linux host, no VM)
	NodeIP     string   `yaml:"nodeIP,omitempty"` // native: override the auto-detected tailscale IP
	VM         VMConfig `yaml:"vm"`               // required for kind=lima; ignored for kind=native
	VMName     string   `yaml:"vmName,omitempty"`
	SSHUser    string   `yaml:"sshUser,omitempty"`
	SSHPort    string   `yaml:"sshPort,omitempty"`
	SSHKeyPath string   `yaml:"sshKeyPath,omitempty"`
}

// GetKind returns the node kind, defaulting to "lima".
func (n *NodeConfig) GetKind() string {
	if n.Kind == "" {
		return "lima"
	}
	return n.Kind
}

// VMConfig describes the resource allocation for a Lima VM.
type VMConfig struct {
	CPUs   int    `yaml:"cpus"`
	Memory string `yaml:"memory"`
	Disk   string `yaml:"disk"`
}

// GetVMName returns the VM name for this node, defaulting to "k8s-node".
func (n *NodeConfig) GetVMName() string {
	if n.VMName != "" {
		return n.VMName
	}
	return "k8s-node"
}

// Load reads and parses a config file from the given path.
// It will search parent directories for the file if only a filename is provided.
func Load(path string) (*Config, error) {
	// Support CLUSTER_CONFIG env var as fallback.
	if path == "cluster.yaml" {
		if envPath := os.Getenv("CLUSTER_CONFIG"); envPath != "" {
			path = envPath
		}
	}

	// Crawl upwards to find the config if it's a simple filename
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	foundPath, foundDir, err := config.FindProjectConfig(cwd, path)
	if err == nil {
		if foundDir != cwd {
			_, _ = fmt.Fprintf(os.Stderr, "📂 Using %s from %s\n", path, foundDir)
		}
		path = foundPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// ServerNodes returns only the nodes with role "server".
func (c *Config) ServerNodes() []NodeConfig {
	var servers []NodeConfig
	for _, n := range c.Nodes {
		if n.Role == "server" {
			servers = append(servers, n)
		}
	}
	return servers
}

// AgentNodes returns only the nodes with role "agent".
func (c *Config) AgentNodes() []NodeConfig {
	var agents []NodeConfig
	for _, n := range c.Nodes {
		if n.Role == "agent" {
			agents = append(agents, n)
		}
	}
	return agents
}

// InitNode returns the first server node, which is used to bootstrap the cluster.
func (c *Config) InitNode() NodeConfig {
	return c.ServerNodes()[0]
}
