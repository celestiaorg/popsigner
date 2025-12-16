# POPSigner

**Point-of-Presence Signing Infrastructure**

[![Go Reference](https://pkg.go.dev/badge/github.com/Bidon15/popsigner.svg)](https://pkg.go.dev/github.com/Bidon15/popsigner)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> POPSigner is a distributed signing layer designed to live inline with execution—not behind an API queue.

---

## What POPSigner Is

POPSigner is Point-of-Presence signing infrastructure. It deploys where your systems already run—the same region, the same rack, the same execution path.

**This isn't custody. This isn't MPC. This is signing at the point of execution.**

```
┌─────────────────────────────────────────────────────────────┐
│  YOUR INFRASTRUCTURE                                         │
│                                                              │
│  ┌──────────────┐    inline    ┌──────────────────────────┐ │
│  │  Execution   │ ───────────▶ │  POPSigner POP           │ │
│  │  (sequencer, │              │  (same region)           │ │
│  │   bot, etc.) │ ◀─────────── │                          │ │
│  └──────────────┘   signature  └──────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Core Principles

| Principle | Description |
|-----------|-------------|
| **Inline Signing** | Signing happens on the execution path, not behind a queue |
| **Sovereignty by Default** | Keys are remote, but you control them. Export anytime. Exit anytime. |
| **Neutral Anchor** | Recovery data is anchored to neutral data availability. If we disappear, you don't. |

---

## Quick Start

### Option 1: POPSigner Cloud

Deploy without infrastructure. Connect and sign.

```bash
# Get your API key at https://popsigner.com
go get github.com/Bidon15/popsigner/sdk-go
```

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    popsigner "github.com/Bidon15/popsigner/sdk-go"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // Connect to POPSigner
    client := popsigner.NewClient(os.Getenv("POPSIGNER_API_KEY"))

    // Create a key
    namespaceID := uuid.MustParse(os.Getenv("POPSIGNER_NAMESPACE_ID"))
    key, err := client.Keys.Create(ctx, popsigner.CreateKeyRequest{
        Name:        "sequencer-key",
        NamespaceID: namespaceID,
        Algorithm:   "secp256k1",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created key: %s (%s)\n", key.Name, key.Address)

    // Sign inline with your execution
    result, err := client.Sign.Sign(ctx, key.ID, []byte("transaction data"), false)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Signature: %x\n", result.Signature)
}
```

### Option 2: Self-Hosted

Run POPSigner on your own infrastructure. Full control. No dependencies.

```go
package main

import (
    "context"
    "os"

    popsigner "github.com/Bidon15/popsigner"
)

func main() {
    ctx := context.Background()

    kr, _ := popsigner.New(ctx, popsigner.Config{
        BaoAddr:   "https://your-openbao.internal:8200",
        BaoToken:  os.Getenv("BAO_TOKEN"),
        StorePath: "./keyring-metadata.json",
    })

    // Create a key
    record, _ := kr.NewAccountWithOptions("sequencer", popsigner.KeyOptions{
        Exportable: true,
    })

    // Sign with OpenBao backend
    sig, pubKey, _ := kr.Sign("sequencer", []byte("transaction data"), nil)
}
```

See [Deployment Guide](doc/product/DEPLOYMENT.md) for Kubernetes setup.

---

## Why POPSigner

| | Local Keyring | Cloud KMS | **POPSigner** |
|--|--------------|-----------|---------------|
| **Key exposure** | On disk | Decrypted in app | **Never exposed** |
| **secp256k1** | ✅ | ❌ | ✅ |
| **Placement** | Local only | Their region | **Your region** |
| **Self-hostable** | ✅ | ❌ | ✅ |
| **Managed option** | ❌ | ✅ | ✅ |
| **Exit guarantee** | N/A | ❌ | **Always** |

---

## Exit Guarantee

POPSigner is designed with exit as a first-class primitive.

- **Key Export**: Your keys are exportable by default. No ceremony. No approval workflow.
- **Recovery Anchor**: Recovery data is anchored to neutral data availability infrastructure.
- **Force Exit**: If POPSigner is unavailable for any reason, you can force recovery. This is not gated.

---

## Plugin Architecture

POPSigner ships with `secp256k1`. But the plugin architecture is the actual product.

- Plugins are chain-agnostic
- Plugins are free
- Plugins don't require approval

```go
// Built-in secp256k1
sig, pubKey, _ := kr.Sign("my-key", signBytes, signMode)

// Your custom algorithm tomorrow
```

---

## SDKs

### Go SDK

```bash
go get github.com/Bidon15/popsigner/sdk-go
```

```go
import popsigner "github.com/Bidon15/popsigner/sdk-go"

client := popsigner.NewClient("psk_live_xxxxx")
```

See [Go SDK README](sdk-go/README.md) for full documentation.

### Rust SDK

```toml
[dependencies]
popsigner = "0.1"
tokio = { version = "1", features = ["full"] }
```

```rust
use popsigner::Client;

let client = Client::new("psk_live_xxxxx");
```

See [Rust SDK README](sdk-rust/README.md) for full documentation.

---

## Integration

### Celestia / Cosmos SDK (Go)

POPSigner provides a Celestia-compatible keyring:

```go
import (
    popsigner "github.com/Bidon15/popsigner/sdk-go"
    "github.com/celestiaorg/celestia-node/api/client"
)

func main() {
    ctx := context.Background()

    // Create a Celestia-compatible keyring backed by POPSigner
    kr, _ := popsigner.NewCelestiaKeyring(
        os.Getenv("POPSIGNER_API_KEY"),
        "your-key-id",
    )

    // Use with Celestia client
    cfg := client.Config{
        ReadConfig: client.ReadConfig{
            BridgeDAAddr: "http://localhost:26658",
            DAAuthToken:  os.Getenv("CELESTIA_AUTH_TOKEN"),
        },
        SubmitConfig: client.SubmitConfig{
            DefaultKeyName: kr.KeyName(),
            Network:        "mocha-4",
        },
    }

    celestiaClient, _ := client.New(ctx, cfg, kr)

    // Submit blobs—signing happens inline via POPSigner
    fmt.Printf("Connected with address: %s\n", kr.CelestiaAddress())
}
```

### Celestia (Rust)

Drop-in replacement for Lumina's client:

```rust
use popsigner::celestia::Client;

let client = Client::builder()
    .rpc_url("ws://localhost:26658")
    .grpc_url("http://localhost:9090")
    .popsigner("psk_live_xxx", "my-key")
    .build()
    .await?;

// Same API as Lumina—keys never exposed
client.blob().submit(&[blob], TxConfig::default()).await?;
```

### Parallel Workers

POPSigner supports worker-native architecture for burst workloads:

```go
// Create signing workers
keys, _ := client.Keys.CreateBatch(ctx, popsigner.CreateBatchRequest{
    Prefix:      "blob-worker",
    Count:       4,
    NamespaceID: namespaceID,
})

// Sign in parallel—no blocking
results, _ := client.Sign.SignBatch(ctx, popsigner.BatchSignRequest{
    Requests: []popsigner.SignRequest{
        {KeyID: keys[0].ID, Data: tx1},
        {KeyID: keys[1].ID, Data: tx2},
        {KeyID: keys[2].ID, Data: tx3},
        {KeyID: keys[3].ID, Data: tx4},
    },
})
```

---

## CLI

POPSigner provides two CLIs for different use cases:

### Cloud CLI (`popctl`)

For managing keys via the POPSigner Control Plane API:

```bash
# Install
go install github.com/Bidon15/popsigner/popctl@latest

# Configure
popctl config init
# or set environment variables:
export POPSIGNER_API_KEY="psk_xxx"
export POPSIGNER_NAMESPACE_ID="your-namespace-uuid"

# Key management
popctl keys list
popctl keys create my-sequencer --exportable
popctl keys create-batch blob-worker --count 4
popctl keys get <key-id>
popctl keys export <key-id>
popctl keys delete <key-id>

# Sign data
popctl sign <key-id> --data "message"
popctl sign <key-id> --file message.txt
```

### Self-Hosted CLI (`popsigner`)

For managing keys directly with your own OpenBao instance:

```bash
# Install
go install github.com/Bidon15/popsigner/cmd/popsigner@latest

# Configure
export BAO_ADDR="http://127.0.0.1:8200"
export BAO_TOKEN="your-bao-token"

# Key management
popsigner keys list
popsigner keys add my-validator --exportable
popsigner keys show my-validator
popsigner keys rename old-name new-name
popsigner keys export-pub my-validator
popsigner keys delete my-validator

# Migration
popsigner migrate import --from ~/.celestia-app/keyring-file --key-name my-key
popsigner migrate export --key my-validator --to ./backup
```

---

## Migration

### Import existing keys (Cloud)

```bash
popctl keys import my-validator --private-key <base64-encoded-key>
```

### Import existing keys (Self-Hosted)

```bash
popsigner migrate import \
  --from ~/.celestia-app/keyring-file \
  --key-name my-validator
```

### Export keys (exit guarantee)

```bash
# Cloud
popctl keys export <key-id>

# Self-hosted
popsigner migrate export --key my-validator --to ./backup
```

See [Migration Guide](doc/product/MIGRATION.md) for all options.

---

## Documentation

| Document | Description |
|----------|-------------|
| [Integration Guide](doc/product/INTEGRATION.md) | Celestia client integration |
| [Migration Guide](doc/product/MIGRATION.md) | Import/export keys |
| [API Reference](doc/product/API_REFERENCE.md) | REST API endpoints |
| [Deployment Guide](doc/product/DEPLOYMENT.md) | Self-hosted Kubernetes setup |
| [Architecture](doc/product/ARCHITECTURE.md) | Technical design |
| [Plugin Design](doc/product/PLUGIN_DESIGN.md) | OpenBao plugin details |

---

## Installation

### Go SDK (Cloud)

```bash
go get github.com/Bidon15/popsigner/sdk-go
```

### Go Library (Self-Hosted)

```bash
go get github.com/Bidon15/popsigner
```

### Rust SDK

```toml
[dependencies]
popsigner = "0.1"
```

### CLI Tools

```bash
# Cloud CLI
go install github.com/Bidon15/popsigner/popctl@latest

# Self-Hosted CLI
go install github.com/Bidon15/popsigner/cmd/popsigner@latest
```

### Requirements

| Deployment | Requirements |
|------------|--------------|
| **Cloud** | API key only |
| **Self-hosted** | OpenBao + secp256k1 plugin, Kubernetes 1.25+ |

---

## About the Name

POPSigner (formerly BanhBaoRing) reflects a clearer articulation of what the system is: **Point-of-Presence signing infrastructure**.

The rename signals a shift from playful internal naming to category-defining infrastructure positioning.

---

## Contributing

```bash
git clone https://github.com/Bidon15/popsigner.git
cd popsigner
go mod download
go test ./...
```

---

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>POPSigner</b> — Signing at the point of execution.
  <br><br>
  <a href="https://popsigner.com">Deploy POPSigner</a> · 
  <a href="doc/product/INTEGRATION.md">Documentation</a> · 
  <a href="https://github.com/Bidon15/popsigner">GitHub</a>
</p>
