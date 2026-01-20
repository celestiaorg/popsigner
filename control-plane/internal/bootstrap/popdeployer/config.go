package popdeployer

// DeploymentConfig holds configuration for a POPKins devnet bundle deployment.
type DeploymentConfig struct {
	// User-configurable parameters
	ChainID   uint64 `json:"chain_id"`
	ChainName string `json:"chain_name"`

	// Hardcoded parameters (populated by orchestrator)
	L1ChainID       uint64 `json:"l1_chain_id"`       // 31337 (Anvil)
	L1RPC           string `json:"l1_rpc"`            // IPC path or HTTP URL to Anvil
	DeployerAddress string `json:"deployer_address"` // anvil-0
	BatcherAddress  string `json:"batcher_address"`  // anvil-1
	ProposerAddress string `json:"proposer_address"` // anvil-2
	BlockTime       uint64 `json:"block_time"`       // 2 seconds
	GasLimit        uint64 `json:"gas_limit"`        // 30000000

	// Note: POPSigner fields removed - not needed during bundle build.
	// We use AnvilSigner for direct ECDSA signing with Anvil's well-known keys.
	// POPSigner-Lite is only used at runtime (in docker-compose for op-batcher/op-proposer).
}
