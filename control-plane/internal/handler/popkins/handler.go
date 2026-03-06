// Package popkins provides HTTP handlers for the POPKins chain deployment platform.
// POPKins is a SEPARATE product from the main POPSigner dashboard.
// - POPKins: popkins.popsigner.com - Chain deployment/orchestration
// - Main Dashboard: dashboard.popsigner.com - Key management, signing
package popkins

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	mainrepo "github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
	"github.com/Bidon15/popsigner/control-plane/templates/components"
	"github.com/Bidon15/popsigner/control-plane/templates/layouts"
	"github.com/Bidon15/popsigner/control-plane/templates/pages"
)

// Session constants (shared with main web handler)
const (
	SessionCookieName = "banhbao_session"
)

// Orchestrator defines the interface for starting deployments.
type Orchestrator interface {
	StartDeployment(ctx context.Context, deploymentID uuid.UUID) error
}

// Handler handles POPKins-specific HTTP requests.
type Handler struct {
	authService  service.AuthService
	orgService   service.OrgService
	keyService   service.KeyService
	deployRepo   repository.Repository
	orchestrator Orchestrator
	sessionRepo  mainrepo.SessionRepository
	userRepo     mainrepo.UserRepository
}

// NewHandler creates a new POPKins handler.
func NewHandler(
	authService service.AuthService,
	orgService service.OrgService,
	keyService service.KeyService,
	deployRepo repository.Repository,
	orchestrator Orchestrator,
	sessionRepo mainrepo.SessionRepository,
	userRepo mainrepo.UserRepository,
) *Handler {
	return &Handler{
		authService:  authService,
		orgService:   orgService,
		keyService:   keyService,
		deployRepo:   deployRepo,
		orchestrator: orchestrator,
		sessionRepo:  sessionRepo,
		userRepo:     userRepo,
	}
}

// ============================================
// Placeholder Page Handlers (to be implemented in TASK-041 to TASK-044)
// ============================================

// DeploymentsList renders the list of deployments
func (h *Handler) DeploymentsList(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	// Mark stale running deployments as failed (pod may have crashed)
	// A deployment is considered stale if it's been "running" for more than 30 minutes
	// without any status update
	// HIGH-028: Scoped to user's organization to prevent cross-org data access
	staleTimeout := 30 * time.Minute
	staleCount, err := h.deployRepo.MarkStaleDeploymentsFailed(r.Context(), org.ID, staleTimeout)
	if err != nil {
		slog.Warn("failed to mark stale deployments as failed", "org_id", org.ID, "error", err)
	} else if staleCount > 0 {
		slog.Info("marked stale deployments as failed", "org_id", org.ID, "count", staleCount)
	}

	// CRIT-014: Fetch deployments for user's organization ONLY
	// Previously this was calling ListAllDeployments which returned ALL deployments globally
	deployments, err := h.deployRepo.ListDeploymentsByOrg(r.Context(), org.ID)
	if err != nil {
		slog.Error("failed to list deployments", "org_id", org.ID, "error", err)
		// Continue with empty list
		deployments = []*repository.Deployment{}
	}

	// Convert to template format
	summaries := make([]pages.DeploymentSummary, 0, len(deployments))
	for _, d := range deployments {
		summaries = append(summaries, pages.DeploymentSummary{
			ID:        d.ID.String(),
			ChainName: extractChainName(d),
			ChainID:   uint64(d.ChainID),
			Stack:     string(d.Stack),
			Status:    string(d.Status),
			CreatedAt: d.CreatedAt.Format("Jan 2, 2006"),
		})
	}

	data := pages.DeploymentsListData{
		PopkinsData: layouts.PopkinsData{
			UserName:   getUserName(user),
			UserEmail:  user.Email,
			AvatarURL:  getAvatarURL(user),
			OrgName:    org.Name,
			ActivePath: "/deployments",
		},
		Deployments: summaries,
		Total:       len(summaries),
	}

	// Render deployments list page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentsListPage(data).Render(r.Context(), w)
}

// extractChainName extracts the chain name from deployment config
func extractChainName(d *repository.Deployment) string {
	// Try to extract name from config JSON
	if d.Config != nil {
		var config struct {
			ChainName string `json:"chain_name"`
			Name      string `json:"name"`
		}
		if err := json.Unmarshal(d.Config, &config); err == nil {
			if config.ChainName != "" {
				return config.ChainName
			}
			if config.Name != "" {
				return config.Name
			}
		}
	}
	// Fallback to Chain ID based name
	return fmt.Sprintf("Chain %d", d.ChainID)
}

// extractBundleStack extracts the bundle_stack from deployment config (for pop-bundle)
func extractBundleStack(config json.RawMessage) string {
	if config == nil {
		return "opstack" // Default to OP Stack
	}
	var cfg struct {
		BundleStack string `json:"bundle_stack"`
	}
	if err := json.Unmarshal(config, &cfg); err == nil && cfg.BundleStack != "" {
		return cfg.BundleStack
	}
	return "opstack" // Default to OP Stack
}

