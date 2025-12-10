//! Integration tests for the BanhBaoRing client.

use banhbaoring::{Client, ClientConfig};
use std::time::Duration;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn test_client_creation() {
    let client = Client::new("test_api_key");
    // Client created successfully
    assert!(client.base_url().starts_with("https://"));
}

#[tokio::test]
async fn test_client_with_custom_config() {
    let client = Client::with_config(
        "test_api_key",
        ClientConfig {
            base_url: Some("https://custom.api.com".to_string()),
            timeout: Some(Duration::from_secs(60)),
            user_agent: Some("test-agent/1.0".to_string()),
        },
    );
    assert_eq!(client.base_url(), "https://custom.api.com");
}

#[tokio::test]
async fn test_get_request_with_auth() {
    let mock_server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/v1/keys"))
        .and(header("Authorization", "Bearer test_api_key"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": []
        })))
        .mount(&mock_server)
        .await;

    let client = Client::with_config(
        "test_api_key",
        ClientConfig {
            base_url: Some(mock_server.uri()),
            ..Default::default()
        },
    );

    let keys: Vec<banhbaoring::Key> = client.keys().list(None).await.unwrap();
    assert!(keys.is_empty());
}

#[tokio::test]
async fn test_unauthorized_error() {
    let mock_server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/v1/keys"))
        .respond_with(ResponseTemplate::new(401).set_body_json(serde_json::json!({
            "error": {
                "code": "unauthorized",
                "message": "Invalid API key"
            }
        })))
        .mount(&mock_server)
        .await;

    let client = Client::with_config(
        "invalid_key",
        ClientConfig {
            base_url: Some(mock_server.uri()),
            ..Default::default()
        },
    );

    let result: Result<Vec<banhbaoring::Key>, _> = client.keys().list(None).await;
    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        banhbaoring::BanhBaoRingError::Unauthorized
    ));
}

#[tokio::test]
async fn test_rate_limited_error() {
    let mock_server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/v1/keys"))
        .respond_with(ResponseTemplate::new(429).set_body_json(serde_json::json!({
            "error": {
                "code": "rate_limited",
                "message": "Too many requests"
            }
        })))
        .mount(&mock_server)
        .await;

    let client = Client::with_config(
        "test_key",
        ClientConfig {
            base_url: Some(mock_server.uri()),
            ..Default::default()
        },
    );

    let result: Result<Vec<banhbaoring::Key>, _> = client.keys().list(None).await;
    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        banhbaoring::BanhBaoRingError::RateLimited
    ));
}

#[tokio::test]
async fn test_api_error_parsing() {
    let mock_server = MockServer::start().await;
    let key_id_str = "00000000-0000-0000-0000-000000000001";

    Mock::given(method("GET"))
        .and(path(format!("/v1/keys/{}", key_id_str)))
        .respond_with(ResponseTemplate::new(404).set_body_json(serde_json::json!({
            "error": {
                "code": "key_not_found",
                "message": "Key does not exist"
            }
        })))
        .mount(&mock_server)
        .await;

    let client = Client::with_config(
        "test_key",
        ClientConfig {
            base_url: Some(mock_server.uri()),
            ..Default::default()
        },
    );

    let key_id = uuid::Uuid::parse_str(key_id_str).unwrap();
    let result = client.keys().get(&key_id).await;

    match result {
        Err(banhbaoring::BanhBaoRingError::Api {
            code,
            message,
            status_code,
        }) => {
            assert_eq!(code, "key_not_found");
            assert_eq!(message, "Key does not exist");
            assert_eq!(status_code, 404);
        }
        _ => panic!("Expected Api error"),
    }
}

