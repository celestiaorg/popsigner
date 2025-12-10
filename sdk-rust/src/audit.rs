//! Audit log operations.
//!
//! This module provides the AuditClient for accessing audit logs.

use crate::client::Client;
use crate::error::Result;
use crate::types::{AuditLog, ListAuditLogsQuery, PaginatedResponse};
use uuid::Uuid;

/// Client for audit log operations.
///
/// Access via `client.audit()`.
pub struct AuditClient {
    client: Client,
}

impl AuditClient {
    pub(crate) fn new(client: Client) -> Self {
        Self { client }
    }

    /// List audit logs with optional filters.
    ///
    /// # Arguments
    ///
    /// * `query` - Optional query parameters for filtering
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::{Client, types::ListAuditLogsQuery};
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     // List all audit logs
    ///     let logs = client.audit().list(None).await?;
    ///     println!("Total logs: {}", logs.total);
    ///     
    ///     // List with filters
    ///     let logs = client.audit().list(Some(ListAuditLogsQuery {
    ///         event: Some("key.created".to_string()),
    ///         limit: Some(10),
    ///         ..Default::default()
    ///     })).await?;
    ///     
    ///     for log in logs.items {
    ///         println!("[{}] {}", log.created_at, log.event);
    ///     }
    ///     Ok(())
    /// }
    /// ```
    pub async fn list(
        &self,
        query: Option<ListAuditLogsQuery>,
    ) -> Result<PaginatedResponse<AuditLog>> {
        let mut path = "/v1/audit".to_string();

        if let Some(q) = query {
            let mut params = Vec::new();

            if let Some(event) = &q.event {
                params.push(format!("event={}", event));
            }
            if let Some(resource_type) = &q.resource_type {
                params.push(format!("resource_type={}", resource_type));
            }
            if let Some(resource_id) = &q.resource_id {
                params.push(format!("resource_id={}", resource_id));
            }
            if let Some(limit) = q.limit {
                params.push(format!("limit={}", limit));
            }
            if let Some(offset) = q.offset {
                params.push(format!("offset={}", offset));
            }

            if !params.is_empty() {
                path = format!("{}?{}", path, params.join("&"));
            }
        }

        self.client.get(&path).await
    }

    /// Get a single audit log entry by ID.
    ///
    /// # Arguments
    ///
    /// * `log_id` - The audit log entry ID
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
    ///     let log_id = Uuid::parse_str("...")?;
    ///     
    ///     let log = client.audit().get(&log_id).await?;
    ///     println!("Event: {}", log.event);
    ///     Ok(())
    /// }
    /// ```
    pub async fn get(&self, log_id: &Uuid) -> Result<AuditLog> {
        self.client.get(&format!("/v1/audit/{}", log_id)).await
    }

    /// List audit logs for a specific resource.
    ///
    /// # Arguments
    ///
    /// * `resource_type` - The type of resource (e.g., "key", "namespace")
    /// * `resource_id` - The resource ID
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
    ///     let logs = client.audit().list_for_resource("key", &key_id).await?;
    ///     for log in logs.items {
    ///         println!("[{}] {}", log.created_at, log.event);
    ///     }
    ///     Ok(())
    /// }
    /// ```
    pub async fn list_for_resource(
        &self,
        resource_type: &str,
        resource_id: &Uuid,
    ) -> Result<PaginatedResponse<AuditLog>> {
        self.list(Some(ListAuditLogsQuery {
            resource_type: Some(resource_type.to_string()),
            resource_id: Some(*resource_id),
            ..Default::default()
        }))
        .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_audit_client_creation() {
        let client = Client::new("test_key");
        let _audit = client.audit();
        // Just verify it compiles and doesn't panic
    }

    #[test]
    fn test_list_audit_logs_query_default() {
        let query = ListAuditLogsQuery::default();
        assert!(query.event.is_none());
        assert!(query.resource_type.is_none());
        assert!(query.limit.is_none());
    }
}

