//! Error types for the BanhBaoRing SDK.
//!
//! This module provides a unified error type for all SDK operations,
//! with rich error information from the API.

use thiserror::Error;

/// Result type for BanhBaoRing operations.
pub type Result<T> = std::result::Result<T, BanhBaoRingError>;

/// Errors that can occur when using the BanhBaoRing SDK.
#[derive(Error, Debug)]
pub enum BanhBaoRingError {
    /// API error from the BanhBaoRing service.
    #[error("API error ({status_code}): [{code}] {message}")]
    Api {
        /// Error code from the API.
        code: String,
        /// Human-readable error message.
        message: String,
        /// HTTP status code.
        status_code: u16,
    },

    /// HTTP request error.
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),

    /// Decoding error (base64, etc).
    #[error("Decode error: {0}")]
    Decode(String),

    /// Authentication error.
    #[error("Unauthorized: invalid API key")]
    Unauthorized,

    /// Rate limit exceeded.
    #[error("Rate limit exceeded")]
    RateLimited,

    /// Quota exceeded.
    #[error("Quota exceeded: {0}")]
    QuotaExceeded(String),

    /// Key not found.
    #[error("Key not found: {0}")]
    KeyNotFound(String),

    /// Namespace not found.
    #[error("Namespace not found: {0}")]
    NamespaceNotFound(String),

    /// Organization not found.
    #[error("Organization not found: {0}")]
    OrgNotFound(String),

    /// Invalid request.
    #[error("Invalid request: {0}")]
    InvalidRequest(String),

    /// Signing error.
    #[error("Signing error: {0}")]
    SigningError(String),

    /// Batch operation partial failure.
    #[error("Batch operation had {failed} failures out of {total} requests")]
    BatchPartialFailure {
        /// Number of failed operations.
        failed: usize,
        /// Total number of operations.
        total: usize,
    },
}

impl BanhBaoRingError {
    /// Returns true if this is a retryable error.
    pub fn is_retryable(&self) -> bool {
        match self {
            BanhBaoRingError::RateLimited => true,
            BanhBaoRingError::Http(_) => true,
            BanhBaoRingError::Api { status_code, .. } => *status_code >= 500,
            _ => false,
        }
    }

    /// Returns true if this is an authentication error.
    pub fn is_auth_error(&self) -> bool {
        matches!(
            self,
            BanhBaoRingError::Unauthorized
                | BanhBaoRingError::Api { status_code: 401, .. }
                | BanhBaoRingError::Api { status_code: 403, .. }
        )
    }

    /// Returns the HTTP status code if available.
    pub fn status_code(&self) -> Option<u16> {
        match self {
            BanhBaoRingError::Api { status_code, .. } => Some(*status_code),
            BanhBaoRingError::Unauthorized => Some(401),
            BanhBaoRingError::RateLimited => Some(429),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_display() {
        let err = BanhBaoRingError::Api {
            code: "key_not_found".to_string(),
            message: "Key does not exist".to_string(),
            status_code: 404,
        };
        assert_eq!(
            err.to_string(),
            "API error (404): [key_not_found] Key does not exist"
        );
    }

    #[test]
    fn test_is_retryable() {
        let rate_limited = BanhBaoRingError::RateLimited;
        assert!(rate_limited.is_retryable());

        let server_error = BanhBaoRingError::Api {
            code: "internal".to_string(),
            message: "Internal server error".to_string(),
            status_code: 500,
        };
        assert!(server_error.is_retryable());

        let not_found = BanhBaoRingError::Api {
            code: "not_found".to_string(),
            message: "Not found".to_string(),
            status_code: 404,
        };
        assert!(!not_found.is_retryable());
    }

    #[test]
    fn test_is_auth_error() {
        let unauthorized = BanhBaoRingError::Unauthorized;
        assert!(unauthorized.is_auth_error());

        let api_401 = BanhBaoRingError::Api {
            code: "unauthorized".to_string(),
            message: "Invalid API key".to_string(),
            status_code: 401,
        };
        assert!(api_401.is_auth_error());
    }

    #[test]
    fn test_status_code() {
        let err = BanhBaoRingError::Api {
            code: "test".to_string(),
            message: "Test".to_string(),
            status_code: 500,
        };
        assert_eq!(err.status_code(), Some(500));

        let decode_err = BanhBaoRingError::Decode("bad base64".to_string());
        assert_eq!(decode_err.status_code(), None);
    }
}

