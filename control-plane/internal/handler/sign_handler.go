// Package handler provides HTTP handlers for the control plane API.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// SignHandler handles batch signing HTTP requests.
type SignHandler struct {
	keyService service.KeyService
}

// NewSignHandler creates a new sign handler.
func NewSignHandler(keyService service.KeyService) *SignHandler {
	return &SignHandler{keyService: keyService}
}

// Routes returns a chi router with sign routes.
func (h *SignHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(middleware.RequireScope("keys:sign")).Post("/batch", h.SignBatch)
	return r
}

// SignBatchHTTPRequest is the HTTP request body for batch signing.
type SignBatchHTTPRequest struct {
	Requests []SignBatchItemHTTPRequest `json:"requests"`
}

// SignBatchItemHTTPRequest is a single item in a batch sign request.
type SignBatchItemHTTPRequest struct {
	KeyID     string `json:"key_id"`
	Data      string `json:"data"`      // base64 encoded
	Prehashed bool   `json:"prehashed"` // true if data is already hashed
}

// SignBatch handles POST /v1/sign/batch
func (h *SignHandler) SignBatch(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var req SignBatchHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	// Validate request
	if len(req.Requests) == 0 {
		response.Error(w, apierrors.NewValidationError("requests", "at least one request is required"))
		return
	}

	if len(req.Requests) > 100 {
		response.Error(w, apierrors.NewValidationError("requests", "maximum 100 requests per batch"))
		return
	}

	// Convert to service request
	signRequests := make([]service.SignKeyRequest, len(req.Requests))
	for i, item := range req.Requests {
		if item.KeyID == "" {
			response.Error(w, apierrors.NewValidationError("requests", "key_id is required for each request"))
			return
		}

		keyID, err := uuid.Parse(item.KeyID)
		if err != nil {
			response.Error(w, apierrors.NewValidationError("requests", "invalid key_id format"))
			return
		}

		if item.Data == "" {
			response.Error(w, apierrors.NewValidationError("requests", "data is required for each request"))
			return
		}

		signRequests[i] = service.SignKeyRequest{
			KeyID:     keyID,
			Data:      item.Data,
			Prehashed: item.Prehashed,
		}
	}

	results, err := h.keyService.SignBatch(r.Context(), service.SignBatchKeyRequest{
		OrgID:    orgID,
		Requests: signRequests,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]any{"signatures": results, "count": len(results)})
}

