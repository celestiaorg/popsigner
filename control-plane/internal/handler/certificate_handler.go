// Package handler provides HTTP handlers for the control plane API.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	apierrors "github.com/Bidon15/popsigner/control-plane/internal/pkg/errors"
	"github.com/Bidon15/popsigner/control-plane/internal/pkg/response"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
)

// CertificateHandler handles certificate-related HTTP requests.
type CertificateHandler struct {
	certService service.CertificateService
}

// NewCertificateHandler creates a new certificate handler.
func NewCertificateHandler(certService service.CertificateService) *CertificateHandler {
	return &CertificateHandler{
		certService: certService,
	}
}

// Routes returns a chi router with certificate routes.
func (h *CertificateHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Certificate CRUD operations
	r.With(middleware.RequireScope("certificates:read")).Get("/", h.List)
	r.With(middleware.RequireScope("certificates:write")).Post("/", h.Create)
	r.Get("/ca", h.GetCA) // CA download doesn't require specific scope
	r.With(middleware.RequireScope("certificates:read")).Get("/{id}", h.Get)
	r.With(middleware.RequireScope("certificates:write")).Post("/{id}/revoke", h.Revoke)
	r.With(middleware.RequireScope("certificates:write")).Delete("/{id}", h.Delete)

	return r
}

// CreateCertificateHTTPRequest is the HTTP request body for creating a certificate.
type CreateCertificateHTTPRequest struct {
	Name           string `json:"name"`
	ValidityPeriod string `json:"validity_period,omitempty"` // e.g., "8760h" for 1 year
}

// Create handles POST /v1/certificates
func (h *CertificateHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	var req CreateCertificateHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid request body"))
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.Error(w, apierrors.NewValidationError("name", "name is required"))
		return
	}

	// Parse validity period
	validityPeriod := models.DefaultValidityPeriod
	if req.ValidityPeriod != "" {
		d, err := time.ParseDuration(req.ValidityPeriod)
		if err != nil {
			response.Error(w, apierrors.NewValidationError("validity_period", "invalid duration format (e.g., '8760h')"))
			return
		}
		validityPeriod = d
	}

	createReq := &models.CreateCertificateRequest{
		OrgID:          orgID,
		Name:           req.Name,
		ValidityPeriod: validityPeriod,
	}

	bundle, err := h.certService.Issue(r.Context(), createReq)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, bundle)
}

// List handles GET /v1/certificates
func (h *CertificateHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	// Parse filter parameter
	filter := repository.CertificateFilterAll
	if filterStr := r.URL.Query().Get("status"); filterStr != "" {
		switch filterStr {
		case "active":
			filter = repository.CertificateFilterActive
		case "revoked":
			filter = repository.CertificateFilterRevoked
		case "expired":
			filter = repository.CertificateFilterExpired
		}
	}

	certs, err := h.certService.List(r.Context(), orgID.String(), filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	// Convert certificates to response format
	certResponses := make([]*CertificateResponse, len(certs.Certificates))
	for i, cert := range certs.Certificates {
		certResponses[i] = toCertificateResponse(&cert)
	}

	response.OK(w, map[string]any{
		"certificates": certResponses,
		"total":        certs.Total,
	})
}

// Get handles GET /v1/certificates/{id}
func (h *CertificateHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	certID := chi.URLParam(r, "id")
	if certID == "" {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid certificate ID"))
		return
	}

	cert, err := h.certService.Get(r.Context(), orgID.String(), certID)
	if err != nil {
		response.Error(w, err)
		return
	}
	if cert == nil {
		response.Error(w, apierrors.NewNotFoundError("Certificate"))
		return
	}

	response.OK(w, toCertificateResponse(cert))
}

// GetCA handles GET /v1/certificates/ca
func (h *CertificateHandler) GetCA(w http.ResponseWriter, r *http.Request) {
	caPEM, err := h.certService.GetCACertificate(r.Context())
	if err != nil {
		response.Error(w, apierrors.NewInternalError("Failed to get CA certificate"))
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=popsigner-ca.crt")
	w.WriteHeader(http.StatusOK)
	w.Write(caPEM)
}

// RevokeCertificateHTTPRequest is the HTTP request body for revoking a certificate.
type RevokeCertificateHTTPRequest struct {
	Reason string `json:"reason,omitempty"`
}

// Revoke handles POST /v1/certificates/{id}/revoke
func (h *CertificateHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	certID := chi.URLParam(r, "id")
	if certID == "" {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid certificate ID"))
		return
	}

	var req RevokeCertificateHTTPRequest
	json.NewDecoder(r.Body).Decode(&req) // Ignore decode errors, reason is optional

	if err := h.certService.Revoke(r.Context(), orgID.String(), certID, req.Reason); err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]string{"status": "revoked"})
}

// Delete handles DELETE /v1/certificates/{id}
func (h *CertificateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		response.Error(w, apierrors.ErrUnauthorized)
		return
	}

	certID := chi.URLParam(r, "id")
	if certID == "" {
		response.Error(w, apierrors.ErrBadRequest.WithMessage("Invalid certificate ID"))
		return
	}

	if err := h.certService.Delete(r.Context(), orgID.String(), certID); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

// CertificateResponse is the API response format for certificates.
type CertificateResponse struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Fingerprint      string                   `json:"fingerprint"`
	CommonName       string                   `json:"common_name"`
	SerialNumber     string                   `json:"serial_number"`
	Status           models.CertificateStatus `json:"status"`
	IssuedAt         string                   `json:"issued_at"`
	ExpiresAt        string                   `json:"expires_at"`
	RevokedAt        *string                  `json:"revoked_at,omitempty"`
	RevocationReason *string                  `json:"revocation_reason,omitempty"`
	CreatedAt        string                   `json:"created_at"`
}

// toCertificateResponse converts a Certificate model to a CertificateResponse.
func toCertificateResponse(cert *models.Certificate) *CertificateResponse {
	resp := &CertificateResponse{
		ID:               cert.ID.String(),
		Name:             cert.Name,
		Fingerprint:      cert.Fingerprint,
		CommonName:       cert.CommonName,
		SerialNumber:     cert.SerialNumber,
		Status:           cert.Status(),
		IssuedAt:         cert.IssuedAt.Format(time.RFC3339),
		ExpiresAt:        cert.ExpiresAt.Format(time.RFC3339),
		RevocationReason: cert.RevocationReason,
		CreatedAt:        cert.CreatedAt.Format(time.RFC3339),
	}

	if cert.RevokedAt != nil {
		revokedAt := cert.RevokedAt.Format(time.RFC3339)
		resp.RevokedAt = &revokedAt
	}

	return resp
}

