# Github

## Overview
Component package providing implementation for `apps/cli/devx/internal/github`.

## Interface
- `[pages.go:EnablePages]`
- `[pages.go:RepoInfo]`
- `[pages.go:PagesEndpoint]`
- `[pages.go:SetCustomDomain]`
- `[pages.go:DomainNotVerifiedError]`
- `[pages.go:Error]`
- `[pages.go:PagesStatus]`
- `[pages.go:HTTPSCert]`
- `[pages.go:GetPagesStatus]`

## Invariants
- None

## Side Effects
- [pages.go:26]: `"os/exec"`
- [pages.go:33]: `out, err := exec.Command(`
- [pages.go:51]: `out, err := exec.Command("gh", "repo", "view", "--json", "owner,name", "-q", ...`

## Verification
- Run: `bazel test //apps/cli/devx/internal/github/...`
