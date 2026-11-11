# Cloudflare

## Overview
Component package providing implementation for `apps/cli/devx/internal/cloudflare`.

## Interface
- `[client.go:Tunnel]`
- `[client.go:CheckLogin]`
- `[client.go:EnsureTunnel]`
- `[client.go:GetToken]`
- `[client.go:RouteDNS]`
- `[client.go:TunnelStatus]`
- `[client.go:CleanupExposedTunnels]`
- `[client.go:ListExposedTunnels]`
- `[client.go:WriteIngressConfig]`
- `[client.go:IngressEntry]`
- `[client.go:WriteMultiIngressConfig]`
- `[dns.go:LookupZoneID]`
- `[dns.go:CreateCNAME]`
- `[dns.go:CreateTXT]`

## Invariants
- None

## Side Effects
- [client.go:27]: `"os/exec"`
- [client.go:65]: `out, err := exec.Command("cloudflared", "tunnel", "create", name).CombinedOut...`
- [client.go:86]: `out, err := exec.Command("cloudflared", "tunnel", "token", tunnelName).Output()`
- [dns.go:28]: `"net/http"`
- [dns.go:86]: `req, err := http.NewRequest(method, cfAPIBase+path, reqBody)`
- [dns.go:93]: `resp, err := http.DefaultClient.Do(req)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/cloudflare/...`
