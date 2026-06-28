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

// Package projectcfg is the first-class home for locating and loading the
// project's devx.yaml. It exists so commands stop re-implementing
// find+read+unmarshal with their own ad-hoc inline structs (the schema itself
// still lives in package cmd today; this package owns the I/O + the small,
// command-agnostic views over it).
package projectcfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/VitruvianSoftware/devx/internal/config"
)

// Find locates devx.yaml by walking up from the current directory, returning its
// absolute path and the directory it was found in (error if absent).
func Find() (path, dir string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting working directory: %w", err)
	}
	return config.FindProjectConfig(cwd, "devx.yaml")
}

// Load finds devx.yaml, reads it, and unmarshals it into out — the single
// reusable entry point so commands don't each re-implement find+read+unmarshal.
func Load(out any) error {
	path, _, err := Find()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

// Settings is the small, command-agnostic slice of devx.yaml that shell-style
// commands need — the secret-source list and the AI bridge toggle — as a typed
// view, so callers don't re-declare ad-hoc inline structs.
type Settings struct {
	Env []string `yaml:"env"`
	AI  struct {
		Bridge *bool `yaml:"bridge"`
	} `yaml:"ai"`
}

// AIBridgeEnabled reports whether the AI bridge override should apply — true
// unless devx.yaml explicitly set `ai: bridge: false`.
func (s Settings) AIBridgeEnabled() bool {
	return s.AI.Bridge == nil || *s.AI.Bridge
}

// LoadSettings loads the Settings view. found is false (with no error) when there
// is no readable devx.yaml, so callers can fall back to a plain .env — matching
// the historic lenient behavior. Env defaults to ["file://.env"] when omitted.
func LoadSettings() (s Settings, found bool) {
	path, _, err := Find()
	if err != nil {
		return Settings{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return Settings{}, false
	}
	_ = yaml.Unmarshal(data, &s) // lenient: a malformed devx.yaml still falls back to defaults
	if len(s.Env) == 0 {
		s.Env = []string{"file://.env"}
	}
	return s, true
}