// DeploymentsNew renders the new deployment wizard form
func (h *Handler) DeploymentsNew(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	// Determine current step (1-5)
	// Steps 1-4 are the normal wizard, step 5 is the POPKins bundle simplified config
	step := 1
	if s := r.URL.Query().Get("step"); s != "" {
		if s == "2-bundle" {
			// POPKins bundle uses a simplified step 2 (rendered as step 5)
			step = 5
		} else if parsed, err := strconv.Atoi(s); err == nil && parsed >= 1 && parsed <= 5 {
			step = parsed
		}
	}

	// Get form data from POST request or query params (for redirect back from key creation)
	formData := pages.DeploymentFormData{}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			formData = pages.DeploymentFormData{
				Stack:       r.FormValue("stack"),
				Environment: r.FormValue("environment"),
				BundleStack: r.FormValue("bundle_stack"),
				ChainName:   r.FormValue("chain_name"),
				ChainID:     r.FormValue("chain_id"),
				L1RPC:       r.FormValue("l1_rpc"),
				L1ChainID:   r.FormValue("l1_chain_id"),
				DA:          r.FormValue("da"),
				DeployerKey: r.FormValue("deployer_key"),
				BatcherKey:  r.FormValue("batcher_key"),
				ProposerKey: r.FormValue("proposer_key"),
			}
			slog.Info("DeploymentsNew POST form data",
				"step", step,
				"stack", formData.Stack,
				"environment", formData.Environment,
				"chain_name", formData.ChainName,
				"chain_id", formData.ChainID,
				"l1_chain_id", formData.L1ChainID,
				"da", formData.DA,
				"deployer_key", formData.DeployerKey,
			)
		} else {
			slog.Error("failed to parse form", "error", err)
		}
	} else {
		// Also check query params (for redirects back from inline key creation)
		q := r.URL.Query()
		formData = pages.DeploymentFormData{
			Stack:       q.Get("stack"),
			Environment: q.Get("environment"),
			BundleStack: q.Get("bundle_stack"),
			ChainName:   q.Get("chain_name"),
			ChainID:     q.Get("chain_id"),
			L1RPC:       q.Get("l1_rpc"),
			L1ChainID:   q.Get("l1_chain_id"),
			DA:          q.Get("da"),
			DeployerKey: q.Get("deployer_key"),
			BatcherKey:  q.Get("batcher_key"),
			ProposerKey: q.Get("proposer_key"),
		}
	}

	// Get user's keys for role assignment
	var keyOptions []components.KeyOption
	if h.keyService != nil {
		keys, err := h.keyService.List(r.Context(), org.ID, nil, nil)
		if err != nil {
			slog.Error("failed to list keys", "error", err)
		} else {
			keyOptions = make([]components.KeyOption, 0, len(keys))
			for _, k := range keys {
				ethAddr := k.Address
				if k.EthAddress != nil && *k.EthAddress != "" {
					ethAddr = *k.EthAddress
				}
				keyOptions = append(keyOptions, components.KeyOption{
					ID:         k.ID.String(),
					Name:       k.Name,
					Address:    ethAddr,
					CosmosAddr: k.Address,
				})
			}
		}
	}

	// Get error message from query if any
	errorMsg := r.URL.Query().Get("error")

	data := pages.DeploymentNewData{
		PopkinsData: layouts.PopkinsData{
			UserName:   getUserName(user),
			UserEmail:  user.Email,
			AvatarURL:  getAvatarURL(user),
			OrgName:    org.Name,
			ActivePath: "/deployments/new",
		},
		Keys:     keyOptions,
		Step:     step,
		FormData: formData,
		ErrorMsg: errorMsg,
	}

	// Render deployment wizard page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentNewPage(data).Render(r.Context(), w)
}

// DeploymentsCreate handles the final form submission and creates the deployment
func (h *Handler) DeploymentsCreate(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}
	_ = user // user context available for audit logging

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/deployments/new?step=4&error=Invalid+form+data", http.StatusFound)
		return
	}

	// Helper to build redirect URL with preserved form data
	buildErrorRedirect := func(step int, errorMsg string) string {
		q := make(url.Values)
		q.Set("step", strconv.Itoa(step))
		q.Set("error", errorMsg)
		q.Set("stack", r.FormValue("stack"))
		q.Set("environment", r.FormValue("environment"))
		q.Set("chain_name", r.FormValue("chain_name"))
		q.Set("chain_id", r.FormValue("chain_id"))
		q.Set("l1_rpc", r.FormValue("l1_rpc"))
		q.Set("l1_chain_id", r.FormValue("l1_chain_id"))
		q.Set("da", r.FormValue("da"))
		q.Set("deployer_key", r.FormValue("deployer_key"))
		q.Set("batcher_key", r.FormValue("batcher_key"))
		q.Set("proposer_key", r.FormValue("proposer_key"))
		return "/deployments/new?" + q.Encode()
	}

	// Parse chain ID
	chainID, err := strconv.ParseInt(r.FormValue("chain_id"), 10, 64)
	if err != nil || chainID <= 0 {
		http.Redirect(w, r, buildErrorRedirect(2, "Invalid chain ID - must be a positive number"), http.StatusFound)
		return
	}

	// Validate required fields
	chainName := r.FormValue("chain_name")
	stack := r.FormValue("stack")
	environment := r.FormValue("environment")
	l1RPC := r.FormValue("l1_rpc")

	if chainName == "" {
		http.Redirect(w, r, buildErrorRedirect(3, "Chain name is required"), http.StatusFound)
		return
	}
	if stack == "" {
		http.Redirect(w, r, buildErrorRedirect(1, "Please select a rollup stack"), http.StatusFound)
		return
	}
	if environment == "" {
		http.Redirect(w, r, buildErrorRedirect(2, "Please select an environment"), http.StatusFound)
		return
	}

	// Check if this is local development (previously detected via pop-bundle)
	isLocal := environment == "local"

	// Local uses Anvil, testnet needs L1 RPC validation
	if !isLocal && l1RPC == "" {
		http.Redirect(w, r, buildErrorRedirect(3, "L1 RPC URL is required for testnet deployment"), http.StatusFound)
		return
	}

	// Parse L1 chain ID as uint64 (local defaults to 31337 for Anvil)
	var l1ChainID uint64
	if isLocal {
		l1ChainID = 31337 // Anvil default
		if l1RPC == "" {
			l1RPC = "http://localhost:8545" // Will be overridden in bundle
		}
	} else {
		l1ChainID, err = strconv.ParseUint(r.FormValue("l1_chain_id"), 10, 64)
		if err != nil || l1ChainID == 0 {
			http.Redirect(w, r, buildErrorRedirect(3, "Invalid L1 chain ID"), http.StatusFound)
			return
		}
	}

	// Validate keys are selected (local uses Anvil placeholder keys)
	deployerKey := r.FormValue("deployer_key")
	batcherKey := r.FormValue("batcher_key")
	proposerKey := r.FormValue("proposer_key")
	if !isLocal && (deployerKey == "" || batcherKey == "" || proposerKey == "") {
		http.Redirect(w, r, buildErrorRedirect(4, "Please select keys for all roles"), http.StatusFound)
		return
	}
	// For local, use Anvil placeholder keys if not provided
	if isLocal {
		if deployerKey == "" {
			deployerKey = "anvil-0"
		}
		if batcherKey == "" {
			batcherKey = "anvil-1"
		}
		if proposerKey == "" {
			proposerKey = "anvil-2"
		}
	}

	// For backwards compatibility: if it's local, use pop-bundle for the deployment stack
	// but store the actual stack (opstack/nitro) in bundle_stack
	actualStack := stack // Save the actual stack selected by user
	if isLocal {
		stack = "pop-bundle" // Use pop-bundle for the deployment record
	}

	// Build deployment config
	config := map[string]interface{}{
		"chain_name":   chainName,
		"chain_id":     uint64(chainID),
		"l1_rpc":       l1RPC,
		"l1_chain_id":  l1ChainID,
		"da":           r.FormValue("da"),
		"environment":  environment,
		"deployer_key": deployerKey,
		"batcher_key":  batcherKey,
		"proposer_key": proposerKey,
		"org_id":       org.ID.String(),
	}

	// For local deployments, add bundle_stack (which is the actual stack to use)
	if isLocal {
		config["bundle_stack"] = actualStack // Store the actual stack (opstack or nitro)
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		http.Redirect(w, r, buildErrorRedirect(4, "Internal error - please try again"), http.StatusFound)
		return
	}

	// Create deployment record with organization ID for authorization
	deployment := &repository.Deployment{
		ID:      uuid.New(),
		OrgID:   org.ID, // CRIT-010: Associate deployment with user's organization
		ChainID: chainID,
		Stack:   repository.Stack(stack),
		Status:  repository.StatusPending,
		Config:  configJSON,
	}

	if err := h.deployRepo.CreateDeployment(r.Context(), deployment); err != nil {
		slog.Error("failed to create deployment", "error", err, "chain_id", chainID, "stack", stack)
		// Provide more specific error messages based on the error
		errMsg := "Failed to create deployment"
		errStr := err.Error()
		if strings.Contains(errStr, "unique") || strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "violates unique constraint") {
			errMsg = fmt.Sprintf("Chain ID %d already exists - please use a different chain ID", chainID)
		} else if strings.Contains(errStr, "invalid input value for enum") {
			errMsg = fmt.Sprintf("Stack type '%s' is not supported - database migration may be needed", stack)
		} else {
			errMsg = fmt.Sprintf("Database error: %s", errStr)
		}
		http.Redirect(w, r, buildErrorRedirect(4, errMsg), http.StatusFound)
		return
	}

	slog.Info("deployment created",
		"deployment_id", deployment.ID,
		"chain_name", chainName,
		"chain_id", chainID,
		"stack", stack,
		"org_id", org.ID,
	)

	// Start the deployment orchestrator
	if h.orchestrator != nil {
		if err := h.orchestrator.StartDeployment(r.Context(), deployment.ID); err != nil {
			slog.Error("failed to start deployment orchestrator",
				"deployment_id", deployment.ID,
				"error", err,
			)
			// Don't fail - deployment was created, orchestrator can retry later
		} else {
			slog.Info("deployment orchestrator started",
				"deployment_id", deployment.ID,
			)
		}
	} else {
		slog.Warn("no orchestrator configured, deployment will remain pending",
			"deployment_id", deployment.ID,
		)
	}

	// Redirect to deployment status page
	http.Redirect(w, r, "/deployments/"+deployment.ID.String()+"/status", http.StatusFound)
}

