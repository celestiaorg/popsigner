package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/bundle"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/templates/pages"
)

// DeploymentWebHandler handles web UI routes for deployments.
type DeploymentWebHandler struct {
	repo    repository.Repository
	bundler *bundle.Bundler
}

// NewDeploymentWebHandler creates a new deployment web handler.
func NewDeploymentWebHandler(repo repository.Repository, bundler *bundle.Bundler) *DeploymentWebHandler {
	return &DeploymentWebHandler{
		repo:    repo,
		bundler: bundler,
	}
}

// DeploymentComplete renders the deployment complete page.
func (h *DeploymentWebHandler) DeploymentComplete(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.renderError(w, r, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Get deployment
	deployment, err := h.repo.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		if err == repository.ErrNotFound {
			h.renderError(w, r, "Deployment not found", http.StatusNotFound)
			return
		}
		h.renderError(w, r, "Failed to load deployment", http.StatusInternalServerError)
		return
	}

	// Only show complete page for completed deployments
	if deployment.Status != repository.StatusCompleted {
		// Redirect to deployment status page or show pending status
		http.Redirect(w, r, "/deployments/"+deploymentID.String()+"/status", http.StatusFound)
		return
	}

	// Get artifacts for this deployment
	artifacts, err := h.repo.GetAllArtifacts(r.Context(), deploymentID)
	if err != nil {
		// Non-fatal: continue without artifacts
		artifacts = nil
	}

	// Extract chain name from config if available
	chainName := h.extractChainName(deployment)

	// Build template data
	deploymentData := pages.DeploymentData{
		DeploymentID: deploymentID.String(),
		ChainName:    chainName,
		ChainID:      uint64(deployment.ChainID),
		Stack:        string(deployment.Stack),
		Status:       string(deployment.Status),
		CreatedAt:    deployment.CreatedAt.Format(time.RFC3339),
	}

	// Convert artifacts to template format
	artifactInfos := make([]pages.ArtifactInfo, 0, len(artifacts))
	for _, a := range artifacts {
		artifactInfos = append(artifactInfos, pages.ArtifactInfo{
			Name:        h.getArtifactDisplayName(a.ArtifactType, deployment.Stack),
			Description: h.getArtifactDescription(a.ArtifactType),
			Size:        h.formatSize(len(a.Content)),
			Type:        a.ArtifactType,
		})
	}

	component := pages.DeploymentCompletePage(deploymentData, artifactInfos)
	templ.Handler(component).ServeHTTP(w, r)
}

// DownloadBundle generates and returns the deployment bundle.
func (h *DeploymentWebHandler) DownloadBundle(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Get deployment to get chain info for filename
	deployment, err := h.repo.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		if err == repository.ErrNotFound {
			http.Error(w, "Deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load deployment", http.StatusInternalServerError)
		return
	}

	// Generate bundle
	bundleResult, err := h.bundler.CreateBundle(r.Context(), deploymentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate bundle: %v", err), http.StatusInternalServerError)
		return
	}

	// Use filename from result or generate from deployment
	filename := bundleResult.Filename
	if filename == "" {
		chainName := h.extractChainName(deployment)
		filename = fmt.Sprintf("%s-%s-artifacts.tar.gz", chainName, deployment.Stack)
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(bundleResult.Data)))

	w.Write(bundleResult.Data)
}

