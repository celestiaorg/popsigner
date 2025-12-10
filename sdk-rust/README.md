# BanhBaoRing Rust SDK

Official Rust SDK for the [BanhBaoRing](https://banhbaoring.io) Control Plane API.

BanhBaoRing provides secure key management for Celestia and Cosmos SDK applications. Keys are stored in OpenBao (Vault fork) and **never leave the secure enclave**.

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
banhbaoring = "0.1"
tokio = { version = "1", features = ["full"] }
```

## Quick Start

```rust
use banhbaoring::{Client, CreateKeyRequest};
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create a client with your API key
    let client = Client::new("bbr_live_xxxxx");
    
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
| **Authentication** | API key authentication |
| **Key Management** | Create, get, list, delete keys |
| **Batch Operations** | Create and sign in batches |
| **Signing** | Sign data with secp256k1 keys |
| **Organizations** | Manage organizations and namespaces |
| **Audit Logs** | Access audit logs for compliance |

## Parallel Workers (Celestia Pattern)

For high-throughput blob submission, use batch operations:

```rust
use banhbaoring::{Client, CreateBatchRequest, BatchSignRequest, BatchSignItem};
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new("bbr_live_xxxxx");
    let namespace_id = Uuid::parse_str("...")?;
    
    // Create 4 worker keys in one API call
    let keys = client.keys().create_batch(CreateBatchRequest {
        prefix: "blob-worker".to_string(),
        count: 4,
        namespace_id,
        exportable: None,
    }).await?;
    // Creates: blob-worker-1, blob-worker-2, blob-worker-3, blob-worker-4
    
    // Sign 4 transactions in parallel with one API call
    let transactions = vec![b"tx1".to_vec(), b"tx2".to_vec(), b"tx3".to_vec(), b"tx4".to_vec()];
    
    let results = client.sign().sign_batch(BatchSignRequest {
        requests: keys.iter().zip(transactions.iter()).map(|(key, tx)| {
            BatchSignItem {
                key_id: key.id,
                data: tx.clone(),
                prehashed: false,
            }
        }).collect(),
    }).await?;
    
    // All 4 signed in parallel - completes in ~200ms, not 800ms!
    println!("Signed {} transactions", results.len());
    Ok(())
}
```

## Client Configuration

```rust
use banhbaoring::{Client, ClientConfig};
use std::time::Duration;

// Default configuration
let client = Client::new("bbr_live_xxxxx");

// Custom configuration
let client = Client::with_config("bbr_live_xxxxx", ClientConfig {
    base_url: Some("https://api.staging.banhbaoring.io".to_string()),
    timeout: Some(Duration::from_secs(60)),
    user_agent: Some("my-app/1.0".to_string()),
});
```

## Error Handling

All operations return `Result<T, BanhBaoRingError>`:

```rust
use banhbaoring::{Client, BanhBaoRingError};

#[tokio::main]
async fn main() {
    let client = Client::new("bbr_live_xxxxx");
    
    match client.keys().list(None).await {
        Ok(keys) => println!("Found {} keys", keys.len()),
        Err(BanhBaoRingError::Unauthorized) => {
            println!("Invalid API key");
        }
        Err(BanhBaoRingError::RateLimited) => {
            println!("Rate limited - implement backoff");
        }
        Err(BanhBaoRingError::QuotaExceeded(msg)) => {
            println!("Quota exceeded: {}", msg);
        }
        Err(e) if e.is_retryable() => {
            println!("Retryable error: {}", e);
        }
        Err(e) => {
            println!("Error: {}", e);
        }
    }
}
```

### Error Types

| Error | Description |
|-------|-------------|
| `Unauthorized` | Invalid API key |
| `RateLimited` | Too many requests |
| `QuotaExceeded` | Monthly quota exceeded |
| `KeyNotFound` | Key does not exist |
| `Api` | Other API errors with code/message |
| `Http` | Network/connection errors |
| `Decode` | Base64 decoding errors |

## API Reference

### Client

```rust
// Create client
let client = Client::new("api_key");
let client = Client::with_config("api_key", config);

// Access sub-clients
client.keys()   // KeysClient
client.sign()   // SignClient
client.orgs()   // OrgsClient
client.audit()  // AuditClient
```

### KeysClient

```rust
// Create a key
client.keys().create(CreateKeyRequest { ... }).await?;

// Create multiple keys
client.keys().create_batch(CreateBatchRequest { ... }).await?;

// Get a key by ID
client.keys().get(&key_id).await?;

// Get a key by name
client.keys().get_by_name(&namespace_id, "key-name").await?;

// List all keys (optionally filtered by namespace)
client.keys().list(None).await?;
client.keys().list(Some(&namespace_id)).await?;

// Delete a key
client.keys().delete(&key_id).await?;
```

### SignClient

```rust
// Sign data
client.sign().sign(&key_id, &data, false).await?;

// Sign pre-hashed data
client.sign().sign(&key_id, &hash, true).await?;

// Batch sign (parallel)
client.sign().sign_batch(BatchSignRequest { ... }).await?;

// Verify signature
client.sign().verify(&key_id, &data, &signature, false).await?;
```

### OrgsClient

```rust
// Get current organization
client.orgs().get_current().await?;

// List namespaces
client.orgs().list_namespaces().await?;

// Create namespace
client.orgs().create_namespace("production").await?;

// Delete namespace
client.orgs().delete_namespace(&namespace_id).await?;
```

### AuditClient

```rust
// List audit logs
client.audit().list(None).await?;

// List with filters
client.audit().list(Some(ListAuditLogsQuery {
    event: Some("key.created".to_string()),
    limit: Some(100),
    ..Default::default()
})).await?;

// Get single log entry
client.audit().get(&log_id).await?;

// List logs for a resource
client.audit().list_for_resource("key", &key_id).await?;
```

## Examples

Run examples with:

```bash
# Set environment variables
export BANHBAORING_API_KEY=bbr_live_xxxxx
export NAMESPACE_ID=your-namespace-uuid

# Run basic example
cargo run --example basic

# Run parallel workers example
cargo run --example parallel_workers
```

## Testing

```bash
# Run all tests
cargo test

# Run with output
cargo test -- --nocapture
```

## License

MIT OR Apache-2.0

## Links

- [BanhBaoRing Documentation](https://docs.banhbaoring.io)
- [API Reference](https://docs.banhbaoring.io/api)
- [GitHub Repository](https://github.com/banhbaoring/sdk-rust)

