package util

import (
	"encoding/base64"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/remote"
)

// Base64Encode encodes a string to base64.
func Base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// NewRunner creates a new remote command runner for the given node config.
func NewRunner(node config.NodeConfig) *remote.Runner {
	if node.SSHUser != "" || node.SSHPort != "" || node.SSHKeyPath != "" {
		return remote.NewRunnerWithOpts(node.Host, node.SSHUser, node.SSHPort, node.SSHKeyPath)
	}
	return remote.NewRunner(node.Host)
}
