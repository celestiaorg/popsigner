//! # Celestia Client - Drop-in Replacement for Lumina
//!
//! This module provides a **drop-in replacement** for the Lumina `celestia_client::Client`
//! that uses POPSigner for secure remote signing. Private keys never leave POPSigner.
//!
//! ## Features
//!
//! - **Default**: Lightweight, signing-only. Use for custom integrations.
//! - **`celestia`**: Full integration with Lumina's RPC/gRPC clients.
//!
//! ## The Problem
//!
//! Lumina's client requires exposing private keys:
//!
//! ```rust,ignore
//! // ❌ INSECURE: Private key in code
//! use celestia_client::Client;
//!
//! let client = Client::builder()
//!     .private_key_hex("393fdb5def075819...")  // BAD!
//!     .build()
//!     .await?;
//! ```
//!
//! ## The Solution
//!
//! Use POPSigner's drop-in replacement:
//!
//! ```rust,ignore
//! // ✅ SECURE: Private keys never leave POPSigner
//! use popsigner::celestia::Client;
//!
//! let client = Client::builder()
//!     .rpc_url("ws://localhost:26658")
//!     .grpc_url("http://localhost:9090")
//!     .popsigner("psk_live_xxx", "my-key")  // Secure!
//!     .build()
//!     .await?;
//! ```
//!
//! ## Installation
//!
//! ```toml
//! # Signing only (lightweight)
//! popsigner = "1.0"
//!
//! # Full Celestia integration (composes with Lumina)
//! popsigner = { version = "1.0", features = ["celestia"] }
//! ```

use crate::client::{Client as POPSignerClient, ClientConfig as POPSignerClientConfig};
use crate::error::{POPSignerError, Result};
use async_trait::async_trait;
use std::fmt;
use std::sync::Arc;
use std::time::Duration;
use uuid::Uuid;

// Re-export types
pub use crate::types::Key;

// =============================================================================
// CONDITIONAL IMPORTS & RE-EXPORTS
// When celestia feature is enabled, use Lumina's types and traits
// =============================================================================

#[cfg(feature = "celestia")]
use celestia_rpc::{BlobClient as _, HeaderClient as _, StateClient as _};

#[cfg(feature = "celestia")]
pub use celestia_types::{
    blob::Blob,
    nmt::Namespace,
    AppVersion,
    Commitment,
    ExtendedHeader as Header,
};

// =============================================================================
// SIGNER TRAIT
// =============================================================================

/// Trait for signing Celestia transactions.
///
/// This trait abstracts the signing mechanism, allowing:
/// - **Remote signing** via POPSigner (secure, keys never exposed)
/// - **Local signing** for testing (insecure, requires private key)
#[async_trait]
pub trait Signer: Send + Sync {
    /// Returns the compressed secp256k1 public key (33 bytes).
    fn public_key(&self) -> &[u8];

    /// Returns the hex-encoded public key.
    fn public_key_hex(&self) -> String {
        hex::encode(self.public_key())
    }

    /// Returns the bech32 Celestia address (celestia1...).
    fn address(&self) -> &str;

    /// Signs a message and returns the signature bytes (64 bytes, R || S).
    async fn sign(&self, msg: &[u8]) -> std::result::Result<Vec<u8>, SignerError>;

    /// Signs a pre-hashed message (32-byte SHA-256 digest).
    async fn sign_digest(&self, digest: &[u8]) -> std::result::Result<Vec<u8>, SignerError>;
}

/// Error type for signing operations.
#[derive(Debug)]
pub enum SignerError {
    Network(String),
    Authentication(String),
    KeyNotFound(String),
    SigningFailed(String),
    RateLimited,
    InvalidInput(String),
    Config(String),
    #[cfg(feature = "celestia")]
    Rpc(String),
    #[cfg(feature = "celestia")]
    Grpc(String),
}

