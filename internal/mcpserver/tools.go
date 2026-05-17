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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// toolSpec describes a single tool: how it's exposed to MCP hosts and how it
// translates a CallToolRequest into a devx CLI invocation. Keeping the spec
// declarative makes the tool surface skimmable and trivial to extend.
type toolSpec struct {
	name        string
	description string
	readOnly    bool
	destructive bool
	options     []mcp.ToolOption // additional schema options (string/number/bool args)
	build       func(args map[string]any) ([]string, error)
}

// toolSpecs is the canonical, ordered list of devx MCP tools. Add new tools by
// appending here — no other place needs editing.
var toolSpecs = []toolSpec{
	// ── Inspection (read-only) ───────────────────────────────────────────
	{
		name:        "devx_doctor",
		description: "Runs `devx doctor` — audits prerequisites (podman, cloudflared, butane, etc.), VM health, AI providers, and configured cloud APIs. Returns the JSON diagnostic report.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"doctor", "--json"}, nil },
	},
	{
		name:        "devx_vm_status",
		description: "Returns VM health, Tailscale auth state, and Cloudflare domain status as structured JSON.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"vm", "status", "--json"}, nil },
	},
	{
		name:        "devx_db_list",
		description: "Lists running database engines (postgres, mysql, redis, mongo) with their exposed host ports and container names.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"db", "list", "--json"}, nil },
	},
	{
		name:        "devx_cloud_list",
		description: "Lists running cloud service emulators (gcs, pubsub, firestore, s3) and the env var keys they expose for SDK auto-discovery.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"cloud", "list", "--json"}, nil },
	},
	{
		name:        "devx_tunnel_list",
		description: "Lists active Cloudflare tunnels exposing localhost ports to the public internet (with their managed *.ipv1337.dev URLs).",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"tunnel", "list", "--json"}, nil },
	},
	{
		name:        "devx_config_validate",
		description: "Validates the project's devx.yaml against its env schema — confirms every required secret is present in the configured vault providers.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"config", "validate", "--json"}, nil },
	},

	// ── Provisioning (mutating, safe) ────────────────────────────────────
	{
		name:        "devx_db_spawn",
		description: "Spawns an ephemeral database engine container (postgres, mysql, redis, mongo). Idempotent — re-running with the same engine reuses the existing container.",
		options: []mcp.ToolOption{
			mcp.WithString("engine", mcp.Required(),
				mcp.Description("Database engine: postgres, mysql, redis, mongo")),
			mcp.WithNumber("port", mcp.Description("Host port to bind (defaults to engine standard).")),
		},
		build: func(args map[string]any) ([]string, error) {
			engine, err := requireString(args, "engine")
			if err != nil {
				return nil, err
			}
			out := []string{"db", "spawn", engine, "-y", "--json"}
			if port, ok := optionalInt(args, "port"); ok {
				out = append(out, "-p", strconv.Itoa(port))
			}
			return out, nil
		},
	},
	{
		name:        "devx_cloud_spawn",
		description: "Spawns a local cloud service emulator (gcs, pubsub, firestore, s3). Each emulator auto-exports the right SDK env var so client libraries pick it up without code changes.",
		options: []mcp.ToolOption{
			mcp.WithString("service", mcp.Required(),
				mcp.Description("Cloud service: gcs, pubsub, firestore, s3")),
			mcp.WithNumber("port", mcp.Description("Host port to bind (defaults to service standard).")),
		},
		build: func(args map[string]any) ([]string, error) {
			svc, err := requireString(args, "service")
			if err != nil {
				return nil, err
			}
			out := []string{"cloud", "spawn", svc}
			if port, ok := optionalInt(args, "port"); ok {
				out = append(out, "-p", strconv.Itoa(port))
			}
			return out, nil
		},
	},
	{
		name:        "devx_db_snapshot_create",
		description: "Creates a named snapshot of a running database engine's volume. Snapshots are instant (volume export, no SQL dump) and stored under the devx VM's snapshots directory.",
		options: []mcp.ToolOption{
			mcp.WithString("engine", mcp.Required(),
				mcp.Description("Database engine to snapshot")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Snapshot name — used to restore later")),
		},
		build: func(args map[string]any) ([]string, error) {
			engine, err := requireString(args, "engine")
			if err != nil {
				return nil, err
			}
			name, err := requireString(args, "name")
			if err != nil {
				return nil, err
			}
			return []string{"db", "snapshot", "create", engine, name, "-y", "--json"}, nil
		},
	},
	{
		name:        "devx_db_snapshot_restore",
		description: "Restores a database engine from a named snapshot. Replaces the current data volume — use db_snapshot_create first if you want to preserve current state.",
		destructive: true,
		options: []mcp.ToolOption{
			mcp.WithString("engine", mcp.Required(),
				mcp.Description("Database engine to restore")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Snapshot name to restore from")),
		},
		build: func(args map[string]any) ([]string, error) {
			engine, err := requireString(args, "engine")
			if err != nil {
				return nil, err
			}
			name, err := requireString(args, "name")
			if err != nil {
				return nil, err
			}
			return []string{"db", "snapshot", "restore", engine, name, "-y", "--json"}, nil
		},
	},
	{
		name:        "devx_state_share",
		description: "Bundles running containers + database volumes into an encrypted artifact, uploads to the configured S3/GCS relay, and returns a passphrase-tagged URI a teammate can paste into devx_state_attach to reproduce your exact environment.",
		options: []mcp.ToolOption{
			mcp.WithBoolean("db_only",
				mcp.Description("Skip CRIU container checkpointing; export database volumes only.")),
		},
		build: func(args map[string]any) ([]string, error) {
			out := []string{"state", "share", "--json"}
			if dbOnly, ok := args["db_only"].(bool); ok && dbOnly {
				out = append(out, "--db-only")
			}
			return out, nil
		},
	},
	{
		name:        "devx_state_attach",
		description: "Downloads, decrypts, and restores a state bundle shared by a teammate via devx_state_share. Destructive — overwrites local containers and database volumes to match the shared state.",
		destructive: true,
		options: []mcp.ToolOption{
			mcp.WithString("id", mcp.Required(),
				mcp.Description("State share ID (e.g. 's3://bucket/path.encrypted:fast-blue-rabbit-dawn')")),
		},
		build: func(args map[string]any) ([]string, error) {
			id, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			return []string{"state", "attach", id, "-y", "--json"}, nil
		},
	},
	{
		name:        "devx_ci_run",
		description: "Executes a GitHub Actions workflow file locally inside containers. Provides ~80% parity with GitHub Actions; uses: actions are skipped with a warning.",
		options: []mcp.ToolOption{
			mcp.WithString("workflow", mcp.Description("Workflow file (e.g. 'ci.yml'). Auto-discovers if omitted.")),
			mcp.WithString("job", mcp.Description("Run only the specified job name (comma-separate for multiple).")),
			mcp.WithBoolean("dry_run", mcp.Description("Print the execution plan without creating containers.")),
		},
		build: func(args map[string]any) ([]string, error) {
			out := []string{"ci", "run", "-y", "--json"}
			if wf, ok := args["workflow"].(string); ok && wf != "" {
				out = append(out, wf)
			}
			if job, ok := args["job"].(string); ok && job != "" {
				out = append(out, "--job", job)
			}
			if dryRun, ok := args["dry_run"].(bool); ok && dryRun {
				out = append(out, "--dry-run")
			}
			return out, nil
		},
	},
	{
		name:        "devx_config_pull",
		description: "Pulls secrets from configured vault providers (1Password, Bitwarden, GCP Secret Manager, .env file) and prints them as `export VAR=value` lines. Useful to inspect what would be injected into a devx shell session.",
		readOnly:    true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"config", "pull"}, nil },
	},
	{
		name:        "devx_ship",
		description: "Runs the full ship lifecycle: pre-flight checks (lint, test, build) → commit → push → PR → merge → CI wait. Use this only when ready to ship a change to main. Destructive: pushes to remote and merges to the base branch.",
		destructive: true,
		options: []mcp.ToolOption{
			mcp.WithString("message", mcp.Required(),
				mcp.Description("Commit message. Use Conventional Commits style (feat:, fix:, docs:, etc.).")),
			mcp.WithString("base", mcp.Description("Base branch (default: main).")),
		},
		build: func(args map[string]any) ([]string, error) {
			msg, err := requireString(args, "message")
			if err != nil {
				return nil, err
			}
			out := []string{"agent", "ship", "-m", msg}
			if base, ok := args["base"].(string); ok && base != "" {
				out = append(out, "--base", base)
			}
			return out, nil
		},
	},

	// ── Destructive (with explicit hints; hosts should prompt) ───────────
	{
		name:        "devx_db_rm",
		description: "Stops and removes a database engine container (data volume retained unless --purge).",
		destructive: true,
		options: []mcp.ToolOption{
			mcp.WithString("engine", mcp.Required(),
				mcp.Description("Database engine to remove")),
			mcp.WithBoolean("purge", mcp.Description("Also delete the persistent data volume (irreversible).")),
		},
		build: func(args map[string]any) ([]string, error) {
			engine, err := requireString(args, "engine")
			if err != nil {
				return nil, err
			}
			out := []string{"db", "rm", engine, "-y"}
			if purge, ok := args["purge"].(bool); ok && purge {
				out = append(out, "--purge")
			}
			return out, nil
		},
	},
	{
		name:        "devx_cloud_rm",
		description: "Stops and removes a cloud emulator container. Since emulators use in-memory backends, no persistent data is lost.",
		destructive: true,
		options: []mcp.ToolOption{
			mcp.WithString("service", mcp.Required(),
				mcp.Description("Cloud service to remove (gcs, pubsub, firestore, s3)")),
		},
		build: func(args map[string]any) ([]string, error) {
			svc, err := requireString(args, "service")
			if err != nil {
				return nil, err
			}
			return []string{"cloud", "rm", svc, "-y"}, nil
		},
	},
	{
		name:        "devx_vm_teardown",
		description: "Tears down the devx VM and all containers running inside it. Bring everything back with `devx vm init`.",
		destructive: true,
		build:       func(_ map[string]any) ([]string, error) { return []string{"vm", "teardown", "-y"}, nil },
	},
}

