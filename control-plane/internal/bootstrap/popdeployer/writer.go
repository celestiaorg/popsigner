package popdeployer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum/go-ethereum/common"
)

// ConfigWriter generates POPKins bundle configuration files.
// It transforms deployment results into docker-compose, genesis,
// rollup config, and other artifacts needed for local devnet.
//
// Not safe for concurrent use. Create one writer per deployment.
type ConfigWriter struct {
	logger        *slog.Logger
	result        *opstack.DeployResult
	config        *DeploymentConfig
	celestiaKeyID string
}

// GenerateAll generates all configuration files and returns them as a map.
// Keys are artifact types (filenames), values are the file contents as bytes.
//
// Returns error on first generation failure. Partial results are discarded.
func (w *ConfigWriter) GenerateAll() (map[string][]byte, error) {
	artifacts := make(map[string][]byte, 9) // 9 known artifacts

	// Generate each config file
	generators := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"genesis.json", w.generateGenesis},
		{"rollup.json", w.generateRollupConfig},
		{"addresses.json", w.generateAddresses},
		{"jwt.txt", w.generateJWT},
		{"config.toml", w.generateConfigToml},
		{"l1-chain-config.json", w.generateL1ChainConfig},
		{"docker-compose.yml", w.generateDockerCompose},
		{".env.example", w.generateEnvExample},
		{"README.md", w.generateREADME},
	}

	for _, gen := range generators {
		w.logger.Info("generating artifact", slog.String("type", gen.name))
		data, err := gen.fn()
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", gen.name, err)
		}
		artifacts[gen.name] = data
	}

	return artifacts, nil
}

// generateGenesis generates the L2 genesis.json file.
func (w *ConfigWriter) generateGenesis() ([]byte, error) {
	if len(w.result.ChainStates) == 0 {
		return nil, fmt.Errorf("no chain states in deployment result")
	}

	chainState := w.result.ChainStates[0]

	// Generate genesis using op-deployer's inspect package
	l2Genesis, _, err := inspect.GenesisAndRollup(w.result.State, chainState.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate genesis: %w", err)
	}
	if l2Genesis == nil {
		return nil, fmt.Errorf("genesis generation returned nil")
	}

	data, err := json.MarshalIndent(l2Genesis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal genesis: %w", err)
	}

	w.logger.Info("genesis.json generated", slog.Int("size_mb", len(data)/(1024*1024)))
	return data, nil
}

// generateRollupConfig generates the rollup.json configuration file.
func (w *ConfigWriter) generateRollupConfig() ([]byte, error) {
	if len(w.result.ChainStates) == 0 {
		return nil, fmt.Errorf("no chain states in deployment result")
	}

	chainState := w.result.ChainStates[0]

	// Generate rollup config using op-deployer's inspect package
	_, rollupCfg, err := inspect.GenesisAndRollup(w.result.State, chainState.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rollup config: %w", err)
	}
	if rollupCfg == nil {
		return nil, fmt.Errorf("rollup config generation returned nil")
	}

	data, err := json.MarshalIndent(rollupCfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal rollup config: %w", err)
	}

	return data, nil
}

// generateAddresses generates the addresses.json file with contract addresses.
func (w *ConfigWriter) generateAddresses() ([]byte, error) {
	addresses := make(map[string]interface{})

	// Superchain contracts
	if w.result.SuperchainContracts != nil {
		addresses["superchain"] = w.result.SuperchainContracts
	}

	// Implementation contracts
	if w.result.ImplementationsContracts != nil {
		addresses["implementations"] = w.result.ImplementationsContracts
	}

	// Chain-specific contracts
	if len(w.result.ChainStates) > 0 {
		chainState := w.result.ChainStates[0]
		addresses["chain_state"] = chainState
	}

	// Deployment info
	addresses["deployment"] = map[string]interface{}{
		"create2_salt":          w.result.Create2Salt.Hex(),
		"infrastructure_reused": w.result.InfrastructureReused,
		"chain_id":              w.config.ChainID,
		"chain_name":            w.config.ChainName,
		"deployer_address":      w.config.DeployerAddress,
		"batcher_address":       w.config.BatcherAddress,
		"proposer_address":      w.config.ProposerAddress,
	}

	data, err := json.MarshalIndent(addresses, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal addresses: %w", err)
	}

	return data, nil
}

