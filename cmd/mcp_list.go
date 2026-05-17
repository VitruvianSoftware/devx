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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/mcpinstall"
	"github.com/VitruvianSoftware/devx/internal/mcpserver"
)

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported agent hosts and MCP tools",
	RunE:  runMCPList,
}

type mcpListOutput struct {
	Hosts []hostInfo `json:"hosts"`
	Tools []string   `json:"tools"`
}

type hostInfo struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Detected    bool   `json:"detected"`
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
}

func runMCPList(_ *cobra.Command, _ []string) error {
	hosts := mcpinstall.Hosts()
	out := mcpListOutput{
		Hosts: make([]hostInfo, 0, len(hosts)),
		Tools: mcpserver.AllToolNames(),
	}
	for _, h := range hosts {
		out.Hosts = append(out.Hosts, hostInfo{
			Key:         h.Key(),
			DisplayName: h.DisplayName(),
			Detected:    h.Detect(),
		})
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	fmt.Println("🤖 Supported agent hosts:")
	for _, h := range out.Hosts {
		marker := "·"
		if h.Detected {
			marker = "✓"
		}
		fmt.Printf("   %s %-14s  %s\n", marker, h.Key, h.DisplayName)
	}
	fmt.Println()
	fmt.Printf("🛠️  Registered MCP tools (%d):\n", len(out.Tools))
	for _, t := range out.Tools {
		fmt.Printf("   • %s\n", t)
	}
	return nil
}
