# Cloudflare Tunnels

The `devx tunnel` commands give you **ngrok-like port exposure** backed by Cloudflare's global edge network. Expose any local port to the internet with a public HTTPS URL — no port forwarding, no firewall rules, no SSL certificates to manage.

## Commands

### `devx tunnel expose`

Expose a local port to the internet:

```bash
devx tunnel expose 3000 --name myapp
# → https://myapp.your-name.ipv1337.dev
```

This creates a DNS CNAME record in Cloudflare and adds an ingress rule to the tunnel config — all without restarting the tunnel.

**Flags:**

| Flag | Description |
|------|-------------|
| `--name` | Subdomain name (e.g., `myapp` → `myapp.your-name.ipv1337.dev`) |
| `--protocol` | Protocol to use (`http`, `https`, `tcp`) |

### `devx tunnel unexpose`

Clean up all exposed ports:

```bash
devx tunnel unexpose
```

This removes the DNS records and ingress rules created by `expose`.

### `devx tunnel list`

Show currently exposed ports and their public URLs:

```bash
devx tunnel list
```

```
PORT    NAME      URL
3000    myapp     https://myapp.james.ipv1337.dev
8080    api       https://api.james.ipv1337.dev
```

### `devx tunnel inspect`

Launch a live TUI to inspect and replay HTTP traffic flowing through the tunnel:

```bash
devx tunnel inspect
```

This provides a real-time view of requests, responses, and headers — useful for debugging webhooks and API integrations.

### `devx tunnel update`

Rotate Cloudflare credentials without rebuilding the VM:

```bash
devx tunnel update
```

## How It Works

1. `devx vm init` creates a persistent Cloudflare Tunnel with credentials stored inside the VM
2. The tunnel runs as a `systemd` unit (`cloudflared.service`) and starts automatically on boot
3. `devx tunnel expose` dynamically adds ingress rules and creates DNS CNAMEs via the Cloudflare API
4. Traffic flows: **Internet → Cloudflare Edge → Encrypted Tunnel → VM → Your Container**

## Use Cases

### Webhook Development

Test Stripe, GitHub, or Slack webhooks against your local server:

```bash
devx tunnel expose 3000 --name webhooks
# Configure Stripe webhook URL: https://webhooks.james.ipv1337.dev/stripe/events
```

### Sharing Work in Progress

Share a prototype with a teammate or stakeholder:

```bash
devx tunnel expose 5173 --name demo
# Send them: https://demo.james.ipv1337.dev
```

### Mobile Testing

Test your web app on a phone without being on the same network:

```bash
devx tunnel expose 3000 --name mobile-test
# Open https://mobile-test.james.ipv1337.dev on your phone
```

## Declarative Mode

Define port exposures in `devx.yaml` alongside your databases:

```yaml
tunnels:
  - port: 3000
    name: frontend
  - port: 8080
    name: api
```

Then bring everything up:

```bash
devx up
```
