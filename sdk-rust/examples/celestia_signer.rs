//! Example: Using POPSigner's Celestia Client (Drop-in Replacement for Lumina)
//!
//! This example demonstrates how to use POPSigner as a drop-in replacement
//! for Lumina's celestia_client, without exposing private keys.
//!
//! Run with:
//! ```bash
//! export POPSIGNER_API_KEY=psk_live_xxxxx
//! cargo run --example celestia_signer -- my-key-name
//! ```

use popsigner::celestia::{CelestiaSigner, Signer};
use std::env;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key = env::var("POPSIGNER_API_KEY")
        .expect("POPSIGNER_API_KEY environment variable not set");

    // Get key name/ID from command line
    let args: Vec<String> = env::args().collect();
    let key_name_or_id = args.get(1).expect("Usage: celestia_signer <key-name-or-id>");

    println!("╔══════════════════════════════════════════════════════════════╗");
    println!("║     POPSigner Celestia Signer - Secure Remote Signing        ║");
    println!("╚══════════════════════════════════════════════════════════════╝");
    println!();

    // =========================================================================
    // CREATE SIGNER
    // =========================================================================

    println!("🔐 Creating CelestiaSigner with POPSigner backend...");
    println!("   Key: {}", key_name_or_id);
    println!();

    let signer = CelestiaSigner::new(&api_key, key_name_or_id, None).await?;

    println!("✅ Signer created successfully!");
    println!();

    // =========================================================================
    // SIGNER INFO
    // =========================================================================

    println!("📋 Signer Details:");
    println!("   Key Name:   {}", signer.key_name());
    println!("   Key ID:     {}", signer.key_id());
    println!("   Address:    {}", signer.address());
    println!("   Public Key: {}...", &signer.public_key_hex()[..16]);
    println!();

    // =========================================================================
    // SIGNING - Secure remote signing via POPSigner
    // =========================================================================

    println!("✍️  Testing signing capabilities...");

    let test_message = b"Hello, Celestia! This is a test message.";
    let signature = signer.sign(test_message).await?;

    println!("   Message:   \"{}\"", String::from_utf8_lossy(test_message));
    println!("   Signature: {} bytes", signature.len());
    println!("   Hex:       {}...", hex::encode(&signature[..16]));
    println!();

    // =========================================================================
    // PREHASHED SIGNING
    // =========================================================================

    println!("✍️  Testing prehashed signing...");

    use sha2::{Sha256, Digest};
    let digest = Sha256::digest(test_message);
    let signature2 = signer.sign_digest(&digest).await?;

    println!("   Digest:    {}...", hex::encode(&digest[..16]));
    println!("   Signature: {} bytes", signature2.len());
    println!();

    // =========================================================================
    // USAGE WITH CELESTIA CLIENT
    // =========================================================================

    println!("═══════════════════════════════════════════════════════════════");
    println!();
    println!("📖 Usage with Celestia Client:");
    println!();
    println!("   // With 'celestia' feature enabled:");
    println!("   use popsigner::celestia::Client;");
    println!();
    println!("   let client = Client::builder()");
    println!("       .rpc_url(\"ws://localhost:26658\")");
    println!("       .grpc_url(\"http://localhost:9090\")");
    println!("       .popsigner(api_key, \"my-key\")  // ✅ SECURE!");
    println!("       .build()");
    println!("       .await?;");
    println!();
    println!("   // Same API as Lumina - headers, blobs, state all work!");
    println!("   let header = client.header().head().await?;");
    println!("   client.blob().submit(&[blob], config).await?;");
    println!();

    Ok(())
}
