# Webhook Catcher (`devx webhook catch`)

`devx webhook catch` starts a local HTTP server that accepts every request your application fires and displays it in a beautiful live terminal UI — complete with per-method colour coding, signature header extraction, and pretty-printed JSON payloads. No external RequestBin account needed.

## Quick Start

```bash
devx webhook catch
```

```
devx webhook catch

  Set in your app's .env:

    WEBHOOK_URL=http://localhost:9999

  Starting TUI... (q to quit)
```

The TUI opens immediately. Point your app's webhook URL at `localhost:9999` and every request appears live:

```
devx webhook catch  │  q to quit

  Local:   http://localhost:9999

  2 request(s) captured

  #1   23:14:08.231   POST     /stripe/events               2ms
       Stripe-Signature:         t=1234,v1=abc123...
       Content-Type:             application/json
       {
         "type":                 "payment_intent.succeeded",
         "data": {
           "object": {
             "id":               "pi_abc123",
             "amount":           4200
           }
         }
       }
  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌

  #2   23:14:09.445   POST     /github/push                 1ms
       X-Hub-Signature:          sha1=abc123
       X-GitHub-Event:           push
       {
         "ref":                  "refs/heads/main",
         "repository": {
           "full_name":          "org/repo"
         }
       }
  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
```

## Public URL via Cloudflare Tunnel

Use `--expose` to get a public HTTPS URL for services that can't reach `localhost` (e.g. Stripe's webhook dashboard, GitHub webhooks):

```bash
devx webhook catch --expose
```

```
⣾ Starting Cloudflare tunnel for http://localhost:9999...

  Set in your app's .env:

    WEBHOOK_URL=http://localhost:9999
    WEBHOOK_URL=https://some-slug.trycloudflare.com   (public)
```

Paste the `https://` URL into Stripe/GitHub/Twilio's webhook configuration — all traffic is forwarded to your local catcher.

## Connecting Your Application

Set `WEBHOOK_URL` (or whatever your framework calls it) to the local address:

::: code-group

```env [.env]
WEBHOOK_URL=http://localhost:9999

# For specific services, use their expected var name:
STRIPE_WEBHOOK_URL=http://localhost:9999
GITHUB_WEBHOOK_URL=http://localhost:9999
```

```python [Python (requests)]
import os, requests

# Instead of real Stripe endpoint, fire at the catcher
requests.post(
    os.environ["WEBHOOK_URL"] + "/my-handler",
    json={"event": "order.paid", "order_id": 42},
    headers={"X-Hook-Secret": "abc123"},
)
```

```go [Go]
http.Post(os.Getenv("WEBHOOK_URL") + "/events",
    "application/json",
    strings.NewReader(`{"event":"order.paid"}`),
)
```

:::

## CI / jq Integration

When not connected to an interactive terminal (or with `--json`), output streams as JSON lines — perfect for piping to `jq`:

```bash
# Run catcher in background, pipe output to jq
devx webhook catch --json 2>/dev/null | jq '.method + " " + .path'

# Result:
"POST /stripe/events"
"POST /github/push"
```

```bash
# Assert the right payload in a test script
PAYLOAD=$(devx webhook catch --json --port 9998 2>/dev/null | head -1 | jq -r '.body')
echo $PAYLOAD | jq '.type'  # "payment_intent.succeeded"
```

## Signature Header Inspection

`devx webhook catch` surfaces webhook signature headers prominently so you can debug HMAC verification:

| Service | Header shown |
|---|---|
| **Stripe** | `Stripe-Signature` |
| **GitHub** | `X-Hub-Signature-256`, `X-GitHub-Event` |
| **Twilio** | `X-Twilio-Signature` |
| **Generic** | `Authorization`, `X-Signature`, `X-Hook` |

## Flags

| Flag | Default | Description |
|---|---|---|
| `-p, --port` | `9999` | Local port to listen on |
| `--expose` | `false` | Wrap in Cloudflare tunnel for a public HTTPS URL |
| `--json` | `false` | Output JSON lines instead of the TUI |

## The Big Picture

```
Your App ──POST──▶ http://localhost:9999 ──▶ devx webhook catch (TUI)
                           ▲
           Cloudflare Tunnel (--expose)
                           │
     Stripe / GitHub / 3rd-party service
```

| Feature | `devx webhook catch` | RequestBin |
|---|---|---|
| Setup | None (built in) | Account + browser tab |
| Local access | ✓ | ✗ |
| Public URL | ✓ (`--expose`) | ✓ |
| JSON streaming | ✓ (`--json`) | ✗ |
| Signature headers | ✓ | ✗ |
| Works offline | ✓ | ✗ |