// CreateKeyInline handles inline key creation from the wizard (step 3)
// Creates a key and redirects back to step 3 with preserved form state
func (h *Handler) CreateKeyInline(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/deployments/new?step=3&error=Invalid+form+data", http.StatusFound)
		return
	}

	keyName := r.FormValue("key_name")
	if keyName == "" {
		http.Redirect(w, r, "/deployments/new?step=3&error=Key+name+is+required", http.StatusFound)
		return
	}

	// Get the default namespace for this org (actorID must be user.ID for permission check)
	namespaces, err := h.orgService.ListNamespaces(r.Context(), org.ID, user.ID)
	if err != nil || len(namespaces) == 0 {
		slog.Error("failed to list namespaces", "org_id", org.ID, "error", err)
		http.Redirect(w, r, "/deployments/new?step=3&error=No+namespace+available", http.StatusFound)
		return
	}
	defaultNS := namespaces[0]

	// Create the key
	_, err = h.keyService.Create(r.Context(), service.CreateKeyRequest{
		OrgID:       org.ID,
		NamespaceID: defaultNS.ID,
		Name:        keyName,
		Exportable:  false,
		NetworkType: "all",
	})
	if err != nil {
		slog.Error("failed to create key", "name", keyName, "error", err)
		http.Redirect(w, r, "/deployments/new?step=3&error=Failed+to+create+key", http.StatusFound)
		return
	}

	slog.Info("key created inline", "name", keyName, "org_id", org.ID)

	// Build redirect URL preserving wizard state (URL-encode values!)
	q := make(url.Values)
	q.Set("step", "3")
	q.Set("stack", r.FormValue("stack"))
	q.Set("chain_name", r.FormValue("chain_name"))
	q.Set("chain_id", r.FormValue("chain_id"))
	q.Set("l1_rpc", r.FormValue("l1_rpc"))
	q.Set("l1_chain_id", r.FormValue("l1_chain_id"))
	q.Set("da", r.FormValue("da"))
	q.Set("deployer_key", r.FormValue("deployer_key"))
	q.Set("batcher_key", r.FormValue("batcher_key"))
	q.Set("proposer_key", r.FormValue("proposer_key"))
	redirectURL := "/deployments/new?" + q.Encode()

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// DeploymentDetail renders a specific deployment detail page (TASK-043)
func (h *Handler) DeploymentDetail(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	// Extract deployment ID from URL
	idStr := chi.URLParam(r, "id")
	deploymentID, err := uuid.Parse(idStr)
	if err != nil {
		slog.Error("invalid deployment ID", "id", idStr, "error", err)
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Fetch deployment from repository
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, "Deployment not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Error(w, "Failed to load deployment", http.StatusInternalServerError)
		return
	}

	// CRIT-010: Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Extract key addresses from config
	deployerAddr, batcherAddr, proposerAddr, l1Chain := extractConfigAddresses(deployment.Config)

	// Fetch transaction history for OP Stack deployments
	var transactions []pages.TransactionInfo
	if deployment.Stack == repository.StackOPStack {
		txs, err := h.deployRepo.GetTransactionsByDeployment(r.Context(), deploymentID)
		if err == nil {
			for _, tx := range txs {
				desc := ""
				if tx.Description != nil {
					desc = *tx.Description
				}
				transactions = append(transactions, pages.TransactionInfo{
					TxHash:      tx.TxHash,
					Stage:       tx.Stage,
					Description: desc,
					CreatedAt:   tx.CreatedAt.Format("Jan 2, 2006 15:04"),
				})
			}
		}
	}

	// Build deployment info
	currentStage := ""
	if deployment.CurrentStage != nil {
		currentStage = *deployment.CurrentStage
	}
	errorMsg := ""
	if deployment.ErrorMessage != nil {
		errorMsg = *deployment.ErrorMessage
	}

	data := pages.DeploymentDetailData{
		PopkinsData: layouts.PopkinsData{
			UserName:   getUserName(user),
			UserEmail:  user.Email,
			AvatarURL:  getAvatarURL(user),
			OrgName:    org.Name,
			ActivePath: "/deployments",
		},
		Deployment: pages.DeploymentInfo{
			ID:           deployment.ID.String(),
			ChainName:    extractChainName(deployment),
			ChainID:      uint64(deployment.ChainID),
			Stack:        string(deployment.Stack),
			Status:       string(deployment.Status),
			CurrentStage: currentStage,
			ErrorMessage: errorMsg,
			L1Chain:      l1Chain,
			DeployerAddr: deployerAddr,
			BatcherAddr:  batcherAddr,
			ProposerAddr: proposerAddr,
			CreatedAt:    deployment.CreatedAt.Format("Jan 2, 2006 15:04 MST"),
			UpdatedAt:    deployment.UpdatedAt.Format("Jan 2, 2006 15:04 MST"),
		},
		Transactions: transactions,
	}

	// Render deployment detail page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentDetailPage(data).Render(r.Context(), w)
}

