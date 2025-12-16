# POPSigner Rust SDK

Official Rust SDK for the [POPSigner](https://popsigner.com) Control Plane API.

POPSigner is Point-of-Presence signing infrastructure. Keys are stored in OpenBao and **never leave the secure enclave**. You remain sovereign.

## Installation

```toml
[dependencies]
popsigner = "0.1"
tokio = { version = "1", features = ["full"] }
```

### Celestia Integration

For Celestia blob submission with secure remote signing:

```toml
[dependencies]
popsigner = { version = "0.1", features = ["celestia"] }
```

## Quick Start

```rust
use popsigner::{Client, CreateKeyRequest};
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new("psk_live_xxxxx");
    
    // Create a key
    let namespace_id = Uuid::parse_str("your-namespace-id")?;
    let key = client.keys().create(CreateKeyRequest {
        name: "my-sequencer".to_string(),
        namespace_id,
        ..Default::default()
    }).await?;
    
    println!("Created key: {} ({})", key.name, key.address);
    
    // Sign data
    let data = b"transaction data";
    let result = client.sign().sign(&key.id, data, false).await?;
    println!("Signature: {} bytes", result.signature.len());
    
    Ok(())
}
```

## Features

| Feature | Description |
|---------|-------------|
| **Key Management** | Create, list, delete, export keys |
| **Signing** | Sign data with secp256k1 keys |
| **Batch Operations** | Create and sign in batches |
| **Celestia** | Drop-in replacement for Lumina's client |
| **Organizations** | Manage organizations and namespaces |
| **Audit Logs** | Access audit logs for compliance |
| **Exit Guarantee** | Export keys anytime—sovereignty by default |

## Celestia Integration

POPSigner provides a **drop-in replacement** for Lumina's `celestia_client::Client`. Private keys never leave the secure enclave.

### The Problem

Lumina requires exposing private keys:

```rust
// ❌ INSECURE: Private key in code
use celestia_client::Client;

let client = Client::builder()
    .rpc_url("ws://localhost:26658")
    .private_key_hex("393fdb5def075819...")  // Exposed!
    .build()
    .await?;
```

### The Solution

Change one import:

```rust
// ✅ SECURE: Keys never exposed
use popsigner::celestia::Client;

let client = Client::builder()
    .rpc_url("ws://localhost:26658")
    .grpc_url("http://localhost:9090")
    .popsigner("psk_live_xxx", "my-key")
    .build()
    .await?;

// Same API as Lumina
client.blob().submit(&[blob], TxConfig::default()).await?;
```

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Your Application                   │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│            popsigner::celestia::Client              │
│       (Drop-in replacement for celestia_client)     │
└─────────────────────────────────────────────────────┘
                 │                   │
                 ▼                   ▼
┌─────────────────────┐   ┌──────────────────────────┐
│    Lumina Client    │   │     POPSigner API        │
│  (RPC/gRPC reads)   │   │   (Remote Signing)       │
└─────────────────────┘   └──────────────────────────┘
```

### Lumina Compatibility

The `celestia` feature tracks [Lumina](https://github.com/eigerco/lumina) main branch. We follow Lumina's semver.

## API Reference

### Client

```rust
let client = Client::new("api_key");
let client = Client::with_config("api_key", config);

client.keys()   // KeysClient
client.sign()   // SignClient
client.orgs()   // OrgsClient
client.audit()  // AuditClient
```

### KeysClient

```rust
client.keys().create(CreateKeyRequest { ... }).await?;
client.keys().create_batch(CreateBatchRequest { ... }).await?;
client.keys().get(&key_id).await?;
client.keys().get_by_name(&namespace_id, "key-name").await?;
client.keys().list(None).await?;
client.keys().delete(&key_id).await?;
client.keys().export(&key_id).await?;
```

### SignClient

```rust
client.sign().sign(&key_id, &data, false).await?;
client.sign().sign(&key_id, &hash, true).await?;  // pre-hashed
client.sign().sign_batch(BatchSignRequest { ... }).await?;
client.sign().verify(&key_id, &data, &signature, false).await?;
```

### OrgsClient

```rust
client.orgs().get_current().await?;
client.orgs().list_namespaces().await?;
client.orgs().create_namespace("production").await?;
client.orgs().delete_namespace(&namespace_id).await?;
```

### AuditClient

```rust
client.audit().list(None).await?;
client.audit().list(Some(ListAuditLogsQuery { ... })).await?;
client.audit().get(&log_id).await?;
client.audit().list_for_resource("key", &key_id).await?;
```

## Error Handling

```rust
use popsigner::{Client, POPSignerError};

match client.keys().list(None).await {
    Ok(keys) => println!("Found {} keys", keys.len()),
    Err(POPSignerError::Unauthorized) => println!("Invalid API key"),
    Err(POPSignerError::RateLimited) => println!("Rate limited"),
    Err(POPSignerError::QuotaExceeded(msg)) => println!("Quota: {}", msg),
    Err(e) if e.is_retryable() => println!("Retryable: {}", e),
    Err(e) => println!("Error: {}", e),
}
```

| Error | Description |
|-------|-------------|
| `Unauthorized` | Invalid API key |
| `RateLimited` | Too many requests |
| `QuotaExceeded` | Monthly quota exceeded |
| `KeyNotFound` | Key does not exist |
| `Api` | Other API errors |
| `Http` | Network errors |

## Examples

```bash
export POPSIGNER_API_KEY=psk_live_xxxxx
export NAMESPACE_ID=your-namespace-uuid

cargo run --example basic
cargo run --example parallel_workers
cargo run --features celestia --example celestia_signer
```

## License

Apache-2.0

## Links

- [Documentation](https://docs.popsigner.com)
- [API Reference](https://docs.popsigner.com/api)
- [GitHub](https://github.com/popsigner/sdk-rust)
