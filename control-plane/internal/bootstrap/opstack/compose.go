package opstack

import (
	"bytes"
	"fmt"
	"text/template"
)

// dockerComposeTemplate is the template for generating OP Stack docker-compose.yml.
const dockerComposeTemplate = `version: "3.8"

services:
  # =============================================================
  # OP NODE - Derives L2 state from L1
  # =============================================================
  op-node:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.9.0
    restart: unless-stopped
    ports:
      - "9545:8545"     # RPC
      - "9003:9003"     # P2P
    volumes:
      - ./config/rollup.json:/config/rollup.json:ro
      - ./secrets:/secrets:ro
      - op-node-data:/data
    environment:
      - OP_NODE_L1_ETH_RPC=${L1_RPC_URL}
      - OP_NODE_L2_ENGINE_RPC=http://op-geth:8551
      - OP_NODE_L2_ENGINE_AUTH=/secrets/jwt.txt
      - OP_NODE_ROLLUP_CONFIG=/config/rollup.json
      - OP_NODE_RPC_ADDR=0.0.0.0
      - OP_NODE_RPC_PORT=8545
      - OP_NODE_P2P_LISTEN_ADDR=0.0.0.0:9003
{{- if .UseAltDA }}
      # Alt-DA configuration
      - OP_NODE_ALTDA_ENABLED=true
      - OP_NODE_ALTDA_DA_SERVER=http://op-alt-da:3100
{{- end }}
    depends_on:
      - op-geth
{{- if .UseAltDA }}
      - op-alt-da
{{- end }}

  # =============================================================
  # OP GETH - L2 execution layer
  # =============================================================
  op-geth:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101408.0
    restart: unless-stopped
    ports:
      - "8545:8545"     # JSON-RPC
      - "8546:8546"     # WebSocket
    volumes:
      - ./genesis/genesis.json:/genesis.json:ro
      - ./secrets:/secrets:ro
      - op-geth-data:/data
    environment:
      - GETH_DATADIR=/data
      - GETH_HTTP_ADDR=0.0.0.0
      - GETH_HTTP_PORT=8545
      - GETH_WS_ADDR=0.0.0.0
      - GETH_WS_PORT=8546
      - GETH_AUTHRPC_ADDR=0.0.0.0
      - GETH_AUTHRPC_PORT=8551
      - GETH_AUTHRPC_JWTSECRET=/secrets/jwt.txt
    command: >
      --http.api=eth,net,web3,debug,txpool
      --ws.api=eth,net,web3
      --gcmode=archive
      --syncmode=full
      --rollup.sequencerhttp=http://op-node:8545

  # =============================================================
  # OP BATCHER - Submits L2 batches (uses POPSigner for signing)
  # =============================================================
  op-batcher:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:v1.9.0
    restart: unless-stopped
    environment:
      - OP_BATCHER_L1_ETH_RPC=${L1_RPC_URL}
      - OP_BATCHER_L2_ETH_RPC=http://op-geth:8545
      - OP_BATCHER_ROLLUP_RPC=http://op-node:8545
      # POPSigner integration (API key auth)
      - OP_BATCHER_SIGNER_ENDPOINT=${POPSIGNER_RPC_URL}
      - OP_BATCHER_SIGNER_ADDRESS=${BATCHER_ADDRESS}
      - OP_BATCHER_SIGNER_HEADER=X-API-Key=${POPSIGNER_API_KEY}
{{- if .UseAltDA }}
      # Alt-DA configuration
      - OP_BATCHER_ALTDA_ENABLED=true
      - OP_BATCHER_ALTDA_DA_SERVER=http://op-alt-da:3100
{{- end }}
      # Batching parameters
      - OP_BATCHER_MAX_CHANNEL_DURATION=1
      - OP_BATCHER_SUB_SAFETY_MARGIN=10
    depends_on:
      - op-node
      - op-geth
{{- if .UseAltDA }}
      - op-alt-da
{{- end }}

  # =============================================================
  # OP PROPOSER - Submits L2 state roots (uses POPSigner for signing)
  # =============================================================
  op-proposer:
    image: us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:v1.9.0
    restart: unless-stopped
    environment:
      - OP_PROPOSER_L1_ETH_RPC=${L1_RPC_URL}
      - OP_PROPOSER_ROLLUP_RPC=http://op-node:8545
      - OP_PROPOSER_L2OO_ADDRESS=${L2_OUTPUT_ORACLE_ADDRESS}
      # POPSigner integration (API key auth)
      - OP_PROPOSER_SIGNER_ENDPOINT=${POPSIGNER_RPC_URL}
      - OP_PROPOSER_SIGNER_ADDRESS=${PROPOSER_ADDRESS}
      - OP_PROPOSER_SIGNER_HEADER=X-API-Key=${POPSIGNER_API_KEY}
    depends_on:
      - op-node

{{- if .UseAltDA }}
  # =============================================================
  # OP-ALT-DA - Celestia DA sidecar
  # =============================================================
  op-alt-da:
    image: ghcr.io/celestiaorg/op-alt-da:latest
    restart: unless-stopped
    ports:
      - "3100:3100"
    environment:
      # Celestia configuration
      - CELESTIA_NODE_AUTH_TOKEN=${CELESTIA_AUTH_TOKEN}
      - CELESTIA_NODE_RPC_URL=${CELESTIA_RPC_URL}
      - CELESTIA_NAMESPACE=${CELESTIA_NAMESPACE}
      # Server config
      - SERVER_LISTEN_ADDR=0.0.0.0:3100
{{- end }}

volumes:
  op-node-data:
  op-geth-data:

networks:
  default:
    name: opstack-{{ .ChainName }}
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
	BatcherAddress  string
	ProposerAddress string

	// Contract addresses
	L2OutputOracle string

	// DA configuration
	DAType        string
	UseAltDA      bool
	CelestiaRPC   string
	CelestiaToken string
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
		BatcherAddress:    cfg.BatcherAddress,
		ProposerAddress:   cfg.ProposerAddress,
		DAType:            cfg.DAType,
		UseAltDA:          cfg.UseAltDA || cfg.DAType == "celestia" || cfg.DAType == "alt-da",
		CelestiaRPC:       cfg.CelestiaRPC,
		CelestiaToken:     cfg.CelestiaToken,
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

// GenerateMinimalCompose generates a minimal docker-compose for testing.
func GenerateMinimalCompose(cfg *DeploymentConfig) (string, error) {
	return GenerateDockerCompose(cfg, nil)
}

