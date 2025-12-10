//! Type definitions for the BanhBaoRing SDK.
//!
//! This module contains all the request and response types used by the SDK.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use uuid::Uuid;

/// A cryptographic key.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Key {
    /// Unique key identifier.
    pub id: Uuid,
    /// Human-readable key name.
    pub name: String,
    /// Namespace the key belongs to.
    pub namespace_id: Uuid,
    /// Base64-encoded public key.
    pub public_key: String,
    /// Bech32-encoded address (for Cosmos/Celestia).
    pub address: String,
    /// Key algorithm (e.g., "secp256k1").
    pub algorithm: String,
    /// Whether the key can be exported.
    pub exportable: bool,
    /// Optional metadata.
    #[serde(default)]
    pub metadata: Option<HashMap<String, String>>,
    /// Creation timestamp.
    pub created_at: String,
}

/// Request to create a key.
#[derive(Debug, Clone, Serialize, Default)]
pub struct CreateKeyRequest {
    /// Key name (must be unique within namespace).
    pub name: String,
    /// Namespace ID for the key.
    pub namespace_id: Uuid,
    /// Key algorithm (default: "secp256k1").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub algorithm: Option<String>,
    /// Whether the key can be exported (default: false).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exportable: Option<bool>,
    /// Optional metadata.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, String>>,
}

/// Request to create multiple keys at once.
#[derive(Debug, Clone, Serialize)]
pub struct CreateBatchRequest {
    /// Prefix for key names (e.g., "blob-worker" creates "blob-worker-1", "blob-worker-2", etc.).
    pub prefix: String,
    /// Number of keys to create.
    pub count: u32,
    /// Namespace ID for all keys.
    pub namespace_id: Uuid,
    /// Whether keys can be exported (default: false).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exportable: Option<bool>,
}

/// Response from a sign operation.
#[derive(Debug, Clone)]
pub struct SignResponse {
    /// Key ID that was used for signing.
    pub key_id: Uuid,
    /// Raw signature bytes.
    pub signature: Vec<u8>,
    /// Base64-encoded public key.
    pub public_key: String,
}

/// Request to sign multiple messages in batch.
#[derive(Debug, Clone)]
pub struct BatchSignRequest {
    /// List of sign requests.
    pub requests: Vec<BatchSignItem>,
}

/// Single item in a batch sign request.
#[derive(Debug, Clone)]
pub struct BatchSignItem {
    /// Key ID to sign with.
    pub key_id: Uuid,
    /// Raw data to sign.
    pub data: Vec<u8>,
    /// Whether the data is already hashed.
    pub prehashed: bool,
}

/// An organization.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Organization {
    /// Organization ID.
    pub id: Uuid,
    /// Organization name.
    pub name: String,
    /// URL-friendly slug.
    pub slug: String,
    /// Billing plan.
    pub plan: String,
    /// Creation timestamp.
    pub created_at: String,
}

/// A namespace within an organization.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Namespace {
    /// Namespace ID.
    pub id: Uuid,
    /// Namespace name.
    pub name: String,
    /// Organization ID.
    pub org_id: Uuid,
    /// Creation timestamp.
    pub created_at: String,
}

/// An audit log entry.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct AuditLog {
    /// Audit log entry ID.
    pub id: Uuid,
    /// Event type.
    pub event: String,
    /// Actor ID (user or API key).
    pub actor_id: Option<Uuid>,
    /// Actor type ("user" or "api_key").
    pub actor_type: String,
    /// Resource type affected.
    pub resource_type: Option<String>,
    /// Resource ID affected.
    pub resource_id: Option<Uuid>,
    /// Additional metadata.
    pub metadata: Option<serde_json::Value>,
    /// Timestamp.
    pub created_at: String,
}

/// Query parameters for listing audit logs.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ListAuditLogsQuery {
    /// Filter by event type.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub event: Option<String>,
    /// Filter by resource type.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resource_type: Option<String>,
    /// Filter by resource ID.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resource_id: Option<Uuid>,
    /// Maximum number of results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub limit: Option<u32>,
    /// Offset for pagination.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub offset: Option<u32>,
}

/// Paginated response wrapper.
#[derive(Debug, Clone, Deserialize)]
pub struct PaginatedResponse<T> {
    /// Items in this page.
    pub items: Vec<T>,
    /// Total number of items.
    pub total: u64,
    /// Current offset.
    pub offset: u64,
    /// Current limit.
    pub limit: u64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_create_key_request_serialization() {
        let req = CreateKeyRequest {
            name: "test-key".to_string(),
            namespace_id: Uuid::nil(),
            algorithm: None,
            exportable: Some(true),
            metadata: None,
        };

        let json = serde_json::to_string(&req).unwrap();
        assert!(json.contains("test-key"));
        assert!(json.contains("exportable"));
        assert!(!json.contains("algorithm")); // None fields are skipped
    }

    #[test]
    fn test_key_deserialization() {
        let json = r#"{
            "id": "00000000-0000-0000-0000-000000000001",
            "name": "my-key",
            "namespace_id": "00000000-0000-0000-0000-000000000002",
            "public_key": "base64pubkey",
            "address": "celestia1...",
            "algorithm": "secp256k1",
            "exportable": false,
            "created_at": "2025-01-01T00:00:00Z"
        }"#;

        let key: Key = serde_json::from_str(json).unwrap();
        assert_eq!(key.name, "my-key");
        assert_eq!(key.algorithm, "secp256k1");
        assert!(!key.exportable);
    }

    #[test]
    fn test_create_batch_request() {
        let req = CreateBatchRequest {
            prefix: "worker".to_string(),
            count: 4,
            namespace_id: Uuid::nil(),
            exportable: None,
        };

        let json = serde_json::to_string(&req).unwrap();
        assert!(json.contains("worker"));
        assert!(json.contains("4"));
    }
}

