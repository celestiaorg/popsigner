# Testing Guide: POPSigner-Lite + Pop-Deployer

This guide walks through testing the complete implementation of popsigner-lite and pop-deployer.

## Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Foundry (anvil, cast)
- ~30GB disk space for bundle creation

## Phase 1: Test POPSigner-Lite Standalone

### 1.1 Build popsigner-lite

```bash
cd popsigner/control-plane/cmd/popsigner-lite
go build -o popsigner-lite .
```

Expected output: Binary `popsigner-lite` (~26MB)

### 1.2 Run popsigner-lite

```bash
./popsigner-lite
```

Expected output:
```
{"level":"INFO","msg":"Starting popsigner-lite","version":"1.0.0"}
{"level":"INFO","msg":"Loading Anvil deterministic keys..."}
{"level":"INFO","msg":"Loaded Anvil keys","count":10}
{"level":"INFO","msg":"Available addresses","addresses":["0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",...]}
{"level":"INFO","msg":"Starting JSON-RPC server","address":":8545"}
{"level":"INFO","msg":"Starting REST API server","address":":3000"}
{"level":"INFO","msg":"popsigner-lite is ready","jsonrpc_url":"http://localhost:8545","rest_api_url":"http://localhost:3000"}
```

### 1.3 Verify REST API (in new terminal)

```bash
# Health check
curl http://localhost:3000/health

# Expected: {"status":"ok","version":"1.0.0"}

# List keys
curl http://localhost:3000/v1/keys

# Expected: JSON array with 10 Anvil keys
```

### 1.4 Verify JSON-RPC API

```bash
# Get accounts
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}'

# Expected: {"jsonrpc":"2.0","result":["0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",...]}

# Health status (required by OP Stack)
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"health_status","params":[],"id":1}'

# Expected: {"jsonrpc":"2.0","result":{"status":"ok"},"id":1}
```

### 1.5 Test transaction signing

```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_signTransaction",
    "params": [{
      "from": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
      "to": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
      "value": "0x1",
      "gas": "0x5208",
      "gasPrice": "0x3b9aca00",
      "nonce": "0x0",
      "chainId": "0x7a69"
    }],
    "id": 2
  }'

# Expected: {"jsonrpc":"2.0","result":"0x...","id":2} (hex-encoded signed transaction)
```

**✅ Success Criteria:**
- popsigner-lite starts without errors
- Both servers (REST + JSON-RPC) respond
- All 10 Anvil keys are loaded
- Transaction signing returns valid signature

Stop popsigner-lite (Ctrl+C) before proceeding.

---

## Phase 2: Test Pop-Deployer (Bundle Creation)

**⚠️ WARNING:** This step will:
- Start Anvil and popsigner-lite automatically
- Deploy OP Stack contracts (~5-10 minutes)
- Use ~4GB of disk space for bundle

### 2.1 Build pop-deployer

```bash
cd popsigner/control-plane/cmd/pop-deployer
go build -o pop-deployer .
```

Expected output: Binary `pop-deployer` (~36MB)

### 2.2 Run pop-deployer

```bash
# Run with default settings (uses /tmp/pop-deployer-bundle)
./pop-deployer

# Or specify custom bundle directory for debugging
./pop-deployer -bundle-dir ./bundle
```

Expected execution flow:
```
🚀 POPKins Bundle Builder
Creating pre-deployed local devnet bundle...
Bundle directory: /tmp/pop-deployer-bundle

1️⃣  Starting ephemeral Anvil...
   Anvil is ready

2️⃣  Starting popsigner-lite...
   popsigner-lite is ready

3️⃣  Deploying OP Stack contracts...
   [Deployment logs...]
   Deployment completed successfully

4️⃣  Exporting Anvil state...
   Anvil state exported (size: ~4MB)

5️⃣  Writing bundle configs...
   genesis.json written (size: ~9MB)
   rollup.json written
   addresses.json written
   jwt.txt written
   docker-compose.yml written
   .env.example and .env written
   README.md written

6️⃣  Creating bundle archive...
   Bundle archive created

✅ Bundle created successfully!
📦 File: opstack-local-devnet-bundle.tar.gz
```

