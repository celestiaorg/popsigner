// Example: Parallel Workers with BanhBaoRing SDK
//
// This example demonstrates how to use BanhBaoRing for parallel blob submission,
// a critical pattern for Celestia sequencers. It shows both the batch API
// approach and manual goroutine parallelism.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/banhbaoring/sdk-go"
	"github.com/google/uuid"
)

const numWorkers = 4

func main() {
	// Initialize the client
	apiKey := os.Getenv("BANHBAORING_API_KEY")
	if apiKey == "" {
		log.Fatal("BANHBAORING_API_KEY environment variable is required")
	}

	client := banhbaoring.NewClient(apiKey)
	ctx := context.Background()

	// Get namespace ID
	namespaceIDStr := os.Getenv("NAMESPACE_ID")
	if namespaceIDStr == "" {
		log.Fatal("NAMESPACE_ID environment variable is required")
	}
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		log.Fatalf("Invalid NAMESPACE_ID: %v", err)
	}

	// ============================================
	// Create Worker Keys (Batch)
	// ============================================

	fmt.Println("=== Creating Worker Keys ===")
	start := time.Now()

	// Create 4 worker keys in parallel using batch API
	keys, err := client.Keys.CreateBatch(ctx, banhbaoring.CreateBatchRequest{
		Prefix:      "blob-worker",
		Count:       numWorkers,
		NamespaceID: namespaceID,
		Exportable:  false, // Production keys shouldn't be exportable
	})
	if err != nil {
		log.Fatalf("Failed to create worker keys: %v", err)
	}

	fmt.Printf("Created %d worker keys in %v:\n", len(keys), time.Since(start))
	for _, k := range keys {
		fmt.Printf("  - %s: %s\n", k.Name, k.Address)
	}

	// ============================================
	// Simulate Blob Transactions
	// ============================================

	fmt.Println("\n=== Preparing Blob Transactions ===")

	// Simulate 4 blob transactions that need to be signed in parallel
	blobs := make([][]byte, numWorkers)
	for i := 0; i < numWorkers; i++ {
		// In a real scenario, this would be actual blob data
		blobs[i] = []byte(fmt.Sprintf("blob-data-%d-with-payload", i))
	}

	// ============================================
	// Method 1: Batch Sign API (Recommended)
	// ============================================

	fmt.Println("\n=== Method 1: Batch Sign API ===")
	start = time.Now()

	// Build batch sign request
	requests := make([]banhbaoring.SignRequest, numWorkers)
	for i := 0; i < numWorkers; i++ {
		requests[i] = banhbaoring.SignRequest{
			KeyID:     keys[i].ID,
			Data:      blobs[i],
			Prehashed: false,
		}
	}

	// Sign all in parallel with a single API call
	results, err := client.Sign.SignBatch(ctx, banhbaoring.BatchSignRequest{
		Requests: requests,
	})
	if err != nil {
		log.Fatalf("Batch sign failed: %v", err)
	}

	batchDuration := time.Since(start)
	fmt.Printf("Signed %d transactions in %v (batch API)\n", len(results), batchDuration)
	for i, r := range results {
		if r.Error != "" {
			fmt.Printf("  - Worker %d: ERROR: %s\n", i+1, r.Error)
		} else {
			fmt.Printf("  - Worker %d: sig=%x...\n", i+1, r.Signature[:8])
		}
	}

	// ============================================
	// Method 2: Manual Goroutine Parallelism
	// ============================================

	fmt.Println("\n=== Method 2: Goroutine Parallelism ===")
	start = time.Now()

	var wg sync.WaitGroup
	sigs := make([]*banhbaoring.SignResponse, numWorkers)
	errs := make([]error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := client.Sign.Sign(ctx, keys[idx].ID, blobs[idx], false)
			if err != nil {
				errs[idx] = err
				return
			}
			sigs[idx] = resp
		}(i)
	}
	wg.Wait()

	goroutineDuration := time.Since(start)
	fmt.Printf("Signed %d transactions in %v (goroutines)\n", numWorkers, goroutineDuration)
	for i := 0; i < numWorkers; i++ {
		if errs[i] != nil {
			fmt.Printf("  - Worker %d: ERROR: %v\n", i+1, errs[i])
		} else if sigs[i] != nil {
			fmt.Printf("  - Worker %d: sig=%x...\n", i+1, sigs[i].Signature[:8])
		}
	}

	// ============================================
	// Method 3: Sequential (for comparison)
	// ============================================

	fmt.Println("\n=== Method 3: Sequential (Baseline) ===")
	start = time.Now()

	sequentialSigs := make([]*banhbaoring.SignResponse, numWorkers)
	for i := 0; i < numWorkers; i++ {
		resp, err := client.Sign.Sign(ctx, keys[i].ID, blobs[i], false)
		if err != nil {
			log.Printf("Sequential sign %d failed: %v", i, err)
			continue
		}
		sequentialSigs[i] = resp
	}

	sequentialDuration := time.Since(start)
	fmt.Printf("Signed %d transactions in %v (sequential)\n", numWorkers, sequentialDuration)

	// ============================================
	// Performance Comparison
	// ============================================

	fmt.Println("\n=== Performance Comparison ===")
	fmt.Printf("Batch API:   %v\n", batchDuration)
	fmt.Printf("Goroutines:  %v\n", goroutineDuration)
	fmt.Printf("Sequential:  %v\n", sequentialDuration)
	fmt.Printf("\nSpeedup (batch vs sequential): %.2fx\n",
		float64(sequentialDuration)/float64(batchDuration))

	// ============================================
	// Cleanup
	// ============================================

	fmt.Println("\n=== Cleanup ===")
	for _, k := range keys {
		if err := client.Keys.Delete(ctx, k.ID); err != nil {
			log.Printf("Failed to delete key %s: %v", k.Name, err)
		} else {
			fmt.Printf("Deleted key: %s\n", k.Name)
		}
	}

	fmt.Println("\nParallel workers example completed!")
}

