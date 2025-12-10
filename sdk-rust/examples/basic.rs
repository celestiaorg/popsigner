//! Basic usage example for the BanhBaoRing SDK.
//!
//! This example demonstrates:
//! - Creating a client
//! - Creating a key
//! - Signing data
//! - Verifying a signature
//!
//! Run with:
//! ```bash
//! BANHBAORING_API_KEY=bbr_live_xxx NAMESPACE_ID=... cargo run --example basic
//! ```

use banhbaoring::{Client, CreateKeyRequest};
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key = std::env::var("BANHBAORING_API_KEY")
        .expect("BANHBAORING_API_KEY environment variable required");
    let namespace_id: Uuid = std::env::var("NAMESPACE_ID")
        .expect("NAMESPACE_ID environment variable required")
        .parse()
        .expect("Invalid NAMESPACE_ID format");

    // Create client
    println!("Creating BanhBaoRing client...");
    let client = Client::new(&api_key);

    // Create a key
    println!("\nCreating a new key...");
    let key = client
        .keys()
        .create(CreateKeyRequest {
            name: format!("example-key-{}", Uuid::new_v4().to_string()[..8].to_string()),
            namespace_id,
            algorithm: Some("secp256k1".to_string()),
            exportable: Some(false),
            metadata: None,
        })
        .await?;

    println!("Created key:");
    println!("  ID:        {}", key.id);
    println!("  Name:      {}", key.name);
    println!("  Address:   {}", key.address);
    println!("  Algorithm: {}", key.algorithm);

    // Sign some data
    println!("\nSigning data...");
    let data = b"Hello, Celestia!";
    let sign_result = client.sign().sign(&key.id, data, false).await?;

    println!("Signature:");
    println!(
        "  Bytes:      {} bytes",
        sign_result.signature.len()
    );
    println!(
        "  Hex (first 16): {}",
        sign_result
            .signature
            .iter()
            .take(16)
            .map(|b| format!("{:02x}", b))
            .collect::<String>()
    );
    println!("  Public Key: {}", sign_result.public_key);

    // Verify the signature
    println!("\nVerifying signature...");
    let valid = client
        .sign()
        .verify(&key.id, data, &sign_result.signature, false)
        .await?;
    println!("Signature valid: {}", valid);

    // List all keys
    println!("\nListing all keys in namespace...");
    let keys = client.keys().list(Some(&namespace_id)).await?;
    println!("Found {} keys:", keys.len());
    for k in &keys {
        println!("  - {} ({})", k.name, k.address);
    }

    // Clean up - delete the key we created
    println!("\nCleaning up - deleting test key...");
    client.keys().delete(&key.id).await?;
    println!("Key deleted successfully.");

    println!("\nDone!");
    Ok(())
}

