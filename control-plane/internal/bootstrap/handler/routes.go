package handler

import (
	"github.com/go-chi/chi/v5"
)

// Routes returns a chi router with all deployment routes configured.
func (h *DeploymentHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Deployment CRUD and management
	r.Post("/", h.Create)           // POST /api/v1/deployments
	r.Get("/", h.List)              // GET /api/v1/deployments
	r.Get("/{id}", h.Get)           // GET /api/v1/deployments/{id}
	r.Get("/{id}/status", h.Get)    // GET /api/v1/deployments/{id}/status (alias)
	r.Post("/{id}/start", h.Start)  // POST /api/v1/deployments/{id}/start

	// Artifacts
	r.Get("/{id}/artifacts", h.GetArtifacts)        // GET /api/v1/deployments/{id}/artifacts
	r.Get("/{id}/artifacts/{type}", h.GetArtifact)  // GET /api/v1/deployments/{id}/artifacts/{type}

	// Transactions
	r.Get("/{id}/transactions", h.GetTransactions)  // GET /api/v1/deployments/{id}/transactions

	return r
}

