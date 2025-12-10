//! BanhBaoRing API client.
//!
//! The main entry point for interacting with the BanhBaoRing Control Plane API.

use crate::audit::AuditClient;
use crate::error::{BanhBaoRingError, Result};
use crate::keys::KeysClient;
use crate::orgs::OrgsClient;
use crate::sign::SignClient;
use reqwest::{header, Client as HttpClient};
use serde::Deserialize;
use std::time::Duration;

const DEFAULT_BASE_URL: &str = "https://api.banhbaoring.io";
const DEFAULT_TIMEOUT_SECS: u64 = 30;

/// BanhBaoRing API client.
///
/// # Example
///
/// ```rust,no_run
/// use banhbaoring::Client;
/// use banhbaoring::types::CreateKeyRequest;
/// use uuid::Uuid;
///
/// #[tokio::main]
/// async fn main() -> Result<(), Box<dyn std::error::Error>> {
///     let client = Client::new("bbr_live_xxxxx");
///     
///     // Create a key
///     let namespace_id = Uuid::parse_str("...")?;
///     let key = client.keys().create(CreateKeyRequest {
///         name: "sequencer".to_string(),
///         namespace_id,
///         ..Default::default()
///     }).await?;
///     
///     // Sign data
///     let tx_bytes = b"transaction data";
///     let sig = client.sign().sign(&key.id, tx_bytes, false).await?;
///     Ok(())
/// }
/// ```
#[derive(Clone)]
pub struct Client {
    pub(crate) http: HttpClient,
    pub(crate) base_url: String,
    pub(crate) api_key: String,
}

/// Configuration options for the client.
#[derive(Debug, Clone)]
pub struct ClientConfig {
    /// Base URL for the API (default: https://api.banhbaoring.io).
    pub base_url: Option<String>,
    /// Request timeout (default: 30 seconds).
    pub timeout: Option<Duration>,
    /// User-Agent header value.
    pub user_agent: Option<String>,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            base_url: None,
            timeout: None,
            user_agent: None,
        }
    }
}

impl Client {
    /// Create a new BanhBaoRing client with default configuration.
    ///
    /// # Arguments
    ///
    /// * `api_key` - Your BanhBaoRing API key (e.g., "bbr_live_xxxxx")
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// let client = Client::new("bbr_live_xxxxx");
    /// ```
    pub fn new(api_key: impl Into<String>) -> Self {
        Self::with_config(api_key, ClientConfig::default())
    }

    /// Get the base URL for the API.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// Create a new BanhBaoRing client with custom configuration.
    ///
    /// # Arguments
    ///
    /// * `api_key` - Your BanhBaoRing API key
    /// * `config` - Client configuration options
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::{Client, ClientConfig};
    /// use std::time::Duration;
    ///
    /// let client = Client::with_config("bbr_live_xxxxx", ClientConfig {
    ///     base_url: Some("https://api.staging.banhbaoring.io".to_string()),
    ///     timeout: Some(Duration::from_secs(60)),
    ///     user_agent: Some("my-app/1.0".to_string()),
    /// });
    /// ```
    pub fn with_config(api_key: impl Into<String>, config: ClientConfig) -> Self {
        let timeout = config
            .timeout
            .unwrap_or(Duration::from_secs(DEFAULT_TIMEOUT_SECS));
        let user_agent = config
            .user_agent
            .unwrap_or_else(|| format!("banhbaoring-rust/{}", env!("CARGO_PKG_VERSION")));

        let http = HttpClient::builder()
            .timeout(timeout)
            .user_agent(user_agent)
            .build()
            .expect("Failed to create HTTP client");

        Self {
            http,
            base_url: config
                .base_url
                .unwrap_or_else(|| DEFAULT_BASE_URL.to_string()),
            api_key: api_key.into(),
        }
    }

    /// Get the keys client for key management operations.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// let client = Client::new("bbr_live_xxxxx");
    /// let keys_client = client.keys();
    /// ```
    pub fn keys(&self) -> KeysClient {
        KeysClient::new(self.clone())
    }

    /// Get the sign client for signing operations.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// let client = Client::new("bbr_live_xxxxx");
    /// let sign_client = client.sign();
    /// ```
    pub fn sign(&self) -> SignClient {
        SignClient::new(self.clone())
    }