// DownloadArtifact downloads a single artifact.
func (h *DeploymentWebHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	artifactType := chi.URLParam(r, "type")
	if artifactType == "" {
		http.Error(w, "Artifact type is required", http.StatusBadRequest)
		return
	}

	artifact, err := h.repo.GetArtifact(r.Context(), deploymentID, artifactType)
	if err != nil {
		if err == repository.ErrNotFound {
			http.Error(w, "Artifact not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load artifact", http.StatusInternalServerError)
		return
	}

	// Get deployment for filename
	deployment, _ := h.repo.GetDeployment(r.Context(), deploymentID)
	chainName := "deployment"
	if deployment != nil {
		chainName = h.extractChainName(deployment)
	}

	// Determine content type and filename based on artifact type
	var contentType, filename string
	var content []byte

	switch artifactType {
	case "docker-compose.yml":
		contentType = "text/yaml"
		filename = fmt.Sprintf("%s-docker-compose.yml", chainName)
		content = unwrapJSONString(artifact.Content)
	case ".env.example":
		contentType = "text/plain"
		filename = fmt.Sprintf("%s.env.example", chainName)
		content = unwrapJSONString(artifact.Content)
	case "jwt.txt":
		contentType = "text/plain"
		filename = fmt.Sprintf("%s-jwt.txt", chainName)
		content = unwrapJSONString(artifact.Content)
	case "config.toml":
		contentType = "text/plain"
		filename = fmt.Sprintf("%s-config.toml", chainName)
		content = unwrapJSONString(artifact.Content)
	case "README.md":
		contentType = "text/markdown"
		filename = fmt.Sprintf("%s-README.md", chainName)
		content = unwrapJSONString(artifact.Content)
	default:
		// JSON artifacts
		contentType = "application/json"
		filename = fmt.Sprintf("%s-%s.json", chainName, artifactType)
		content = artifact.Content
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(content)
}

// unwrapJSONString unwraps a JSON-encoded string back to plain text.
// Used for non-JSON artifacts that were stored as JSON strings.
func unwrapJSONString(data []byte) []byte {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return []byte(s)
	}
	return data
}

// DeploymentStatus shows the deployment progress/status page.
func (h *DeploymentWebHandler) DeploymentStatus(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.renderError(w, r, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		if err == repository.ErrNotFound {
			h.renderError(w, r, "Deployment not found", http.StatusNotFound)
			return
		}
		h.renderError(w, r, "Failed to load deployment", http.StatusInternalServerError)
		return
	}

	// If completed, redirect to complete page
	if deployment.Status == repository.StatusCompleted {
		http.Redirect(w, r, "/deployments/"+deploymentID.String()+"/complete", http.StatusFound)
		return
	}

	// TODO: Render deployment status page with progress
	// For now, show a simple pending message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Deployment Status</title>
<meta http-equiv="refresh" content="5"></head>
<body style="background:#000;color:#33FF00;font-family:monospace;padding:40px;text-align:center;">
<h1>DEPLOYMENT IN PROGRESS</h1>
<p>Status: %s</p>
<p>Stage: %s</p>
<p>This page will refresh automatically...</p>
</body></html>`, deployment.Status, safeString(deployment.CurrentStage))
}

// Helper methods

func (h *DeploymentWebHandler) extractChainName(d *repository.Deployment) string {
	// Try to extract chain name from config JSON
	// Fall back to chain ID if not available
	if d.Config != nil {
		// Simple extraction - could be enhanced
		type config struct {
			ChainName string `json:"chain_name"`
			Name      string `json:"name"`
		}
		var cfg config
		if err := json.Unmarshal(d.Config, &cfg); err == nil {
			if cfg.ChainName != "" {
				return cfg.ChainName
			}
			if cfg.Name != "" {
				return cfg.Name
			}
		}
	}
	return fmt.Sprintf("chain-%d", d.ChainID)
}

func (h *DeploymentWebHandler) getArtifactDisplayName(artifactType string, stack repository.Stack) string {
	names := map[string]string{
		"genesis":        "genesis.json",
		"rollup":         "rollup.json",
		"addresses":      "addresses.json",
		"deploy_config":  "deploy-config.json",
		"chain_info":     "chain-info.json",
		"node_config":    "node-config.json",
		"core_contracts": "core-contracts.json",
	}
	if name, ok := names[artifactType]; ok {
		return name
	}
	return artifactType + ".json"
}

func (h *DeploymentWebHandler) getArtifactDescription(artifactType string) string {
	descriptions := map[string]string{
		"genesis":        "L2 genesis state",
		"rollup":         "Rollup configuration",
		"addresses":      "Deployed contract addresses",
		"deploy_config":  "Deployment configuration",
		"chain_info":     "Chain metadata and info",
		"node_config":    "Node configuration",
		"core_contracts": "Core contract addresses",
	}
	if desc, ok := descriptions[artifactType]; ok {
		return desc
	}
	return "Deployment artifact"
}

func (h *DeploymentWebHandler) formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func (h *DeploymentWebHandler) renderError(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Error</title></head>
<body style="background:#000;color:#FF3300;font-family:monospace;padding:40px;text-align:center;">
<h1>ERROR</h1>
<p>%s</p>
<a href="/dashboard" style="color:#33FF00;">← Back to Dashboard</a>
</body></html>`, message)
}

// MockBundler is used when bundler is not available
type MockBundler struct{}

func (m *MockBundler) CreateBundle(ctx context.Context, deploymentID uuid.UUID) ([]byte, error) {
	return nil, fmt.Errorf("bundler not configured")
}