// generateJWT generates a random JWT secret.
func (w *ConfigWriter) generateJWT() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate random secret: %w", err)
	}

	// Return as hex string without 0x prefix
	jwtSecret := hex.EncodeToString(secret)
	return []byte(jwtSecret), nil
}

// generateConfigToml generates the config.toml file for op-alt-da pointing to localestia.
func (w *ConfigWriter) generateConfigToml() ([]byte, error) {
	// Generate namespace from chain ID (29 bytes / 58 hex chars)
	// Format: version(1) + reserved_zeros(18) + "pop"(3) + zeros(4) + chain_id(3) = 29 bytes
	namespace := fmt.Sprintf("00%036x706f70%014x", 0, w.config.ChainID)

	config := fmt.Sprintf(`# OP-ALT-DA Configuration for Localestia
# This configures op-alt-da to use localestia as the Celestia backend

addr = "0.0.0.0"
port = 3100
log_level = "info"

[celestia]
# Celestia namespace for this chain
namespace = "%s"

# Localestia endpoint (JSON-RPC for both reads and submits)
bridge_addr = "ws://localestia:26658"

# Gas settings
gas_limit = 100000
fee = 2000

# SIGNER CONFIGURATION
# Use POPSigner for remote key management
[celestia.signer]
mode = "popsigner"

[celestia.signer.popsigner]
# POPSigner-Lite REST API endpoint
base_url = "http://popsigner-lite:3000"

# API key (can also be set via POPSIGNER_API_KEY env var)
api_key = "psk_local_dev_00000000000000000000000000000000"

# Key ID - UUID of the Celestia key in popsigner-lite
key_id = "%s"
`, namespace, w.celestiaKeyID)

	return []byte(config), nil
}

// generateL1ChainConfig generates the l1-chain-config.json file.
// This is required by op-node for non-standard L1 chains (like Anvil).
func (w *ConfigWriter) generateL1ChainConfig() ([]byte, error) {
	// Full Anvil L1 config with all required EIP fields
	chainConfig := map[string]interface{}{
		"chainId":             w.config.L1ChainID,
		"homesteadBlock":      0,
		"eip150Block":         0,
		"eip155Block":         0,
		"eip158Block":         0,
		"byzantiumBlock":      0,
		"constantinopleBlock": 0,
		"petersburgBlock":     0,
		"istanbulBlock":       0,
		"muirGlacierBlock":    0,
		"berlinBlock":         0,
		"londonBlock":         0,
		"arrowGlacierBlock":   0,
		"grayGlacierBlock":    0,
		"shanghaiTime":        0,
		"cancunTime":          0,
		"blobSchedule": map[string]interface{}{
			"cancun": map[string]interface{}{
				"target":                3,
				"max":                   6,
				"baseFeeUpdateFraction": 3338477,
			},
		},
	}

	data, err := json.MarshalIndent(chainConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal l1 chain config: %w", err)
	}

	return data, nil
}

