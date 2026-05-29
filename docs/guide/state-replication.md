# Peer-to-Peer State Replication

Devx enables you to bundle your exact running environment state and share it with teammates to eliminate "it works on my machine" debugging friction. 

By using `devx state share` and `devx state attach`, you can snapshot running containers, exported database volumes, and environment metadata into a single encrypted artifact and securely transfer it via your own S3 or Google Cloud Storage buckets.

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of `devx state share` and `devx state attach`.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph SenderHost ["Sender Host"]
        senderCLI["devx CLI (sender)"]
        criuExport["CRIU Checkpointer"]
        dbExport["Database Volume Exporter"]
        bundler["Bundler (tar.gz)"]
        encryptor["AES-256-GCM Encryptor"]
        passGen["Passphrase Generator (4-word)"]
        uploader["Upload Agent (aws / gcloud CLI)"]
    end

    subgraph Relay ["Cloud Relay (Bring-Your-Own-Bucket)"]
        bucket["S3 / GCS Bucket"]
    end

    subgraph ReceiverHost ["Receiver Host"]
        receiverCLI["devx CLI (receiver)"]
        downloader["Download Agent (aws / gcloud CLI)"]
        decryptor["AES-256-GCM Decryptor"]
        unbundler["Unbundler (tar.gz)"]
        restorer["Topology Restorer"]
    end

    senderCLI -->|"devx state share"| criuExport
    senderCLI -->|"devx state share"| dbExport
    criuExport -->|"Container memory & FS state"| bundler
    dbExport -->|"Volume .tar archives"| bundler
    bundler -->|"Compressed bundle"| encryptor
    passGen -->|"PBKDF2-derived 32-byte key"| encryptor
    encryptor -->|"Encrypted blob"| uploader
    uploader -->|"Upload ciphertext"| bucket

    receiverCLI -->|"devx state attach"| downloader
    downloader -->|"Download ciphertext"| bucket
    bucket -->|"Encrypted blob"| downloader
    downloader --> decryptor
    decryptor -->|"Passphrase via PBKDF2"| unbundler
    unbundler -->|"Extracted archives"| restorer
    restorer -->|"Restore containers & volumes"| receiverRuntime["Container Runtime (Podman)"]
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["devx state &lt;subcommand&gt;"]) --> Route{Which subcommand?}

    Route -->|"share"| CheckCRIU{"Provider is Podman?"}
    CheckCRIU -->|Yes| FullShare["Checkpoint containers via CRIU"]
    CheckCRIU -->|No| DBOnly["Fallback: export database volumes only"]
    FullShare --> ExportVols["Export database volumes to .tar archives"]
    DBOnly --> ExportVols
    ExportVols --> Bundle["Compress all artifacts into .tar.gz"]
    Bundle --> GenPass["Generate 4-word passphrase"]
    GenPass --> DeriveKey["Derive 32-byte key via PBKDF2 (SHA-256, 600k iterations)"]
    DeriveKey --> Encrypt["Encrypt bundle with AES-256-GCM"]
    Encrypt --> CheckRelay{"Relay configured in devx.yaml?"}
    CheckRelay -->|No| RelayErr(["Error: No relay bucket configured"])
    CheckRelay -->|Yes| Upload["Upload encrypted blob via aws/gcloud CLI"]
    Upload --> UploadOk{"Upload successful?"}
    UploadOk -->|No| UploadErr(["Error: Upload failed (check credentials)"])
    UploadOk -->|Yes| PrintID["Print share ID + passphrase to terminal"]
    PrintID --> ShareDone([Done])

    Route -->|"attach"| Download["Download encrypted blob from S3/GCS"]
    Download --> DownloadOk{"Download successful?"}
    DownloadOk -->|No| DlErr(["Error: Download failed (check credentials/URL)"])
    DownloadOk -->|Yes| Decrypt["Decrypt blob using provided passphrase"]
    Decrypt --> DecryptOk{"Passphrase valid?"}
    DecryptOk -->|No| PassErr(["Error: Invalid passphrase"])
    DecryptOk -->|Yes| Unbundle["Extract .tar.gz bundle"]
    Unbundle --> Confirm{"User confirms destructive restore?"}
    Confirm -->|No| AttachAbort([Aborted])
    Confirm -->|Yes| StopLocal["Stop & remove current local containers"]
    StopLocal --> Restore["Restore containers & volumes from bundle"]
    Restore --> AttachDone([Topology restored])

    Route -->|"share --db-only"| DBOnly