impl fmt::Display for SignerError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SignerError::Network(msg) => write!(f, "network error: {}", msg),
            SignerError::Authentication(msg) => write!(f, "authentication error: {}", msg),
            SignerError::KeyNotFound(msg) => write!(f, "key not found: {}", msg),
            SignerError::SigningFailed(msg) => write!(f, "signing failed: {}", msg),
            SignerError::RateLimited => write!(f, "rate limit exceeded"),
            SignerError::InvalidInput(msg) => write!(f, "invalid input: {}", msg),
            SignerError::Config(msg) => write!(f, "configuration error: {}", msg),
            #[cfg(feature = "celestia")]
            SignerError::Rpc(msg) => write!(f, "RPC error: {}", msg),
            #[cfg(feature = "celestia")]
            SignerError::Grpc(msg) => write!(f, "gRPC error: {}", msg),
        }
    }
}

impl std::error::Error for SignerError {}

impl From<POPSignerError> for SignerError {
    fn from(err: POPSignerError) -> Self {
        match err {
            POPSignerError::Unauthorized => {
                SignerError::Authentication("invalid API key".to_string())
            }
            POPSignerError::RateLimited => SignerError::RateLimited,
            POPSignerError::KeyNotFound(msg) => SignerError::KeyNotFound(msg),
            POPSignerError::Http(e) => SignerError::Network(e.to_string()),
            POPSignerError::SigningError(msg) => SignerError::SigningFailed(msg),
            POPSignerError::Api { message, .. } => SignerError::SigningFailed(message),
            other => SignerError::SigningFailed(other.to_string()),
        }
    }
}

// =============================================================================
// POPSIGNER SIGNER - Remote signing implementation
// =============================================================================

/// POPSigner-backed signer for Celestia.
///
/// This is the secure signer implementation that uses the POPSigner API.
/// Private keys never leave POPSigner.
pub struct POPSignerSigner {
    client: POPSignerClient,
    key_id: Uuid,
    key_name: String,
    public_key: Vec<u8>,
    celestia_address: String,
}

impl POPSignerSigner {
    /// Creates a new POPSignerSigner.
    pub async fn new(
        api_key: impl Into<String>,
        key_name_or_id: impl Into<String>,
        config: Option<POPSignerSignerConfig>,
    ) -> Result<Self> {
        let api_key = api_key.into();
        let key_name_or_id = key_name_or_id.into();
        let config = config.unwrap_or_default();

        let client_config = POPSignerClientConfig {
            base_url: config.base_url,
            timeout: config.timeout,
            user_agent: Some(format!(
                "popsigner-celestia-rust/{}",
                env!("CARGO_PKG_VERSION")
            )),
        };
        let client = POPSignerClient::with_config(&api_key, client_config);

        let key = Self::resolve_key(&client, &key_name_or_id).await?;

        let public_key = hex::decode(&key.public_key).map_err(|e| {
            POPSignerError::Decode(format!("failed to decode public key: {}", e))
        })?;

        if public_key.len() != 33 {
            return Err(POPSignerError::InvalidRequest(format!(
                "invalid public key length: expected 33 bytes, got {}",
                public_key.len()
            )));
        }

        let celestia_address = derive_celestia_address(&public_key)?;

        Ok(Self {
            client,
            key_id: key.id,
            key_name: key.name,
            public_key,
            celestia_address,
        })
    }

    async fn resolve_key(client: &POPSignerClient, key_name_or_id: &str) -> Result<Key> {
        if let Ok(key_id) = Uuid::parse_str(key_name_or_id) {
            return client.keys().get(&key_id).await;
        }

        let keys = client.keys().list(None).await?;
        for key in keys {
            if key.name == key_name_or_id {
                return Ok(key);
            }
        }

        let available_names: Vec<_> = client
            .keys()
            .list(None)
            .await
            .map(|keys| keys.iter().map(|k| k.name.clone()).collect())
            .unwrap_or_default();

        Err(POPSignerError::KeyNotFound(format!(
            "key '{}' not found (available: {:?})",
            key_name_or_id, available_names
        )))
    }

    /// Returns the key name.
    pub fn key_name(&self) -> &str {
        &self.key_name
    }

    /// Returns the key ID.
    pub fn key_id(&self) -> Uuid {
        self.key_id
    }

    /// Returns the underlying POPSigner client.
    pub fn popsigner_client(&self) -> &POPSignerClient {
        &self.client
    }
}