**Note:** By default, pop-deployer uses `/tmp/pop-deployer-bundle` for temporary build files. The directory is auto-cleaned on each run and by OS tmpwatch. Use `-bundle-dir ./bundle` if you need to inspect build artifacts.

### 2.3 Verify bundle contents

```bash
# Check bundle size
ls -lh opstack-local-devnet-bundle.tar.gz

# Expected: ~15-20MB compressed

# List contents
tar tzf opstack-local-devnet-bundle.tar.gz

# Expected files:
# - anvil-state.json
# - genesis.json
# - rollup.json
# - addresses.json
# - docker-compose.yml
# - .env
# - .env.example
# - README.md
# - jwt.txt
```

### 2.4 Extract and inspect bundle

```bash
# Extract to temporary directory
mkdir -p /tmp/test-bundle
tar xzf opstack-local-devnet-bundle.tar.gz -C /tmp/test-bundle
cd /tmp/test-bundle

# Verify critical files
ls -lh genesis.json    # Should be ~9MB
ls -lh anvil-state.json # Should be ~4MB
ls -lh rollup.json
cat .env               # Should show all environment variables

# Check genesis.json structure
head -20 genesis.json

# Expected: Valid JSON with config, alloc, etc.
```

**✅ Success Criteria:**
- pop-deployer completes without errors
- Bundle archive created (~15-20MB)
- All 9 files present in bundle
- genesis.json and anvil-state.json have expected sizes

---

## Phase 3: Test Bundle (Docker Compose)

**⚠️ NOTE:** Due to localestia limitations mentioned, this test focuses on verifying the bundle structure and service startup. For full DA validation, use Mocha testnet.

### 3.1 Start the devnet

```bash
cd /tmp/test-bundle

# Start all services
docker compose up -d

# Watch logs
docker compose logs -f
```

Expected services to start:
1. anvil (loads pre-deployed state)
2. popsigner-lite (Anvil keys)
3. localestia (mock Celestia)
4. op-geth (initializes from genesis.json)
5. op-node (waits for op-geth)
6. op-batcher (waits for op-node + popsigner-lite)
7. op-proposer (waits for op-node + popsigner-lite)

### 3.2 Verify service health

```bash
# Check all services are running
docker compose ps

# Expected: All services "Up" with healthy status

# Test Anvil (L1)
cast block-number --rpc-url http://localhost:8545

# Expected: Block number > 0 (from pre-deployed state)

# Test POPSigner-Lite
curl http://localhost:3000/health

# Expected: {"status":"ok","version":"1.0.0"}

curl -X POST http://localhost:8555 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}'

# Expected: 10 Anvil addresses

# Test OP-Geth (L2)
cast block-number --rpc-url http://localhost:9545

# Expected: Block number >= 0

# Test OP-Node
curl http://localhost:7545

# Expected: JSON-RPC response or method not found (healthy)
```

### 3.3 Verify pre-deployed contracts on L1

```bash
# Check deployer balance (should be less than 10000 ETH - used for deployment)
cast balance 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 --rpc-url http://localhost:8545

# Check addresses.json for deployed contracts
cat addresses.json | jq '.deployment'

# Expected: Shows create2_salt, deployer/batcher/proposer addresses

cat addresses.json | jq '.superchain'

# Expected: Shows superchain contract addresses (non-empty)
```

### 3.4 Test L2 transaction submission

```bash
# Send transaction on L2
cast send \
  --rpc-url http://localhost:9545 \
  --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  --value 1ether \
  0x70997970C51812dc3A010C7d01b50e0d17dc79C8

# Expected: Transaction hash

# Verify transaction
cast tx <TX_HASH> --rpc-url http://localhost:9545

# Expected: Transaction details
```

### 3.5 Monitor batch submission (optional)

```bash
# Watch op-batcher logs
docker compose logs -f op-batcher

# Expected: Should see batch submission attempts
# NOTE: May show errors about Celestia DA if localestia is incomplete
# This is expected per the localestia limitation mentioned
```

### 3.6 Stop devnet

```bash
docker compose down

# To completely wipe state:
docker compose down -v
```

**✅ Success Criteria:**
- All services start successfully
- Anvil loads pre-deployed state (block number > 0)
- POPSigner-Lite responds to RPC calls
- OP-Geth initializes and accepts transactions
- Deployed contract addresses present in addresses.json

