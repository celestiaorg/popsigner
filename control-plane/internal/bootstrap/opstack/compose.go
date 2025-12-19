package opstack

import (
	"bytes"
	"fmt"
	"text/template"
)

// Docker image versions for OP Stack services
const (
	OpNodeVersion    = "v1.9.4"
	OpBatcherVersion = "v1.9.4"
	OpProposerVersion = "v1.9.4"
	OpGethVersion    = "v1.101408.0"
	OpAltDAVersion   = "v0.10.0"
)

// dockerComposeTemplate is the template for generating OP Stack docker-compose.yml with Celestia DA.
const dockerComposeTemplate = `version: "3.8"

services:
{{- if .UseAltDA }}
  # =============================================================
  # OP-ALT-DA - Celestia DA Server (ghcr.io/celestiaorg/op-alt-da)
  # Posts batches to Celestia and serves commitments to op-node/op-batcher
  # =============================================================
  op-alt-da:
    image: ghcr.io/celestiaorg/op-alt-da:{{ .OpAltDAVersion }}
    restart: unless-stopped
    ports:
      - "3100:3100"
    volumes:
      - ./config.toml:/config/config.toml:ro
      - celestia-keys:/keys:ro
    command:
      - --config=/config/config.toml

{{- end }}
  # =============================================================
  # OP GETH - L2 execution layer
  # Must be initialized with genesis before first run
  # =============================================================
  op-geth:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:{{ .OpGethVersion }}
    restart: unless-stopped
    ports:
      - "8545:8545"     # JSON-RPC
      - "8546:8546"     # WebSocket
    volumes:
      - ./genesis.json:/config/genesis.json:ro
      - ./jwt.txt:/config/jwt.txt:ro
      - geth-data:/data
    command:
      - geth
      - --datadir=/data
      - --http
      - --http.addr=0.0.0.0
      - --http.port=8545
      - --http.api=eth,net,web3,debug,txpool
      - --http.corsdomain=*
      - --http.vhosts=*
      - --ws
      - --ws.addr=0.0.0.0
      - --ws.port=8546
      - --ws.api=eth,net,web3
      - --ws.origins=*
      - --authrpc.addr=0.0.0.0
      - --authrpc.port=8551
      - --authrpc.jwtsecret=/config/jwt.txt
      - --authrpc.vhosts=*
      - --networkid=${CHAIN_ID}
      - --gcmode=archive
      - --syncmode=full
      - --rollup.sequencerhttp=http://op-node:8547

  # =============================================================
  # OP NODE - Derives L2 state from L1, serves as rollup consensus
  # =============================================================
  op-node:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:{{ .OpNodeVersion }}
    restart: unless-stopped
    ports:
      - "9545:8545"     # RPC
      - "9003:9003"     # P2P
    volumes:
      - ./rollup.json:/config/rollup.json:ro
      - ./jwt.txt:/config/jwt.txt:ro
    depends_on:
      - op-geth
{{- if .UseAltDA }}
      - op-alt-da
{{- end }}
    command:
      - op-node
      - --l1=${L1_RPC_URL}
      - --l2=http://op-geth:8551
      - --l2.jwt-secret=/config/jwt.txt
      - --rollup.config=/config/rollup.json
      - --rpc.addr=0.0.0.0
      - --rpc.port=8545
      - --p2p.listen.tcp=9003
      - --p2p.listen.udp=9003
      - --sequencer.enabled
      - --sequencer.l1-confs=5
      # POPSigner integration for sequencer signing
      - --signer.endpoint=${POPSIGNER_ENDPOINT}
      - --signer.address=${SEQUENCER_ADDRESS}
      - --signer.header=X-API-Key:${POPSIGNER_API_KEY}
{{- if .UseAltDA }}
      # Celestia Alt-DA configuration
      - --altda.enabled=true
      - --altda.da-service=http://op-alt-da:3100
{{- end }}

  # =============================================================
  # OP BATCHER - Submits L2 batches to DA layer
  # Uses POPSigner for transaction signing
  # =============================================================
  op-batcher:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:{{ .OpBatcherVersion }}
    restart: unless-stopped
    depends_on:
      - op-node
      - op-geth
{{- if .UseAltDA }}
      - op-alt-da
{{- end }}
    command:
      - op-batcher
      - --l1-eth-rpc=${L1_RPC_URL}
      - --l2-eth-rpc=http://op-geth:8545
      - --rollup-rpc=http://op-node:8545
      - --poll-interval=1s
      - --sub-safety-margin=10
      - --max-channel-duration=1
      # POPSigner integration for batcher signing
      - --signer.endpoint=${POPSIGNER_ENDPOINT}
      - --signer.address=${BATCHER_ADDRESS}
      - --signer.header=X-API-Key:${POPSIGNER_API_KEY}
{{- if .UseAltDA }}
      # Celestia Alt-DA configuration
      - --altda.enabled=true
      - --altda.da-service=http://op-alt-da:3100
{{- end }}

  # =============================================================
  # OP PROPOSER - Submits L2 state roots to L1
  # Uses POPSigner for transaction signing
  # =============================================================
  op-proposer:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:{{ .OpProposerVersion }}
    restart: unless-stopped
    depends_on:
      - op-node
    command:
      - op-proposer
      - --l1-eth-rpc=${L1_RPC_URL}
      - --rollup-rpc=http://op-node:8545
      - --l2oo-address=${L2_OUTPUT_ORACLE_ADDRESS}
      - --poll-interval=12s
      # POPSigner integration for proposer signing
      - --signer.endpoint=${POPSIGNER_ENDPOINT}
      - --signer.address=${PROPOSER_ADDRESS}
      - --signer.header=X-API-Key:${POPSIGNER_API_KEY}

volumes:
  geth-data:
{{- if .UseAltDA }}
  celestia-keys:
    # User must populate with their Celestia keyring directory
    # See README.md for instructions on setting up Celestia keys
{{- end }}

networks:
  default:
    name: opstack-{{ .ChainName }}
`

