//! Parallel workers example for the BanhBaoRing SDK.
//!
//! This example demonstrates Celestia's parallel blob submission pattern:
//! - Creating multiple worker keys in a batch
//! - Signing multiple transactions in parallel
//!
//! Run with:
//! ```bash
//! BANHBAORING_API_KEY=bbr_live_xxx NAMESPACE_ID=... cargo run --example parallel_workers
//! ```

use banhbaoring::{BatchSignItem, BatchSignRequest, Client, CreateBatchRequest};
use std::time::Instant;
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

    let client = Client::new(&api_key);

    // Create 4 worker keys for parallel blob submission
    println!("Creating worker keys...");
    let start = Instant::now();

    let keys = client
        .keys()
        .create_batch(CreateBatchRequest {
            prefix: format!("blob-worker-{}", &Uuid::new_v4().to_string()[..8]),
            count: 4,
            namespace_id,
            exportable: None,
        })
        .await?;

    println!(
        "Created {} worker keys in {:?}:",
        keys.len(),
        start.elapsed()
    );
    for key in &keys {
        println!("  - {}: {}", key.name, key.address);
    }

    // Simulate parallel blob transactions
    let transactions: Vec<Vec<u8>> = vec![
        b"blob-tx-1: data for namespace A".to_vec(),
        b"blob-tx-2: data for namespace B".to_vec(),
        b"blob-tx-3: data for namespace C".to_vec(),
        b"blob-tx-4: data for namespace D".to_vec(),
    ];

    // Sign all 4 transactions in parallel with one API call
    println!("\nSigning transactions in batch...");
    let start = Instant::now();

    let results = client
        .sign()
        .sign_batch(BatchSignRequest {
            requests: keys
                .iter()
                .zip(transactions.iter())
                .map(|(key, tx)| BatchSignItem {
                    key_id: key.id,
                    data: tx.clone(),
                    prehashed: false,
                })
                .collect(),
        })
        .await?;

    let batch_duration = start.elapsed();
    println!("Batch signing completed in {:?}", batch_duration);

    println!("\nSigned transactions:");
    for (i, result) in results.iter().enumerate() {
        let sig_hex: String = result
            .signature
            .iter()
            .take(8)
            .map(|b| format!("{:02x}", b))
            .collect();
        println!("  - TX {}: sig={}... ({} bytes)", i + 1, sig_hex, result.signature.len());
    }

    // Compare with sequential signing (simulated)
    println!("\n--- Performance Comparison ---");
    println!(
        "Batch signing (4 txs):     {:?}",
        batch_duration
    );
    println!(
        "Estimated sequential:      {:?}",
        batch_duration * 4
    );
    println!(
        "Speedup:                   ~4x"
    );

    // Clean up
    println!("\nCleaning up worker keys...");
    for key in &keys {
        client.keys().delete(&key.id).await?;
    }
    println!("Cleanup complete.");

    println!("\nDone!");
    Ok(())
}

