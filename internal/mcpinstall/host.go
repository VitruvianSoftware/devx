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

// Package mcpinstall configures supported AI coding agents to use devx as
// their MCP server. Each agent host has its own config file location and
// format; the Host interface hides those differences behind a uniform
// install/uninstall/status surface.
package mcpinstall

// Scope picks where to write a host's MCP config. Project scope keeps each
// repo's tool set sandboxed and travels with the codebase; user scope is the
// only option for hosts that don't have per-project config.
type Scope int

const (
	// ScopeAuto picks per-project where supported, otherwise per-user.
	ScopeAuto Scope = iota
	// ScopeProject forces per-project config; errors if the host doesn't
	// support it.
	ScopeProject
	// ScopeUser forces per-user (global) config.
	ScopeUser
)

func (s Scope) String() string {
	switch s {
	case ScopeProject:
		return "project"
	case ScopeUser:
		return "user"
	default:
		return "auto"
	}
}

// Status describes the current state of a host's devx MCP install.
type Status struct {
	HostKey     string `json:"host"`
	DisplayName string `json:"display_name"`
	Detected    bool   `json:"detected"`
	Installed   bool   `json:"installed"`
	ConfigPath  string `json:"config_path,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Command     string `json:"command,omitempty"`
	Note        string `json:"note,omitempty"`
}

// InstallResult is returned per host after an install/uninstall action.
type InstallResult struct {
	HostKey     string `json:"host"`
	DisplayName string `json:"display_name"`
	OK          bool   `json:"ok"`
	Changed     bool   `json:"changed"`
	ConfigPath  string `json:"config_path,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Message     string `json:"message"`
}

// Options control install behavior across hosts.
type Options struct {
	// ProjectDir is the absolute path used as the root for project-scoped
	// installs. Empty means cwd at call time.
	ProjectDir string
	// DevxBinary is the absolute path to the devx binary to invoke via
	// `command`. Empty falls back to os.Executable().
	DevxBinary string
	// Scope controls per-project vs per-user when both are supported.
	Scope Scope
	// DryRun skips disk writes; just reports what would happen.
	DryRun bool
}

// Host is the contract every agent-tool adapter implements.
type Host interface {
	// Key is the stable lookup identifier (e.g. "claude-code").
	Key() string
	// DisplayName is the human-friendly name shown in tables and logs.
	DisplayName() string
	// Detect returns true if the host appears installed on this machine.
	Detect() bool
	// Status reports the current install state without making changes.
	Status(opts Options) (Status, error)
	// Install ensures the devx MCP entry exists in the host's config.
	// Idempotent: re-running with the same Options is a no-op.
	Install(opts Options) (InstallResult, error)
	// Uninstall removes the devx MCP entry. Leaves all other host
	// configuration untouched.
	Uninstall(opts Options) (InstallResult, error)
}
