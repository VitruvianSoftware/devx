# Usb

## Overview
Component package providing implementation for `apps/cli/devx/internal/multinode/usb`.

## Interface
- `[assemble_script.go:AssemblyParams]`
- `[assemble_script.go:RenderAssemblyScript]`
- `[baked.go:BakedImage]`
- `[baked.go:BakedRenderer]`
- `[baked.go:Name]`
- `[baked.go:Modes]`
- `[baked.go:Render]`
- `[build.go:BuildOptions]`
- `[build.go:PlanEntry]`
- `[build.go:BuildResult]`
- `[build.go:Build]`
- `[build.go:Add]`
- `[builder.go:ButaneVersion]`
- `[builder.go:RenderBuilderProvision]`
- `[builder.go:Builder]`
- `[builder.go:NewBuilder]`
- `[builder.go:EnsureBuilderVM]`
- `[builder.go:BuildImage]`
- `[coords.go:Coordinates]`
- `[coords.go:ResolveCoordinates]`

## Invariants
- None

## Side Effects
- [assemble_script.go:85]: `// "udevd socket missing". --net=host lets it fetch the metal image from the`
- [builder.go:100]: `defer func() { _ = os.Remove(f.Name()) }()`
- [builder.go:119]: `defer func() { _ = os.Remove(pf.Name()) }()`
- [builder.go:147]: `defer func() { _ = os.Remove(scriptF.Name()) }()`
- [fcos.go:141]: `// relabels them so SELinux (Enforcing on FCOS) can exec them.`
- [fcos.go:144]: `# devx: fetch + install Tailscale static binaries on immutable FCOS.`
- [fcos.go:148]: `curl -fL --retry 5 --retry-connrefused -o ts.tgz "https://pkgs.tailscale.com/...`
- [flash.go:26]: `"os/exec"`
- [flash.go:117]: `out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()`
- [joinspec.go:260]: `timeout 3 bash -c "exec 3<>/dev/tcp/${host}/${port}" 2>/dev/null`

## Verification
- Run: `bazel test //apps/cli/devx/internal/multinode/usb/...`
