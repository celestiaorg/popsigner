//! # POPSigner Rust SDK
//!
//! Official Rust SDK for the POPSigner Control Plane API.
//!
//! POPSigner is Point-of-Presence signing infrastructure.
//! Deploy inline with execution. Keys remain remote. You remain sovereign.
//!
//! ## Quick Start
//!
//! ```rust,no_run
//! use popsigner::{Client, types::CreateKeyRequest};
//! use uuid::Uuid;
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     // Create a client with your API key
//!     let client = Client::new("psk_live_xxxxx");
//!     
//!     // Create a key
//!     let namespace_id = Uuid::parse_str("...")?;
//!     let key = client.keys().create(CreateKeyRequest {
//!         name: "my-sequencer".to_string(),
//!         namespace_id,
//!         ..Default::default()
//!     }).await?;
//!     
//!     println!("Created key: {} ({})", key.name, key.address);
//!     
//!     // Sign data
//!     let data = b"transaction data";
//!     let result = client.sign().sign(&key.id, data, false).await?;
//!     println!("Signature: {} bytes", result.signature.len());
//!     
//!     Ok(())
//! }
//! ```
//!
//! ## Parallel Workers Pattern
//!
//! For high-throughput signing, use batch operations:
//!
//! ```rust,no_run
//! use popsigner::{Client, types::{CreateBatchRequest, BatchSignRequest, BatchSignItem}};
//! use uuid::Uuid;
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let client = Client::new("psk_live_xxxxx");
//!     let namespace_id = Uuid::parse_str("...")?;
//!     
//!     // Create 4 worker keys
//!     let keys = client.keys().create_batch(CreateBatchRequest {
//!         prefix: "blob-worker".to_string(),
//!         count: 4,
//!         namespace_id,
//!         exportable: None,
//!     }).await?;
//!     
//!     // Sign 4 transactions in parallel
//!     let results = client.sign().sign_batch(BatchSignRequest {
//!         requests: keys.iter().enumerate().map(|(i, key)| {
//!             BatchSignItem {
//!                 key_id: key.id,
//!                 data: format!("tx-{}", i).into_bytes(),
//!                 prehashed: false,
//!             }
//!         }).collect(),
//!     }).await?;
//!     
//!     // All 4 signed in parallel!
//!     Ok(())
//! }
//! ```
//!
//! ## Features
//!
//! - **Authentication**: API key authentication
//! - **Key Management**: Create, get, list, delete keys
//! - **Batch Operations**: Create and sign in batches for parallel workers
//! - **Signing**: Sign data with keys (single or batch)
//! - **Organizations**: Manage organizations and namespaces
//! - **Audit Logs**: Access audit logs for compliance
//!
//! ## Error Handling
//!
//! All operations return `Result<T, POPSignerError>`:
//!
//! ```rust,no_run
//! use popsigner::{Client, error::POPSignerError};
//!
//! #[tokio::main]
//! async fn main() {
//!     let client = Client::new("psk_live_xxxxx");
//!     
//!     match client.keys().list(None).await {
//!         Ok(keys) => println!("Found {} keys", keys.len()),
//!         Err(POPSignerError::Unauthorized) => println!("Invalid API key"),
//!         Err(POPSignerError::RateLimited) => println!("Rate limited, retry later"),
//!         Err(e) => println!("Error: {}", e),
//!     }
//! }
//! ```

pub mod audit;
pub mod celestia;
pub mod client;
pub mod error;
pub mod keys;
pub mod orgs;
pub mod sign;
pub mod types;

// Re-export main types at the crate root
pub use client::{Client, ClientConfig};
pub use error::{POPSignerError, Result};

// Re-export types module for easy access
pub use types::{
    AuditLog, BatchSignItem, BatchSignRequest, CreateBatchRequest, CreateKeyRequest, Key,
    ListAuditLogsQuery, Namespace, Organization, PaginatedResponse, SignResponse,
};
