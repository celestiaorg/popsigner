package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum/go-ethereum/common"
)

// ConfigWriter handles writing all bundle configuration files.
type ConfigWriter struct {
	logger        *slog.Logger
	bundleDir     string
	result        *opstack.DeployResult
	config        *opstack.DeploymentConfig
	celestiaKeyID string
}

// WriteAll writes all configuration files to the bundle directory.
func (w *ConfigWriter) WriteAll() error {
	if err := w.writeGenesis(); err != nil {
		return fmt.Errorf("write genesis: %w", err)
	}

	if err := w.writeRollupConfig(); err != nil {
		return fmt.Errorf("write rollup config: %w", err)
	}

	if err := w.writeAddresses(); err != nil {
		return fmt.Errorf("write addresses: %w", err)
	}

	if err := w.writeJWT(); err != nil {
		return fmt.Errorf("write JWT: %w", err)
	}

	if err := w.writeConfigToml(); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}

	if err := w.writeL1ChainConfig(); err != nil {
		return fmt.Errorf("write l1-chain-config.json: %w", err)
	}

	if err := w.writeDockerCompose(); err != nil {
		return fmt.Errorf("write docker-compose: %w", err)
	}

	if err := w.writeEnvExample(); err != nil {
		return fmt.Errorf("write .env.example: %w", err)
	}

	if err := w.writeREADME(); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	return nil
}