---

## Phase 4: Test with Mocha Testnet (Optional - For Full Validation)

To test with real Celestia DA instead of localestia:

### 4.1 Update configuration

```bash
cd /tmp/test-bundle

# Edit .env
nano .env

# Change:
# CELESTIA_DA_SERVER=http://localestia:26658
# To:
# CELESTIA_DA_SERVER=https://mocha-4-consensus.celestia-mocha.com
```

### 4.2 Restart with Mocha

```bash
# Stop localestia (not needed)
docker compose stop localestia

# Restart op-node and op-batcher
docker compose restart op-node op-batcher

# Monitor logs
docker compose logs -f op-batcher

# Expected: Successful blob posting to Mocha testnet
```

**✅ Success Criteria:**
- op-batcher successfully posts blobs to Mocha
- No ClientTX consensus API errors

---

## Quick Validation Checklist

### POPSigner-Lite
- [ ] Builds successfully (26MB binary)
- [ ] Starts on ports 3000 (REST) and 8545 (JSON-RPC)
- [ ] Loads 10 Anvil keys
- [ ] REST API /health responds
- [ ] JSON-RPC eth_accounts returns 10 addresses
- [ ] JSON-RPC eth_signTransaction signs transactions

### Pop-Deployer
- [ ] Builds successfully (36MB binary)
- [ ] Starts Anvil automatically
- [ ] Starts popsigner-lite automatically
- [ ] Deploys OP Stack contracts
- [ ] Exports anvil-state.json (~4MB)
- [ ] Generates genesis.json (~9MB)
- [ ] Creates bundle.tar.gz (~15-20MB)
- [ ] Bundle contains all 9 required files

### Docker Compose Bundle
- [ ] Extracts successfully
- [ ] docker-compose.yml present
- [ ] All services start (7 total)
- [ ] Anvil loads pre-deployed state
- [ ] POPSigner-Lite accessible
- [ ] OP-Geth initializes from genesis.json
- [ ] L2 accepts transactions
- [ ] Contract addresses in addresses.json

---

## Troubleshooting

### POPSigner-Lite fails to start
- Check port 3000 and 8545 are not in use: `lsof -i :3000 -i :8545`
- Check logs for errors

### Pop-Deployer fails during deployment
- Ensure Anvil is not already running: `pkill anvil`
- Check disk space: `df -h`
- Check logs for specific deployment errors

### Docker Compose fails to start
- Check Docker daemon: `docker ps`
- Check logs: `docker compose logs`
- Verify .env file exists: `ls -la .env`

### Localestia DA errors
- Expected limitation - switch to Mocha testnet (see Phase 4)

---

## Expected Artifacts

After successful execution:

```
popsigner/control-plane/cmd/popsigner-lite/
└── popsigner-lite (26MB binary)

popsigner/control-plane/cmd/pop-deployer/
├── pop-deployer (36MB binary)
└── opstack-local-devnet-bundle.tar.gz (~15-20MB)

/tmp/pop-deployer-bundle/ (temporary, auto-cleaned)
├── anvil-state.json (~4MB)
├── genesis.json (~9MB)
├── rollup.json
├── addresses.json
├── docker-compose.yml
├── .env
├── .env.example
├── README.md
└── jwt.txt
```

**Note:** The `/tmp/pop-deployer-bundle/` directory is temporary and removed on each run. The persistent output is the `.tar.gz` file.

---

## Timeline

- **Phase 1** (POPSigner-Lite): ~2 minutes
- **Phase 2** (Pop-Deployer): ~10-15 minutes (includes contract deployment)
- **Phase 3** (Docker Compose): ~5 minutes (service startup)
- **Total**: ~20-25 minutes for complete validation

---

## Next Steps After Validation

1. ✅ POPSigner-Lite working → Can be used as drop-in replacement for POPSigner Cloud
2. ✅ Pop-Deployer working → Can generate bundles for distribution
3. ✅ Docker Compose working → Bundle is ready for users

To publish:
- Build Docker image for popsigner-lite: `docker build -t popsigner-lite .`
- Push to registry: `docker push ghcr.io/celestiaorg/popsigner-lite:latest`
- Upload bundle to popsigner.com or GitHub releases
