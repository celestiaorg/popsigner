// Package handler provides HTTP handlers for the control plane API.
package handler

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// KeyHandler handles key-related HTTP requests.
type KeyHandler struct {
	keyService service.KeyService
	validate   *validator.Validate
}

// NewKeyHandler creates a new key handler.
func NewKeyHandler(keyService service.KeyService) *KeyHandler {
	return &KeyHandler{
		keyService: keyService,
		validate:   validator.New(),
	}
}

// Routes returns a chi router with key routes.
func (h *KeyHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Key CRUD operations
	r.With(middleware.RequireScope("keys:read")).Get("/", h.List)
	r.With(middleware.RequireScope("keys:write")).Post("/", h.Create)
	r.With(middleware.RequireScope("keys:write")).Post("/batch", h.CreateBatch)
	r.With(middleware.RequireScope("keys:read")).Get("/{id}", h.Get)
	r.With(middleware.RequireScope("keys:write")).Delete("/{id}", h.Delete)

	// Signing operations
	r.With(middleware.RequireScope("keys:sign")).Post("/{id}/sign", h.Sign)

	// Import/Export operations
	r.With(middleware.RequireScope("keys:write")).Post("/import", h.Import)
	r.With(middleware.RequireScope("keys:export")).Post("/{id}/export", h.Export)

	return r
}

