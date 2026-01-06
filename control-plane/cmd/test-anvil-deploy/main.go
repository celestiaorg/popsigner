// test-anvil-deploy is a standalone test for deploying Nitro infrastructure to local Anvil.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
)

const (
	// Anvil default account (DO NOT use in production!)
	anvilPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	anvilRPCURL     = "http://localhost:8545"
	anvilChainID    = 31337
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	fmt.Println("===========================================")
	fmt.Println("Nitro Infrastructure Deployment Test")
	fmt.Println("Target: Local Anvil (chain ID: 31337)")
	fmt.Println("===========================================")
	fmt.Println()

	// 1. Connect to Anvil
	fmt.Println("1. Connecting to Anvil...")
	client, err := ethclient.DialContext(ctx, anvilRPCURL)
	if err != nil {
		log.Fatalf("Failed to connect to Anvil: %v", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}
	fmt.Printf("   Connected! Chain ID: %d\n\n", chainID.Int64())

	// 2. Create local signer
	fmt.Println("2. Setting up local signer...")
	signer, err := nitro.NewLocalSigner(anvilPrivateKey, anvilChainID)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
	}

	balance, err := client.BalanceAt(ctx, signer.Address(), nil)
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
	fmt.Printf("   Deployer: %s\n", signer.Address().Hex())
	fmt.Printf("   Balance: %s ETH\n\n", new(big.Int).Div(balance, big.NewInt(1e18)).String())

	// 3. Download contract artifacts
	fmt.Println("3. Downloading contract artifacts from S3...")
	downloader := nitro.NewContractArtifactDownloader("")
	artifacts, err := downloader.DownloadDefault(ctx)
	if err != nil {
		log.Fatalf("Failed to download artifacts: %v", err)
	}
	fmt.Printf("   Downloaded version: %s\n", artifacts.Version)
	fmt.Printf("   Contracts loaded: RollupCreator, BridgeCreator, OneStepProvers, etc.\n\n")

	// 4. Create infrastructure deployer
	fmt.Println("4. Creating infrastructure deployer...")
	deployer := nitro.NewInfrastructureDeployer(
		artifacts,
		signer,
		nil, // No database for this test
		logger,
	)

	// 5. Deploy infrastructure
	fmt.Println("5. Deploying Nitro infrastructure...")
	fmt.Println("   This will deploy ~20 contracts. Please wait...")
	fmt.Println()

	startTime := time.Now()
	result, err := deployer.EnsureInfrastructure(ctx, &nitro.InfrastructureConfig{
		ParentChainID: anvilChainID,
		ParentRPC:     anvilRPCURL,
	})
	if err != nil {
		log.Fatalf("Failed to deploy infrastructure: %v", err)
	}
	duration := time.Since(startTime)

	// 6. Print results
	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("✅ DEPLOYMENT SUCCESSFUL!")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Println()
	fmt.Println("Key Addresses:")
	fmt.Printf("  RollupCreator:  %s\n", result.RollupCreatorAddress.Hex())
	fmt.Printf("  BridgeCreator:  %s\n", result.BridgeCreatorAddress.Hex())
	fmt.Println()

	if result.DeployedContracts != nil {
		fmt.Println("All Deployed Contracts:")
		for name, addr := range result.DeployedContracts {
			fmt.Printf("  %-25s %s\n", name+":", addr.Hex())
		}
	}
	fmt.Println()
	fmt.Println("You can now use RollupCreator.createRollup() to deploy Nitro chains!")
}