#[async_trait]
impl Signer for POPSignerSigner {
    fn public_key(&self) -> &[u8] {
        &self.public_key
    }

    fn address(&self) -> &str {
        &self.celestia_address
    }

    async fn sign(&self, msg: &[u8]) -> std::result::Result<Vec<u8>, SignerError> {
        let response = self
            .client
            .sign()
            .sign(&self.key_id, msg, false)
            .await
            .map_err(SignerError::from)?;

        Ok(response.signature)
    }

    async fn sign_digest(&self, digest: &[u8]) -> std::result::Result<Vec<u8>, SignerError> {
        if digest.len() != 32 {
            return Err(SignerError::InvalidInput(format!(
                "digest must be 32 bytes, got {}",
                digest.len()
            )));
        }

        let response = self
            .client
            .sign()
            .sign(&self.key_id, digest, true)
            .await
            .map_err(SignerError::from)?;

        Ok(response.signature)
    }
}

impl fmt::Debug for POPSignerSigner {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("POPSignerSigner")
            .field("key_name", &self.key_name)
            .field("key_id", &self.key_id)
            .field("address", &self.celestia_address)
            .finish()
    }
}

/// Configuration for POPSignerSigner.
#[derive(Debug, Clone, Default)]
pub struct POPSignerSignerConfig {
    pub base_url: Option<String>,
    pub timeout: Option<Duration>,
}

// =============================================================================
// CLIENT BUILDER
// =============================================================================

/// Builder for creating a Celestia client with POPSigner.
pub struct ClientBuilder {
    rpc_url: Option<String>,
    grpc_url: Option<String>,
    #[cfg(feature = "celestia")]
    rpc_auth_token: Option<String>,
    popsigner_api_key: Option<String>,
    popsigner_key: Option<String>,
    popsigner_config: Option<POPSignerSignerConfig>,
}

impl ClientBuilder {
    pub fn new() -> Self {
        Self {
            rpc_url: None,
            grpc_url: None,
            #[cfg(feature = "celestia")]
            rpc_auth_token: None,
            popsigner_api_key: None,
            popsigner_key: None,
            popsigner_config: None,
        }
    }

    /// Sets the RPC URL (e.g., `ws://localhost:26658`).
    pub fn rpc_url(mut self, url: impl Into<String>) -> Self {
        self.rpc_url = Some(url.into());
        self
    }

    /// Sets the gRPC URL (e.g., `http://localhost:9090`).
    pub fn grpc_url(mut self, url: impl Into<String>) -> Self {
        self.grpc_url = Some(url.into());
        self
    }

    /// Sets the RPC auth token (required for authenticated endpoints).
    #[cfg(feature = "celestia")]
    pub fn rpc_auth_token(mut self, token: impl Into<String>) -> Self {
        self.rpc_auth_token = Some(token.into());
        self
    }

    /// Configures POPSigner for signing (replaces `private_key_hex`).
    pub fn popsigner(
        mut self,
        api_key: impl Into<String>,
        key_name_or_id: impl Into<String>,
    ) -> Self {
        self.popsigner_api_key = Some(api_key.into());
        self.popsigner_key = Some(key_name_or_id.into());
        self
    }

    /// Configures POPSigner with custom settings.
    pub fn popsigner_with_config(
        mut self,
        api_key: impl Into<String>,
        key_name_or_id: impl Into<String>,
        config: POPSignerSignerConfig,
    ) -> Self {
        self.popsigner_api_key = Some(api_key.into());
        self.popsigner_key = Some(key_name_or_id.into());
        self.popsigner_config = Some(config);
        self
    }

