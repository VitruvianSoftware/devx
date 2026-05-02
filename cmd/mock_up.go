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

package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	devxmock "github.com/VitruvianSoftware/devx/internal/mock"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	mockUpRuntime string
)

var mockUpCmd = &cobra.Command{
	Use:   "up [name...]",
	Short: "Start mock servers defined in devx.yaml (or specify names inline)",
	Long: `Start persistent Prism mock server containers for all mocks declared in devx.yaml.
If one or more names are provided, only those mocks are started.

Examples:

  # Start all mocks from devx.yaml
  devx mock up

  # Start only the stripe mock
  devx mock up stripe

  # Start an ad-hoc mock without devx.yaml (requires --url)
  devx mock up stripe --url https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml`,
	RunE: runMockUp,
}

var mockUpURL string
var mockUpPort int

func init() {
	mockUpCmd.Flags().StringVar(&mockUpRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	mockUpCmd.Flags().StringVar(&mockUpURL, "url", "",
		"OpenAPI spec URL (only required for ad-hoc mocks not defined in devx.yaml)")
	mockUpCmd.Flags().IntVar(&mockUpPort, "port", 0,
		"Host port to bind (defaults to next available free port)")
	mockCmd.AddCommand(mockUpCmd)
}

func runMockUp(cmd *cobra.Command, args []string) error {
	runtime := mockUpRuntime
	if runtime != "podman" && runtime != "docker" {
		return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
	}

	// Load devx.yaml
	var cfg DevxConfig
	if b, err := os.ReadFile("devx.yaml"); err == nil {
		_ = yaml.Unmarshal(b, &cfg)
	}

	// Determine which mocks to boot
	var targets []DevxConfigMock

	if mockUpURL != "" {
		// Ad-hoc single mock from CLI flags
		if len(args) == 0 {
			return fmt.Errorf("a name argument is required when using --url (e.g. devx mock up stripe --url ...)")
		}
		targets = append(targets, DevxConfigMock{Name: args[0], URL: mockUpURL, Port: mockUpPort})
	} else if len(args) > 0 {
		// Filter YAML mocks by the specified names
		wantSet := make(map[string]bool, len(args))
		for _, a := range args {
			wantSet[strings.ToLower(a)] = true
		}
		for _, m := range cfg.Mocks {
			if wantSet[strings.ToLower(m.Name)] {
				targets = append(targets, m)
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no mocks matching %v found in devx.yaml", args)
		}
	} else {
		// Boot all mocks from devx.yaml
		targets = cfg.Mocks
	}

	if len(targets) == 0 {
		return fmt.Errorf("no mocks defined — add entries to devx.yaml or use --url")
	}

	for _, m := range targets {
		srv, err := devxmock.Up(runtime, m.Name, m.URL, m.Port)
		if err != nil {
			return err
		}

		fmt.Printf("⏳ Waiting for mock %q to become ready...\n", m.Name)
		if err := devxmock.WaitForReady(srv.HostPort, 30*time.Second); err != nil {
			return err
		}

		key := devxmock.EnvKey(m.Name)
		fmt.Printf("✅ Mock %q is live!\n", m.Name)
		fmt.Printf("   Spec:   %s\n", m.URL)
		fmt.Printf("   URL:    http://localhost:%d\n", srv.HostPort)
		fmt.Printf("   Inject: %s=http://localhost:%d\n\n", key, srv.HostPort)
	}
	return nil
}