// extractConfigAddresses extracts key addresses from deployment config JSON
func extractConfigAddresses(config json.RawMessage) (deployer, batcher, proposer, l1Chain string) {
	if config == nil {
		return "", "", "", "Ethereum Mainnet"
	}

	var cfg struct {
		DeployerAddress string `json:"deployer_address"`
		BatcherAddress  string `json:"batcher_address"`
		ProposerAddress string `json:"proposer_address"`
		// Alternative field names
		Deployer  string `json:"deployer"`
		Batcher   string `json:"batcher"`
		Proposer  string `json:"proposer"`
		Validator string `json:"validator"`
		// L1 chain
		L1Chain   string `json:"l1_chain"`
		L1ChainID int64  `json:"l1_chain_id"`
	}

	if err := json.Unmarshal(config, &cfg); err != nil {
		return "", "", "", "Ethereum Mainnet"
	}

	// Get deployer address
	deployer = cfg.DeployerAddress
	if deployer == "" {
		deployer = cfg.Deployer
	}

	// Get batcher address
	batcher = cfg.BatcherAddress
	if batcher == "" {
		batcher = cfg.Batcher
	}

	// Get proposer/validator address
	proposer = cfg.ProposerAddress
	if proposer == "" {
		proposer = cfg.Proposer
	}
	if proposer == "" {
		proposer = cfg.Validator
	}

	// Determine L1 chain name
	l1Chain = cfg.L1Chain
	if l1Chain == "" {
		switch cfg.L1ChainID {
		case 1:
			l1Chain = "Ethereum Mainnet"
		case 11155111:
			l1Chain = "Sepolia Testnet"
		case 17000:
			l1Chain = "Holesky Testnet"
		default:
			l1Chain = "Ethereum Mainnet"
		}
	}

	return deployer, batcher, proposer, l1Chain
}

