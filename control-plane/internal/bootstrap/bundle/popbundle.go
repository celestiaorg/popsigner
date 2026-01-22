package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"time"
)

// createPopBundleBundle creates the POPKins Devnet Bundle structure.
//
// Bundle structure:
//
//	{chain-name}-popdeployer-bundle/
//	├── README.md
//	├── manifest.json
//	├── docker-compose.yml
//	├── .env.example
//	├── genesis.json              # L2 genesis state (~9MB)
//	├── rollup.json                # Rollup configuration
//	├── addresses.json             # Deployed contract addresses
//	├── config.toml                # Celestia DA configuration
//	├── jwt.txt                    # JWT secret for authentication
//	└── anvil-state.json           # Pre-deployed L1 state (~4MB)
func (b *Bundler) createPopBundleBundle(cfg *BundleConfig) (*BundleResult, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tarW := newTarWriter(tw)

	baseDir := fmt.Sprintf("%s-popdeployer-bundle", sanitizeName(cfg.ChainName))

	// Track files for manifest
	var files []FileEntry

	// ===========================================
	// ROOT FILES
	// ===========================================

	// docker-compose.yml
	if dockerCompose, ok := cfg.Artifacts["docker-compose.yml"]; ok {
		if err := tarW.addFile(baseDir+"/docker-compose.yml", dockerCompose); err != nil {
			return nil, fmt.Errorf("add docker-compose.yml: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "docker-compose.yml",
			Description: "Docker Compose configuration for full devnet (9 services)",
			Required:    true,
			SizeBytes:   int64(len(dockerCompose)),
		})
	}

	// .env.example
	if envExample, ok := cfg.Artifacts[".env.example"]; ok {
		if err := tarW.addFile(baseDir+"/.env.example", envExample); err != nil {
			return nil, fmt.Errorf("add .env.example: %w", err)
		}
		files = append(files, FileEntry{
			Path:        ".env.example",
			Description: "Environment variables (ready to use - no changes needed)",
			Required:    true,
			SizeBytes:   int64(len(envExample)),
		})
	}

	// genesis.json
	if genesis, ok := cfg.Artifacts["genesis.json"]; ok {
		if err := tarW.addFile(baseDir+"/genesis.json", genesis); err != nil {
			return nil, fmt.Errorf("add genesis.json: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "genesis.json",
			Description: "L2 genesis state with pre-deployed contracts",
			Required:    true,
			SizeBytes:   int64(len(genesis)),
		})
	}

	// rollup.json
	if rollup, ok := cfg.Artifacts["rollup.json"]; ok {
		if err := tarW.addFile(baseDir+"/rollup.json", rollup); err != nil {
			return nil, fmt.Errorf("add rollup.json: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "rollup.json",
			Description: "Rollup configuration",
			Required:    true,
			SizeBytes:   int64(len(rollup)),
		})
	}

	// addresses.json
	if addresses, ok := cfg.Artifacts["addresses.json"]; ok {
		if err := tarW.addFile(baseDir+"/addresses.json", addresses); err != nil {
			return nil, fmt.Errorf("add addresses.json: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "addresses.json",
			Description: "All deployed contract addresses",
			Required:    true,
			SizeBytes:   int64(len(addresses)),
		})
	}

	// config.toml
	if configToml, ok := cfg.Artifacts["config.toml"]; ok {
		if err := tarW.addFile(baseDir+"/config.toml", configToml); err != nil {
			return nil, fmt.Errorf("add config.toml: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "config.toml",
			Description: "Celestia DA configuration",
			Required:    true,
			SizeBytes:   int64(len(configToml)),
		})
	}

	// jwt.txt
	if jwt, ok := cfg.Artifacts["jwt.txt"]; ok {
		if err := tarW.addFileWithMode(baseDir+"/jwt.txt", jwt, 0600); err != nil {
			return nil, fmt.Errorf("add jwt.txt: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "jwt.txt",
			Description: "JWT secret for op-node/op-geth authentication",
			Required:    true,
			SizeBytes:   int64(len(jwt)),
		})
	}

	// anvil-state.json (pre-deployed L1 state)
	if anvilState, ok := cfg.Artifacts["anvil-state.json"]; ok {
		if err := tarW.addFile(baseDir+"/anvil-state.json", anvilState); err != nil {
			return nil, fmt.Errorf("add anvil-state.json: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "anvil-state.json",
			Description: "Pre-deployed L1 Anvil state (saves 10-15 minutes)",
			Required:    true,
			SizeBytes:   int64(len(anvilState)),
		})
	}

	// l1-chain-config.json (L1 chain configuration for op-node)
	if l1ChainConfig, ok := cfg.Artifacts["l1-chain-config.json"]; ok {
		if err := tarW.addFile(baseDir+"/l1-chain-config.json", l1ChainConfig); err != nil {
			return nil, fmt.Errorf("add l1-chain-config.json: %w", err)
		}
		files = append(files, FileEntry{
			Path:        "l1-chain-config.json",
			Description: "L1 chain configuration (required for op-node with Anvil)",
			Required:    true,
			SizeBytes:   int64(len(l1ChainConfig)),
		})
	}

	// ===========================================
	// README
	// ===========================================

	readme := generatePopBundleReadme(cfg)
	if err := tarW.addFile(baseDir+"/README.md", []byte(readme)); err != nil {
		return nil, fmt.Errorf("add README.md: %w", err)
	}
	files = append(files, FileEntry{
		Path:        "README.md",
		Description: "Quick start guide and documentation",
		Required:    false,
	})

	// ===========================================
	// MANIFEST
	// ===========================================

	manifest := &BundleManifest{
		Version:     "1.0",
		Stack:       StackPopBundle,
		ChainID:     cfg.ChainID,
		ChainName:   cfg.ChainName,
		GeneratedAt: time.Now().UTC(),
		Files:       files,
		POPSignerInfo: POPSignerInfo{
			Endpoint:         "http://localhost:8555",
			APIKeyConfigured: true,
			BatcherAddr:      cfg.BatcherAddress,
			ProposerAddr:     cfg.ProposerAddress,
		},
		Checksums: tarW.checksums,
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := tarW.addFile(baseDir+"/manifest.json", manifestBytes); err != nil {
		return nil, fmt.Errorf("add manifest.json: %w", err)
	}

	// Finalize
	data, err := finalizeTarGz(tw, gw, &buf)
	if err != nil {
		return nil, err
	}

	return &BundleResult{
		Data:      data,
		Filename:  fmt.Sprintf("%s-popdeployer-bundle.tar.gz", sanitizeName(cfg.ChainName)),
		Manifest:  manifest,
		SizeBytes: int64(len(data)),
		Checksum:  calculateBundleChecksum(data),
	}, nil
}

// generatePopBundleReadme creates the README.md documentation for POPKins Devnet Bundle.
func generatePopBundleReadme(cfg *BundleConfig) string {
	const readmeTemplate = `# %s - POPKins Devnet Bundle

This bundle contains a **complete, pre-deployed OP Stack + Celestia DA local devnet**.

All contracts are pre-deployed to save you 10-15 minutes of setup time!

## What's Included

- **Anvil L1** with pre-deployed OP Stack contracts
- **OP Stack L2** (op-geth + op-node + op-batcher + op-proposer)
- **Celestia DA** (Localestia mock network + op-alt-da server)
- **POPSigner-Lite** for local key management
- **Redis** (Localestia backend)

## Quick Start

### 1. Extract the Bundle

%scd %s-popdeployer-bundle%s

### 2. Start All Services

%sdocker compose up -d%s

That's it! No configuration needed - everything is pre-configured for local development.

### 3. Verify Services

Check that all 9 services are running:

%sdocker compose ps%s

You should see:
- redis
- anvil (L1)
- popsigner-lite
- localestia
- op-alt-da
- op-geth (L2 execution)
- op-node (L2 consensus)
- op-batcher
- op-proposer

## Chain Information

| Property | Value |
|----------|-------|
| L2 Chain ID | %d |
| L2 Chain Name | %s |
| L1 Chain ID | 31337 (Anvil) |
| DA Layer | Celestia (Localestia mock) |

## Endpoints

| Service | URL |
|---------|-----|
| L2 JSON-RPC | http://localhost:9545 |
| L1 JSON-RPC (Anvil) | http://localhost:8545 |
| POPSigner-Lite RPC | http://localhost:8555 |
| POPSigner-Lite REST | http://localhost:3000 |

## Testing Your L2

### Get the latest block number

%scurl -X POST http://localhost:9545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'%s

### Send a transaction

Use any Ethereum tool (cast, ethers.js, web3.py) with RPC http://localhost:9545

## Bundle Contents

| File | Description | Size |
|------|-------------|------|
| docker-compose.yml | All 9 services | ~5KB |
| .env.example | Ready-to-use config | ~1KB |
| genesis.json | L2 genesis state | ~9MB |
| rollup.json | Rollup config | ~2KB |
| addresses.json | Contract addresses | ~3KB |
| config.toml | Celestia DA config | ~1KB |
| jwt.txt | Authentication secret | 64 bytes |
| anvil-state.json | Pre-deployed L1 state | ~4MB |
| manifest.json | Bundle metadata | ~2KB |

**Total: ~15-20MB**

## Pre-Deployed Contracts

All OP Stack contracts are pre-deployed on the L1 (Anvil):

- L2OutputOracle
- OptimismPortal
- SystemConfig
- L1CrossDomainMessenger
- L1StandardBridge
- And more...

See **addresses.json** for the complete list with addresses.

## Key Management

This bundle uses **popsigner-lite** - a lightweight local signing service.

**Pre-configured keys:**
- Deployer: Anvil account #0
- Batcher: Anvil account #1
- Proposer: Anvil account #2

**API Key:** psk_local_dev_insecure_do_not_use_in_production

⚠️ **WARNING:** These keys are for local development only. Never use them in production!

## Stopping the Devnet

%sdocker compose down%s

To remove all data (reset to fresh state):

%sdocker compose down -v%s

## Troubleshooting

### Services not starting?

1. Check Docker is running
2. Check port availability (8545, 9545, 8555, 3000)
3. View logs: %sdocker compose logs -f [service]%s

### L2 not producing blocks?

1. Check op-node logs: %sdocker compose logs op-node%s
2. Check op-batcher logs: %sdocker compose logs op-batcher%s
3. Verify Anvil is running: %scurl http://localhost:8545%s

### Reset everything

%sdocker compose down -v
docker compose up -d%s

## What Makes This Special?

Unlike standard OP Stack deployments that require:
- Setting up L1 infrastructure
- Deploying contracts (10-15 minutes)
- Configuring keys and endpoints

This bundle is **ready to run** with:
- ✅ Contracts already deployed
- ✅ Keys pre-configured
- ✅ Services pre-wired
- ✅ Just run docker compose up!

## Use Cases

Perfect for:
- 🧪 Testing OP Stack features
- 📚 Learning rollup architecture
- 🔬 Experimenting with Celestia DA
- 🚀 Rapid prototyping
- 🎓 Educational demos

## Documentation

- POPSigner: https://docs.popsigner.com
- OP Stack: https://docs.optimism.io
- Celestia: https://docs.celestia.org

## Support

For questions or issues:
- GitHub: https://github.com/Bidon15/popsigner
- Email: support@popsigner.com
- Website: https://popsigner.com

---

**Powered by POPSigner** - Secure key management for blockchain infrastructure
`

	codeStart := "```bash\n"
	codeEnd := "\n```"
	return fmt.Sprintf(readmeTemplate,
		cfg.ChainName,
		codeStart, cfg.ChainName, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		cfg.ChainID, cfg.ChainName,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
		codeStart, codeEnd,
	)
}