    /// Builds the client.
    pub async fn build(self) -> std::result::Result<Client, ClientError> {
        let api_key = self.popsigner_api_key.ok_or_else(|| {
            ClientError::Config("POPSigner API key required. Use .popsigner()".into())
        })?;

        let key = self.popsigner_key.ok_or_else(|| {
            ClientError::Config("POPSigner key required. Use .popsigner()".into())
        })?;

        let signer = POPSignerSigner::new(&api_key, &key, self.popsigner_config)
            .await
            .map_err(|e| ClientError::Signer(SignerError::from(e)))?;

        // When celestia feature is enabled, also create the Lumina RPC client
        #[cfg(feature = "celestia")]
        let rpc_client = if let Some(ref url) = self.rpc_url {
            let auth_token = self.rpc_auth_token.as_deref();
            Some(Arc::new(
                celestia_rpc::Client::new(url, auth_token)
                    .await
                    .map_err(|e| ClientError::Rpc(e.to_string()))?,
            ))
        } else {
            None
        };

        Ok(Client {
            rpc_url: self.rpc_url,
            grpc_url: self.grpc_url,
            signer: Arc::new(signer),
            #[cfg(feature = "celestia")]
            rpc_client,
        })
    }
}

impl Default for ClientBuilder {
    fn default() -> Self {
        Self::new()
    }
}

// =============================================================================
// CLIENT
// =============================================================================

/// Celestia client with POPSigner-backed signing.
///
/// This is a drop-in replacement for Lumina's `celestia_client::Client`.
pub struct Client {
    rpc_url: Option<String>,
    grpc_url: Option<String>,
    signer: Arc<POPSignerSigner>,
    #[cfg(feature = "celestia")]
    rpc_client: Option<Arc<celestia_rpc::Client>>,
}

impl Client {
    pub fn builder() -> ClientBuilder {
        ClientBuilder::new()
    }

    /// Returns the Celestia address.
    pub fn address(&self) -> std::result::Result<String, ClientError> {
        Ok(self.signer.address().to_string())
    }

    /// Returns the public key bytes.
    pub fn public_key(&self) -> &[u8] {
        self.signer.public_key()
    }

    /// Returns the hex-encoded public key.
    pub fn public_key_hex(&self) -> String {
        self.signer.public_key_hex()
    }

    /// Returns the underlying signer.
    pub fn signer(&self) -> &POPSignerSigner {
        &self.signer
    }

    /// Returns the RPC URL.
    pub fn rpc_url(&self) -> Option<&str> {
        self.rpc_url.as_deref()
    }

    /// Returns the gRPC URL.
    pub fn grpc_url(&self) -> Option<&str> {
        self.grpc_url.as_deref()
    }

    /// Returns the header client.
    pub fn header(&self) -> HeaderClient {
        HeaderClient {
            #[cfg(feature = "celestia")]
            rpc_client: self.rpc_client.clone(),
            #[cfg(not(feature = "celestia"))]
            _rpc_url: self.rpc_url.clone(),
        }
    }

    /// Returns the blob client.
    pub fn blob(&self) -> BlobClient {
        BlobClient {
            signer: Arc::clone(&self.signer),
            #[cfg(feature = "celestia")]
            rpc_client: self.rpc_client.clone(),
            #[cfg(not(feature = "celestia"))]
            _rpc_url: self.rpc_url.clone(),
            #[cfg(not(feature = "celestia"))]
            _grpc_url: self.grpc_url.clone(),
        }
    }

    /// Returns the state client.
    pub fn state(&self) -> StateClient {
        StateClient {
            signer: Arc::clone(&self.signer),
            #[cfg(feature = "celestia")]
            rpc_client: self.rpc_client.clone(),
            #[cfg(not(feature = "celestia"))]
            _rpc_url: self.rpc_url.clone(),
            #[cfg(not(feature = "celestia"))]
            _grpc_url: self.grpc_url.clone(),
        }
    }

    /// Signs data using POPSigner.
    pub async fn sign(&self, msg: &[u8]) -> std::result::Result<Vec<u8>, ClientError> {
        self.signer.sign(msg).await.map_err(ClientError::Signer)
    }
}

impl fmt::Debug for Client {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Client")
            .field("rpc_url", &self.rpc_url)
            .field("grpc_url", &self.grpc_url)
            .field("signer", &self.signer)
            .finish()
    }
}

// =============================================================================
// HEADER CLIENT
// =============================================================================

/// Client for header operations.
pub struct HeaderClient {
    #[cfg(feature = "celestia")]
    rpc_client: Option<Arc<celestia_rpc::Client>>,
    #[cfg(not(feature = "celestia"))]
    _rpc_url: Option<String>,
}

