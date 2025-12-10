//! Integration tests for key management.

use banhbaoring::{Client, ClientConfig, CreateBatchRequest, CreateKeyRequest, Key};
use wiremock::matchers::{body_json, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn create_mock_key(id: &str, name: &str) -> serde_json::Value {
    serde_json::json!({
        "id": id,
        "name": name,
        "namespace_id": "00000000-0000-0000-0000-000000000002",
        "public_key": "A1234567890abcdef",
        "address": "celestia1abc123",
        "algorithm": "secp256k1",
        "exportable": false,
        "created_at": "2025-01-01T00:00:00Z"
    })
}

#[tokio::test]
async fn test_create_key() {
    let mock_server = MockServer::start().await;

    let expected_key = create_mock_key("00000000-0000-0000-0000-000000000001", "test-key");

    Mock::given(method("POST"))
        .and(path("/v1/keys"))
        .and(body_json(serde_json::json!({
            "name": "test-key",
            "namespace_id": "00000000-0000-0000-0000-000000000002",
            "algorithm": "secp256k1"
        })))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": expected_key
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

    let key = client
        .keys()
        .create(CreateKeyRequest {
            name: "test-key".to_string(),
            namespace_id: uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000002").unwrap(),
            algorithm: Some("secp256k1".to_string()),
            ..Default::default()
        })
        .await
        .unwrap();

    assert_eq!(key.name, "test-key");
    assert_eq!(key.algorithm, "secp256k1");
}

#[tokio::test]
async fn test_create_batch() {
    let mock_server = MockServer::start().await;

    Mock::given(method("POST"))
        .and(path("/v1/keys/batch"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "keys": [
                    create_mock_key("00000000-0000-0000-0000-000000000001", "worker-1"),
                    create_mock_key("00000000-0000-0000-0000-000000000002", "worker-2"),
                    create_mock_key("00000000-0000-0000-0000-000000000003", "worker-3"),
                    create_mock_key("00000000-0000-0000-0000-000000000004", "worker-4"),
                ]
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

    let keys = client
        .keys()
        .create_batch(CreateBatchRequest {
            prefix: "worker".to_string(),
            count: 4,
            namespace_id: uuid::Uuid::nil(),
            exportable: None,
        })
        .await
        .unwrap();

    assert_eq!(keys.len(), 4);
    assert_eq!(keys[0].name, "worker-1");
    assert_eq!(keys[3].name, "worker-4");
}

#[tokio::test]
async fn test_get_key() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    Mock::given(method("GET"))
        .and(path(format!("/v1/keys/{}", key_id)))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": create_mock_key(key_id, "my-key")
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

    let key = client
        .keys()
        .get(&uuid::Uuid::parse_str(key_id).unwrap())
        .await
        .unwrap();

    assert_eq!(key.name, "my-key");
}

#[tokio::test]
async fn test_list_keys() {
    let mock_server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/v1/keys"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": [
                create_mock_key("00000000-0000-0000-0000-000000000001", "key-1"),
                create_mock_key("00000000-0000-0000-0000-000000000002", "key-2"),
            ]
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

    let keys: Vec<Key> = client.keys().list(None).await.unwrap();
    assert_eq!(keys.len(), 2);
}

#[tokio::test]
async fn test_list_keys_with_namespace_filter() {
    let mock_server = MockServer::start().await;
    let namespace_id = "00000000-0000-0000-0000-000000000099";

    Mock::given(method("GET"))
        .and(path("/v1/keys"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": [
                create_mock_key("00000000-0000-0000-0000-000000000001", "filtered-key"),
            ]
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

    let ns_id = uuid::Uuid::parse_str(namespace_id).unwrap();
    let keys = client.keys().list(Some(&ns_id)).await.unwrap();
    assert_eq!(keys.len(), 1);
    assert_eq!(keys[0].name, "filtered-key");
}

#[tokio::test]
async fn test_delete_key() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    Mock::given(method("DELETE"))
        .and(path(format!("/v1/keys/{}", key_id)))
        .respond_with(ResponseTemplate::new(204))
        .mount(&mock_server)
        .await;

    let client = Client::with_config(
        "test_key",
        ClientConfig {
            base_url: Some(mock_server.uri()),
            ..Default::default()
        },
    );

    let result = client
        .keys()
        .delete(&uuid::Uuid::parse_str(key_id).unwrap())
        .await;

    assert!(result.is_ok());
}

