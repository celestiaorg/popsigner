//! Key management operations.
//!
//! This module provides the KeysClient for creating, retrieving, listing,
//! and deleting cryptographic keys.

use crate::client::Client;
use crate::error::Result;
use crate::types::{CreateBatchRequest, CreateKeyRequest, Key};
use serde::Deserialize;
use uuid::Uuid;

/// Client for key management operations.
///
/// Access via `client.keys()`.
pub struct KeysClient {
    client: Client,
}

impl KeysClient {
    pub(crate) fn new(client: Client) -> Self {
        Self { client }
    }

    /// Create a new key.
    ///
    /// # Arguments
    ///
    /// * `request` - Key creation parameters
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::{Client, types::CreateKeyRequest};
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let namespace_id = Uuid::parse_str("...")?;
    ///     
    ///     let key = client.keys().create(CreateKeyRequest {
    ///         name: "my-key".to_string(),
    ///         namespace_id,
    ///         algorithm: Some("secp256k1".to_string()),
    ///         exportable: Some(false),
    ///         metadata: None,
    ///     }).await?;
    ///     
    ///     println!("Created key: {} ({})", key.name, key.address);
    ///     Ok(())
    /// }
    /// ```
    pub async fn create(&self, request: CreateKeyRequest) -> Result<Key> {
        self.client.post("/v1/keys", &request).await
    }

    /// Create multiple keys in parallel.
    ///
    /// Optimized for Celestia's parallel worker pattern. Creates keys with
    /// sequential names like "prefix-1", "prefix-2", etc.
    ///
    /// # Arguments
    ///
    /// * `request` - Batch creation parameters
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::{Client, types::CreateBatchRequest};
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let namespace_id = Uuid::parse_str("...")?;
    ///     
    ///     let keys = client.keys().create_batch(CreateBatchRequest {
    ///         prefix: "blob-worker".to_string(),
    ///         count: 4,
    ///         namespace_id,
    ///         exportable: None,
    ///     }).await?;
    ///     
    ///     // Creates: blob-worker-1, blob-worker-2, blob-worker-3, blob-worker-4
    ///     for key in &keys {
    ///         println!("  {}: {}", key.name, key.address);
    ///     }
    ///     Ok(())
    /// }
    /// ```
    pub async fn create_batch(&self, request: CreateBatchRequest) -> Result<Vec<Key>> {
        #[derive(Deserialize)]
        struct Response {
            keys: Vec<Key>,
        }

        let response: Response = self.client.post("/v1/keys/batch", &request).await?;
        Ok(response.keys)
    }

    /// Get a key by ID.
    ///
    /// # Arguments
    ///
    /// * `key_id` - The key's unique identifier
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let key_id = Uuid::parse_str("...")?;
    ///     
    ///     let key = client.keys().get(&key_id).await?;
    ///     println!("Key: {} ({})", key.name, key.algorithm);
    ///     Ok(())
    /// }
    /// ```
    pub async fn get(&self, key_id: &Uuid) -> Result<Key> {
        self.client.get(&format!("/v1/keys/{}", key_id)).await
    }

    /// List all keys, optionally filtered by namespace.
    ///
    /// # Arguments
    ///
    /// * `namespace_id` - Optional namespace ID to filter by
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     // List all keys
    ///     let all_keys = client.keys().list(None).await?;
    ///     println!("Total keys: {}", all_keys.len());
    ///     
    ///     // List keys in a specific namespace
    ///     let namespace_id = Uuid::parse_str("...")?;
    ///     let namespace_keys = client.keys().list(Some(&namespace_id)).await?;
    ///     Ok(())
    /// }
    /// ```
    pub async fn list(&self, namespace_id: Option<&Uuid>) -> Result<Vec<Key>> {
        let path = match namespace_id {
            Some(id) => format!("/v1/keys?namespace_id={}", id),
            None => "/v1/keys".to_string(),
        };
        self.client.get(&path).await
    }

    /// Delete a key.
    ///
    /// **Warning:** This operation is irreversible. The key and all associated
    /// data will be permanently deleted.
    ///
    /// # Arguments
    ///
    /// * `key_id` - The key's unique identifier
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let key_id = Uuid::parse_str("...")?;
    ///     
    ///     client.keys().delete(&key_id).await?;
    ///     println!("Key deleted");
    ///     Ok(())
    /// }
    /// ```
    pub async fn delete(&self, key_id: &Uuid) -> Result<()> {
        self.client.delete(&format!("/v1/keys/{}", key_id)).await
    }

    /// Get a key by name within a namespace.
    ///
    /// # Arguments
    ///
    /// * `namespace_id` - The namespace ID
    /// * `name` - The key name
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let namespace_id = Uuid::parse_str("...")?;
    ///     
    ///     let key = client.keys().get_by_name(&namespace_id, "my-key").await?;
    ///     println!("Key ID: {}", key.id);
    ///     Ok(())
    /// }
    /// ```
    pub async fn get_by_name(&self, namespace_id: &Uuid, name: &str) -> Result<Key> {
        self.client
            .get(&format!("/v1/keys/by-name/{}/{}", namespace_id, name))
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_keys_client_creation() {
        let client = Client::new("test_key");
        let _keys = client.keys();
        // Just verify it compiles and doesn't panic
    }
}