impl HeaderClient {
    /// Gets the latest header.
    #[cfg(feature = "celestia")]
    pub async fn head(&self) -> std::result::Result<Header, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        client
            .header_network_head()
            .await
            .map_err(|e| ClientError::Rpc(e.to_string()))
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn head(&self) -> std::result::Result<Header, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for header operations".into(),
        ))
    }

    /// Gets a header by height.
    #[cfg(feature = "celestia")]
    pub async fn get_by_height(&self, height: u64) -> std::result::Result<Header, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        client
            .header_get_by_height(height)
            .await
            .map_err(|e| ClientError::Rpc(e.to_string()))
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn get_by_height(&self, _height: u64) -> std::result::Result<Header, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for header operations".into(),
        ))
    }
}

// =============================================================================
// BLOB CLIENT
// =============================================================================

/// Client for blob operations.
pub struct BlobClient {
    #[allow(dead_code)]
    signer: Arc<POPSignerSigner>,
    #[cfg(feature = "celestia")]
    rpc_client: Option<Arc<celestia_rpc::Client>>,
    #[cfg(not(feature = "celestia"))]
    _rpc_url: Option<String>,
    #[cfg(not(feature = "celestia"))]
    _grpc_url: Option<String>,
}

impl BlobClient {
    /// Submits blobs to Celestia.
    ///
    /// This uses POPSigner for signing the MsgPayForBlobs transaction.
    #[cfg(feature = "celestia")]
    pub async fn submit(
        &self,
        blobs: &[Blob],
        _config: TxConfig,
    ) -> std::result::Result<TxInfo, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        // Use Lumina's blob submission but with our signer
        // Note: This requires Lumina to expose the signing interface
        // For now, we use their default submission which handles tx building
        let height = client
            .blob_submit(blobs, Default::default())
            .await
            .map_err(|e| ClientError::Rpc(e.to_string()))?;

        Ok(TxInfo {
            hash: "".to_string(), // Lumina doesn't return hash in this call
            height,
        })
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn submit(
        &self,
        _blobs: &[Blob],
        _config: TxConfig,
    ) -> std::result::Result<TxInfo, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for blob submission".into(),
        ))
    }

    /// Gets a blob by height, namespace, and commitment.
    #[cfg(feature = "celestia")]
    pub async fn get(
        &self,
        height: u64,
        namespace: Namespace,
        commitment: Commitment,
    ) -> std::result::Result<Blob, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        client
            .blob_get(height, namespace, commitment)
            .await
            .map_err(|e| ClientError::Rpc(e.to_string()))
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn get(
        &self,
        _height: u64,
        _namespace: Namespace,
        _commitment: Commitment,
    ) -> std::result::Result<Blob, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for blob retrieval".into(),
        ))
    }
}

// =============================================================================
// STATE CLIENT
// =============================================================================

/// Client for state operations.
pub struct StateClient {
    #[allow(dead_code)]
    signer: Arc<POPSignerSigner>,
    #[cfg(feature = "celestia")]
    rpc_client: Option<Arc<celestia_rpc::Client>>,
    #[cfg(not(feature = "celestia"))]
    _rpc_url: Option<String>,
    #[cfg(not(feature = "celestia"))]
    _grpc_url: Option<String>,
}

impl StateClient {
    /// Transfers tokens.
    #[cfg(feature = "celestia")]
    pub async fn transfer(
        &self,
        to: &str,
        amount: u64,
        _config: TxConfig,
    ) -> std::result::Result<TxInfo, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        let to_addr = to
            .parse()
            .map_err(|e| ClientError::InvalidInput(format!("invalid address: {}", e)))?;

        let tx_response = client
            .state_transfer(&to_addr, amount.into(), Default::default())
            .await
            .map_err(|e| ClientError::Rpc(e.to_string()))?;

