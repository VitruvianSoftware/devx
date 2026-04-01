# Cloud Emulators (GCS & more)

`devx cloud` spins up local GCP service emulators inside the devx VM so your application can use real GCP SDKs without touching actual cloud infrastructure during development.

## Why Emulators?

- **No credentials needed** — no GCP project, no service account JSON
- **Zero cloud costs** — everything runs on your local machine
- **Safe by default** — data only lives in memory; no accidental production writes
- **SDK compatible** — GCP client libraries automatically pick up the emulator endpoint via standard env vars

## Spawn an Emulator

```bash
devx cloud spawn gcs      # Google Cloud Storage (fake-gcs-server)
devx cloud spawn pubsub   # Google Cloud Pub/Sub
devx cloud spawn firestore # Google Cloud Firestore
```

On success, `devx cloud spawn` prints the environment variable your application needs:

```
✅ Google Cloud Storage emulator is running!

  Container: devx-cloud-gcs
  Port:      4443

  Add to your .env:

    STORAGE_EMULATOR_HOST=http://localhost:4443

  Or use 'devx shell' to have these injected automatically.
```

::: tip Auto-injection via `devx shell`
When you use `devx shell` to launch your dev container, it automatically discovers all running
`devx cloud` emulators and injects their endpoint env vars. No manual `.env` changes needed.
:::

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | emulator default | Host port to bind |
| `--runtime` | `podman` | Container runtime (`podman` or `docker`) |

## List Running Emulators

```bash
devx cloud list
```

Shows all running emulators and the env var keys they expose:

```
devx — Cloud Emulators

  Google Cloud Storage  devx-cloud-gcs  Up 5 minutes  0.0.0.0:4443->4443/tcp
    env:  STORAGE_EMULATOR_HOST  http://localhost:4443
```

## Remove an Emulator

```bash
devx cloud rm gcs
```

Stops and removes the container. Since emulators run with in-memory backends, no persistent data is left behind.

## Supported Services

| Key | Name | Port | SDK Env Var |
|-----|------|------|-------------|
| `gcs` | Google Cloud Storage | 4443 | `STORAGE_EMULATOR_HOST` |
| `pubsub` | Google Cloud Pub/Sub | 8085 | `PUBSUB_EMULATOR_HOST` |
| `firestore` | Google Cloud Firestore | 8080 | `FIRESTORE_EMULATOR_HOST` |

> **TODO:** AWS S3 emulation via MinIO (`devx cloud spawn s3`) is planned for a future release.

## Connecting from Your Code

### Go (GCS)

```go
// The GCP client library automatically reads STORAGE_EMULATOR_HOST
client, err := storage.NewClient(ctx)
```

### Node.js (GCS via @google-cloud/storage)

```js
// No code changes needed — just set STORAGE_EMULATOR_HOST in your env
const { Storage } = require('@google-cloud/storage');
const storage = new Storage();
```

### Python (GCS via google-cloud-storage)

```python
# No code changes needed when STORAGE_EMULATOR_HOST is set
from google.cloud import storage
client = storage.Client()
```
