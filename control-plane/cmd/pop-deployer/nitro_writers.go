package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
)

// NitroConfigWriter handles writing all Nitro bundle configuration files.
type NitroConfigWriter struct {
	logger        *slog.Logger
	bundleDir     string
	result        *nitroDeployResult
	celestiaKeyID string
}

// WriteAll writes all configuration files to the bundle directory.
func (w *NitroConfigWriter) WriteAll() error {
	if err := w.createDirectories(); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	if err := w.writeChainInfo(); err != nil {
		return fmt.Errorf("write chain-info.json: %w", err)
	}

	if err := w.writeCelestiaConfig(); err != nil {
		return fmt.Errorf("write celestia-config.toml: %w", err)
	}

	if err := w.writeAddresses(); err != nil {
		return fmt.Errorf("write addresses.json: %w", err)
	}

	if err := w.writeJWT(); err != nil {
		return fmt.Errorf("write jwt.txt: %w", err)
	}

	if err := w.writeDockerCompose(); err != nil {
		return fmt.Errorf("write docker-compose.yml: %w", err)
	}

	if err := w.writeEnv(); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	if err := w.writeScripts(); err != nil {
		return fmt.Errorf("write scripts: %w", err)
	}

	if err := w.writeREADME(); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	return nil
}

// createDirectories creates the bundle directory structure.
func (w *NitroConfigWriter) createDirectories() error {
	dirs := []string{
		filepath.Join(w.bundleDir, "state"),
		filepath.Join(w.bundleDir, "config"),
		filepath.Join(w.bundleDir, "scripts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// writeChainInfo writes the chain-info.json for Nitro node.
func (w *NitroConfigWriter) writeChainInfo() error {
	w.logger.Info("writing chain-info.json")

	chainInfo := []map[string]interface{}{
		{
			"chain-id":                 nitroL2ChainID,
			"chain-name":               nitroL2ChainName,
			"parent-chain-id":          l1ChainID,
			"parent-chain-is-arbitrum": false,
			"chain-config":             w.result.chainConfig,
			"rollup": map[string]interface{}{
				"bridge":                   w.result.contracts.Bridge.Hex(),
				"inbox":                    w.result.contracts.Inbox.Hex(),
				"sequencer-inbox":          w.result.contracts.SequencerInbox.Hex(),
				"rollup":                   w.result.contracts.Rollup.Hex(),
				"validator-utils":          common.Address{}.Hex(),
				"validator-wallet-creator": w.result.contracts.ValidatorWalletCreator.Hex(),
				"deployed-at":              w.result.deploymentBlock,
			},
			"_deployment": map[string]interface{}{
				"method":           "pop-deployer-go",
				"version":          "v3.2.0",
				"stake-token":      w.result.stakeToken.Hex(),
				"native-token":     w.result.contracts.NativeToken.Hex(),
				"upgrade-executor": w.result.contracts.UpgradeExecutor.Hex(),
			},
		},
	}

	data, err := json.MarshalIndent(chainInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal chain info: %w", err)
	}

	path := filepath.Join(w.bundleDir, "config", "chain-info.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("chain-info.json written", slog.String("path", path))
	return nil
}

// writeCelestiaConfig writes the celestia-config.toml for Celestia DAS server.
func (w *NitroConfigWriter) writeCelestiaConfig() error {
	w.logger.Info("writing celestia-config.toml")

	// Generate namespace from chain ID
	namespace := fmt.Sprintf("00000000000000000000000000000000000000706f70%014x", nitroL2ChainID)

	config := fmt.Sprintf(`# =============================================================================
# Celestia DAS Server Configuration (v0.8.2)
# Local Development with Localestia + POPSigner-Lite
# =============================================================================

[server]
rpc_addr = "0.0.0.0"
rpc_port = 9876
rpc_body_limit = 0
read_timeout = "30s"
read_header_timeout = "10s"
write_timeout = "30s"
idle_timeout = "120s"

[celestia]
namespace_id = "%s"
gas_price = 0.01
gas_multiplier = 1.01
network = "private"
with_writer = false
noop_writer = false
cache_time = "30m"

[celestia.reader]
rpc = "http://localestia:26658"
auth_token = ""
enable_tls = false

[celestia.writer]
core_grpc = "localestia:26658"
core_token = ""
enable_tls = false

[celestia.signer]
type = "local"

[celestia.signer.local]
key_name = "nitro-local-celestia-key"
key_path = ""
backend = "test"

[celestia.retry]
max_retries = 5
initial_backoff = "10s"
max_backoff = "120s"
backoff_factor = 2.0

[celestia.validator]
eth_rpc = ""
blobstream_addr = ""
sleep_time = 3600

[fallback]
enabled = false
das_rpc = ""

[logging]
level = "INFO"
type = "plaintext"

[metrics]
enabled = true
addr = "0.0.0.0"
port = 6060
pprof = false
pprof_addr = "127.0.0.1"
pprof_port = 6061
`, namespace)

	path := filepath.Join(w.bundleDir, "config", "celestia-config.toml")
	if err := os.WriteFile(path, []byte(config), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("celestia-config.toml written", slog.String("path", path))
	return nil
}

// writeAddresses writes the addresses.json with all deployed contract addresses.
func (w *NitroConfigWriter) writeAddresses() error {
	w.logger.Info("writing addresses.json")

	addresses := map[string]interface{}{
		"rollup":                 w.result.contracts.Rollup.Hex(),
		"inbox":                  w.result.contracts.Inbox.Hex(),
		"outbox":                 w.result.contracts.Outbox.Hex(),
		"bridge":                 w.result.contracts.Bridge.Hex(),
		"sequencerInbox":         w.result.contracts.SequencerInbox.Hex(),
		"rollupEventInbox":       w.result.contracts.RollupEventInbox.Hex(),
		"challengeManager":       w.result.contracts.ChallengeManager.Hex(),
		"adminProxy":             w.result.contracts.AdminProxy.Hex(),
		"upgradeExecutor":        w.result.contracts.UpgradeExecutor.Hex(),
		"validatorWalletCreator": w.result.contracts.ValidatorWalletCreator.Hex(),
		"nativeToken":            w.result.contracts.NativeToken.Hex(),
		"stakeToken":             w.result.stakeToken.Hex(),
		"deployedAtBlockNumber":  w.result.deploymentBlock,
	}

	data, err := json.MarshalIndent(addresses, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal addresses: %w", err)
	}

	path := filepath.Join(w.bundleDir, "config", "addresses.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("addresses.json written", slog.String("path", path))
	return nil
}

// writeJWT generates and writes a random JWT secret.
func (w *NitroConfigWriter) writeJWT() error {
	w.logger.Info("generating jwt.txt")

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate random secret: %w", err)
	}

	hexSecret := hex.EncodeToString(secret)

	path := filepath.Join(w.bundleDir, "config", "jwt.txt")
	if err := os.WriteFile(path, []byte(hexSecret), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("jwt.txt written", slog.String("path", path))
	return nil
}

// writeDockerCompose writes the docker-compose.yml for the Nitro bundle.
func (w *NitroConfigWriter) writeDockerCompose() error {
	w.logger.Info("writing docker-compose.yml")

	compose := `# Nitro + Celestia + Anvil Local Devnet
# Generated by pop-deployer
#
# Usage:
#   ./scripts/start.sh    # Start the devnet (handles two-phase init)
#   ./scripts/stop.sh     # Stop the devnet
#   ./scripts/reset.sh    # Reset all state and restart
#
# Services:
#   - anvil: L1 chain with pre-deployed Nitro contracts
#   - popsigner-lite: Local signing service
#   - localestia: Mock Celestia network
#   - celestia-das-server: Celestia DA adapter for Nitro
#   - nitro-sequencer: L2 sequencer (monolithic: sequencer + batch-poster + validator)

services:
  # =============================================================
  # Redis - Backend for Localestia
  # =============================================================
  redis:
    image: redis:7-alpine
    container_name: nitro-redis
    restart: unless-stopped
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks:
      - nitro-network

  # =============================================================
  # Anvil - L1 chain with pre-deployed Nitro contracts
  # =============================================================
  anvil:
    image: ghcr.io/foundry-rs/foundry:v1.5.1
    platform: linux/amd64
    container_name: nitro-anvil
    restart: unless-stopped
    environment:
      - L1_CHAIN_ID=${L1_CHAIN_ID}
      - GAS_LIMIT=${GAS_LIMIT}
      - BLOCK_TIME=${BLOCK_TIME}
    entrypoint: ["/bin/sh", "-c"]
    user: root
    command:
      - |
        if [ ! -f /data/state.json ]; then
          echo "First run - copying bundled state to volume..."
          cp /state/anvil-state.json /data/state.json
        fi
        echo "Starting anvil with state from volume..."
        anvil --host 0.0.0.0 --port 8545 \
          --chain-id $L1_CHAIN_ID \
          --gas-limit $GAS_LIMIT \
          --block-time $BLOCK_TIME \
          --state /data/state.json \
          --preserve-historical-states
    ports:
      - "8545:8545"
    volumes:
      - ./state/anvil-state.json:/state/anvil-state.json:ro
      - anvil-data:/data
    healthcheck:
      test: ["CMD", "cast", "block-number", "--rpc-url", "http://localhost:8545"]
      interval: 5s
      timeout: 3s
      retries: 30
    networks:
      - nitro-network

  # =============================================================
  # POPSigner-Lite - Local signing service
  # =============================================================
  popsigner-lite:
    image: rg.nl-ams.scw.cloud/banhbao/popsigner-lite:v0.1.2
    container_name: nitro-popsigner
    restart: unless-stopped
    environment:
      - JSONRPC_PORT=8555
      - REST_API_PORT=3000
      - POPSIGNER_API_KEY=${POPSIGNER_API_KEY}
    ports:
      - "3000:3000"
      - "8555:8555"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3000/health"]
      interval: 5s
      timeout: 3s
      retries: 10
    networks:
      - nitro-network

  # =============================================================
  # Localestia - Mock Celestia network
  # =============================================================
  localestia:
    image: rg.nl-ams.scw.cloud/banhbao/localestia:v0.1.5
    container_name: nitro-localestia
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
    networks:
      - nitro-network

  # =============================================================
  # Celestia DAS Server - Celestia DA adapter for Nitro
  # =============================================================
  celestia-das-server:
    image: ${NITRO_DAS_IMAGE}
    container_name: nitro-celestia-das
    restart: unless-stopped
    depends_on:
      localestia:
        condition: service_healthy
      popsigner-lite:
        condition: service_healthy
    command:
      - --config
      - /config/celestia-config.toml
    ports:
      - "9876:9876"
      - "6060:6060"
    volumes:
      - ./config/celestia-config.toml:/config/celestia-config.toml:ro
    environment:
      - POPSIGNER_API_KEY=${POPSIGNER_API_KEY}
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9876/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s
    networks:
      - nitro-network

  # =============================================================
  # Nitro Sequencer - L2 sequencer (monolithic)
  # NOTE: Two-phase startup required (Issue #4208)
  # =============================================================
  nitro-sequencer:
    image: ${NITRO_IMAGE}
    container_name: nitro-sequencer
    restart: unless-stopped
    depends_on:
      anvil:
        condition: service_healthy
      celestia-das-server:
        condition: service_healthy
      popsigner-lite:
        condition: service_healthy
    ports:
      - "8547:8547"
      - "8548:8548"
      - "9642:9642"
      - "9644:9644"
    volumes:
      - ./config:/config:ro
      - nitro-data:/home/user/.arbitrum
    environment:
      - L1_RPC_URL=${L1_RPC_URL}
      - POPSIGNER_RPC_URL=${POPSIGNER_RPC_URL}
      - BATCH_POSTER_ADDRESS=${BATCH_POSTER_ADDRESS}
      - STAKER_ADDRESS=${STAKER_ADDRESS}
    command:
      - --chain.id=${L2_CHAIN_ID}
      - --chain.name=${L2_CHAIN_NAME}
      - --chain.info-files=/config/chain-info.json
      - --parent-chain.connection.url=${L1_RPC_URL}
      - --http.addr=0.0.0.0
      - --http.port=8547
      - --http.api=eth,net,web3,arb,debug
      - --http.vhosts=*
      - --http.corsdomain=*
      - --ws.addr=0.0.0.0
      - --ws.port=8548
      - --ws.api=eth,net,web3
      - --ws.origins=*
      - --node.sequencer=true
      - --execution.sequencer.enable=true
      - --node.delayed-sequencer.enable=true
      - --node.dangerous.no-sequencer-coordinator=true
      - --node.delayed-sequencer.use-merge-finality=false
      - --node.delayed-sequencer.finalize-distance=1
      - --node.dangerous.disable-blob-reader
      - --node.batch-poster.post-4844-blobs=false
      - --execution.sequencer.expected-surplus-gas-price-mode=CalldataPrice
      - --node.da.external-provider.enable=true
      - --node.da.external-provider.with-writer=true
      - --node.da.external-provider.rpc.url=http://celestia-das-server:9876
      - --node.batch-poster.enable=${BATCH_POSTER_ENABLE:-false}
      - --node.batch-poster.data-poster.external-signer.url=${POPSIGNER_RPC_URL}
      - --node.batch-poster.data-poster.external-signer.address=${BATCH_POSTER_ADDRESS}
      - --node.batch-poster.data-poster.external-signer.method=eth_signTransaction
      - --node.staker.enable=${STAKER_ENABLE:-false}
      - --node.staker.strategy=MakeNodes
      - --node.staker.data-poster.external-signer.url=${POPSIGNER_RPC_URL}
      - --node.staker.data-poster.external-signer.address=${STAKER_ADDRESS}
      - --node.staker.data-poster.external-signer.method=eth_signTransaction
      - --node.feed.output.enable=true
      - --node.feed.output.addr=0.0.0.0
      - --node.feed.output.port=9644
      - --metrics
      - --metrics-server.addr=0.0.0.0
      - --metrics-server.port=9642
    networks:
      - nitro-network

networks:
  nitro-network:
    name: nitro-local-devnet
    driver: bridge

volumes:
  nitro-data:
  anvil-data:
  redis-data:
`

	path := filepath.Join(w.bundleDir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(compose), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("docker-compose.yml written", slog.String("path", path))
	return nil
}

// writeEnv writes the .env file with pre-filled values.
func (w *NitroConfigWriter) writeEnv() error {
	w.logger.Info("writing .env")

	env := fmt.Sprintf(`# =============================================================================
# Nitro + Celestia + Anvil Local Devnet Configuration
# Generated by pop-deployer
# =============================================================================

# L1 Configuration (Anvil)
L1_RPC_URL=http://anvil:8545
L1_CHAIN_ID=%d
BLOCK_TIME=%d
GAS_LIMIT=%d

# L2 Configuration (Nitro)
L2_CHAIN_ID=%d
L2_CHAIN_NAME=%s

# POPSigner-Lite Configuration
POPSIGNER_RPC_URL=http://popsigner-lite:8555
POPSIGNER_API_URL=http://popsigner-lite:3000
POPSIGNER_API_KEY=psk_local_dev_00000000000000000000000000000000

# Role Addresses (Anvil Deterministic Accounts)
DEPLOYER_ADDRESS=%s
BATCH_POSTER_ADDRESS=%s
STAKER_ADDRESS=%s

# Docker Images
NITRO_IMAGE=rg.nl-ams.scw.cloud/banhbao/nitro-node-dev:v3.10.0
NITRO_DAS_IMAGE=rg.nl-ams.scw.cloud/banhbao/nitro-das-server:v0.8.2

# Startup Phase Control
BATCH_POSTER_ENABLE=true
STAKER_ENABLE=false

# Contract Addresses
ROLLUP_ADDRESS=%s
INBOX_ADDRESS=%s
SEQUENCER_INBOX_ADDRESS=%s
BRIDGE_ADDRESS=%s
VALIDATOR_WALLET_CREATOR=%s
STAKE_TOKEN=%s
`,
		l1ChainID,
		blockTime,
		gasLimit,
		nitroL2ChainID,
		nitroL2ChainName,
		deployerAddress,
		batcherAddress,
		proposerAddress,
		w.result.contracts.Rollup.Hex(),
		w.result.contracts.Inbox.Hex(),
		w.result.contracts.SequencerInbox.Hex(),
		w.result.contracts.Bridge.Hex(),
		w.result.contracts.ValidatorWalletCreator.Hex(),
		w.result.stakeToken.Hex(),
	)

	path := filepath.Join(w.bundleDir, ".env")
	if err := os.WriteFile(path, []byte(env), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info(".env written", slog.String("path", path))
	return nil
}

// writeScripts writes the start.sh, stop.sh, reset.sh, and test.sh scripts.
func (w *NitroConfigWriter) writeScripts() error {
	if err := w.writeStartScript(); err != nil {
		return fmt.Errorf("write start.sh: %w", err)
	}
	if err := w.writeStopScript(); err != nil {
		return fmt.Errorf("write stop.sh: %w", err)
	}
	if err := w.writeResetScript(); err != nil {
		return fmt.Errorf("write reset.sh: %w", err)
	}
	if err := w.writeTestScript(); err != nil {
		return fmt.Errorf("write test.sh: %w", err)
	}
	return nil
}

// writeStartScript writes the two-phase startup script.
func (w *NitroConfigWriter) writeStartScript() error {
	w.logger.Info("writing start.sh")

	script := `#!/bin/bash
# =============================================================================
# Nitro Local Devnet - Two-Phase Startup Script
# Handles Issue #4208: batch-poster must be disabled during initial startup
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$BUNDLE_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

echo ""
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Nitro Local Devnet - Starting${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# Check if this is a restart (nitro-data volume exists and has content)
VOLUME_NAME="${PWD##*/}_nitro-data"
IS_RESTART=false

if docker volume inspect "$VOLUME_NAME" >/dev/null 2>&1; then
    # Volume exists, check if it has Nitro database (any /data/*/nitro directory)
    if docker run --rm -v "$VOLUME_NAME:/data" alpine sh -c 'find /data -type d -name nitro | grep -q .' >/dev/null 2>&1; then
        IS_RESTART=true
        log_info "Detected existing Nitro data - performing restart..."
    fi
fi

if [ "$IS_RESTART" = true ]; then
    # Simple restart - chain already initialized
    log_info "Starting all services with batch-poster enabled..."
    BATCH_POSTER_ENABLE=true docker compose up -d

    log_info "Waiting for services to be healthy..."
    sleep 10
else
    # Fresh start - need two-phase initialization
    log_info "Fresh deployment detected - using two-phase startup..."

    # Phase 1: Start WITHOUT batch-poster
    log_info "Phase 1: Starting infrastructure (batch-poster disabled)..."
    BATCH_POSTER_ENABLE=false docker compose up -d

    log_info "Waiting for services to be healthy..."
    sleep 5

    # Wait for Nitro to initialize (can take up to 120 seconds)
    log_info "Waiting for Nitro to initialize..."
    for i in {1..120}; do
        if docker logs nitro-sequencer 2>&1 | grep -q "HTTP server started.*8547"; then
            log_success "Nitro HTTP server started"
            break
        fi
        if [ $((i % 10)) -eq 0 ]; then
            echo "  Still initializing... ($i seconds)"
        fi
        sleep 1
    done

    sleep 5

    # Phase 2: Restart WITH batch-poster
    log_info "Phase 2: Enabling batch-poster..."
    BATCH_POSTER_ENABLE=true docker compose up -d nitro-sequencer

    log_info "Waiting for batch-poster to start..."
    sleep 10
fi

# Verify using RPC endpoint
if curl -sf http://localhost:8547 -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' >/dev/null 2>&1; then
    log_success "Nitro sequencer is running"
else
    log_error "Nitro sequencer failed to start"
    exit 1
fi

echo ""
log_success "Devnet started successfully!"
echo ""
echo "Endpoints:"
echo "  L1 (Anvil):     http://localhost:8545"
echo "  L2 (Nitro):     http://localhost:8547"
echo "  POPSigner:      http://localhost:3000"
echo "  Celestia DAS:   http://localhost:9876"
echo ""
echo "Run ./scripts/test.sh to verify functionality"
echo ""
`

	path := filepath.Join(w.bundleDir, "scripts", "start.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("start.sh written", slog.String("path", path))
	return nil
}

// writeStopScript writes the stop script.
func (w *NitroConfigWriter) writeStopScript() error {
	w.logger.Info("writing stop.sh")

	script := `#!/bin/bash
# Stop Nitro Local Devnet
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$BUNDLE_DIR"

echo "Stopping Nitro Local Devnet..."
docker compose down

echo "Done!"
`

	path := filepath.Join(w.bundleDir, "scripts", "stop.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("stop.sh written", slog.String("path", path))
	return nil
}

// writeResetScript writes the reset script.
func (w *NitroConfigWriter) writeResetScript() error {
	w.logger.Info("writing reset.sh")

	script := `#!/bin/bash
# Reset Nitro Local Devnet - Stops all services and removes volumes
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$BUNDLE_DIR"

echo "Stopping all services..."
docker compose down

echo "Removing Docker volumes..."
docker compose down -v

echo "Pruning unused Docker resources..."
docker system prune -f >/dev/null 2>&1 || true

echo "Reset complete!"
echo ""

read -p "Start the devnet now? [y/N] " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./scripts/start.sh
else
    echo "To start: ./scripts/start.sh"
fi
`

	path := filepath.Join(w.bundleDir, "scripts", "reset.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("reset.sh written", slog.String("path", path))
	return nil
}

// writeTestScript writes the health check and functionality test script.
func (w *NitroConfigWriter) writeTestScript() error {
	w.logger.Info("writing test.sh")

	script := fmt.Sprintf(`#!/bin/bash
# =============================================================================
# test.sh - Health check for Nitro Local Devnet
# =============================================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0

L1_RPC_URL="http://127.0.0.1:8545"
L2_RPC_URL="http://127.0.0.1:8547"
POPSIGNER_URL="http://127.0.0.1:3000"
CELESTIA_DAS_URL="http://127.0.0.1:9876"

DEPLOYER="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
DEPLOYER_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

test_pass() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

test_fail() {
    echo -e "${RED}✗${NC} $1"
    [ -n "$2" ] && echo "  $2"
    FAILED=$((FAILED + 1))
}

echo ""
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Nitro Local Devnet - Health Check${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# Service Health
echo -e "${YELLOW}>>> Service Health${NC}"

if L1_BLOCK=$(cast block-number --rpc-url "$L1_RPC_URL" 2>/dev/null); then
    L1_CHAIN=$(cast chain-id --rpc-url "$L1_RPC_URL" 2>/dev/null)
    test_pass "Anvil L1 - Chain $L1_CHAIN, Block $L1_BLOCK"
else
    test_fail "Anvil L1 not responding"
fi

if L2_BLOCK=$(cast block-number --rpc-url "$L2_RPC_URL" 2>/dev/null); then
    L2_CHAIN=$(cast chain-id --rpc-url "$L2_RPC_URL" 2>/dev/null)
    test_pass "Nitro L2 - Chain $L2_CHAIN, Block $L2_BLOCK"
else
    test_fail "Nitro L2 not responding"
fi

if curl -sf "$POPSIGNER_URL/health" >/dev/null 2>&1; then
    test_pass "POPSigner-Lite healthy"
else
    test_fail "POPSigner-Lite not responding"
fi

if curl -sf "$CELESTIA_DAS_URL/health" >/dev/null 2>&1; then
    test_pass "Celestia DAS healthy"
else
    test_fail "Celestia DAS not responding"
fi

if nc -z 127.0.0.1 26658 2>/dev/null; then
    test_pass "Localestia healthy"
else
    test_fail "Localestia not responding"
fi

# Contract Verification
echo ""
echo -e "${YELLOW}>>> Contract Verification (L1)${NC}"

BRIDGE="%s"
INBOX="%s"
ROLLUP="%s"

for contract in "Bridge:$BRIDGE" "Inbox:$INBOX" "Rollup:$ROLLUP"; do
    name="${contract%%:*}"
    addr="${contract##*:}"
    if [ -n "$addr" ] && [ "$addr" != "0x0000000000000000000000000000000000000000" ]; then
        code_len=$(cast code "$addr" --rpc-url "$L1_RPC_URL" 2>/dev/null | wc -c)
        if [ "$code_len" -gt 10 ]; then
            test_pass "$name deployed at ${addr:0:12}..."
        else
            test_fail "$name NOT deployed at $addr"
        fi
    fi
done

# Summary
echo ""
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Summary${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
    echo "Endpoints:"
    echo "  L1 (Anvil):  $L1_RPC_URL"
    echo "  L2 (Nitro):  $L2_RPC_URL"
    echo "  POPSigner:   $POPSIGNER_URL"
    echo "  Celestia:    $CELESTIA_DAS_URL"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  docker compose ps"
    echo "  docker compose logs nitro-sequencer --tail=50"
    exit 1
fi
`, w.result.contracts.Bridge.Hex(), w.result.contracts.Inbox.Hex(), w.result.contracts.Rollup.Hex())

	path := filepath.Join(w.bundleDir, "scripts", "test.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("test.sh written", slog.String("path", path))
	return nil
}

// writeREADME writes the README.md for the Nitro bundle.
func (w *NitroConfigWriter) writeREADME() error {
	w.logger.Info("writing README.md")

	readme := fmt.Sprintf(`# Nitro Local Devnet with Celestia DA

Pre-deployed development environment for Arbitrum Nitro rollups with Celestia DA.

## Quick Start

1. Ensure Docker is running

2. Start the devnet:
`+"```bash"+`
./scripts/start.sh
`+"```"+`

3. Verify services:
`+"```bash"+`
./scripts/test.sh
`+"```"+`

## Services

| Service | Port | Description |
|---------|------|-------------|
| Anvil (L1) | 8545 | Pre-deployed L1 chain |
| Nitro (L2) | 8547 | L2 sequencer RPC |
| POPSigner-Lite | 3000/8555 | Local signing service |
| Celestia DAS | 9876 | Celestia DA adapter |
| Localestia | 26658 | Mock Celestia network |

## Chain Info

- **L1 Chain ID**: %d
- **L2 Chain ID**: %d
- **L2 Chain Name**: %s

## Contract Addresses

- **Rollup**: %s
- **Inbox**: %s
- **Bridge**: %s
- **Sequencer Inbox**: %s
- **Stake Token (WETH)**: %s

## Scripts

- `+"`./scripts/start.sh`"+` - Start devnet (two-phase for Issue #4208)
- `+"`./scripts/stop.sh`"+` - Stop devnet
- `+"`./scripts/reset.sh`"+` - Reset all state
- `+"`./scripts/test.sh`"+` - Health check

## Two-Phase Startup

Due to [Nitro Issue #4208](https://github.com/OffchainLabs/nitro/issues/4208), the sequencer must start without the batch-poster enabled, initialize, then restart with batch-poster enabled. The start.sh script handles this automatically.

## Security Notice

**⚠️ FOR DEVELOPMENT USE ONLY ⚠️**

This bundle uses Anvil's well-known deterministic keys. Never use with real funds.

For production, migrate to [POPSigner Cloud](https://popsigner.com).
`,
		l1ChainID,
		nitroL2ChainID,
		nitroL2ChainName,
		w.result.contracts.Rollup.Hex(),
		w.result.contracts.Inbox.Hex(),
		w.result.contracts.Bridge.Hex(),
		w.result.contracts.SequencerInbox.Hex(),
		w.result.stakeToken.Hex(),
	)

	path := filepath.Join(w.bundleDir, "README.md")
	if err := os.WriteFile(path, []byte(readme), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	w.logger.Info("README.md written", slog.String("path", path))
	return nil
}