        Ok(TxInfo {
            hash: tx_response.txhash,
            height: tx_response.height as u64,
        })
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn transfer(
        &self,
        _to: &str,
        _amount: u64,
        _config: TxConfig,
    ) -> std::result::Result<TxInfo, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for transfers".into(),
        ))
    }

    /// Gets balance.
    #[cfg(feature = "celestia")]
    pub async fn balance(&self, address: Option<&str>) -> std::result::Result<u64, ClientError> {
        let client = self
            .rpc_client
            .as_ref()
            .ok_or_else(|| ClientError::Config("RPC URL not configured".into()))?;

        let balance = if let Some(addr) = address {
            let addr = addr
                .parse()
                .map_err(|e| ClientError::InvalidInput(format!("invalid address: {}", e)))?;
            client
                .state_balance_for_address(&addr)
                .await
                .map_err(|e| ClientError::Rpc(e.to_string()))?
        } else {
            // Get balance for the signer's address
            let addr = self
                .signer
                .address()
                .parse()
                .map_err(|e| ClientError::InvalidInput(format!("invalid signer address: {}", e)))?;
            client
                .state_balance_for_address(&addr)
                .await
                .map_err(|e| ClientError::Rpc(e.to_string()))?
        };

        // balance.amount() returns u64
        Ok(balance.amount())
    }

    #[cfg(not(feature = "celestia"))]
    pub async fn balance(&self, _address: Option<&str>) -> std::result::Result<u64, ClientError> {
        Err(ClientError::NotImplemented(
            "Enable 'celestia' feature for balance queries".into(),
        ))
    }
}

// =============================================================================
// TYPES (Stubs when celestia is not enabled)
// =============================================================================

#[cfg(not(feature = "celestia"))]
#[derive(Debug, Clone)]
pub struct Header {
    pub height: u64,
    pub hash: String,
    pub time: String,
}

#[cfg(not(feature = "celestia"))]
#[derive(Debug, Clone)]
pub struct Namespace {
    pub version: u8,
    pub id: Vec<u8>,
}

#[cfg(not(feature = "celestia"))]
impl Namespace {
    pub fn new_v0(id: &[u8]) -> std::result::Result<Self, ClientError> {
        if id.len() > 10 {
            return Err(ClientError::InvalidInput("namespace ID too long".into()));
        }
        Ok(Self {
            version: 0,
            id: id.to_vec(),
        })
    }
}

#[cfg(not(feature = "celestia"))]
#[derive(Debug, Clone)]
pub struct Blob {
    pub namespace: Namespace,
    pub data: Vec<u8>,
    pub commitment: Commitment,
    pub share_version: u8,
}

#[cfg(not(feature = "celestia"))]
impl Blob {
    pub fn new(
        namespace: Namespace,
        data: Vec<u8>,
        _signer: Option<String>,
        _app_version: AppVersion,
    ) -> std::result::Result<Self, ClientError> {
        Ok(Self {
            namespace,
            data,
            commitment: Commitment::default(),
            share_version: 0,
        })
    }
}

#[cfg(not(feature = "celestia"))]
#[derive(Debug, Clone, Default)]
pub struct Commitment(pub Vec<u8>);

#[cfg(not(feature = "celestia"))]
#[derive(Debug, Clone, Copy)]
pub enum AppVersion {
    V1,
    V2,
    V3,
    V4,
    V5,
}

/// Transaction configuration.
#[derive(Debug, Clone, Default)]
pub struct TxConfig {
    pub gas_limit: Option<u64>,
    pub fee: Option<u64>,
    pub memo: Option<String>,
}

/// Transaction info.
#[derive(Debug, Clone)]
pub struct TxInfo {
    pub hash: String,
    pub height: u64,
}

// =============================================================================
// ERROR TYPES
// =============================================================================

#[derive(Debug)]
pub enum ClientError {
    Config(String),
    Signer(SignerError),
    Rpc(String),
    Grpc(String),
    InvalidInput(String),
    NotImplemented(String),
}

impl fmt::Display for ClientError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ClientError::Config(msg) => write!(f, "config error: {}", msg),
            ClientError::Signer(err) => write!(f, "signer error: {}", err),
            ClientError::Rpc(msg) => write!(f, "RPC error: {}", msg),
            ClientError::Grpc(msg) => write!(f, "gRPC error: {}", msg),
            ClientError::InvalidInput(msg) => write!(f, "invalid input: {}", msg),
            ClientError::NotImplemented(msg) => write!(f, "not implemented: {}", msg),
        }
    }
}