// generateDockerCompose generates the docker-compose.yml file.
// This mirrors the CLI's writers.go docker-compose generation.
func (w *ConfigWriter) generateDockerCompose() ([]byte, error) {
	// Use static docker-compose matching the CLI's writers.go
	// Port assignments:
	// - Anvil L1: 9546
	// - op-geth L2 RPC: 8545, WS: 8546, Engine: 8551
	// - op-node: 9545
	// - popsigner-lite: 8555 (RPC), 3000 (REST)
	// - localestia: 26658
	// - op-alt-da: 3100
	// - op-batcher: 8548
	// - op-proposer: 8560

	compose := `# Local OP Stack Devnet with Celestia DA
# Generated by POPKins for local development
#
# Usage:
#   docker compose up -d
#
# Services:
#   - anvil: L1 chain with pre-deployed OP Stack contracts
#   - popsigner-lite: Local signing service
#   - localestia: Mock Celestia network
#   - op-alt-da: Celestia DA server
#   - op-geth: L2 execution layer
#   - op-node: L2 consensus layer
#   - op-batcher: Batch submitter
#   - op-proposer: State root proposer

services:
  # =============================================================
  # Redis - Backend for Localestia
  # =============================================================
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

  # =============================================================
  # Anvil - L1 chain with pre-deployed OP Stack contracts
  # =============================================================
  anvil:
    image: ghcr.io/foundry-rs/foundry:v1.5.1
    platform: linux/amd64
    restart: unless-stopped
    entrypoint: ["anvil"]
    command:
      - "--host"
      - "0.0.0.0"
      - "--port"
      - "9546"
      - "--state"
      - "/state/anvil-state.json"
      - "--chain-id"
      - "${L1_CHAIN_ID}"
      - "--gas-limit"
      - "${GAS_LIMIT}"
      - "--block-time"
      - "${BLOCK_TIME}"
    ports:
      - "9546:9546"
    volumes:
      - ./anvil-state.json:/state/anvil-state.json
    healthcheck:
      test: ["CMD", "cast", "block-number", "--rpc-url", "http://localhost:9546"]
      interval: 5s
      timeout: 3s
      retries: 30

  # =============================================================
  # POPSigner-Lite - Local signing service
  # =============================================================
  popsigner-lite:
    image: rg.nl-ams.scw.cloud/banhbao/popsigner-lite:v0.1.2
    restart: unless-stopped
    environment:
      - JSONRPC_PORT=8555
      - REST_API_PORT=3000
    ports:
      - "3000:3000"
      - "8555:8555"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3000/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  # =============================================================
  # Localestia - Mock Celestia network
  # =============================================================
  localestia:
    image: rg.nl-ams.scw.cloud/banhbao/localestia:v0.1.5
    restart: unless-stopped
    depends_on:
      redis:
        condition: service_healthy
    environment:
      - REDIS_URL=redis://redis:6379
      - LISTEN_ADDR=0.0.0.0:26658
      - CLEAR_REDIS=true
    ports:
      - "26658:26658"
    healthcheck:
      test: ["CMD", "nc", "-z", "localhost", "26658"]
      interval: 2s
      timeout: 2s
      retries: 30
      start_period: 5s

  # =============================================================
  # OP-ALT-DA - Celestia DA Server
  # =============================================================
  op-alt-da:
    image: rg.nl-ams.scw.cloud/banhbao/op-alt-da:v0.10.1
    restart: unless-stopped
    depends_on:
      localestia:
        condition: service_healthy
    volumes:
      - ./config.toml:/config/config.toml:ro
    command:
      - --config=/config/config.toml
    ports:
      - "3100:3100"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3100/health"]
      interval: 5s
      timeout: 3s
      retries: 60

  # =============================================================
  # OP GETH INIT - Initialize genesis (runs once, then exits)
  # =============================================================
  op-geth-init:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101602.3
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        if [ -f /data/geth/chaindata/CURRENT ] || [ -d /data/geth/chaindata ]; then
          echo "op-geth already initialized, skipping genesis init"
        else
          echo "Initializing op-geth with genesis..."
          geth init --datadir=/data /config/genesis.json
        fi
    volumes:
      - op-geth-data:/data
      - ./genesis.json:/config/genesis.json:ro

  # =============================================================
  # OP GETH - L2 execution layer
  # =============================================================
  op-geth:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101602.3
    restart: unless-stopped
    depends_on:
      op-geth-init:
        condition: service_completed_successfully
    command:
      - --datadir=/data
      - --http
      - --http.addr=0.0.0.0
      - --http.port=8545
      - --http.vhosts=*
      - --http.corsdomain=*
      - --http.api=web3,debug,eth,txpool,net,engine,miner
      - --ws
      - --ws.addr=0.0.0.0
      - --ws.port=8546
      - --ws.origins=*
      - --ws.api=debug,eth,txpool,net,engine,miner
      - --syncmode=full
      - --gcmode=archive
      - --nodiscover
      - --maxpeers=0
      - --networkid=${L2_CHAIN_ID}
      - --authrpc.addr=0.0.0.0
      - --authrpc.port=8551
      - --authrpc.vhosts=*
      - --authrpc.jwtsecret=/config/jwt.txt
      - --rollup.disabletxpoolgossip=true
      - --ipcdisable
      - --metrics
      - --metrics.port=7299
    volumes:
      - op-geth-data:/data
      - ./jwt.txt:/config/jwt.txt:ro
    ports:
      - "8545:8545"   # JSON-RPC
      - "8546:8546"   # WebSocket
      - "8551:8551"   # Engine API
      - "7299:7299"   # Metrics
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8545"]
      interval: 15s
      timeout: 5s
      retries: 20

  # =============================================================
  # OP NODE - Derives L2 state from L1, rollup consensus
  # =============================================================
  op-node:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.16.3
    restart: unless-stopped
    depends_on:
      op-geth:
        condition: service_healthy
      op-alt-da:
        condition: service_healthy
    command:
      - op-node
      - --l2=http://op-geth:8551
      - --l2.jwt-secret=/config/jwt.txt
      - --sequencer.enabled
      - --sequencer.l1-confs=5
      - --verifier.l1-confs=4
      - --rollup.config=/config/rollup.json
      - --rollup.l1-chain-config=/config/l1-chain-config.json
      - --rpc.addr=0.0.0.0
      - --rpc.port=9545
      - --rpc.enable-admin
      - --p2p.disable
      - --l1=http://anvil:9546
      - --l1.beacon=http://localhost:5052
      - --l1.beacon.ignore
      - --l1.rpckind=${L1_RPC_KIND:-basic}
      - --l1.trustrpc
      # Celestia Alt-DA
      - --altda.enabled=true
      - --altda.verify-on-read=true
      - --altda.da-server=http://op-alt-da:3100
      - --metrics.enabled
      - --metrics.port=7300
    volumes:
      - ./l1-chain-config.json:/config/l1-chain-config.json:ro
      - ./rollup.json:/config/rollup.json:ro
      - ./jwt.txt:/config/jwt.txt:ro
    ports:
      - "9545:9545"   # op-node RPC
      - "7300:7300"   # Metrics
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:9545"]
      interval: 15s
      timeout: 5s
      retries: 20

  # =============================================================
  # OP BATCHER - Submits L2 batches to DA layer
  # =============================================================
  op-batcher:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:v1.16.3
    restart: unless-stopped
    depends_on:
      op-geth:
        condition: service_healthy
      op-node:
        condition: service_healthy
      op-alt-da:
        condition: service_healthy
    command:
      - op-batcher
      - --l2-eth-rpc=http://op-geth:8545
      - --rollup-rpc=http://op-node:9545
      - --poll-interval=1s
      - --sub-safety-margin=6
      - --num-confirmations=1
      - --safe-abort-nonce-too-low-count=3
      - --resubmission-timeout=30s
      - --rpc.addr=0.0.0.0
      - --rpc.port=8548
      - --max-channel-duration=25
      - --l1-eth-rpc=http://anvil:9546
      # POPSigner for batcher signing
      - --signer.endpoint=http://popsigner-lite:8555
      - --signer.address=${BATCHER_ADDRESS}
      - --signer.header=X-API-Key:${POPSIGNER_API_KEY}
      - --signer.tls.enabled=false
      # Celestia Alt-DA
      - --altda.da-service=true
      - --altda.enabled=true
      - --altda.da-server=http://op-alt-da:3100
      - --metrics.enabled
      - --metrics.port=7301
    ports:
      - "8548:8548"   # Batcher RPC
      - "7301:7301"   # Metrics
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8548/healthz"]
      interval: 15s
      timeout: 5s
      retries: 20

  # =============================================================
  # OP PROPOSER - Submits L2 state roots to L1
  # =============================================================
  op-proposer:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:v1.10.0
    restart: unless-stopped
    depends_on:
      op-node:
        condition: service_healthy
    command:
      - op-proposer
      - --poll-interval=12s
      - --rpc.port=8560
      - --rollup-rpc=http://op-node:9545
      - --game-factory-address=${DISPUTE_GAME_FACTORY_ADDRESS}
      - --proposal-interval=6h
      - --l1-eth-rpc=http://anvil:9546
      # POPSigner for proposer signing
      - --signer.endpoint=http://popsigner-lite:8555
      - --signer.address=${PROPOSER_ADDRESS}
      - --signer.header=X-API-Key:${POPSIGNER_API_KEY}
      - --signer.tls.enabled=false
      - --metrics.enabled
      - --metrics.port=7302
    ports:
      - "8560:8560"   # Proposer RPC
      - "7302:7302"   # Metrics
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8560/healthz"]
      interval: 15s
      timeout: 5s
      retries: 20

volumes:
  op-geth-data:

networks:
  default:
    name: local-opstack-devnet
    driver: bridge
`

	return []byte(compose), nil
}