// opAltDAConfigTemplate is the config.toml template for op-alt-da v0.10.0
const opAltDAConfigTemplate = `# op-alt-da Configuration for Celestia DA
# See https://github.com/celestiaorg/op-alt-da for documentation

[server]
addr = "0.0.0.0"
port = 3100
read_timeout = "30s"
write_timeout = "120s"

[celestia]
# Celestia namespace for your chain's data (hex-encoded, 10 bytes)
# Generate with: openssl rand -hex 10
namespace = "{{ .CelestiaNamespace }}"

# Core gRPC endpoint for submitting blobs
# Testnet (mocha-4): consensus-full-mocha-4.celestia-mocha.com:9090
# Mainnet: public endpoints or your own consensus node
core_grpc_addr = "{{ .CelestiaCoreGRPC }}"
core_grpc_tls_enabled = true

# Local keyring configuration (required for v0.10.0)
# Auth tokens do NOT work for write operations in v0.10.0
keyring_path = "/keys"
default_key_name = "{{ .CelestiaKeyName }}"

# P2P network for header sync
# Options: mocha-4 (testnet), celestia (mainnet)
p2p_network = "{{ .CelestiaNetwork }}"

[submission]
max_blob_size = "2MB"
gas_price = 0.002
gas_multiplier = 1.1
`

// ComposeTemplateVars holds variables for Docker Compose template.
type ComposeTemplateVars struct {
	// Chain identification
	ChainName string
	ChainID   uint64

	// L1 configuration
	L1RPC     string
	L1ChainID uint64

	// POPSigner configuration
	POPSignerEndpoint string
	POPSignerAPIKey   string

	// Role addresses
	SequencerAddress string
	BatcherAddress   string
	ProposerAddress  string

	// Contract addresses
	L2OutputOracle string

	// DA configuration - always Celestia
	UseAltDA bool // Always true - POPKins only supports Celestia

	// Celestia configuration
	CelestiaNamespace string
	CelestiaCoreGRPC  string
	CelestiaKeyName   string
	CelestiaNetwork   string

	// Image versions
	OpNodeVersion    string
	OpBatcherVersion string
	OpProposerVersion string
	OpGethVersion    string
	OpAltDAVersion   string
}