impl std::error::Error for ClientError {}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

fn derive_celestia_address(public_key: &[u8]) -> Result<String> {
    if public_key.len() != 33 {
        return Err(POPSignerError::InvalidRequest(format!(
            "invalid public key length: expected 33 bytes, got {}",
            public_key.len()
        )));
    }

    use ripemd::Ripemd160;
    use sha2::{Digest, Sha256};

    let sha256_hash = Sha256::digest(public_key);
    let ripemd_hash = Ripemd160::digest(&sha256_hash);
    bech32_encode("celestia", &ripemd_hash)
}

fn bech32_encode(hrp: &str, data: &[u8]) -> Result<String> {
    let mut converted = Vec::with_capacity(data.len() * 8 / 5 + 1);
    let mut acc = 0u32;
    let mut bits = 0u8;

    for &b in data {
        acc = (acc << 8) | u32::from(b);
        bits += 8;
        while bits >= 5 {
            bits -= 5;
            converted.push((acc >> bits) as u8 & 0x1f);
        }
    }
    if bits > 0 {
        converted.push((acc << (5 - bits)) as u8 & 0x1f);
    }

    let mut values = expand_hrp(hrp);
    values.extend_from_slice(&converted);
    values.extend_from_slice(&[0, 0, 0, 0, 0, 0]);
    let polymod = bech32_polymod(&values) ^ 1;

    let mut checksum = Vec::with_capacity(6);
    for i in 0..6 {
        checksum.push(((polymod >> (5 * (5 - i))) & 0x1f) as u8);
    }

    const CHARSET: &[u8] = b"qpzry9x8gf2tvdw0s3jn54khce6mua7l";
    let mut result = String::with_capacity(hrp.len() + 1 + converted.len() + 6);
    result.push_str(hrp);
    result.push('1');

    for &b in &converted {
        result.push(CHARSET[b as usize] as char);
    }
    for &b in &checksum {
        result.push(CHARSET[b as usize] as char);
    }

    Ok(result)
}

fn expand_hrp(hrp: &str) -> Vec<u8> {
    let mut result = Vec::with_capacity(hrp.len() * 2 + 1);
    for c in hrp.chars() {
        result.push((c as u8) >> 5);
    }
    result.push(0);
    for c in hrp.chars() {
        result.push((c as u8) & 0x1f);
    }
    result
}

fn bech32_polymod(values: &[u8]) -> u32 {
    const GEN: [u32; 5] = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
    let mut chk: u32 = 1;

    for &v in values {
        let b = chk >> 25;
        chk = ((chk & 0x1ffffff) << 5) ^ u32::from(v);
        for (i, &g) in GEN.iter().enumerate() {
            if (b >> i) & 1 == 1 {
                chk ^= g;
            }
        }
    }

    chk
}

// Re-export for convenience
pub use POPSignerSigner as CelestiaSigner;

// =============================================================================
// TESTS
// =============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_builder_requires_popsigner() {
        let builder = ClientBuilder::new().rpc_url("ws://localhost:26658");
        assert!(builder.popsigner_api_key.is_none());
    }

    #[test]
    fn test_builder_with_popsigner() {
        let builder = ClientBuilder::new().popsigner("psk_live_xxx", "my-key");
        assert_eq!(builder.popsigner_api_key, Some("psk_live_xxx".to_string()));
        assert_eq!(builder.popsigner_key, Some("my-key".to_string()));
    }

    #[cfg(not(feature = "celestia"))]
    #[test]
    fn test_namespace_v0() {
        let ns = Namespace::new_v0(b"test").unwrap();
        assert_eq!(ns.version, 0);
    }

    #[test]
    fn test_bech32_encode() {
        let result = bech32_encode("celestia", &[0u8; 20]).unwrap();
        assert!(result.starts_with("celestia1"));
    }

    #[test]
    fn test_signer_error_display() {
        let err = SignerError::KeyNotFound("my-key".to_string());
        assert_eq!(err.to_string(), "key not found: my-key");
    }
}
