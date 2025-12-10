// Package service provides business logic implementations.
package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/repository"
)

// BaoKeyringInterface defines the interface for OpenBao keyring operations.
// This allows the control plane to work with the core BaoKeyring library.
type BaoKeyringInterface interface {
	// NewAccountWithOptions creates a new key in OpenBao with the given options.
	// Returns the public key bytes and address.
	NewAccountWithOptions(uid string, opts KeyOptions) (pubKey []byte, address string, err error)

	// Sign signs a message with the given key.
	// Returns the signature and public key.
	Sign(uid string, msg []byte) (signature []byte, pubKey []byte, err error)

	// Delete removes a key from OpenBao.
	Delete(uid string) error

	// GetMetadata returns the metadata for a key.
	GetMetadata(uid string) (*KeyMetadata, error)

	// ImportKey imports a key into OpenBao.
	// Returns the public key bytes and address.
	ImportKey(uid string, ciphertext string, exportable bool) (pubKey []byte, address string, err error)

	// ExportKey exports a key from OpenBao.
	ExportKey(uid string) (string, error)
}

// KeyOptions configures key creation.
type KeyOptions struct {
	Exportable bool
}

// KeyMetadata contains locally stored key information.
type KeyMetadata struct {
	UID         string
	Name        string
	PubKeyBytes []byte
	Address     string
}

// KeyService defines the interface for key management operations.
type KeyService interface {
	// Single key operations
	Create(ctx context.Context, req CreateKeyRequest) (*models.Key, error)
	Get(ctx context.Context, orgID, keyID uuid.UUID) (*models.Key, error)
	List(ctx context.Context, orgID uuid.UUID, namespaceID *uuid.UUID) ([]*models.Key, error)
	Delete(ctx context.Context, orgID, keyID uuid.UUID) error
	Sign(ctx context.Context, orgID, keyID uuid.UUID, data []byte, prehashed bool) (*SignKeyResponse, error)

	// Batch operations for parallel workers
	CreateBatch(ctx context.Context, req CreateBatchKeyRequest) ([]*models.Key, error)
	SignBatch(ctx context.Context, req SignBatchKeyRequest) ([]*SignKeyResponse, error)

	// Import/Export
	Import(ctx context.Context, req ImportKeyRequest) (*models.Key, error)
	Export(ctx context.Context, orgID, keyID uuid.UUID) (string, error)
}