// GenerateDockerCompose generates a docker-compose.yml from the deployment config.
func GenerateDockerCompose(cfg *DeploymentConfig, artifacts *OPStackArtifacts) (string, error) {
	tmpl, err := template.New("docker-compose").Parse(dockerComposeTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	vars := ComposeTemplateVars{
		ChainName:         sanitizeChainName(cfg.ChainName),
		ChainID:           cfg.ChainID,
		L1RPC:             cfg.L1RPC,
		L1ChainID:         cfg.L1ChainID,
		POPSignerEndpoint: cfg.POPSignerEndpoint,
		POPSignerAPIKey:   "${POPSIGNER_API_KEY}", // Placeholder for .env
		SequencerAddress:  cfg.SequencerAddress,
		BatcherAddress:    cfg.BatcherAddress,
		ProposerAddress:   cfg.ProposerAddress,
		// Always use Celestia DA - POPKins only supports Celestia
		UseAltDA:          true,

		// Celestia configuration
		CelestiaNamespace: cfg.CelestiaNamespace, // Generated or user-provided
		CelestiaCoreGRPC:  "${CELESTIA_CORE_GRPC}",
		CelestiaKeyName:   "${CELESTIA_KEY_NAME}",
		CelestiaNetwork:   "${CELESTIA_NETWORK}",

		// Image versions
		OpNodeVersion:    OpNodeVersion,
		OpBatcherVersion: OpBatcherVersion,
		OpProposerVersion: OpProposerVersion,
		OpGethVersion:    OpGethVersion,
		OpAltDAVersion:   OpAltDAVersion,
	}

	// Get L2OutputOracle address from artifacts if available
	if artifacts != nil {
		vars.L2OutputOracle = artifacts.Addresses.L2OutputOracle
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// AltDAConfigVars holds variables for the op-alt-da config.toml template.
type AltDAConfigVars struct {
	CelestiaNamespace string
	CelestiaCoreGRPC  string
	CelestiaKeyName   string
	CelestiaNetwork   string
}

// GenerateAltDAConfig generates the config.toml for op-alt-da (Celestia).
// POPKins only supports Celestia as the DA layer, so this always generates config.
func GenerateAltDAConfig(cfg *DeploymentConfig) (string, error) {
	tmpl, err := template.New("altda-config").Parse(opAltDAConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Use the namespace from config (generated or user-provided)
	// Other values use environment variable placeholders
	vars := AltDAConfigVars{
		CelestiaNamespace: cfg.CelestiaNamespace,
		CelestiaCoreGRPC:  "${CELESTIA_CORE_GRPC}",
		CelestiaKeyName:   "${CELESTIA_KEY_NAME}",
		CelestiaNetwork:   "${CELESTIA_NETWORK}",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// sanitizeChainName converts a chain name to a valid Docker network name.
func sanitizeChainName(name string) string {
	if name == "" {
		return "opstack"
	}

	// Replace spaces and special characters with hyphens
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}

	if len(result) == 0 {
		return "opstack"
	}

	return string(result)
}

// SanitizeChainNameForFilename converts a chain name to a valid filename.
// Exported for use by the handler package.
func SanitizeChainNameForFilename(name string) string {
	return sanitizeChainName(name)
}

// GenerateMinimalCompose generates a minimal docker-compose for testing.
func GenerateMinimalCompose(cfg *DeploymentConfig) (string, error) {
	return GenerateDockerCompose(cfg, nil)
}