// generateEnvExample generates the .env.example file.
// Variables must match docker-compose.yml ${VARIABLE} interpolation.
func (w *ConfigWriter) generateEnvExample() ([]byte, error) {
	// Extract DisputeGameFactoryProxy address from deployment result
	disputeGameFactory := "0x0000000000000000000000000000000000000000"
	if w.result != nil && len(w.result.ChainStates) > 0 {
		chainState := w.result.ChainStates[0]
		if chainState.DisputeGameFactoryProxy != (common.Address{}) {
			disputeGameFactory = chainState.DisputeGameFactoryProxy.Hex()
		}
	}

	env := fmt.Sprintf(`# L1 Configuration
L1_RPC_URL=http://anvil:9546
L1_CHAIN_ID=%d
L1_RPC_KIND=basic
BLOCK_TIME=%d
GAS_LIMIT=%d

# L2 Configuration
L2_CHAIN_ID=%d
L2_CHAIN_NAME=%s

# POPSigner-Lite
POPSIGNER_RPC_URL=http://popsigner-lite:8555
POPSIGNER_API_URL=http://popsigner-lite:3000
POPSIGNER_API_KEY=psk_local_dev_00000000000000000000000000000000

# Role Addresses (Anvil deterministic keys)
DEPLOYER_ADDRESS=%s
BATCHER_ADDRESS=%s
PROPOSER_ADDRESS=%s

# Contract Addresses (from deployment)
DISPUTE_GAME_FACTORY_ADDRESS=%s
`,
		w.config.L1ChainID,
		w.config.BlockTime,
		w.config.GasLimit,
		w.config.ChainID,
		w.config.ChainName,
		w.config.DeployerAddress,
		w.config.BatcherAddress,
		w.config.ProposerAddress,
		disputeGameFactory,
	)

	return []byte(env), nil
}

