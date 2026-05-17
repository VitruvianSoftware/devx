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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var mcpDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate the devx MCP server starts and lists tools correctly",
	Long: `Spawns 'devx mcp serve' in a subprocess, sends an initialize and tools/list
request over its stdio transport, and confirms the expected tool set comes
back. Useful right after 'devx mcp install' to catch problems before opening
your agent.`,
	RunE: runMCPDoctor,
}

type mcpDoctorResult struct {
	OK         bool     `json:"ok"`
	ToolCount  int      `json:"tool_count"`
	Tools      []string `json:"tools,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	Error      string   `json:"error,omitempty"`
}

func init() {
	mcpCmd.AddCommand(mcpDoctorCmd)
}

func runMCPDoctor(_ *cobra.Command, _ []string) error {
	start := time.Now()
	tools, err := probeMCPServer(context.Background())
	res := mcpDoctorResult{
		Tools:      tools,
		ToolCount:  len(tools),
		DurationMs: time.Since(start).Milliseconds(),
		OK:         err == nil,
	}
	if err != nil {
		res.Error = err.Error()
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(res)
	}

	if res.OK {
		fmt.Printf("✅ devx MCP server healthy — %d tools advertised in %dms\n",
			res.ToolCount, res.DurationMs)
	} else {
		fmt.Printf("❌ devx MCP server failed: %s\n", res.Error)
		return err
	}
	return nil
}

// probeMCPServer launches devx mcp serve, performs the MCP handshake, calls
// tools/list, and returns the advertised tool names. Errors are surfaced
// verbatim so the user sees exactly what broke.
func probeMCPServer(ctx context.Context) ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolving devx binary: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, exe, "mcp", "serve")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp serve: %w", err)
	}

	// Minimal JSON-RPC 2.0 init then tools/list. mcp-go's stdio server is
	// line-delimited so each request gets a newline.
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"devx-doctor","version":"1.0"}}}` + "\n",
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n",
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n",
	}
	for _, r := range requests {
		if _, err := stdin.Write([]byte(r)); err != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("write request: %w", err)
		}
	}
	_ = stdin.Close()

	// Wait either for the process to settle or for the timeout. The server
	// keeps stdin open by default; closing stdin should make ServeStdio
	// return EOF and exit cleanly.
	waitErr := cmd.Wait()
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return nil, fmt.Errorf("mcp serve exited: %w\nstderr: %s", waitErr, stderr.String())
		}
		// Non-zero exit codes are tolerated here — closing stdin can cause
		// SIGPIPE-style returns. What matters is whether tools/list got a
		// well-formed response.
	}

	tools, err := extractToolNamesFromStream(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%w\nstderr: %s", err, stderr.String())
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("server returned no tools\nstderr: %s", stderr.String())
	}
	return tools, nil
}

// extractToolNamesFromStream walks line-delimited JSON-RPC responses and
// returns the tool names from the first tools/list result it finds.
func extractToolNamesFromStream(data []byte) ([]string, error) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if len(msg.Result.Tools) == 0 {
			continue
		}
		out := make([]string, 0, len(msg.Result.Tools))
		for _, t := range msg.Result.Tools {
			out = append(out, t.Name)
		}
		return out, nil
	}
	return nil, fmt.Errorf("no tools/list response found in server output")
}
