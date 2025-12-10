//! Integration tests for signing operations.

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use banhbaoring::{BatchSignItem, BatchSignRequest, Client, ClientConfig};
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn test_sign() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    let signature_bytes = vec![0x30, 0x45, 0x02, 0x21]; // Sample DER signature prefix
    let signature_b64 = BASE64.encode(&signature_bytes);

    Mock::given(method("POST"))
        .and(path(format!("/v1/keys/{}/sign", key_id)))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "signature": signature_b64,
                "public_key": "A1234567890abcdef"
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

    let result = client
        .sign()
        .sign(
            &uuid::Uuid::parse_str(key_id).unwrap(),
            b"hello world",
            false,
        )
        .await
        .unwrap();

    assert_eq!(result.signature, signature_bytes);
    assert_eq!(result.public_key, "A1234567890abcdef");
    assert_eq!(
        result.key_id,
        uuid::Uuid::parse_str(key_id).unwrap()
    );
}

#[tokio::test]
async fn test_sign_prehashed() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    let signature_bytes = vec![0x30, 0x44, 0x02, 0x20];
    let signature_b64 = BASE64.encode(&signature_bytes);

    Mock::given(method("POST"))
        .and(path(format!("/v1/keys/{}/sign", key_id)))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "signature": signature_b64,
                "public_key": "A1234567890abcdef"
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

    // Simulate a pre-hashed message (32 bytes)
    let hash = [0u8; 32];
    let result = client
        .sign()
        .sign(&uuid::Uuid::parse_str(key_id).unwrap(), &hash, true)
        .await
        .unwrap();

    assert_eq!(result.signature, signature_bytes);
}

#[tokio::test]
async fn test_sign_batch() {
    let mock_server = MockServer::start().await;

    let sig1 = BASE64.encode(&[1, 2, 3, 4]);
    let sig2 = BASE64.encode(&[5, 6, 7, 8]);
    let sig3 = BASE64.encode(&[9, 10, 11, 12]);
    let sig4 = BASE64.encode(&[13, 14, 15, 16]);

    Mock::given(method("POST"))
        .and(path("/v1/sign/batch"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "signatures": [
                    {
                        "key_id": "00000000-0000-0000-0000-000000000001",
                        "signature": sig1,
                        "public_key": "pubkey1"
                    },
                    {
                        "key_id": "00000000-0000-0000-0000-000000000002",
                        "signature": sig2,
                        "public_key": "pubkey2"
                    },
                    {
                        "key_id": "00000000-0000-0000-0000-000000000003",
                        "signature": sig3,
                        "public_key": "pubkey3"
                    },
                    {
                        "key_id": "00000000-0000-0000-0000-000000000004",
                        "signature": sig4,
                        "public_key": "pubkey4"
                    }
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

    let results = client
        .sign()
        .sign_batch(BatchSignRequest {
            requests: vec![
                BatchSignItem {
                    key_id: uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap(),
                    data: b"tx1".to_vec(),
                    prehashed: false,
                },
                BatchSignItem {
                    key_id: uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000002").unwrap(),
                    data: b"tx2".to_vec(),
                    prehashed: false,
                },
                BatchSignItem {
                    key_id: uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000003").unwrap(),
                    data: b"tx3".to_vec(),
                    prehashed: false,
                },
                BatchSignItem {
                    key_id: uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000004").unwrap(),
                    data: b"tx4".to_vec(),
                    prehashed: false,
                },
            ],
        })
        .await
        .unwrap();

    assert_eq!(results.len(), 4);
    assert_eq!(results[0].signature, vec![1, 2, 3, 4]);
    assert_eq!(results[1].signature, vec![5, 6, 7, 8]);
    assert_eq!(results[2].signature, vec![9, 10, 11, 12]);
    assert_eq!(results[3].signature, vec![13, 14, 15, 16]);
}

#[tokio::test]
async fn test_verify() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    Mock::given(method("POST"))
        .and(path(format!("/v1/keys/{}/verify", key_id)))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "valid": true
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

    let valid = client
        .sign()
        .verify(
            &uuid::Uuid::parse_str(key_id).unwrap(),
            b"hello world",
            &[0x30, 0x45, 0x02, 0x21],
            false,
        )
        .await
        .unwrap();

    assert!(valid);
}

#[tokio::test]
async fn test_verify_invalid_signature() {
    let mock_server = MockServer::start().await;
    let key_id = "00000000-0000-0000-0000-000000000001";

    Mock::given(method("POST"))
        .and(path(format!("/v1/keys/{}/verify", key_id)))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "data": {
                "valid": false
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

    let valid = client
        .sign()
        .verify(
            &uuid::Uuid::parse_str(key_id).unwrap(),
            b"tampered data",
            &[0x00, 0x00, 0x00, 0x00],
            false,
        )
        .await
        .unwrap();

    assert!(!valid);
}