    /// Get the orgs client for organization management.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// let client = Client::new("bbr_live_xxxxx");
    /// let orgs_client = client.orgs();
    /// ```
    pub fn orgs(&self) -> OrgsClient {
        OrgsClient::new(self.clone())
    }

    /// Get the audit client for audit log access.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use banhbaoring::Client;
    ///
    /// let client = Client::new("bbr_live_xxxxx");
    /// let audit_client = client.audit();
    /// ```
    pub fn audit(&self) -> AuditClient {
        AuditClient::new(self.clone())
    }

    /// Make an authenticated GET request.
    pub(crate) async fn get<T: serde::de::DeserializeOwned>(&self, path: &str) -> Result<T> {
        let url = format!("{}{}", self.base_url, path);

        let response = self
            .http
            .get(&url)
            .header(header::AUTHORIZATION, format!("Bearer {}", self.api_key))
            .header(header::CONTENT_TYPE, "application/json")
            .send()
            .await?;

        self.handle_response(response).await
    }

    /// Make an authenticated POST request.
    pub(crate) async fn post<T, B>(&self, path: &str, body: &B) -> Result<T>
    where
        T: serde::de::DeserializeOwned,
        B: serde::Serialize,
    {
        let url = format!("{}{}", self.base_url, path);

        let response = self
            .http
            .post(&url)
            .header(header::AUTHORIZATION, format!("Bearer {}", self.api_key))
            .header(header::CONTENT_TYPE, "application/json")
            .json(body)
            .send()
            .await?;

        self.handle_response(response).await
    }

    /// Make an authenticated DELETE request.
    pub(crate) async fn delete(&self, path: &str) -> Result<()> {
        let url = format!("{}{}", self.base_url, path);

        let response = self
            .http
            .delete(&url)
            .header(header::AUTHORIZATION, format!("Bearer {}", self.api_key))
            .send()
            .await?;

        if response.status().is_success() {
            Ok(())
        } else {
            Err(self.parse_error(response).await)
        }
    }

    async fn handle_response<T: serde::de::DeserializeOwned>(
        &self,
        response: reqwest::Response,
    ) -> Result<T> {
        if response.status().is_success() {
            let wrapper: ApiResponse<T> = response.json().await?;
            Ok(wrapper.data)
        } else {
            Err(self.parse_error(response).await)
        }
    }

    async fn parse_error(&self, response: reqwest::Response) -> BanhBaoRingError {
        let status = response.status().as_u16();

        // Handle specific status codes
        if status == 401 {
            return BanhBaoRingError::Unauthorized;
        }
        if status == 429 {
            return BanhBaoRingError::RateLimited;
        }

        let error: std::result::Result<ApiErrorResponse, _> = response.json().await;

        match error {
            Ok(e) => {
                // Check for quota exceeded
                if e.error.code == "quota_exceeded" {
                    return BanhBaoRingError::QuotaExceeded(e.error.message);
                }
                BanhBaoRingError::Api {
                    code: e.error.code,
                    message: e.error.message,
                    status_code: status,
                }
            }
            Err(_) => BanhBaoRingError::Api {
                code: "unknown".to_string(),
                message: "Unknown error".to_string(),
                status_code: status,
            },
        }
    }
}

#[derive(Deserialize)]
pub(crate) struct ApiResponse<T> {
    pub data: T,
}

#[derive(Deserialize)]
struct ApiErrorResponse {
    error: ApiError,
}

#[derive(Deserialize)]
struct ApiError {
    code: String,
    message: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_client_new() {
        let client = Client::new("test_key");
        assert_eq!(client.base_url(), DEFAULT_BASE_URL);
    }

    #[test]
    fn test_client_with_config() {
        let client = Client::with_config(
            "test_key",
            ClientConfig {
                base_url: Some("https://custom.api.com".to_string()),
                timeout: Some(Duration::from_secs(60)),
                user_agent: None,
            },
        );
        assert_eq!(client.base_url(), "https://custom.api.com");
    }

    #[test]
    fn test_default_config() {
        let config = ClientConfig::default();
        assert!(config.base_url.is_none());
        assert!(config.timeout.is_none());
        assert!(config.user_agent.is_none());
    }
}

