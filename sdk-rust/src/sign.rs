//! Signing operations.
//!
//! This module provides the SignClient for signing data with keys stored
//! in BanhBaoRing. Supports both single and batch signing operations.

use crate::client::Client;
use crate::error::{BanhBaoRingError, Result};
use crate::types::{BatchSignRequest, SignResponse};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Client for signing operations.
///
/// Access via `client.sign()`.
pub struct SignClient {
    client: Client,
}

impl SignClient {
    pub(crate) fn new(client: Client) -> Self {
        Self { client }
    }

    /// Sign data with a key.
    ///
    /// # Arguments
    ///
    /// * `key_id` - The key ID to sign with
    /// * `data` - The data to sign
    /// * `prehashed` - If true, data is already hashed (SHA-256)
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
    ///     // Sign raw data (will be hashed by the server)
    ///     let data = b"hello world";
    ///     let result = client.sign().sign(&key_id, data, false).await?;
    ///     
    ///     println!("Signature: {} bytes", result.signature.len());
    ///     println!("Public key: {}", result.public_key);
    ///     Ok(())
    /// }
    /// ```
    ///
    /// # Example with prehashed data
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    /// use uuid::Uuid;
    /// use sha2::{Sha256, Digest};
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     let key_id = Uuid::parse_str("...")?;
    ///     
    ///     // Hash data locally (requires sha2 crate)
    ///     let data = b"hello world";
    ///     let hash = Sha256::digest(data);
    ///     
    ///     // Sign the hash
    ///     let result = client.sign().sign(&key_id, &hash, true).await?;
    ///     Ok(())
    /// }
    /// ```
    pub async fn sign(&self, key_id: &Uuid, data: &[u8], prehashed: bool) -> Result<SignResponse> {
        #[derive(Serialize)]
        struct Request {
            data: String,
            prehashed: bool,
        }

        #[derive(Deserialize)]
        struct Response {
            signature: String,
            public_key: String,
        }

        let request = Request {
            data: BASE64.encode(data),
            prehashed,
        };

        let response: Response = self
            .client
            .post(&format!("/v1/keys/{}/sign", key_id), &request)
            .await?;

        let signature = BASE64
            .decode(&response.signature)
            .map_err(|e| BanhBaoRingError::Decode(e.to_string()))?;

        Ok(SignResponse {
            key_id: *key_id,
            signature,
            public_key: response.public_key,
        })
    }

    /// Sign multiple messages in parallel.
    ///
    /// This is critical for Celestia's parallel blob submission pattern.
    /// All signing operations are performed in a single API call, reducing
    /// latency significantly.
    ///
    /// # Arguments
    ///
    /// * `request` - Batch sign request containing multiple sign items
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::{Client, types::{BatchSignRequest, BatchSignItem}};
    /// use uuid::Uuid;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     let worker1 = Uuid::parse_str("...")?;
    ///     let worker2 = Uuid::parse_str("...")?;
    ///     let worker3 = Uuid::parse_str("...")?;
    ///     let worker4 = Uuid::parse_str("...")?;
    ///     
    ///     let results = client.sign().sign_batch(BatchSignRequest {
    ///         requests: vec![
    ///             BatchSignItem { key_id: worker1, data: b"tx1".to_vec(), prehashed: false },
    ///             BatchSignItem { key_id: worker2, data: b"tx2".to_vec(), prehashed: false },
    ///             BatchSignItem { key_id: worker3, data: b"tx3".to_vec(), prehashed: false },
    ///             BatchSignItem { key_id: worker4, data: b"tx4".to_vec(), prehashed: false },
    ///         ],
    ///     }).await?;
    ///     
    ///     // All 4 sign in parallel - completes in ~200ms, not 800ms!
    ///     println!("Signed {} transactions", results.len());
    ///     Ok(())
    /// }
    /// ```
    pub async fn sign_batch(&self, request: BatchSignRequest) -> Result<Vec<SignResponse>> {
        #[derive(Serialize)]
        struct ApiRequest {
            requests: Vec<ApiRequestItem>,
        }

        #[derive(Serialize)]
        struct ApiRequestItem {
            key_id: Uuid,
            data: String,
            prehashed: bool,
        }

        #[derive(Deserialize)]
        struct ApiResponse {
            signatures: Vec<ApiSignature>,
        }

        #[derive(Deserialize)]
        struct ApiSignature {
            key_id: Uuid,
            signature: String,
            public_key: String,
            error: Option<String>,
        }

        let api_request = ApiRequest {
            requests: request
                .requests
                .iter()
                .map(|r| ApiRequestItem {
                    key_id: r.key_id,
                    data: BASE64.encode(&r.data),
                    prehashed: r.prehashed,
                })
                .collect(),
        };

        let response: ApiResponse = self.client.post("/v1/sign/batch", &api_request).await?;

        let mut results = Vec::new();
        let mut errors = 0;

        for sig in response.signatures {
            if sig.error.is_some() {
                errors += 1;
                continue;
            }

            let signature = BASE64
                .decode(&sig.signature)
                .map_err(|e| BanhBaoRingError::Decode(e.to_string()))?;

            results.push(SignResponse {
                key_id: sig.key_id,
                signature,
                public_key: sig.public_key,
            });
        }

        if errors > 0 && results.is_empty() {
            return Err(BanhBaoRingError::BatchPartialFailure {
                failed: errors,
                total: request.requests.len(),
            });
        }

        Ok(results)
    }

    /// Verify a signature against the public key of a key.
    ///
    /// # Arguments
    ///
    /// * `key_id` - The key ID to verify with
    /// * `data` - The original data that was signed
    /// * `signature` - The signature to verify
    /// * `prehashed` - If true, data was prehashed before signing
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
    ///     let data = b"hello world";
    ///     let result = client.sign().sign(&key_id, data, false).await?;
    ///     
    ///     // Verify the signature
    ///     let valid = client.sign().verify(&key_id, data, &result.signature, false).await?;
    ///     assert!(valid);
    ///     Ok(())
    /// }
    /// ```
    pub async fn verify(
        &self,
        key_id: &Uuid,
        data: &[u8],
        signature: &[u8],
        prehashed: bool,
    ) -> Result<bool> {
        #[derive(Serialize)]
        struct Request {
            data: String,
            signature: String,
            prehashed: bool,
        }

        #[derive(Deserialize)]
        struct Response {
            valid: bool,
        }

        let request = Request {
            data: BASE64.encode(data),
            signature: BASE64.encode(signature),
            prehashed,
        };

        let response: Response = self
            .client
            .post(&format!("/v1/keys/{}/verify", key_id), &request)
            .await?;

        Ok(response.valid)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sign_client_creation() {
        let client = Client::new("test_key");
        let _sign = client.sign();
        // Just verify it compiles and doesn't panic
    }
}

