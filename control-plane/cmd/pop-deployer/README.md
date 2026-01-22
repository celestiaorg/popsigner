# Local OP Stack Devnet with Celestia DA

Pre-deployed development environment for OP Stack rollups.

## Quick Start

1. Ensure Docker is running

2. Start the devnet:
```bash
docker compose up -d
```

3. Wait for L2 to start (~30 seconds):
```bash
docker compose logs -f op-node
```

4. Verify services:
```bash
# Check Anvil (L1)
cast block-number --rpc-url http://localhost:9546

# Check OP-Geth (L2)
cast block-number --rpc-url http://localhost:8545

# Check POPSigner-Lite
curl http://localhost:3000/health
```

## Services

- **Anvil** (L1): http://localhost:9546
- **POPSigner-Lite**: http://localhost:3000 (REST), http://localhost:8555 (JSON-RPC)
- **Localestia** (Mock Celestia): http://localhost:26658
- **OP-Geth** (L2): http://localhost:8545
- **OP-Node**: http://localhost:9545

## What's Inside

This bundle contains:
- **Pre-deployed OP Stack contracts** on Anvil L1
- **genesis.json** - L2 genesis with contract state
- **rollup.json** - L2 rollup configuration
- **addresses.json** - All contract addresses
- **anvil-state.json** - Pre-deployed Anvil state
- **popsigner-lite** - Local signing service
- **localestia** - Mock Celestia DA layer

## Chain Info

- **Chain ID**: 42069
- **Chain Name**: local-opstack-devnet
- **L1 Chain ID**: 31337
- **Block Time**: 2 seconds
- **Gas Limit**: 30000000

## Deployment Info

- **Deployer**: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
- **Batcher**: 0x70997970C51812dc3A010C7d01b50e0d17dc79C8
- **Proposer**: 0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC
- **CREATE2 Salt**: 0x7e0b0fd606843623a14b91cf4393ceca253ce03b9179bbaec3e3bfa4c51fe5ef

## Reset

To wipe L2 state and restart:
```bash
docker compose down -v
docker compose up -d
```

Note: This only resets L2 data. L1 contracts remain pre-deployed.

## Security Notice

**⚠️ FOR DEVELOPMENT USE ONLY ⚠️**

This bundle uses Anvil's well-known deterministic keys. Never use with real funds.

For production, migrate to [POPSigner Cloud](https://popsigner.com).
