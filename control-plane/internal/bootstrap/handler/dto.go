// Package handler provides HTTP handlers for deployment management.
package handler

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// CreateDeploymentRequest is the request body for creating a new deployment.
type CreateDeploymentRequest struct {
	ChainID int64           `json:"chain_id"`
	Stack   string          `json:"stack"`
	Config  json.RawMessage `json:"config"`
}

// DeploymentResponse is the API response for a deployment.
type DeploymentResponse struct {
	ID           uuid.UUID       `json:"id"`
	ChainID      int64           `json:"chain_id"`
	Stack        string          `json:"stack"`
	Status       string          `json:"status"`
	CurrentStage *string         `json:"current_stage,omitempty"`
	Config       json.RawMessage `json:"config"`
	Error        *string         `json:"error,omitempty"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// TransactionResponse is the API response for a deployment transaction.
type TransactionResponse struct {
	ID          uuid.UUID `json:"id"`
	Stage       string    `json:"stage"`
	TxHash      string    `json:"tx_hash"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   string    `json:"created_at"`
}

// ArtifactResponse is the API response for a deployment artifact.
type ArtifactResponse struct {
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content"`
	CreatedAt string          `json:"created_at"`
}

// ArtifactListResponse wraps a list of artifacts.
type ArtifactListResponse struct {
	Artifacts []ArtifactResponse `json:"artifacts"`
}

// StartResponse is the response for starting a deployment.
type StartResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// toDeploymentResponse converts a repository Deployment to an API response.
func toDeploymentResponse(d *repository.Deployment) *DeploymentResponse {
	return &DeploymentResponse{
		ID:           d.ID,
		ChainID:      d.ChainID,
		Stack:        string(d.Stack),
		Status:       string(d.Status),
		CurrentStage: d.CurrentStage,
		Config:       d.Config,
		Error:        d.ErrorMessage,
		CreatedAt:    d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    d.UpdatedAt.Format(time.RFC3339),
	}
}

// toTransactionResponse converts a repository Transaction to an API response.
func toTransactionResponse(tx *repository.Transaction) *TransactionResponse {
	return &TransactionResponse{
		ID:          tx.ID,
		Stage:       tx.Stage,
		TxHash:      tx.TxHash,
		Description: tx.Description,
		CreatedAt:   tx.CreatedAt.Format(time.RFC3339),
	}
}

// toArtifactResponse converts a repository Artifact to an API response.
func toArtifactResponse(a *repository.Artifact) *ArtifactResponse {
	return &ArtifactResponse{
		Type:      a.ArtifactType,
		Content:   a.Content,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
}