// CreateKeyHTTPRequest is the HTTP request body for creating a key.
type CreateKeyHTTPRequest struct {
	NamespaceID string            `json:"namespace_id"`
	Name        string            `json:"name"`
	Algorithm   string            `json:"algorithm,omitempty"`
	Exportable  bool              `json:"exportable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Create handles POST /v1/keys
func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var req CreateKeyHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.Error(w, apierrors.NewValidationError("name", "name is required"))
		return
	}

	if req.NamespaceID == "" {
		response.Error(w, apierrors.NewValidationError("namespace_id", "namespace_id is required"))
		return
	}

	namespaceID, err := uuid.Parse(req.NamespaceID)
	if err != nil {
		response.Error(w, apierrors.NewValidationError("namespace_id", "invalid UUID format"))
		return
	}

	key, err := h.keyService.Create(r.Context(), service.CreateKeyRequest{
		OrgID:       orgID,
		NamespaceID: namespaceID,
		Name:        req.Name,
		Algorithm:   req.Algorithm,
		Exportable:  req.Exportable,
		Metadata:    req.Metadata,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, toKeyResponse(key))
}

// CreateBatchHTTPRequest is the HTTP request body for batch key creation.
type CreateBatchHTTPRequest struct {
	NamespaceID string `json:"namespace_id"`
	Prefix      string `json:"prefix"`
	Count       int    `json:"count"`
	Exportable  bool   `json:"exportable"`
}

// CreateBatch handles POST /v1/keys/batch
func (h *KeyHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var req CreateBatchHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	// Validate required fields
	if req.Prefix == "" {
		response.Error(w, apierrors.NewValidationError("prefix", "prefix is required"))
		return
	}

	if req.NamespaceID == "" {
		response.Error(w, apierrors.NewValidationError("namespace_id", "namespace_id is required"))
		return
	}

	namespaceID, err := uuid.Parse(req.NamespaceID)
	if err != nil {
		response.Error(w, apierrors.NewValidationError("namespace_id", "invalid UUID format"))
		return
	}

	if req.Count < 1 || req.Count > 100 {
		response.Error(w, apierrors.NewValidationError("count", "count must be between 1 and 100"))
		return
	}

	keys, err := h.keyService.CreateBatch(r.Context(), service.CreateBatchKeyRequest{
		OrgID:       orgID,
		NamespaceID: namespaceID,
		Prefix:      req.Prefix,
		Count:       req.Count,
		Exportable:  req.Exportable,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	// Convert to response format
	keyResponses := make([]*KeyResponse, len(keys))
	for i, key := range keys {
		keyResponses[i] = toKeyResponse(key)
	}

	response.Created(w, map[string]any{"keys": keyResponses, "count": len(keyResponses)})
}

// List handles GET /v1/keys
func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var nsID *uuid.UUID
	if nsStr := r.URL.Query().Get("namespace_id"); nsStr != "" {
		id, err := uuid.Parse(nsStr)
		if err == nil {
			nsID = &id
		}
	}

	keys, err := h.keyService.List(r.Context(), orgID, nsID)
	if err != nil {
		response.Error(w, err)
		return
	}

	// Convert to response format
	keyResponses := make([]*KeyResponse, len(keys))
	for i, key := range keys {
		keyResponses[i] = toKeyResponse(key)
	}

	response.OK(w, keyResponses)
}

// Get handles GET /v1/keys/{id}
func (h *KeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid key ID"))
		return
	}

	key, err := h.keyService.Get(r.Context(), orgID, keyID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, toKeyResponse(key))
}

// Delete handles DELETE /v1/keys/{id}
func (h *KeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid key ID"))
		return
	}

	if err := h.keyService.Delete(r.Context(), orgID, keyID); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

// SignHTTPRequest is the HTTP request body for signing.
type SignHTTPRequest struct {
	Data      string `json:"data"`      // base64 encoded
	Prehashed bool   `json:"prehashed"` // true if data is already hashed
}

// Sign handles POST /v1/keys/{id}/sign
func (h *KeyHandler) Sign(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid key ID"))
		return
	}

	var req SignHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	if req.Data == "" {
		response.Error(w, apierrors.NewValidationError("data", "data is required"))
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		response.Error(w, apierrors.NewValidationError("data", "invalid base64 encoding"))
		return
	}

	result, err := h.keyService.Sign(r.Context(), orgID, keyID, data, req.Prehashed)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, result)
}

// ImportHTTPRequest is the HTTP request body for importing a key.
type ImportHTTPRequest struct {
	NamespaceID string `json:"namespace_id"`
	Name        string `json:"name"`
	PrivateKey  string `json:"private_key"` // base64 encoded
	Exportable  bool   `json:"exportable"`
}

// Import handles POST /v1/keys/import
func (h *KeyHandler) Import(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var req ImportHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.Error(w, apierrors.NewValidationError("name", "name is required"))
		return
	}

	if req.NamespaceID == "" {
		response.Error(w, apierrors.NewValidationError("namespace_id", "namespace_id is required"))
		return
	}

	namespaceID, err := uuid.Parse(req.NamespaceID)
	if err != nil {
		response.Error(w, apierrors.NewValidationError("namespace_id", "invalid UUID format"))
		return
	}

	if req.PrivateKey == "" {
		response.Error(w, apierrors.NewValidationError("private_key", "private_key is required"))
		return
	}

	// Validate base64
	if _, err := base64.StdEncoding.DecodeString(req.PrivateKey); err != nil {
		response.Error(w, apierrors.NewValidationError("private_key", "invalid base64 encoding"))
		return
	}

	key, err := h.keyService.Import(r.Context(), service.ImportKeyRequest{
		OrgID:       orgID,
		NamespaceID: namespaceID,
		Name:        req.Name,
		PrivateKey:  req.PrivateKey,
		Exportable:  req.Exportable,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, toKeyResponse(key))
}

// Export handles POST /v1/keys/{id}/export
func (h *KeyHandler) Export(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid key ID"))
		return
	}

	privateKey, err := h.keyService.Export(r.Context(), orgID, keyID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]string{
		"private_key": privateKey,
		"warning":     "This private key provides full control over the associated account. Store it securely.",
	})
}

// KeyResponse is the API response format for keys.
type KeyResponse struct {
	ID          uuid.UUID              `json:"id"`
	NamespaceID uuid.UUID              `json:"namespace_id"`
	Name        string                 `json:"name"`
	PublicKey   string                 `json:"public_key"` // Hex encoded
	Address     string                 `json:"address"`
	Algorithm   models.Algorithm       `json:"algorithm"`
	Exportable  bool                   `json:"exportable"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Version     int                    `json:"version"`
	CreatedAt   string                 `json:"created_at"`
}

// toKeyResponse converts a Key model to a KeyResponse.
func toKeyResponse(key *models.Key) *KeyResponse {
	var metadata map[string]interface{}
	if len(key.Metadata) > 0 {
		_ = json.Unmarshal(key.Metadata, &metadata)
	}

	return &KeyResponse{
		ID:          key.ID,
		NamespaceID: key.NamespaceID,
		Name:        key.Name,
		PublicKey:   hex.EncodeToString(key.PublicKey),
		Address:     key.Address,
		Algorithm:   key.Algorithm,
		Exportable:  key.Exportable,
		Metadata:    metadata,
		Version:     key.Version,
		CreatedAt:   key.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

