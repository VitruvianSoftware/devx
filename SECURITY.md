# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in devx, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email **opensource@vitruviansoftware.com** with:

1. A description of the vulnerability
2. Steps to reproduce the issue
3. Potential impact
4. Suggested fix (if any)

We will acknowledge receipt within **48 hours** and provide a detailed response
within **7 days** indicating next steps.

## Security Considerations

devx manages sensitive credentials locally:

- **Cloudflare tunnel tokens** — stored in `.env` (gitignored) and injected into the VM at boot
- **Tailscale auth keys** — used transiently during `devx vm init` and not persisted
- **SSH keys** — managed by Podman Machine, stored in `~/.ssh/`

### Best Practices

- Never commit `.env` files to version control
- Use `devx config secrets` to rotate credentials regularly
- Review the Butane template (`dev-machine.template.bu`) before provisioning to understand what runs inside the VM
- Keep `cloudflared`, `podman`, and `tailscale` updated to their latest versions
