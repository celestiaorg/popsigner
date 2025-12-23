package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/openbao"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
)

// DefaultAPIKeyManager implements APIKeyManager using the existing APIKeyService.
// It creates ONE deployment API key per organization, stored in OpenBao for reuse.
type DefaultAPIKeyManager struct {
	apiKeySvc service.APIKeyService
	baoClient *openbao.Client
	logger    *slog.Logger
}

// NewDefaultAPIKeyManager creates a new DefaultAPIKeyManager.
func NewDefaultAPIKeyManager(apiKeySvc service.APIKeyService, baoClient *openbao.Client, logger *slog.Logger) *DefaultAPIKeyManager {
	return &DefaultAPIKeyManager{
		apiKeySvc: apiKeySvc,
		baoClient: baoClient,
		logger:    logger,
	}
}

// baoKeyPath returns the OpenBao KV path for storing the org's deployment API key.
func (m *DefaultAPIKeyManager) baoKeyPath(orgID uuid.UUID) string {
	return fmt.Sprintf("orgs/%s/deployment-api-key", orgID.String())
}

// GetOrCreateForDeployment returns the org's deployment API key.
// Creates one if it doesn't exist, otherwise returns the existing one.
// Keys are stored in OpenBao KV for secure retrieval across deployments.
func (m *DefaultAPIKeyManager) GetOrCreateForDeployment(ctx context.Context, orgID uuid.UUID) (string, error) {
	// 1. Try to retrieve existing key from OpenBao
	existingKey, err := m.getStoredKey(ctx, orgID)
	if err == nil && existingKey != "" {
		// Verify the key is still valid (not revoked/expired)
		if _, err := m.apiKeySvc.Validate(ctx, existingKey); err == nil {
			if m.logger != nil {
				m.logger.Debug("reusing existing deployment API key",
					slog.String("org_id", orgID.String()),
				)
			}
			return existingKey, nil
		}
		// Key is invalid (revoked/expired), delete from OpenBao and create new one
		if m.logger != nil {
			m.logger.Info("existing deployment API key is invalid, creating new one",
				slog.String("org_id", orgID.String()),
			)
		}
		_ = m.deleteStoredKey(ctx, orgID)
	}

	// 2. Create a new deployment API key
	req := service.CreateAPIKeyRequest{
		Name:   "Deployment Orchestrator",
		Scopes: []string{"keys:sign", "keys:read"},
	}

	apiKey, rawKey, err := m.apiKeySvc.Create(ctx, orgID, req)
	if err != nil {
		return "", fmt.Errorf("create API key: %w", err)
	}

	_ = apiKey // We don't need the key metadata, just the raw key

	// 3. Store in OpenBao for future deployments
	if err := m.storeKey(ctx, orgID, rawKey); err != nil {
		// Non-fatal: key was created, just can't reuse it next time
		if m.logger != nil {
			m.logger.Warn("failed to store API key in OpenBao, will create new key on next deployment",
				slog.String("org_id", orgID.String()),
				slog.String("error", err.Error()),
			)
		}
	} else {
		if m.logger != nil {
			m.logger.Info("created and stored deployment API key",
				slog.String("org_id", orgID.String()),
			)
		}
	}

	return rawKey, nil
}

// getStoredKey retrieves the deployment API key from OpenBao.
func (m *DefaultAPIKeyManager) getStoredKey(ctx context.Context, orgID uuid.UUID) (string, error) {
	if m.baoClient == nil {
		return "", fmt.Errorf("bao client not configured")
	}

	path := m.baoKeyPath(orgID)
	data, err := m.baoClient.ReadKVSecret(path)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", fmt.Errorf("no secret found")
	}

	apiKey, ok := data["api_key"].(string)
	if !ok || apiKey == "" {
		return "", fmt.Errorf("api_key not found in secret")
	}

	return apiKey, nil
}

// storeKey stores the deployment API key in OpenBao.
func (m *DefaultAPIKeyManager) storeKey(ctx context.Context, orgID uuid.UUID, apiKey string) error {
	if m.baoClient == nil {
		return fmt.Errorf("bao client not configured")
	}

	path := m.baoKeyPath(orgID)
	data := map[string]interface{}{
		"api_key": apiKey,
	}

	return m.baoClient.WriteKVSecret(path, data)
}

// deleteStoredKey removes the deployment API key from OpenBao.
func (m *DefaultAPIKeyManager) deleteStoredKey(ctx context.Context, orgID uuid.UUID) error {
	if m.baoClient == nil {
		return fmt.Errorf("bao client not configured")
	}

	path := m.baoKeyPath(orgID)
	return m.baoClient.DeleteKVSecret(path)
}
