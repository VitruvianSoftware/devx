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

package config

import (
	"fmt"
	"strings"
)

// Validate checks that the config is well-formed and internally consistent.
func (c *Config) Validate() error {
	var errs []string

	if c.Cluster.Name == "" {
		errs = append(errs, "cluster.name is required")
	}

	if len(c.Nodes) == 0 {
		errs = append(errs, "at least one node must be defined")
	}

	servers := c.ServerNodes()
	if len(servers) == 0 {
		errs = append(errs, "at least one server node is required")
	}

	if len(servers) > 1 && len(servers)%2 == 0 {
		errs = append(errs, fmt.Sprintf(
			"HA requires an odd number of server nodes for etcd quorum, got %d",
			len(servers),
		))
	}

	hosts := make(map[string]bool)
	pools := make(map[string]bool)
	for i, n := range c.Nodes {
		if n.Host == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].host is required", i))
		}
		if n.Role != "server" && n.Role != "agent" {
			errs = append(errs, fmt.Sprintf("nodes[%d].role must be 'server' or 'agent', got %q", i, n.Role))
		}
		if n.Pool == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].pool is required", i))
		}
		if n.VM.CPUs < 1 {
			errs = append(errs, fmt.Sprintf("nodes[%d].vm.cpus must be >= 1", i))
		}
		if n.VM.Memory == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].vm.memory is required", i))
		}
		if n.VM.Disk == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].vm.disk is required", i))
		}

		if hosts[n.Host] {
			errs = append(errs, fmt.Sprintf("duplicate host: %q", n.Host))
		}
		hosts[n.Host] = true

		if pools[n.Pool] {
			errs = append(errs, fmt.Sprintf("duplicate pool label: %q", n.Pool))
		}
		pools[n.Pool] = true
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
