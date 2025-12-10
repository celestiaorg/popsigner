//! Organization management operations.
//!
//! This module provides the OrgsClient for managing organizations and namespaces.

use crate::client::Client;
use crate::error::Result;
use crate::types::{Namespace, Organization};
use serde::Serialize;
use uuid::Uuid;

/// Client for organization management operations.
///
/// Access via `client.orgs()`.
pub struct OrgsClient {
    client: Client,
}

impl OrgsClient {
    pub(crate) fn new(client: Client) -> Self {
        Self { client }
    }

    /// Get the current organization.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     let org = client.orgs().get_current().await?;
    ///     println!("Organization: {} ({})", org.name, org.plan);
    ///     Ok(())
    /// }
    /// ```
    pub async fn get_current(&self) -> Result<Organization> {
        self.client.get("/v1/org").await
    }

    /// List all namespaces in the organization.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     let namespaces = client.orgs().list_namespaces().await?;
    ///     for ns in namespaces {
    ///         println!("Namespace: {}", ns.name);
    ///     }
    ///     Ok(())
    /// }
    /// ```
    pub async fn list_namespaces(&self) -> Result<Vec<Namespace>> {
        self.client.get("/v1/namespaces").await
    }

    /// Get a namespace by ID.
    ///
    /// # Arguments
    ///
    /// * `namespace_id` - The namespace's unique identifier
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
    ///     let ns = client.orgs().get_namespace(&namespace_id).await?;
    ///     println!("Namespace: {}", ns.name);
    ///     Ok(())
    /// }
    /// ```
    pub async fn get_namespace(&self, namespace_id: &Uuid) -> Result<Namespace> {
        self.client
            .get(&format!("/v1/namespaces/{}", namespace_id))
            .await
    }

    /// Create a new namespace.
    ///
    /// # Arguments
    ///
    /// * `name` - The namespace name
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let client = Client::new("bbr_live_xxxxx");
    ///     
    ///     let ns = client.orgs().create_namespace("production").await?;
    ///     println!("Created namespace: {} ({})", ns.name, ns.id);
    ///     Ok(())
    /// }
    /// ```
    pub async fn create_namespace(&self, name: &str) -> Result<Namespace> {
        #[derive(Serialize)]
        struct Request<'a> {
            name: &'a str,
        }

        self.client
            .post("/v1/namespaces", &Request { name })
            .await
    }

    /// Delete a namespace.
    ///
    /// **Warning:** This will delete all keys in the namespace.
    ///
    /// # Arguments
    ///
    /// * `namespace_id` - The namespace's unique identifier
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
    ///     client.orgs().delete_namespace(&namespace_id).await?;
    ///     println!("Namespace deleted");
    ///     Ok(())
    /// }
    /// ```
    pub async fn delete_namespace(&self, namespace_id: &Uuid) -> Result<()> {
        self.client
            .delete(&format!("/v1/namespaces/{}", namespace_id))
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_orgs_client_creation() {
        let client = Client::new("test_key");
        let _orgs = client.orgs();
        // Just verify it compiles and doesn't panic
    }
}

