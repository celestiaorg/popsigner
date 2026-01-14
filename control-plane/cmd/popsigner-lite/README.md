# POPSigner-Lite

A lightweight, local-first signing service for OP Stack development. POPSigner-Lite is API-compatible with POPSigner Cloud but runs entirely offline with Anvil's deterministic keys.

## Features

- **API Compatible**: Drop-in replacement for POPSigner Cloud API
- **JSON-RPC Server**: Ethereum-compatible JSON-RPC 2.0 interface on port 8545
- **REST API**: Management API on port 3000 with Celestia/Cosmos SDK support
- **Anvil Keys**: Pre-loaded with 10 deterministic Anvil accounts
- **In-Memory Keystore**: Zero dependencies, fast startup
- **OP Stack Ready**: Supports all OP Stack signing methods:
  - `health_status`
  - `eth_accounts`
  - `eth_signTransaction`
  - `eth_sign`
  - `personal_sign`
  - `opsigner_signBlockPayload`
  - `opsigner_signBlockPayloadV2`
- **Celestia/Cosmos SDK Support**: REST API signing with SHA-256 hashing (via `prehashed` parameter)

## Quick Start

### Using Docker

```bash
# Build
docker build -t popsigner-lite -f control-plane/cmd/popsigner-lite/Dockerfile .

# Run
docker run -p 3000:3000 -p 8545:8545 popsigner-lite

# Health check
curl http://localhost:3000/health
```

### Using Go

```bash
# Install dependencies
cd control-plane
go mod download

# Run
go run ./cmd/popsigner-lite

# Or build and run
go build -o popsigner-lite ./cmd/popsigner-lite
./popsigner-lite
```

## API Reference

### JSON-RPC Endpoints (Port 8545)

Compatible with Ethereum JSON-RPC clients:

```bash
# Get accounts
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_accounts",
    "params": [],
    "id": 1
  }'

# Sign transaction
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
```

### REST API Endpoints (Port 3000)

#### List Keys
```bash
GET /v1/keys
```

#### Get Key
```bash
GET /v1/keys/:id
```

#### Create Key
```bash
POST /v1/keys
Content-Type: application/json

{
  "name": "my-key"
}
```

#### Delete Key
```bash
DELETE /v1/keys/:id
```

#### Sign Data
```bash
POST /v1/keys/:id/sign
Content-Type: application/json

{
  "data": "0x1234...",
  "prehashed": false  # Optional: true if data is already hashed (default: false)
}
```

**Ethereum signing** (Keccak256, default for JSON-RPC):
```bash
# Data will be hashed with Keccak256 automatically
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_signTransaction","params":[...],"id":1}'
```

**Celestia/Cosmos SDK signing** (SHA-256, via REST API):
```bash
# Data will be hashed with SHA-256 (prehashed=false)
curl -X POST http://localhost:3000/v1/keys/anvil-9/sign \
  -H "Content-Type: application/json" \
  -d '{"data":"0x48656c6c6f","prehashed":false}'
```

See [SIGNING-COMPATIBILITY.md](./SIGNING-COMPATIBILITY.md) for details on maintaining compatibility with POPSigner Cloud.

#### Batch Sign
```bash
POST /v1/sign/batch
Content-Type: application/json

{
  "items": [
    {
      "key_id": "anvil-0",
      "data": "0x1234..."
    },
    {
      "key_id": "anvil-1",
      "data": "0x5678..."
    }
  ]
}
```

## Anvil Keys

POPSigner-Lite comes pre-loaded with Anvil's 10 deterministic keys:

| Index | Address | Key ID |
|-------|---------|--------|
| 0 | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` | anvil-0 |
| 1 | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | anvil-1 |
| 2 | `0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC` | anvil-2 |
| 3 | `0x90F79bf6EB2c4f870365E785982E1f101E93b906` | anvil-3 |
| 4 | `0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65` | anvil-4 |
| 5 | `0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc` | anvil-5 |
| 6 | `0x976EA74026E726554dB657fA54763abd0C3a0aa9` | anvil-6 |
| 7 | `0x14dC79964da2C08b23698B3D3cc7Ca32193d9955` | anvil-7 |
| 8 | `0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f` | anvil-8 |
| 9 | `0xa0Ee7A142d267C1f36714E4a8F75612F20a79720` | anvil-9 |

These keys are derived from Anvil's default mnemonic:
```
test test test test test test test test test test test junk
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `JSONRPC_PORT` | `8545` | JSON-RPC server port |
| `REST_API_PORT` | `3000` | REST API server port |

## OP Stack Integration

POPSigner-Lite works seamlessly with OP Stack components:

### op-batcher
```bash
op-batcher \
  --l2-eth-rpc=http://op-geth:8545 \
  --rollup-rpc=http://op-node:9545 \
  --l1-eth-rpc=http://anvil:8545 \
  --signer.endpoint=http://popsigner-lite:8545 \
  --signer.address=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
  --signer.header=X-API-Key:psk_local_dev_00000000000000000000000000000000
```

### op-proposer
```bash
op-proposer \
  --rollup-rpc=http://op-node:9545 \
  --l1-eth-rpc=http://anvil:8545 \
  --signer.endpoint=http://popsigner-lite:8545 \
  --signer.address=0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC \
  --signer.header=X-API-Key:psk_local_dev_00000000000000000000000000000000
```

## Architecture

```
popsigner-lite/
├── internal/
│   ├── keystore/       # In-memory key storage
│   │   ├── keystore.go # Thread-safe keystore
│   │   └── anvil.go    # Anvil key loader
│   ├── signer/         # Signing logic
│   │   ├── ethereum.go      # ECDSA signing
│   │   └── transaction.go   # Transaction signing
│   ├── jsonrpc/        # JSON-RPC server
│   │   ├── handler.go       # Request handler
│   │   ├── server.go        # Server setup
│   │   ├── types.go         # RPC types
│   │   ├── health.go        # health_status
│   │   ├── eth_accounts.go  # eth_accounts
│   │   ├── eth_sign_transaction.go  # eth_signTransaction
│   │   ├── eth_sign.go      # eth_sign, personal_sign
│   │   └── opsigner_sign_block.go   # OP Stack signing
│   └── api/            # REST API
│       ├── router.go   # Route setup
│       ├── types.go    # API types
│       ├── keys.go     # Key management
│       └── sign.go     # Signing endpoints
├── main.go             # Entry point
├── Dockerfile          # Container image
└── README.md           # This file
```

## Security Notice

**⚠️ FOR DEVELOPMENT USE ONLY ⚠️**

POPSigner-Lite uses Anvil's well-known deterministic keys. These keys are publicly known and should **NEVER** be used in production or with real funds.

For production use, migrate to [POPSigner Cloud](https://popsigner.com) with proper key management and HSM backing.

## License

Same as parent POPSigner project.

## Support

- Documentation: https://docs.popsigner.com
- Issues: https://github.com/celestiaorg/popsigner/issues