// registerTools wires every spec onto the MCPServer with the correct
// annotations and a shared subprocess handler.
func registerTools(s *server.MCPServer) {
	for _, spec := range toolSpecs {
		spec := spec // capture for closure
		opts := []mcp.ToolOption{
			mcp.WithDescription(spec.description),
			mcp.WithReadOnlyHintAnnotation(spec.readOnly),
			mcp.WithDestructiveHintAnnotation(spec.destructive),
			// Most devx commands are idempotent (re-running spawn or
			// snapshot_create with the same name reuses or no-ops).
			mcp.WithIdempotentHintAnnotation(!spec.destructive),
		}
		opts = append(opts, spec.options...)
		s.AddTool(mcp.NewTool(spec.name, opts...), makeHandler(spec))
	}
}

func makeHandler(spec toolSpec) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		cliArgs, err := spec.build(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		stdout, err := runDevx(ctx, cliArgs...)
		if err != nil {
			// Surface stdout too — devx often emits partial JSON before
			// failure, which gives the agent context for retries.
			if strings.TrimSpace(stdout) != "" {
				return mcp.NewToolResultError(fmt.Sprintf("%v\n\nstdout:\n%s", err, stdout)), nil
			}
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(stdout), nil
	}
}

func requireString(args map[string]any, key string) (string, error) {
	v, ok := args[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("missing required string argument %q", key)
	}
	return v, nil
}

func optionalInt(args map[string]any, key string) (int, bool) {
	if v, ok := args[key].(float64); ok {
		return int(v), true
	}
	if v, ok := args[key].(int); ok {
		return v, true
	}
	return 0, false
}