// DeploymentStatus renders the deployment status/progress page (TASK-044)
func (h *Handler) DeploymentStatus(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	// Get deployment ID from URL
	deploymentID := chi.URLParam(r, "id")
	if deploymentID == "" {
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	deploymentUUID, err := uuid.Parse(deploymentID)
	if err != nil {
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// Get deployment from repository
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deploymentUUID)
	if err != nil {
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// CRIT-010: Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// If completed, redirect to complete page
	if deployment.Status == repository.StatusCompleted {
		http.Redirect(w, r, "/deployments/"+deploymentID+"/complete", http.StatusFound)
		return
	}

	// Build progress data
	data := h.buildProgressData(r.Context(), user, org, deployment)

	// Render progress page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentProgressPage(data).Render(r.Context(), w)
}

// DeploymentProgressPartial returns just the progress content for HTMX polling
func (h *Handler) DeploymentProgressPartial(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Get deployment ID from URL
	deploymentID := chi.URLParam(r, "id")
	if deploymentID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	deploymentUUID, err := uuid.Parse(deploymentID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get deployment from repository
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deploymentUUID)
	if err != nil {
		slog.Error("failed to get deployment for partial", "id", deploymentID, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// CRIT-010: Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Build progress data
	data := h.buildProgressData(r.Context(), user, org, deployment)

	// Render just the progress content (partial)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentProgressPartial(data).Render(r.Context(), w)
}

// buildProgressData constructs the progress page data from a deployment
func (h *Handler) buildProgressData(ctx context.Context, user *models.User, org *models.Organization, deployment *repository.Deployment) pages.DeploymentProgressData {
	// Build stage list based on stack
	stages := h.buildStagesInfo(deployment)

	// Get latest transaction if available
	var latestTx *components.TxInfo
	txs, err := h.deployRepo.GetTransactionsByDeployment(ctx, deployment.ID)
	if err == nil && len(txs) > 0 {
		lastTx := txs[len(txs)-1]
		explorerURL := getExplorerURL(lastTx.TxHash, extractL1ChainID(deployment))
		latestTx = &components.TxInfo{
			Hash:        lastTx.TxHash,
			Status:      "confirmed",
			ExplorerURL: explorerURL,
		}
	}

	// Get error message if any
	var errorMsg *string
	if deployment.ErrorMessage != nil && *deployment.ErrorMessage != "" {
		errorMsg = deployment.ErrorMessage
	}

	// Count completed stages
	completedCount := 0
	for _, s := range stages {
		if s.Status == "completed" {
			completedCount++
		}
	}

	currentStage := ""
	if deployment.CurrentStage != nil {
		currentStage = *deployment.CurrentStage
	}

	return pages.DeploymentProgressData{
		PopkinsData: layouts.PopkinsData{
			UserName:   getUserName(user),
			UserEmail:  user.Email,
			AvatarURL:  getAvatarURL(user),
			OrgName:    org.Name,
			ActivePath: "/deployments",
		},
		DeploymentID:    deployment.ID.String(),
		ChainName:       extractChainName(deployment),
		ChainID:         deployment.ChainID,
		Stack:           string(deployment.Stack),
		Status:          string(deployment.Status),
		CurrentStage:    currentStage,
		TotalStages:     len(stages),
		CompletedStages: completedCount,
		Stages:          stages,
		LatestTx:        latestTx,
		ErrorMsg:        errorMsg,
		L1ChainID:       extractL1ChainID(deployment),
	}
}

// buildStagesInfo creates stage info list for the progress display
func (h *Handler) buildStagesInfo(deployment *repository.Deployment) []components.StageInfo {
	var stages []components.StageInfo

	currentStage := ""
	if deployment.CurrentStage != nil {
		currentStage = *deployment.CurrentStage
	}

	switch deployment.Stack {
	case repository.StackOPStack:
		// OP Stack stages - these match the orchestrator stage names
		// The op-deployer runs the full pipeline, so we track high-level phases
		stages = []components.StageInfo{
			{Name: "Preflight Checks", Status: "pending", TxCount: 0},
			{Name: "Deploy Superchain", Status: "pending", TxCount: 0},
			{Name: "Deploy Implementations", Status: "pending", TxCount: 0},
			{Name: "Deploy OP Chain", Status: "pending", TxCount: 0},
			{Name: "Generate Genesis", Status: "pending", TxCount: 0},
			{Name: "Create Bundle", Status: "pending", TxCount: 0},
		}
	case repository.StackPopBundle:
		// Pop-bundle stages - check bundle_stack to determine OP vs Nitro
		bundleStack := extractBundleStack(deployment.Config)
		if bundleStack == "nitro" {
			// Nitro bundle stages - match orchestrator stages
			stages = []components.StageInfo{
				{Name: "Start Anvil", Status: "pending", TxCount: 0, Details: "Starting local L1"},
				{Name: "Download Artifacts", Status: "pending", TxCount: 0, Details: "Fetching Nitro contracts"},
				{Name: "Deploy Infrastructure", Status: "pending", TxCount: 22, Details: "RollupCreator + templates"},
				{Name: "Deploy WETH", Status: "pending", TxCount: 1, Details: "Stake token for BOLD"},
				{Name: "Create Rollup", Status: "pending", TxCount: 1, Details: "RollupCreator.createRollup()"},
				{Name: "Capture State", Status: "pending", TxCount: 0, Details: "Dumping Anvil state"},
				{Name: "Generate Configs", Status: "pending", TxCount: 0, Details: "Creating bundle files"},
			}
		} else {
			// OP Stack bundle stages (default)
			stages = []components.StageInfo{
				{Name: "Start Anvil", Status: "pending", TxCount: 0, Details: "Starting local L1"},
				{Name: "Deploy Contracts", Status: "pending", TxCount: 0, Details: "OP Stack to Anvil"},
				{Name: "Capture State", Status: "pending", TxCount: 0, Details: "Dumping Anvil state"},
				{Name: "Generate Configs", Status: "pending", TxCount: 0, Details: "Creating bundle files"},
			}
		}
	default:
		// Nitro stages - detailed breakdown of actual deployment steps
		stages = []components.StageInfo{
			{Name: "Initialize", Status: "pending", TxCount: 0, Details: "Loading configuration"},
			{Name: "Fetch Certificates", Status: "pending", TxCount: 0, Details: "Getting PopSigner certs"},
			{Name: "Download Contracts", Status: "pending", TxCount: 0, Details: "Fetching from S3"},
			{Name: "Deploy Infrastructure", Status: "pending", TxCount: 22, Details: "Core contracts"},
			{Name: "Create Rollup", Status: "pending", TxCount: 1, Details: "RollupCreator.createRollup()"},
			{Name: "Configure Staking", Status: "pending", TxCount: 1, Details: "Wrap ETH → WETH"},
			{Name: "Generate Bundle", Status: "pending", TxCount: 0, Details: "Node configs & artifacts"},
		}
	}

	// Map orchestrator stage names to UI stage indices
	opStackStageIndex := map[string]int{
		"init":                   0, // Preflight Checks
		"deploy_superchain":      1, // Deploy Superchain
		"deploy_implementations": 2, // Deploy Implementations
		"deploy_opchain":         3, // Deploy OP Chain
		"deploy_alt_da":          3, // Part of Deploy OP Chain (Celestia DA)
		"generate_genesis":       4, // Generate Genesis
		"set_start_block":        4, // Part of Generate Genesis
		"completed":              5, // Create Bundle (all done)
	}

	// Map orchestrator stage names to UI stage indices for Nitro
	nitroStageIndex := map[string]int{
		"init":               0, // Initialize
		"certificates":       1, // Fetch Certificates
		"download_artifacts": 2, // Download Contracts
		"infrastructure":     3, // Deploy Infrastructure (22 contracts)
		"deploying":          4, // Create Rollup
		"staking":            5, // Configure Staking (WETH wrap)
		"artifacts":          6, // Generate Bundle
		"completed":          6, // All done
	}

	// Map orchestrator stage names to UI stage indices for Pop-bundle (OP Stack)
	popBundleStageIndex := map[string]int{
		"starting_anvil":      0, // Start Anvil
		"deploying_contracts": 1, // Deploy Contracts
		"capturing_state":     2, // Capture State
		"generating_configs":  3, // Generate Configs
		"complete":            3, // All done
	}

	// Map orchestrator stage names to UI stage indices for Pop-bundle (Nitro)
	popBundleNitroStageIndex := map[string]int{
		"starting_anvil":            0, // Start Anvil
		"downloading_artifacts":     1, // Download Artifacts
		"deploying_infrastructure":  2, // Deploy Infrastructure
		"deploying_weth":            3, // Deploy WETH
		"creating_rollup":           4, // Create Rollup
		"capturing_state":           5, // Capture State
		"generating_configs":        6, // Generate Configs
		"complete":                  6, // All done
	}

	// Helper function to get stage index from current stage name
	getStageIndex := func(stageName string) int {
		switch deployment.Stack {
		case repository.StackOPStack:
			if idx, ok := opStackStageIndex[stageName]; ok {
				return idx
			}
		case repository.StackPopBundle:
			// Check if it's a Nitro bundle
			bundleStack := extractBundleStack(deployment.Config)
			if bundleStack == "nitro" {
				if idx, ok := popBundleNitroStageIndex[stageName]; ok {
					return idx
				}
			} else {
				if idx, ok := popBundleStageIndex[stageName]; ok {
					return idx
				}
			}
		default:
			if idx, ok := nitroStageIndex[stageName]; ok {
				return idx
			}
		}
		return 0
	}

	// Update stages based on deployment status
	if deployment.Status == repository.StatusCompleted {
		// All stages complete
		for i := range stages {
			stages[i].Status = "completed"
			stages[i].TxComplete = stages[i].TxCount
		}
	} else if deployment.Status == repository.StatusFailed {
		// Determine which stage failed based on currentStage
		failedIndex := getStageIndex(currentStage)

		// Mark stages before failed as complete, failed stage as failed
		for i := range stages {
			if i < failedIndex {
				stages[i].Status = "completed"
				stages[i].TxComplete = stages[i].TxCount
			} else if i == failedIndex {
				stages[i].Status = "failed"
			}
			// Remaining stages stay "pending"
		}
	} else if deployment.Status == repository.StatusRunning {
		// Determine current stage index
		currentIndex := getStageIndex(currentStage)

		// Mark stages before current as complete, current as in_progress
		for i := range stages {
			if i < currentIndex {
				stages[i].Status = "completed"
				stages[i].TxComplete = stages[i].TxCount
			} else if i == currentIndex {
				stages[i].Status = "in_progress"
				// Keep the predefined details for better UX
			}
			// Remaining stages stay "pending"
		}
	} else if deployment.Status == repository.StatusPaused {
		// Paused - show current stage as paused indicator
		currentIndex := getStageIndex(currentStage)

		for i := range stages {
			if i < currentIndex {
				stages[i].Status = "completed"
				stages[i].TxComplete = stages[i].TxCount
			} else if i == currentIndex {
				stages[i].Status = "pending" // Paused at this stage
				stages[i].Details = "Paused"
			}
		}
	}
	// StatusPending: all stages remain "pending" (the default)

	return stages
}

// getExplorerURL returns the block explorer URL for a transaction hash
func getExplorerURL(txHash, l1ChainID string) string {
	switch l1ChainID {
	case "1":
		return "https://etherscan.io/tx/" + txHash
	case "11155111":
		return "https://sepolia.etherscan.io/tx/" + txHash
	case "17000":
		return "https://holesky.etherscan.io/tx/" + txHash
	default:
		return "https://etherscan.io/tx/" + txHash
	}
}

// extractL1ChainID extracts L1 chain ID from deployment config
func extractL1ChainID(d *repository.Deployment) string {
	if d.Config != nil {
		var config struct {
			L1ChainID string `json:"l1_chain_id"`
		}
		if err := json.Unmarshal(d.Config, &config); err == nil && config.L1ChainID != "" {
			return config.L1ChainID
		}
	}
	return "11155111" // Default to Sepolia
}

// DeploymentComplete renders the deployment complete page (reuses TASK-032)
func (h *Handler) DeploymentComplete(w http.ResponseWriter, r *http.Request) {
	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	deploymentID := chi.URLParam(r, "id")
	if deploymentID == "" {
		http.Error(w, "Missing deployment ID", http.StatusBadRequest)
		return
	}

	deployID, err := uuid.Parse(deploymentID)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Get deployment
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deployID)
	if err != nil {
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Extract chain name from config
	chainName := extractChainName(deployment)

	// Build deployment data for template
	deploymentData := pages.DeploymentData{
		DeploymentID: deploymentID,
		ChainName:    chainName,
		ChainID:      uint64(deployment.ChainID),
		Stack:        string(deployment.Stack),
		Status:       string(deployment.Status),
		CreatedAt:    deployment.CreatedAt.Format("Jan 2, 2006"),
	}

	// Get artifacts for the artifact list
	var artifactInfos []pages.ArtifactInfo
	artifacts, err := h.deployRepo.GetAllArtifacts(r.Context(), deployID)
	if err == nil && len(artifacts) > 0 {
		for _, a := range artifacts {
			// Skip internal artifacts
			if a.ArtifactType == "deployment_state" {
				continue
			}

			info := pages.ArtifactInfo{
				Name: a.ArtifactType,
				Type: a.ArtifactType,
				Size: formatBytes(len(a.Content)),
			}

			// Add descriptions based on artifact type
			switch a.ArtifactType {
			case "genesis.json":
				info.Description = "L2 genesis state"
			case "rollup.json":
				info.Description = "Rollup configuration"
			case "addresses.json":
				info.Description = "Deployed contract addresses"
			case "docker-compose.yml":
				info.Description = "Docker Compose configuration"
			case "jwt.txt":
				info.Description = "JWT secret for Engine API"
			case "config.toml":
				info.Description = "Celestia DA configuration"
			case ".env.example":
				info.Description = "Environment variables template"
			case "README.md":
				info.Description = "Setup instructions"
			case "anvil-state.json":
				info.Description = "Pre-deployed L1 state"
			}

			artifactInfos = append(artifactInfos, info)
		}
	}

	// Render the complete page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentCompletePage(deploymentData, artifactInfos).Render(r.Context(), w)
}

// DownloadBundle handles artifact bundle downloads as a ZIP file.
// The bundle includes: genesis.json, rollup.json, addresses.json, docker-compose.yml,
// .env.example, jwt.txt, config.toml (for Celestia DA), and README.md.
func (h *Handler) DownloadBundle(w http.ResponseWriter, r *http.Request) {
	// CRIT-010: Get authenticated user's org for authorization
	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	deploymentID := chi.URLParam(r, "id")
	if deploymentID == "" {
		http.Error(w, "Missing deployment ID", http.StatusBadRequest)
		return
	}

	deployID, err := uuid.Parse(deploymentID)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Get deployment to verify it exists
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deployID)
	if err != nil {
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// CRIT-010: Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Only allow download for completed deployments
	if deployment.Status != "completed" {
		http.Error(w, "Artifacts only available for completed deployments", http.StatusBadRequest)
		return
	}

	// Get all artifacts for this deployment
	artifacts, err := h.deployRepo.GetAllArtifacts(r.Context(), deployID)
	if err != nil {
		slog.Error("failed to get artifacts", "id", deploymentID, "error", err)
		http.Error(w, "Failed to retrieve artifacts", http.StatusInternalServerError)
		return
	}

	if len(artifacts) == 0 {
		http.Error(w, "No artifacts found for this deployment", http.StatusNotFound)
		return
	}

	// Extract chain name and bundle stack from config
	chainName := extractChainName(deployment)
	safeName := opstack.SanitizeChainNameForFilename(chainName)
	bundleStack := extractBundleStack(deployment.Config) // "opstack" or "nitro"
	isNitroBundle := bundleStack == "nitro"

	// Create ZIP bundle
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Bundle directory prefix - use stack-specific naming
	stackName := string(deployment.Stack)
	bundlePrefix := fmt.Sprintf("%s-%s-bundle/", safeName, stackName)

	slog.Info("DownloadBundle: creating bundle",
		slog.String("deployment_id", deploymentID),
		slog.String("stack", stackName),
		slog.String("bundle_stack", bundleStack),
		slog.Bool("is_nitro", isNitroBundle),
		slog.Int("artifact_count", len(artifacts)),
	)

	// Map artifact types to file paths in the ZIP
	for _, artifact := range artifacts {
		var path string
		var isPlainText bool  // Non-JSON files need to be unwrapped from JSON string encoding
		var isExecutable bool // Script files need executable permission

		switch artifact.ArtifactType {
		case "genesis.json", "genesis":
			path = bundlePrefix + "genesis.json"
		case "rollup.json", "rollup_config":
			path = bundlePrefix + "rollup.json"
		case "addresses.json":
			// For Nitro bundles, put in config/ directory
			if isNitroBundle {
				path = bundlePrefix + "config/addresses.json"
			} else {
				path = bundlePrefix + "addresses.json"
			}
		case "deploy-config.json":
			path = bundlePrefix + "deploy-config.json"
		case "docker-compose.yml":
			path = bundlePrefix + "docker-compose.yml"
			isPlainText = true
		case ".env.example":
			path = bundlePrefix + ".env.example"
			isPlainText = true
		case "jwt.txt":
			// For Nitro bundles, put in config/ directory
			if isNitroBundle {
				path = bundlePrefix + "config/jwt.txt"
			} else {
				path = bundlePrefix + "jwt.txt"
			}
			isPlainText = true
		case "config.toml":
			// op-alt-da config for Celestia DA
			path = bundlePrefix + "config.toml"
			isPlainText = true
		case "anvil-state.json":
			// For Nitro bundles, put in state/ directory
			if isNitroBundle {
				path = bundlePrefix + "state/anvil-state.json"
			} else {
				path = bundlePrefix + "anvil-state.json"
			}
		case "l1-chain-config.json":
			// L1 chain configuration for op-node (required for Anvil)
			path = bundlePrefix + "l1-chain-config.json"
		case "README.md":
			path = bundlePrefix + "README.md"
			isPlainText = true

		// ========================================
		// Nitro POPKins Bundle artifacts
		// (from NitroConfigWriter.GenerateAll)
		// ========================================
		case "chain-info.json":
			// Nitro chain configuration
			path = bundlePrefix + "config/chain-info.json"
		case "celestia-config.toml":
			// Celestia DA server configuration
			path = bundlePrefix + "config/celestia-config.toml"
			isPlainText = true
		case ".env":
			// Ready-to-use .env file for Nitro
			path = bundlePrefix + ".env"
			isPlainText = true
		case "scripts/start.sh":
			// Two-phase startup script (handles Issue #4208)
			path = bundlePrefix + "scripts/start.sh"
			isPlainText = true
			isExecutable = true
		case "scripts/stop.sh":
			// Stop devnet script
			path = bundlePrefix + "scripts/stop.sh"
			isPlainText = true
			isExecutable = true
		case "scripts/reset.sh":
			// Reset all state script
			path = bundlePrefix + "scripts/reset.sh"
			isPlainText = true
			isExecutable = true
		case "scripts/test.sh":
			// Health check script
			path = bundlePrefix + "scripts/test.sh"
			isPlainText = true
			isExecutable = true

		// ========================================
		// Legacy Nitro artifacts (underscore naming)
		// ========================================
		case "chain_info":
			path = bundlePrefix + "config/chain-info.json"
		case "node_config":
			path = bundlePrefix + "config/node-config.json"
		case "core_contracts":
			path = bundlePrefix + "config/core-contracts.json"
		case "docker_compose":
			path = bundlePrefix + "docker-compose.yaml"
			isPlainText = true
		case "celestia_config":
			path = bundlePrefix + "config/celestia-config.toml"
			isPlainText = true
		case "env_example":
			path = bundlePrefix + ".env.example"
			isPlainText = true
		case "readme":
			path = bundlePrefix + "README.md"
			isPlainText = true
		case "client_cert":
			path = bundlePrefix + "certs/client.crt"
			isPlainText = true
		case "client_key":
			path = bundlePrefix + "certs/client.key"
			isPlainText = true
		case "ca_cert":
			path = bundlePrefix + "certs/ca.crt"
			isPlainText = true
		default:
			// Skip internal artifacts like deployment_state
			slog.Debug("DownloadBundle: skipping artifact",
				slog.String("type", artifact.ArtifactType),
			)
			continue
		}

		// Log artifact being added
		slog.Info("DownloadBundle: adding artifact",
			slog.String("type", artifact.ArtifactType),
			slog.String("path", path),
			slog.Bool("is_plain_text", isPlainText),
			slog.Int("size_bytes", len(artifact.Content)),
		)

		// Get content - unwrap JSON string encoding for plain text files
		content := artifact.Content
		if isPlainText {
			content = unwrapJSONString(content)
		}

		// Add file to ZIP with proper permissions
		var fw io.Writer
		if isExecutable {
			// Use CreateHeader for executable files to set permissions
			header := &zip.FileHeader{
				Name:   path,
				Method: zip.Deflate,
			}
			header.SetMode(0755) // rwxr-xr-x
			var err error
			fw, err = zw.CreateHeader(header)
			if err != nil {
				slog.Error("failed to create zip entry", "path", path, "error", err)
				continue
			}
		} else {
			var err error
			fw, err = zw.Create(path)
			if err != nil {
				slog.Error("failed to create zip entry", "path", path, "error", err)
				continue
			}
		}
		if _, err := fw.Write(content); err != nil {
			slog.Error("failed to write zip entry", "path", path, "error", err)
			continue
		}
	}

	// For Nitro deployments, add a README to the certs directory
	if stackName == "nitro" {
		certReadme := `# PopSigner mTLS Certificates

These certificates are used by the Nitro sequencer for L1 transaction signing.

## Files Included

- client.crt  - Client certificate (PEM format)
- client.key  - Client private key (PEM format)
- ca.crt      - CA certificate (PEM format, if applicable)

## Security Notes

- Keep your private key (client.key) secure!
- Do not commit these files to public version control
- These certificates are used by Nitro for batch poster and staker transactions
- Certificates were automatically generated during deployment

## Usage

The docker-compose.yaml is already configured to mount these certificates:

    volumes:
      - ./certs:/certs:ro

Nitro uses them via:

    --node.batch-poster.data-poster.external-signer.client-cert=/certs/client.crt
    --node.batch-poster.data-poster.external-signer.client-private-key=/certs/client.key
`
		certReadmePath := bundlePrefix + "certs/README.md"
		fw, err := zw.Create(certReadmePath)
		if err != nil {
			slog.Error("failed to create cert readme", "path", certReadmePath, "error", err)
		} else if _, err := fw.Write([]byte(certReadme)); err != nil {
			slog.Error("failed to write cert readme", "path", certReadmePath, "error", err)
		}
	}

	if err := zw.Close(); err != nil {
		slog.Error("failed to close zip writer", "error", err)
		http.Error(w, "Failed to generate bundle", http.StatusInternalServerError)
		return
	}

	// Set headers for ZIP download
	filename := fmt.Sprintf("%s-%s-bundle.zip", safeName, stackName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Error("failed to write zip response", "error", err)
		return
	}

	slog.Info("bundle downloaded",
		"deployment_id", deploymentID,
		"chain_name", chainName,
		"size_bytes", buf.Len(),
	)
}

// DeploymentResume handles starting or resuming a pending/failed/paused deployment
func (h *Handler) DeploymentResume(w http.ResponseWriter, r *http.Request) {
	// CRIT-010: Get authenticated user's org for authorization
	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	deploymentID := chi.URLParam(r, "id")
	if deploymentID == "" {
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	deployID, err := uuid.Parse(deploymentID)
	if err != nil {
		slog.Error("invalid deployment ID", "id", deploymentID, "error", err)
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// Get deployment to verify it exists and can be resumed
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deployID)
	if err != nil {
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// CRIT-010: Verify deployment belongs to user's organization
	if deployment.OrgID != org.ID {
		http.Redirect(w, r, "/deployments", http.StatusFound)
		return
	}

	// Only start/resume pending, failed, or paused deployments
	if deployment.Status != "pending" && deployment.Status != "failed" && deployment.Status != "paused" {
		slog.Warn("cannot start/resume deployment", "id", deploymentID, "status", deployment.Status)
		http.Redirect(w, r, "/deployments/"+deploymentID, http.StatusFound)
		return
	}

	// For pending deployments, we don't need to update status - orchestrator will handle it
	// For failed/paused, reset to pending so orchestrator picks it up
	if deployment.Status != "pending" {
		if err := h.deployRepo.UpdateDeploymentStatus(r.Context(), deployID, "pending", nil); err != nil {
			slog.Error("failed to reset deployment status", "id", deploymentID, "error", err)
			http.Redirect(w, r, "/deployments/"+deploymentID, http.StatusFound)
			return
		}
	}

	// Clear any previous error message when resuming
	if err := h.deployRepo.ClearDeploymentError(r.Context(), deployID); err != nil {
		slog.Warn("failed to clear deployment error", "id", deploymentID, "error", err)
		// Continue anyway - not fatal
	}

	// Start deployment asynchronously
	go func(deployID uuid.UUID) {
		// Use a timeout context for deployment operations
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := h.orchestrator.StartDeployment(ctx, deployID); err != nil {
			slog.Error("failed to resume deployment", "deployment_id", deployID, "error", err)
			// Update deployment status on error
			if setErr := h.deployRepo.SetDeploymentError(context.Background(), deployID, err.Error()); setErr != nil {
				slog.Error("failed to set deployment error", "deployment_id", deployID, "error", setErr)
			}
		}
	}(deployID)

	slog.Info("deployment started", "id", deploymentID)
	
	// Redirect to status page to see progress
	http.Redirect(w, r, "/deployments/"+deploymentID+"/status", http.StatusFound)
}

// ============================================
// Helper Methods
// ============================================

// getUserAndOrg retrieves the current user and organization from the session.
// Uses the same session mechanism as the main dashboard (cookie + DB lookup).
func (h *Handler) getUserAndOrg(r *http.Request) (*models.User, *models.Organization, error) {
	// Get session cookie (same as main dashboard)
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, nil, errors.New("no session cookie")
	}

	// Look up session in database
	session, err := h.sessionRepo.Get(r.Context(), cookie.Value)
	if err != nil || session == nil {
		return nil, nil, errors.New("invalid session")
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, nil, errors.New("session expired")
	}

	// Get user from user repo
	user, err := h.userRepo.GetByID(r.Context(), session.UserID)
	if err != nil || user == nil {
		return nil, nil, errors.New("user not found")
	}

	// Get first org for user
	var org *models.Organization
	orgs, err := h.orgService.ListUserOrgs(r.Context(), session.UserID)
	if err == nil && len(orgs) > 0 {
		org = orgs[0]
	}

	if org == nil {
		return nil, nil, errors.New("no organization")
	}

	return user, org, nil
}

// handleAuthError handles authentication errors by redirecting to login.
func (h *Handler) handleAuthError(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func getUserName(user *models.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	// Return part before @ in email
	email := user.Email
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

func getAvatarURL(user *models.User) string {
	if user.AvatarURL != nil {
		return *user.AvatarURL
	}
	return ""
}

// ============================================
// Placeholder Components (temporary until other tasks)
// ============================================

func placeholderDeploymentsList() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`
		<div class="max-w-4xl mx-auto p-8">
			<div class="text-center py-16">
				<div class="text-6xl mb-6">🚀</div>
				<h2 class="text-2xl font-bold text-[#33FF00] mb-4 uppercase">MY CHAINS</h2>
				<p class="text-[#666600] mb-8">Your deployed chains will appear here.</p>
				<a href="/deployments/new" 
				   class="inline-block px-6 py-3 bg-[#33FF00] text-black font-bold uppercase hover:bg-[#44FF11] transition-colors">
					DEPLOY NEW CHAIN →
				</a>
			</div>
			
			<div class="border border-dashed border-[#333300] p-8 text-center text-[#666600]">
				<p class="text-sm uppercase">This page will be implemented in TASK-041</p>
			</div>
		</div>
		`))
		return err
	})
}

func placeholderDeploymentsNew() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`
		<div class="max-w-2xl mx-auto p-8">
			<h2 class="text-2xl font-bold text-[#33FF00] mb-6 uppercase">DEPLOY_NEW_CHAIN</h2>
			
			<div class="border border-dashed border-[#333300] p-8 text-center text-[#666600]">
				<p class="text-sm uppercase mb-4">Deployment wizard will be implemented in TASK-042</p>
				<a href="/deployments" 
				   class="text-[#FFB000] hover:underline uppercase">
					← BACK TO MY CHAINS
				</a>
			</div>
		</div>
		`))
		return err
	})
}