```

### Sharing & Attaching Sequence

```mermaid
sequenceDiagram
    actor Sender as Sender Developer
    participant sCLI as devx CLI (sender)
    participant Bucket as S3 / GCS Bucket
    participant rCLI as devx CLI (receiver)
    actor Receiver as Receiver Developer

    Note over Sender, Receiver: Share Phase

    Sender->>sCLI: devx state share
    activate sCLI
    sCLI->>sCLI: Checkpoint containers via CRIU
    sCLI->>sCLI: Export database volumes to .tar archives
    sCLI->>sCLI: Bundle all artifacts into .tar.gz
    sCLI->>sCLI: Generate 4-word passphrase
    sCLI->>sCLI: Derive 32-byte key (PBKDF2 SHA-256, 600k iter)
    sCLI->>sCLI: Encrypt bundle with AES-256-GCM
    sCLI->>Bucket: Upload encrypted blob (aws/gcloud CLI)
    Bucket-->>sCLI: Upload confirmed
    sCLI-->>Sender: Print share ID + passphrase
    deactivate sCLI

    Note over Sender, Receiver: Out-of-Band Passphrase Exchange

    Sender-->>Receiver: Share ID + passphrase (Slack, email, etc.)

    Note over Sender, Receiver: Attach Phase

    Receiver->>rCLI: devx state attach '<share-id>:<passphrase>'
    activate rCLI
    rCLI->>Bucket: Download encrypted blob (aws/gcloud CLI)
    Bucket-->>rCLI: Return ciphertext
    rCLI->>rCLI: Derive key from passphrase (PBKDF2)
    rCLI->>rCLI: Decrypt blob with AES-256-GCM
    alt Passphrase invalid
        rCLI-->>Receiver: Error: Invalid passphrase
    else Passphrase valid
        rCLI->>rCLI: Extract .tar.gz bundle
        rCLI-->>Receiver: Prompt: confirm destructive restore?
        Receiver->>rCLI: Confirm
        rCLI->>rCLI: Stop & remove current local containers
        rCLI->>rCLI: Restore containers & volumes from bundle
        rCLI-->>Receiver: Topology restored successfully
    end
    deactivate rCLI
```

## How it works

When you run `devx state share`, Devx performs the following steps:
1. **Container Checkpointing:** Uses CRIU (via Podman) to capture the exact memory and filesystem state of your running containers.
2. **Database Snapshotting:** Exports all local database volumes into `.tar` archives.
3. **Bundling:** Compresses all artifacts into a `.tar.gz` bundle.
4. **Encryption:** Generates a secure, human-readable 4-word passphrase and encrypts the entire bundle locally using AES-256-GCM.
5. **Upload:** Uploads the encrypted blob to your configured S3 or GCS bucket using the native `aws` or `gcloud` CLIs.

## Configuration (Bring-Your-Own-Bucket)

Devx strictly enforces a Bring-Your-Own-Bucket (BYOB) model. You must configure an S3 or GCS bucket to act as the relay.

Add the following to your `devx.yaml`:

```yaml
state:
  # Use your own S3/GCS bucket for secure team sharing
  relay: "s3://my-team-devx-state/checkpoints"
  # relay: "gs://my-team-devx-state/checkpoints"
```

*Note: You must have the corresponding CLI (`aws` or `gcloud`) installed and authenticated.*

## Usage

### Sharing State

To share your state, simply run:

```bash
devx state share
```

You will receive an output similar to this:
```
✅ State successfully bundled, encrypted, and uploaded!

Share this ID with your teammate to attach:

  s3://my-team-devx-state/checkpoints/_share_1680000000.encrypted:fast-blue-rabbit-dawn

They can run: devx state attach 's3://...:fast-blue-rabbit-dawn'
```

### Attaching State

To restore a shared state on your machine, run the attach command with the provided ID:

```bash
devx state attach 's3://my-team-devx-state/checkpoints/_share_1680000000.encrypted:fast-blue-rabbit-dawn'
```

> [!WARNING]
> Attaching state is destructive! It will stop and overwrite your current local containers and database volumes to match the shared state. Devx will prompt for confirmation before proceeding.

### Fallback: Database-Only Mode

CRIU container checkpointing is only supported when using **Podman** as your VM backend. If you or your teammate are using Docker Desktop, Lima, or Colima, Devx gracefully falls back to sharing database volumes only.

You can explicitly force this mode:
```bash
devx state share --db-only
```

## Security

The relay destination (S3/GCS) only ever sees opaque ciphertext. The data is encrypted entirely on your local machine using a 32-byte key derived from the generated passphrase using PBKDF2 (SHA-256, 600,000 iterations). The passphrase is required to decrypt the bundle.