// CreateKeyRequest is the request for creating a new key.
type CreateKeyRequest struct {
	OrgID       uuid.UUID         `json:"-"`
	NamespaceID uuid.UUID         `json:"namespace_id" validate:"required"`
	Name        string            `json:"name" validate:"required,min=1,max=100"`
	Algorithm   string            `json:"algorithm" validate:"omitempty,oneof=secp256k1"`
	Exportable  bool              `json:"exportable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CreateBatchKeyRequest is the request for batch key creation.
type CreateBatchKeyRequest struct {
	OrgID       uuid.UUID `json:"-"`
	NamespaceID uuid.UUID `json:"namespace_id" validate:"required"`
	Prefix      string    `json:"prefix" validate:"required,min=1,max=50"`
	Count       int       `json:"count" validate:"required,min=1,max=100"`
	Exportable  bool      `json:"exportable"`
}

// SignKeyRequest is a single signing request.
type SignKeyRequest struct {
	KeyID     uuid.UUID `json:"key_id" validate:"required"`
	Data      string    `json:"data" validate:"required"` // base64
	Prehashed bool      `json:"prehashed"`
}

// SignBatchKeyRequest is the request for batch signing.
type SignBatchKeyRequest struct {
	OrgID    uuid.UUID        `json:"-"`
	Requests []SignKeyRequest `json:"requests" validate:"required,min=1,max=100"`
}

// SignKeyResponse is the response from a signing operation.
type SignKeyResponse struct {
	KeyID     uuid.UUID `json:"key_id"`
	Signature string    `json:"signature"` // base64
	PublicKey string    `json:"public_key"` // hex
	Error     string    `json:"error,omitempty"`
}

// ImportKeyRequest is the request for importing a key.
type ImportKeyRequest struct {
	OrgID       uuid.UUID `json:"-"`
	NamespaceID uuid.UUID `json:"namespace_id" validate:"required"`
	Name        string    `json:"name" validate:"required"`
	PrivateKey  string    `json:"private_key" validate:"required"` // base64
	Exportable  bool      `json:"exportable"`
}

type keyService struct {
	keyRepo    repository.KeyRepository
	orgRepo    repository.OrgRepository
	auditRepo  repository.AuditRepository
	usageRepo  repository.UsageRepository
	baoKeyring BaoKeyringInterface
}

// NewKeyService creates a new key service.
func NewKeyService(
	keyRepo repository.KeyRepository,
	orgRepo repository.OrgRepository,
	auditRepo repository.AuditRepository,
	usageRepo repository.UsageRepository,
	baoKeyring BaoKeyringInterface,
) KeyService {
	return &keyService{
		keyRepo:    keyRepo,
		orgRepo:    orgRepo,
		auditRepo:  auditRepo,
		usageRepo:  usageRepo,
		baoKeyring: baoKeyring,
	}
}

// Create creates a new key.
func (s *keyService) Create(ctx context.Context, req CreateKeyRequest) (*models.Key, error) {
	// Check quota
	if err := s.checkKeyQuota(ctx, req.OrgID); err != nil {
		return nil, err
	}

	// Verify namespace belongs to org
	ns, err := s.orgRepo.GetNamespace(ctx, req.NamespaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}
	if ns == nil || ns.OrgID != req.OrgID {
		return nil, apierrors.NewNotFoundError("Namespace")
	}

	// Generate unique OpenBao key name
	baoKeyName := fmt.Sprintf("%s_%s_%s", req.OrgID, req.NamespaceID, req.Name)

	// Create in BaoKeyring
	pubKey, address, err := s.baoKeyring.NewAccountWithOptions(baoKeyName, KeyOptions{
		Exportable: req.Exportable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create key in OpenBao: %w", err)
	}

	// Convert metadata to JSON if provided
	var metadataJSON json.RawMessage
	if len(req.Metadata) > 0 {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			_ = s.baoKeyring.Delete(baoKeyName)
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Save metadata to database
	key := &models.Key{
		ID:          uuid.New(),
		OrgID:       req.OrgID,
		NamespaceID: req.NamespaceID,
		Name:        req.Name,
		PublicKey:   pubKey,
		Address:     address,
		Algorithm:   models.AlgorithmSecp256k1,
		BaoKeyPath:  baoKeyName,
		Exportable:  req.Exportable,
		Metadata:    metadataJSON,
	}

	if err := s.keyRepo.Create(ctx, key); err != nil {
		// Cleanup OpenBao key on failure
		_ = s.baoKeyring.Delete(baoKeyName)
		return nil, fmt.Errorf("failed to save key metadata: %w", err)
	}

	// Audit log
	s.auditLog(ctx, req.OrgID, models.AuditEventKeyCreated, models.ResourceTypeKey, key.ID)

	return key, nil
}

// CreateBatch creates multiple keys in parallel.
func (s *keyService) CreateBatch(ctx context.Context, req CreateBatchKeyRequest) ([]*models.Key, error) {
	// Check quota for all keys
	org, err := s.orgRepo.GetByID(ctx, req.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	if org == nil {
		return nil, apierrors.NewNotFoundError("Organization")
	}

	limits := models.GetPlanLimits(org.Plan)
	currentCount, err := s.keyRepo.CountByOrg(ctx, req.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to count keys: %w", err)
	}

	if limits.Keys > 0 && currentCount+req.Count > limits.Keys {
		return nil, apierrors.ErrQuotaExceeded.WithMessage(
			fmt.Sprintf("Creating %d keys would exceed your limit of %d keys", req.Count, limits.Keys),
		)
	}

	// Create keys in parallel
	keys := make([]*models.Key, req.Count)
	errs := make([]error, req.Count)
	var wg sync.WaitGroup

	for i := 0; i < req.Count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("%s-%d", req.Prefix, idx+1)
			key, err := s.Create(ctx, CreateKeyRequest{
				OrgID:       req.OrgID,
				NamespaceID: req.NamespaceID,
				Name:        name,
				Exportable:  req.Exportable,
			})
			keys[idx] = key
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// Collect results (partial success is possible)
	var result []*models.Key
	for i, key := range keys {
		if errs[i] == nil && key != nil {
			result = append(result, key)
		}
	}

	if len(result) == 0 && len(errs) > 0 {
		// All failed, return first error
		for _, err := range errs {
			if err != nil {
				return nil, fmt.Errorf("batch create failed: %w", err)
			}
		}
	}

	return result, nil
}

// Get retrieves a key by ID.
func (s *keyService) Get(ctx context.Context, orgID, keyID uuid.UUID) (*models.Key, error) {
	key, err := s.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	if key == nil || key.OrgID != orgID || key.DeletedAt != nil {
		return nil, apierrors.NewNotFoundError("Key")
	}
	return key, nil
}

// List lists keys for an organization, optionally filtered by namespace.
func (s *keyService) List(ctx context.Context, orgID uuid.UUID, namespaceID *uuid.UUID) ([]*models.Key, error) {
	if namespaceID != nil {
		// Verify namespace belongs to org
		ns, err := s.orgRepo.GetNamespace(ctx, *namespaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespace: %w", err)
		}
		if ns == nil || ns.OrgID != orgID {
			return nil, apierrors.NewNotFoundError("Namespace")
		}
		return s.keyRepo.ListByNamespace(ctx, *namespaceID)
	}
	return s.keyRepo.ListByOrg(ctx, orgID)
}

// Delete deletes a key.
func (s *keyService) Delete(ctx context.Context, orgID, keyID uuid.UUID) error {
	key, err := s.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}
	if key == nil || key.OrgID != orgID {
		return apierrors.NewNotFoundError("Key")
	}

	// Delete from OpenBao
	if err := s.baoKeyring.Delete(key.BaoKeyPath); err != nil {
		return fmt.Errorf("failed to delete from OpenBao: %w", err)
	}

	// Soft delete in database
	if err := s.keyRepo.SoftDelete(ctx, keyID); err != nil {
		return fmt.Errorf("failed to delete key metadata: %w", err)
	}

	// Audit log
	s.auditLog(ctx, orgID, models.AuditEventKeyDeleted, models.ResourceTypeKey, keyID)

	return nil
}

// Sign signs data using a key.
func (s *keyService) Sign(ctx context.Context, orgID, keyID uuid.UUID, data []byte, prehashed bool) (*SignKeyResponse, error) {
	// Get key
	key, err := s.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	if key == nil || key.OrgID != orgID || key.DeletedAt != nil {
		return nil, apierrors.NewNotFoundError("Key")
	}

	// Check signature quota
	if err := s.checkSignatureQuota(ctx, orgID); err != nil {
		return nil, err
	}

	// Sign via BaoKeyring
	sig, pubKey, err := s.baoKeyring.Sign(key.BaoKeyPath, data)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// Increment usage counter
	s.incrementUsage(ctx, orgID, "signatures", 1)

	// Audit log
	s.auditLog(ctx, orgID, models.AuditEventKeySigned, models.ResourceTypeKey, keyID)

	return &SignKeyResponse{
		KeyID:     keyID,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: hex.EncodeToString(pubKey),
	}, nil
}

// SignBatch signs multiple messages in parallel.
func (s *keyService) SignBatch(ctx context.Context, req SignBatchKeyRequest) ([]*SignKeyResponse, error) {
	// Check quota for all signatures
	if err := s.checkSignatureQuota(ctx, req.OrgID); err != nil {
		return nil, err
	}

	// Sign in parallel (no head-of-line blocking!)
	results := make([]*SignKeyResponse, len(req.Requests))
	var wg sync.WaitGroup

	for i, signReq := range req.Requests {
		wg.Add(1)
		go func(idx int, r SignKeyRequest) {
			defer wg.Done()

			data, err := base64.StdEncoding.DecodeString(r.Data)
			if err != nil {
				results[idx] = &SignKeyResponse{KeyID: r.KeyID, Error: "invalid base64"}
				return
			}

			resp, err := s.Sign(ctx, req.OrgID, r.KeyID, data, r.Prehashed)
			if err != nil {
				results[idx] = &SignKeyResponse{KeyID: r.KeyID, Error: err.Error()}
				return
			}
			results[idx] = resp
		}(i, signReq)
	}
	wg.Wait()

	return results, nil
}

// Import imports a key from a base64-encoded private key.
func (s *keyService) Import(ctx context.Context, req ImportKeyRequest) (*models.Key, error) {
	// Check quota
	if err := s.checkKeyQuota(ctx, req.OrgID); err != nil {
		return nil, err
	}

	// Verify namespace belongs to org
	ns, err := s.orgRepo.GetNamespace(ctx, req.NamespaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}
	if ns == nil || ns.OrgID != req.OrgID {
		return nil, apierrors.NewNotFoundError("Namespace")
	}

	// Generate unique OpenBao key name
	baoKeyName := fmt.Sprintf("%s_%s_%s", req.OrgID, req.NamespaceID, req.Name)

	// Import into BaoKeyring
	pubKey, address, err := s.baoKeyring.ImportKey(baoKeyName, req.PrivateKey, req.Exportable)
	if err != nil {
		return nil, fmt.Errorf("failed to import key into OpenBao: %w", err)
	}

	// Save metadata to database
	key := &models.Key{
		ID:          uuid.New(),
		OrgID:       req.OrgID,
		NamespaceID: req.NamespaceID,
		Name:        req.Name,
		PublicKey:   pubKey,
		Address:     address,
		Algorithm:   models.AlgorithmSecp256k1,
		BaoKeyPath:  baoKeyName,
		Exportable:  req.Exportable,
	}

	if err := s.keyRepo.Create(ctx, key); err != nil {
		_ = s.baoKeyring.Delete(baoKeyName)
		return nil, fmt.Errorf("failed to save key metadata: %w", err)
	}

	// Audit log
	s.auditLog(ctx, req.OrgID, models.AuditEventKeyCreated, models.ResourceTypeKey, key.ID)

	return key, nil
}

// Export exports a key if it's exportable.
func (s *keyService) Export(ctx context.Context, orgID, keyID uuid.UUID) (string, error) {
	key, err := s.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return "", fmt.Errorf("failed to get key: %w", err)
	}
	if key == nil || key.OrgID != orgID || key.DeletedAt != nil {
		return "", apierrors.NewNotFoundError("Key")
	}

	if !key.Exportable {
		return "", apierrors.ErrForbidden.WithMessage("Key is not exportable")
	}

	// Export from BaoKeyring
	privateKey, err := s.baoKeyring.ExportKey(key.BaoKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to export key: %w", err)
	}

	// Audit log
	s.auditLog(ctx, orgID, models.AuditEventKeyExported, models.ResourceTypeKey, keyID)

	return privateKey, nil
}

// --- Helper methods ---

func (s *keyService) checkKeyQuota(ctx context.Context, orgID uuid.UUID) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}
	if org == nil {
		return apierrors.NewNotFoundError("Organization")
	}

	limits := models.GetPlanLimits(org.Plan)
	if limits.Keys < 0 { // unlimited
		return nil
	}

	count, err := s.keyRepo.CountByOrg(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to count keys: %w", err)
	}

	if count >= limits.Keys {
		return apierrors.ErrQuotaExceeded.WithMessage(
			fmt.Sprintf("You've reached your limit of %d keys", limits.Keys),
		)
	}
	return nil
}

func (s *keyService) checkSignatureQuota(ctx context.Context, orgID uuid.UUID) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}
	if org == nil {
		return apierrors.NewNotFoundError("Organization")
	}

	limits := models.GetPlanLimits(org.Plan)
	if limits.SignaturesPerMonth < 0 { // unlimited
		return nil
	}

	usage, err := s.usageRepo.GetCurrentPeriod(ctx, orgID, "signatures")
	if err != nil {
		return fmt.Errorf("failed to get usage: %w", err)
	}

	if usage >= limits.SignaturesPerMonth {
		return apierrors.ErrQuotaExceeded.WithMessage(
			fmt.Sprintf("You've reached your limit of %d signatures per month", limits.SignaturesPerMonth),
		)
	}
	return nil
}

func (s *keyService) incrementUsage(ctx context.Context, orgID uuid.UUID, metric string, value int64) {
	// Run asynchronously to not block the request
	go func() {
		_ = s.usageRepo.Increment(context.Background(), orgID, metric, value)
	}()
}

func (s *keyService) auditLog(ctx context.Context, orgID uuid.UUID, event models.AuditEvent, resourceType models.ResourceType, resourceID uuid.UUID) {
	// Run asynchronously to not block the request
	go func() {
		_ = s.auditRepo.Create(context.Background(), &models.AuditLog{
			OrgID:        orgID,
			Event:        event,
			ActorType:    models.ActorTypeAPIKey, // Default to API key, can be overridden
			ResourceType: &resourceType,
			ResourceID:   &resourceID,
		})
	}()
}

// Compile-time check to ensure keyService implements KeyService.
var _ KeyService = (*keyService)(nil)
