// Package compose provides a unified Docker Compose generator for both OP Stack and Nitro rollup deployments.
package compose

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templates embed.FS

// Stack represents the rollup stack type.
type Stack string

const (
	// StackOPStack represents the OP Stack rollup framework.
	StackOPStack Stack = "opstack"
	// StackNitro represents the Arbitrum Nitro rollup framework.
	StackNitro Stack = "nitro"
)

// Generator creates docker-compose files for different rollup stacks.
type Generator struct {
	funcMap template.FuncMap
}

// NewGenerator creates a new Docker Compose generator.
func NewGenerator() *Generator {
	return &Generator{
		funcMap: template.FuncMap{
			"lower": strings.ToLower,
			"sanitize": func(s string) string {
				return sanitizeName(s)
			},
		},
	}
}

// ComposeConfig contains all values needed to generate docker-compose files.
type ComposeConfig struct {
	// ========================================
	// COMMON FIELDS (both stacks)
	// ========================================

	// ChainID is the L2/Orbit chain ID.
	ChainID uint64
	// ChainName is the human-readable chain name for network naming.
	ChainName string
	// L1RPC is the parent chain RPC URL (templated as env var in output).
	L1RPC string
	// DAType specifies the data availability layer: "celestia", "anytrust", or empty for L1 DA.
	DAType string
	// CelestiaRPC is the Celestia node RPC URL.
	CelestiaRPC string

	// ========================================
	// OP STACK SPECIFIC
	// ========================================

	// POPSignerRPCEndpoint is the POPSigner RPC URL for API key auth.
	// Example: "https://rpc.popsigner.com"
	POPSignerRPCEndpoint string
	// BatcherAddress is the Ethereum address for the batcher role.
	BatcherAddress string
	// ProposerAddress is the Ethereum address for the proposer role.
	ProposerAddress string
	// APIKey is the POPSigner API key (templated as env var in output).
	APIKey string
	// Contracts contains deployed contract addresses.
	// Keys: "l2_output_oracle", "dispute_game_factory", etc.
	Contracts map[string]string

	// ========================================
	// NITRO SPECIFIC
	// ========================================

	// POPSignerMTLSEndpoint is the POPSigner mTLS URL for certificate auth.
	// Example: "https://rpc-mtls.popsigner.com"
	POPSignerMTLSEndpoint string
	// BatchPosterAddress is the Ethereum address for batch posting.
	BatchPosterAddress string
	// ValidatorAddress is the Ethereum address for the validator/staker role.
	ValidatorAddress string
	// RollupAddress is the deployed Rollup contract address.
	RollupAddress string
	// InboxAddress is the deployed Inbox contract address.
	InboxAddress string
	// SequencerInboxAddr is the deployed SequencerInbox contract address.
	SequencerInboxAddr string
}

// GenerateResult contains both compose and env file contents.
type GenerateResult struct {
	// ComposeYAML is the docker-compose.yml content.
	ComposeYAML string
	// EnvExample is the .env.example content with placeholder values.
	EnvExample string
}

// Generate creates docker-compose.yml and .env.example for the specified stack.
func (g *Generator) Generate(stack Stack, cfg *ComposeConfig) (*GenerateResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if cfg.ChainName == "" {
		cfg.ChainName = "rollup"
	}

	composeResult, err := g.generateFile(stack, "yml", cfg)
	if err != nil {
		return nil, fmt.Errorf("generate compose: %w", err)
	}

	envResult, err := g.generateFile(stack, "env", cfg)
	if err != nil {
		return nil, fmt.Errorf("generate env: %w", err)
	}

	return &GenerateResult{
		ComposeYAML: composeResult,
		EnvExample:  envResult,
	}, nil
}

// generateFile generates a single file from the appropriate template.
func (g *Generator) generateFile(stack Stack, fileType string, cfg *ComposeConfig) (string, error) {
	tmplName := fmt.Sprintf("%s.%s.tmpl", stack, fileType)

	tmplData, err := templates.ReadFile("templates/" + tmplName)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", tmplName, err)
	}

	tmpl, err := template.New(tmplName).Funcs(g.funcMap).Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", tmplName, err)
	}

	// Build template data with computed fields
	data := g.buildTemplateData(stack, cfg)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", tmplName, err)
	}

	return buf.String(), nil
}

// templateData holds the computed values for template rendering.
type templateData struct {
	*ComposeConfig

	// Computed fields
	SanitizedChainName string
	UseAltDA           bool
	UseCelestia        bool
}

// buildTemplateData creates the template data with computed fields.
func (g *Generator) buildTemplateData(stack Stack, cfg *ComposeConfig) *templateData {
	data := &templateData{
		ComposeConfig:      cfg,
		SanitizedChainName: sanitizeName(cfg.ChainName),
		UseAltDA:           cfg.DAType == "celestia" || cfg.DAType == "alt-da" || cfg.DAType == "anytrust",
		UseCelestia:        cfg.DAType == "celestia",
	}

	// Ensure contracts map is initialized
	if data.Contracts == nil {
		data.Contracts = make(map[string]string)
	}

	return data
}

// GetL2OutputOracle safely retrieves the L2 output oracle address from contracts.
func (t *templateData) GetL2OutputOracle() string {
	if t.Contracts == nil {
		return ""
	}
	return t.Contracts["l2_output_oracle"]
}

// sanitizeName converts a chain name to a valid Docker network/service name.
// Only allows alphanumeric characters, hyphens, and underscores.
func sanitizeName(name string) string {
	if name == "" {
		return "rollup"
	}

	var result strings.Builder
	result.Grow(len(name))
	hasAlphaNum := false

	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			result.WriteRune(r)
			hasAlphaNum = true
		case r >= '0' && r <= '9':
			result.WriteRune(r)
			hasAlphaNum = true
		case r == '-' || r == '_':
			result.WriteRune(r)
		case r == ' ':
			result.WriteRune('-')
		}
	}

	// If no alphanumeric characters were found, return default
	if !hasAlphaNum {
		return "rollup"
	}

	return result.String()
}

// GenerateOPStack is a convenience function to generate OP Stack compose files.
func GenerateOPStack(cfg *ComposeConfig) (*GenerateResult, error) {
	return NewGenerator().Generate(StackOPStack, cfg)
}

// GenerateNitro is a convenience function to generate Nitro compose files.
func GenerateNitro(cfg *ComposeConfig) (*GenerateResult, error) {
	return NewGenerator().Generate(StackNitro, cfg)
}

