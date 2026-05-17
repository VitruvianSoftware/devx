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

package mcpserver

import (
	"github.com/mark3labs/mcp-go/server"
)

// ServerName is the canonical name advertised to MCP hosts.
const ServerName = "devx"

// NewServer constructs a fully-registered MCPServer ready to be served over
// any transport. The version is stamped onto the InitializeResult so MCP
// hosts can display "devx vX.Y.Z" in their tool listings.
func NewServer(version string) *server.MCPServer {
	s := server.NewMCPServer(
		ServerName,
		version,
		server.WithToolCapabilities(true),
	)
	registerTools(s)
	return s
}

// AllToolNames returns the canonical tool names registered by NewServer.
// Exposed for tests and for `devx mcp serve --list-tools`.
func AllToolNames() []string {
	names := make([]string, 0, len(toolSpecs))
	for _, t := range toolSpecs {
		names = append(names, t.name)
	}
	return names
}