// writeGenesis writes the L2 genesis.json file.
func (w *ConfigWriter) writeGenesis() error {
	w.logger.Info("writing genesis.json")

	if len(w.result.ChainStates) == 0 {
		return fmt.Errorf("no chain states in deployment result")
	}

	chainState := w.result.ChainStates[0]

	// Generate genesis using op-deployer's inspect package
	l2Genesis, _, err := inspect.GenesisAndRollup(w.result.State, chainState.ID)
	if err != nil {
		return fmt.Errorf("failed to generate genesis: %w", err)
	}
	if l2Genesis == nil {
		return fmt.Errorf("genesis generation returned nil")
	}

	data, err := json.MarshalIndent(l2Genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal genesis: %w", err)
	}

	path := filepath.Join(w.bundleDir, "genesis.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("genesis.json written", slog.String("path", path), slog.Int("size_mb", len(data)/(1024*1024)))
	return nil
}

// writeRollupConfig writes the rollup.json configuration file.
func (w *ConfigWriter) writeRollupConfig() error {
	w.logger.Info("writing rollup.json")

	if len(w.result.ChainStates) == 0 {
		return fmt.Errorf("no chain states in deployment result")
	}

	chainState := w.result.ChainStates[0]

	// Generate rollup config using op-deployer's inspect package
	_, rollupCfg, err := inspect.GenesisAndRollup(w.result.State, chainState.ID)
	if err != nil {
		return fmt.Errorf("failed to generate rollup config: %w", err)
	}
	if rollupCfg == nil {
		return fmt.Errorf("rollup config generation returned nil")
	}

	data, err := json.MarshalIndent(rollupCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal rollup config: %w", err)
	}

	path := filepath.Join(w.bundleDir, "rollup.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("rollup.json written", slog.String("path", path))
	return nil
}

// writeAddresses writes the addresses.json file with all deployed contract addresses.
func (w *ConfigWriter) writeAddresses() error {
	w.logger.Info("writing addresses.json")

	addresses := make(map[string]interface{})

	// Superchain contracts
	if w.result.SuperchainContracts != nil {
		addresses["superchain"] = w.result.SuperchainContracts
	}

	// Implementation contracts
	if w.result.ImplementationsContracts != nil {
		addresses["implementations"] = w.result.ImplementationsContracts
	}

	// Chain-specific contracts (include whole chainstate)
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
		return fmt.Errorf("marshal addresses: %w", err)
	}

	path := filepath.Join(w.bundleDir, "addresses.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("addresses.json written", slog.String("path", path))
	return nil
}

// writeJWT generates and writes a random JWT secret.
func (w *ConfigWriter) writeJWT() error {
	w.logger.Info("generating jwt.txt")

	// Generate random 32-byte secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate random secret: %w", err)
	}

	hexSecret := hex.EncodeToString(secret)

	path := filepath.Join(w.bundleDir, "jwt.txt")
	if err := os.WriteFile(path, []byte(hexSecret), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("jwt.txt written", slog.String("path", path))
	return nil
}

// writeConfigToml writes the config.toml for op-alt-da pointing to localestia.
func (w *ConfigWriter) writeConfigToml() error {
	w.logger.Info("writing config.toml")

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

	path := filepath.Join(w.bundleDir, "config.toml")
	if err := os.WriteFile(path, []byte(config), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("config.toml written", slog.String("path", path))
	return nil
}

// writeL1ChainConfig writes the L1 chain config used by op-node for unknown chain IDs.
func (w *ConfigWriter) writeL1ChainConfig() error {
	w.logger.Info("writing l1-chain-config.json")

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
		return fmt.Errorf("marshal l1 chain config: %w", err)
	}

	path := filepath.Join(w.bundleDir, "l1-chain-config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("l1-chain-config.json written", slog.String("path", path))
	return nil
}

// writeDockerCompose writes the docker-compose.yml for the bundle.
func (w *ConfigWriter) writeDockerCompose() error {
	w.logger.Info("writing docker-compose.yml")

	compose := `# Local OP Stack Devnet with Celestia DA
# Generated by pop-deployer for local development
#
# Usage:
#   docker compose up -d
#
# Services:
#   - anvil: L1 chain with pre-deployed OP Stack contracts
#   - popsigner-lite: Local signing service (built from source)
#   - localestia: Mock Celestia network (built from source)
#   - op-alt-da: Celestia DA server (connects to localestia)
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
  # POPSigner-Lite - Local signing service (pre-built locally)
  # To build: docker build -t popsigner-lite:local -f popsigner/control-plane/cmd/popsigner-lite/Dockerfile popsigner
  # =============================================================
  popsigner-lite:
    image: popsigner-lite:local
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
  # Localestia - Mock Celestia network (pre-built locally)
  # To build: docker build -t localestia:local localestia
  # =============================================================
  localestia:
    image: localestia:local
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
  # Posts blobs to Celestia (localestia), serves commitments to op-node/op-batcher
  # =============================================================
  op-alt-da:
    image: ghcr.io/celestiaorg/op-alt-da:v0.10.0
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

	path := filepath.Join(w.bundleDir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(compose), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("docker-compose.yml written", slog.String("path", path))
	return nil
}

// writeEnvExample writes the .env.example file.
func (w *ConfigWriter) writeEnvExample() error {
	w.logger.Info("writing .env.example")

	// Extract DisputeGameFactoryProxy address from deployment
	disputeGameFactory := "0x0000000000000000000000000000000000000000"
	if len(w.result.ChainStates) > 0 && w.result.ChainStates[0].DisputeGameFactoryProxy != (common.Address{}) {
		disputeGameFactory = w.result.ChainStates[0].DisputeGameFactoryProxy.Hex()
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

	path := filepath.Join(w.bundleDir, ".env.example")
	if err := os.WriteFile(path, []byte(env), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	// Also copy to .env for immediate use
	envPath := filepath.Join(w.bundleDir, ".env")
	if err := os.WriteFile(envPath, []byte(env), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", envPath, err)
	}

	w.logger.Info(".env.example and .env written", slog.String("path", path))
	return nil
}

// writeREADME writes the README.md for the bundle.
func (w *ConfigWriter) writeREADME() error {
	w.logger.Info("writing README.md")

	// Build README content
	readme := "# Local OP Stack Devnet with Celestia DA\n\n"
	readme += "Pre-deployed development environment for OP Stack rollups.\n\n"
	readme += "## Quick Start\n\n"
	readme += "1. Ensure Docker is running\n\n"
	readme += "2. Start the devnet:\n```bash\ndocker compose up -d\n```\n\n"
	readme += "3. Wait for L2 to start (~30 seconds):\n```bash\ndocker compose logs -f op-node\n```\n\n"
	readme += "4. Verify services:\n```bash\n# Check Anvil (L1)\ncast block-number --rpc-url http://localhost:9546\n\n"
	readme += "# Check OP-Geth (L2)\ncast block-number --rpc-url http://localhost:8545\n\n"
	readme += "# Check POPSigner-Lite\ncurl http://localhost:3000/health\n```\n\n"
	readme += "## Services\n\n"
	readme += "- **Anvil** (L1): http://localhost:9546\n"
	readme += "- **POPSigner-Lite**: http://localhost:3000 (REST), http://localhost:8555 (JSON-RPC)\n"
	readme += "- **Localestia** (Mock Celestia): http://localhost:26658\n"
	readme += "- **OP-Geth** (L2): http://localhost:8545\n"
	readme += "- **OP-Node**: http://localhost:9545\n\n"
	readme += "## What's Inside\n\n"
	readme += "This bundle contains:\n"
	readme += "- **Pre-deployed OP Stack contracts** on Anvil L1\n"
	readme += "- **genesis.json** - L2 genesis with contract state\n"
	readme += "- **rollup.json** - L2 rollup configuration\n"
	readme += "- **addresses.json** - All contract addresses\n"
	readme += "- **anvil-state.json** - Pre-deployed Anvil state\n"
	readme += "- **popsigner-lite** - Local signing service\n"
	readme += "- **localestia** - Mock Celestia DA layer\n\n"
	readme += "## Chain Info\n\n"
	readme += fmt.Sprintf("- **Chain ID**: %d\n", w.config.ChainID)
	readme += fmt.Sprintf("- **Chain Name**: %s\n", w.config.ChainName)
	readme += fmt.Sprintf("- **L1 Chain ID**: %d\n", w.config.L1ChainID)
	readme += fmt.Sprintf("- **Block Time**: %d seconds\n", w.config.BlockTime)
	readme += fmt.Sprintf("- **Gas Limit**: %d\n\n", w.config.GasLimit)
	readme += "## Deployment Info\n\n"
	readme += fmt.Sprintf("- **Deployer**: %s\n", w.config.DeployerAddress)
	readme += fmt.Sprintf("- **Batcher**: %s\n", w.config.BatcherAddress)
	readme += fmt.Sprintf("- **Proposer**: %s\n", w.config.ProposerAddress)
	readme += fmt.Sprintf("- **CREATE2 Salt**: %s\n\n", w.result.Create2Salt.Hex())
	readme += "## Reset\n\n"
	readme += "To wipe L2 state and restart:\n```bash\ndocker compose down -v\ndocker compose up -d\n```\n\n"
	readme += "Note: This only resets L2 data. L1 contracts remain pre-deployed.\n\n"
	readme += "## Security Notice\n\n"
	readme += "**⚠️ FOR DEVELOPMENT USE ONLY ⚠️**\n\n"
	readme += "This bundle uses Anvil's well-known deterministic keys. Never use with real funds.\n\n"
	readme += "For production, migrate to [POPSigner Cloud](https://popsigner.com).\n"

	path := filepath.Join(w.bundleDir, "README.md")
	if err := os.WriteFile(path, []byte(readme), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	w.logger.Info("README.md written", slog.String("path", path))
	return nil
}
