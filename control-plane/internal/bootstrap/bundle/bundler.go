package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/compose"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// Bundler creates downloadable artifact bundles for rollup deployments.
type Bundler struct {
	repo     repository.Repository
	composer *compose.Generator
}

// NewBundler creates a new artifact bundler.
func NewBundler(repo repository.Repository) *Bundler {
	return &Bundler{
		repo:     repo,
		composer: compose.NewGenerator(),
	}
}

// CreateBundle generates a complete artifact bundle for the given deployment.
func (b *Bundler) CreateBundle(ctx context.Context, deploymentID uuid.UUID) (*BundleResult, error) {
	// 1. Load deployment
	deployment, err := b.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("load deployment: %w", err)
	}

	// 2. Load all artifacts from DB
	artifacts, err := b.repo.GetAllArtifacts(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("load artifacts: %w", err)
	}

	// 3. Build bundle config
	cfg := b.buildBundleConfig(deployment, artifacts)

	// 4. Generate docker-compose and env files using compose generator
	composeResult, err := b.composer.Generate(compose.Stack(cfg.Stack), &compose.ComposeConfig{
		ChainID:               cfg.ChainID,
		ChainName:             cfg.ChainName,
		DAType:                cfg.DAType,
		POPSignerRPCEndpoint:  cfg.POPSignerEndpoint,
		POPSignerMTLSEndpoint: cfg.POPSignerMTLSEndpoint,
		BatcherAddress:        cfg.BatcherAddress,
		ProposerAddress:       cfg.ProposerAddress,
		ValidatorAddress:      cfg.ValidatorAddress,
		Contracts:             cfg.Contracts,
	})
	if err != nil {
		return nil, fmt.Errorf("generate compose: %w", err)
	}
	cfg.DockerCompose = composeResult.ComposeYAML
	cfg.EnvExample = composeResult.EnvExample

	// 5. Create the bundle based on stack type
	return b.createBundle(cfg)
}

// CreateBundleFromConfig generates a bundle from a pre-built config.
// This is useful for testing or when you already have all the data.
func (b *Bundler) CreateBundleFromConfig(cfg *BundleConfig) (*BundleResult, error) {
	// Generate compose files if not provided
	if cfg.DockerCompose == "" || cfg.EnvExample == "" {
		composeResult, err := b.composer.Generate(compose.Stack(cfg.Stack), &compose.ComposeConfig{
			ChainID:               cfg.ChainID,
			ChainName:             cfg.ChainName,
			DAType:                cfg.DAType,
			POPSignerRPCEndpoint:  cfg.POPSignerEndpoint,
			POPSignerMTLSEndpoint: cfg.POPSignerMTLSEndpoint,
			BatcherAddress:        cfg.BatcherAddress,
			ProposerAddress:       cfg.ProposerAddress,
			ValidatorAddress:      cfg.ValidatorAddress,
			Contracts:             cfg.Contracts,
		})
		if err != nil {
			return nil, fmt.Errorf("generate compose: %w", err)
		}
		cfg.DockerCompose = composeResult.ComposeYAML
		cfg.EnvExample = composeResult.EnvExample
	}

	return b.createBundle(cfg)
}

// createBundle creates the appropriate bundle based on stack type.
func (b *Bundler) createBundle(cfg *BundleConfig) (*BundleResult, error) {
	switch cfg.Stack {
	case StackOPStack:
		return b.createOPStackBundle(cfg)
	case StackNitro:
		return b.createNitroBundle(cfg)
	case StackPopBundle:
		return b.createPopBundleBundle(cfg)
	default:
		return nil, fmt.Errorf("unsupported stack: %s", cfg.Stack)
	}
}

// buildBundleConfig constructs a BundleConfig from deployment and artifacts.
func (b *Bundler) buildBundleConfig(deployment *repository.Deployment, artifacts []repository.Artifact) *BundleConfig {
	cfg := &BundleConfig{
		Stack:     Stack(deployment.Stack),
		ChainID:   uint64(deployment.ChainID),
		ChainName: fmt.Sprintf("chain-%d", deployment.ChainID),
		Artifacts: make(map[string][]byte),
		Contracts: make(map[string]string),
	}

	// Try to extract chain name from config if available
	if deployment.Config != nil {
		var config map[string]interface{}
		if err := json.Unmarshal(deployment.Config, &config); err == nil {
			if name, ok := config["chain_name"].(string); ok && name != "" {
				cfg.ChainName = name
			}
			if daType, ok := config["da_type"].(string); ok {
				cfg.DAType = daType
			}
		}
	}

	// Copy artifacts to config
	for _, a := range artifacts {
		cfg.Artifacts[a.ArtifactType] = a.Content
	}

	// Extract ready-to-use .env file if present (from env_file artifact)
	if envFileData, ok := cfg.Artifacts["env_file"]; ok {
		cfg.EnvFile = string(envFileData)
	}

	// Set default POPSigner endpoints
	switch cfg.Stack {
	case StackOPStack:
		cfg.POPSignerEndpoint = "https://rpc.popsigner.com"
	case StackNitro:
		cfg.POPSignerMTLSEndpoint = "https://rpc-mtls.popsigner.com:8546"
	}

	return cfg
}

// ============================================================================
// Tar Helper Functions
// ============================================================================

// tarWriter wraps tar writing with checksum tracking.
type tarWriter struct {
	tw        *tar.Writer
	checksums map[string]string
}

// newTarWriter creates a new tar writer with checksum tracking.
func newTarWriter(tw *tar.Writer) *tarWriter {
	return &tarWriter{
		tw:        tw,
		checksums: make(map[string]string),
	}
}

// addFile adds a file to the tar archive with default permissions.
func (tw *tarWriter) addFile(name string, content []byte) error {
	return tw.addFileWithMode(name, content, 0644)
}

// addFileWithMode adds a file to the tar archive with specific permissions.
func (tw *tarWriter) addFileWithMode(name string, content []byte, mode int64) error {
	// Calculate checksum
	hash := sha256.Sum256(content)
	tw.checksums[name] = hex.EncodeToString(hash[:])

	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(content)),
		Mode:    mode,
		ModTime: time.Now(),
	}
	if err := tw.tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header for %s: %w", name, err)
	}
	if _, err := tw.tw.Write(content); err != nil {
		return fmt.Errorf("write content for %s: %w", name, err)
	}
	return nil
}

// addExecutable adds an executable script to the tar archive.
func (tw *tarWriter) addExecutable(name string, content []byte) error {
	return tw.addFileWithMode(name, content, 0755)
}

// ============================================================================
// Helper Functions
// ============================================================================

// generateJWT creates a random 32-byte hex JWT for op-node/op-geth auth.
func generateJWT() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a predictable but unique value
		return hex.EncodeToString([]byte(time.Now().String())[:32])
	}
	return hex.EncodeToString(b)
}

// sanitizeName converts a chain name to a valid directory name.
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

	if !hasAlphaNum {
		return "rollup"
	}

	return result.String()
}

// calculateBundleChecksum calculates the SHA256 checksum of the bundle data.
func calculateBundleChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// finalizeTarGz closes the tar and gzip writers and returns the buffer content.
func finalizeTarGz(tw *tar.Writer, gw *gzip.Writer, buf *bytes.Buffer) ([]byte, error) {
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}
	return buf.Bytes(), nil
}

