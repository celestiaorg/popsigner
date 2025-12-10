// Example: Basic BanhBaoRing SDK usage
//
// This example demonstrates basic key management and signing operations
// using the BanhBaoRing Go SDK.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/banhbaoring/sdk-go"
	"github.com/google/uuid"
)

func main() {
	// Initialize the client with your API key
	apiKey := os.Getenv("BANHBAORING_API_KEY")
	if apiKey == "" {
		log.Fatal("BANHBAORING_API_KEY environment variable is required")
	}

	// Create client with default settings
	client := banhbaoring.NewClient(apiKey)

	// Or use custom options:
	// client := banhbaoring.NewClient(apiKey,
	//     banhbaoring.WithBaseURL("https://api.banhbaoring.io"),
	//     banhbaoring.WithTimeout(60*time.Second),
	// )

	ctx := context.Background()

	// Get namespace ID from environment or use default
	namespaceIDStr := os.Getenv("NAMESPACE_ID")
	if namespaceIDStr == "" {
		log.Fatal("NAMESPACE_ID environment variable is required")
	}
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		log.Fatalf("Invalid NAMESPACE_ID: %v", err)
	}

	// ============================================
	// Key Management
	// ============================================

	fmt.Println("=== Key Management ===")

	// Create a new key
	key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
		Name:        "example-key",
		NamespaceID: namespaceID,
		Algorithm:   "secp256k1",
		Exportable:  true,
		Metadata: map[string]string{
			"environment": "development",
			"purpose":     "example",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create key: %v", err)
	}
	fmt.Printf("Created key: %s\n", key.Name)
	fmt.Printf("  ID:        %s\n", key.ID)
	fmt.Printf("  PublicKey: %s\n", key.PublicKey)
	fmt.Printf("  Address:   %s\n", key.Address)

	// Get the key
	fetchedKey, err := client.Keys.Get(ctx, key.ID)
	if err != nil {
		log.Fatalf("Failed to get key: %v", err)
	}
	fmt.Printf("\nFetched key: %s (version %d)\n", fetchedKey.Name, fetchedKey.Version)

	// List all keys in the namespace
	keys, err := client.Keys.List(ctx, &banhbaoring.ListOptions{
		NamespaceID: &namespaceID,
	})
	if err != nil {
		log.Fatalf("Failed to list keys: %v", err)
	}
	fmt.Printf("\nKeys in namespace: %d\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  - %s (%s)\n", k.Name, k.ID)
	}

	// ============================================
	// Signing Operations
	// ============================================

	fmt.Println("\n=== Signing Operations ===")

	// Sign a message
	message := []byte("Hello, BanhBaoRing!")
	signResult, err := client.Sign.Sign(ctx, key.ID, message, false)
	if err != nil {
		log.Fatalf("Failed to sign: %v", err)
	}
	fmt.Printf("Signed message with key %s\n", key.Name)
	fmt.Printf("  Signature: %x\n", signResult.Signature)
	fmt.Printf("  PublicKey: %s\n", signResult.PublicKey)

	// Sign pre-hashed data (useful for blockchain transactions)
	hashedData := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	prehashedResult, err := client.Sign.Sign(ctx, key.ID, hashedData, true)
	if err != nil {
		log.Fatalf("Failed to sign prehashed: %v", err)
	}
	fmt.Printf("\nSigned pre-hashed data: %x...\n", prehashedResult.Signature[:16])

	// ============================================
	// Export Key (if exportable)
	// ============================================

	fmt.Println("\n=== Export Key ===")

	exportResult, err := client.Keys.Export(ctx, key.ID)
	if err != nil {
		log.Fatalf("Failed to export key: %v", err)
	}
	fmt.Printf("Exported private key: %s...%s\n",
		exportResult.PrivateKey[:8],
		exportResult.PrivateKey[len(exportResult.PrivateKey)-8:])
	fmt.Printf("Warning: %s\n", exportResult.Warning)

	// ============================================
	// Cleanup
	// ============================================

	fmt.Println("\n=== Cleanup ===")

	// Delete the key
	if err := client.Keys.Delete(ctx, key.ID); err != nil {
		log.Fatalf("Failed to delete key: %v", err)
	}
	fmt.Printf("Deleted key: %s\n", key.Name)

	// ============================================
	// Error Handling
	// ============================================

	fmt.Println("\n=== Error Handling ===")

	// Try to get a non-existent key
	_, err = client.Keys.Get(ctx, uuid.New())
	if err != nil {
		if apiErr, ok := banhbaoring.IsAPIError(err); ok {
			fmt.Printf("API Error: %s (code: %s)\n", apiErr.Message, apiErr.Code)
			if apiErr.IsNotFound() {
				fmt.Println("  -> This is a not found error")
			}
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	fmt.Println("\nExample completed successfully!")
}