// generateREADME generates the README.md file.
func (w *ConfigWriter) generateREADME() ([]byte, error) {
	readme := fmt.Sprintf(`# %s - POPKins Devnet Bundle

This bundle contains a complete, pre-deployed OP Stack + Celestia DA local devnet.

## What's Included

- **Anvil L1**: Ethereum L1 with pre-deployed OP Stack contracts
- **POPSigner-Lite**: Transaction signing service
- **Localestia**: Mock Celestia DA network
- **OP-ALT-DA**: Celestia DA server
- **OP-Geth**: L2 execution layer
- **OP-Node**: L2 consensus layer
- **OP-Batcher**: Batch submitter
- **OP-Proposer**: State root proposer

## Quick Start

1. Start the devnet:
   `+"```bash\n   docker compose up -d\n   ```"+`

2. Wait for services to be healthy (~30-60 seconds):
   `+"```bash\n   docker compose ps\n   ```"+`

3. Test L2 RPC:
   `+"```bash\n   curl -X POST http://localhost:9545 \\\n     -H \"Content-Type: application/json\" \\\n     -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}'\n   ```"+`

## Configuration

- **L1 RPC**: http://localhost:8545
- **L2 RPC**: http://localhost:9545
- **Chain ID**: %d
- **Block Time**: %d seconds

## Contract Addresses

See `+"`addresses.json`"+` for all deployed contract addresses.

## Troubleshooting

View logs for any service:
`+"```bash\ndocker compose logs -f [service-name]\n```"+`

Stop the devnet:
`+"```bash\ndocker compose down\n```"+`

## Generated by POPSigner

This bundle was generated using POPSigner's POPKins Bundle Builder.
`,
		w.config.ChainName,
		w.config.ChainID,
		w.config.BlockTime,
	)

	return []byte(readme), nil
}
