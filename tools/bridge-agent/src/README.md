# Src

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/tools/bridge-agent/src`.

## Interface
- `[lib.rs:PortSpec]`
- `[lib.rs:AgentFlags]`
- `[lib.rs:format_log_line]`
- `[lib.rs:log_info]`
- `[lib.rs:parse_flags]`
- `[lib.rs:parse_ports]`
- `[lib.rs:build_restore_selector_patch]`
- `[lib.rs:CommandRunner]`
- `[lib.rs:ProcessCommandRunner]`
- `[lib.rs:restore_selector_with_runner]`
- `[lib.rs:restore_selector]`
- `[lib.rs:route_health_request]`
- `[lib.rs:handle_health_conn]`
- `[lib.rs:serve_health_listener]`
- `[lib.rs:start_health_server]`
- `[lib.rs:listen_service_port_on]`
- `[lib.rs:listen_service_port]`
- `[lib.rs:YAMUX_VERSION]`
- `[lib.rs:YAMUX_TYPE_DATA]`
- `[lib.rs:YAMUX_TYPE_WINDOW_UPDATE]`

## Invariants
- None

## Side Effects
- [lib.rs:32]: `/// Parsed service port specification (`--ports=8080:http-api,9090:metrics`).`
- [lib.rs:140]: `/// Parses `"8080:http-api,9090:metrics"` into port specs, matching Go's `par...`
- [lib.rs:252]: `/// Handles a single HTTP request on the health server, returning `(status_co...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/tools/bridge-agent/src/...`