func placeholderDeploymentProgress() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`
		<div class="max-w-4xl mx-auto p-8">
			<div class="border border-dashed border-[#333300] p-8 text-center text-[#666600]">
				<p class="text-sm uppercase">Deployment progress page - TASK-044</p>
			</div>
		</div>
		`))
		return err
	})
}

func placeholderDeploymentComplete() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`
		<div class="max-w-4xl mx-auto p-8">
			<div class="text-center py-8">
				<div class="text-6xl mb-4">✅</div>
				<h2 class="text-2xl font-bold text-[#33FF00] mb-4 uppercase">DEPLOYMENT COMPLETE</h2>
				<p class="text-[#666600]">Your chain is ready to run.</p>
			</div>
		</div>
		`))
		return err
	})
}

// unwrapJSONString unwraps a JSON-encoded string back to plain text.
// unwrapJSONString unwraps content that was stored for JSONB column.
// Supports two formats:
// 1. NEW: base64 wrapper {"_type":"base64","data":"..."}
// 2. LEGACY: JSON string "content..." (with PostgreSQL normalization issues)
// Used for non-JSON artifacts (docker-compose.yml, .env.example, etc.) that were
// stored as JSON strings or base64 objects to satisfy the JSONB column requirement.
func unwrapJSONString(data []byte) []byte {
	// Try new base64 wrapper format first
	var wrapper struct {
		Type string `json:"_type"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Type == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(wrapper.Data)
		if err == nil {
			return decoded
		}
	}

	// Try legacy JSON string format
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return []byte(s)
	}

	// If it's not a recognized format, return as-is
	return data
}

// formatBytes converts bytes to a human-readable string (KB, MB, etc.)
func formatBytes(bytes int) string {
	const (
		KB = 1024
		MB = KB * 1024
	)

	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

