// Package popkins provides HTTP handlers for the POPKins chain deployment platform.
// POPKins is a SEPARATE product from the main POPSigner dashboard.
// - POPKins: popkins.popsigner.com - Chain deployment/orchestration
// - Main Dashboard: dashboard.popsigner.com - Key management, signing
package popkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/bundle"
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

// Handler handles POPKins-specific HTTP requests.
type Handler struct {
	authService service.AuthService
	orgService  service.OrgService
	keyService  service.KeyService
	deployRepo  repository.Repository
	bundler     *bundle.Bundler
	sessionRepo mainrepo.SessionRepository
	userRepo    mainrepo.UserRepository
}

// NewHandler creates a new POPKins handler.
func NewHandler(
	authService service.AuthService,
	orgService service.OrgService,
	keyService service.KeyService,
	deployRepo repository.Repository,
	bundler *bundle.Bundler,
	sessionRepo mainrepo.SessionRepository,
	userRepo mainrepo.UserRepository,
) *Handler {
	return &Handler{
		authService: authService,
		orgService:  orgService,
		keyService:  keyService,
		deployRepo:  deployRepo,
		bundler:     bundler,
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
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

	// Fetch all deployments from repository
	deployments, err := h.deployRepo.ListAllDeployments(r.Context())
	if err != nil {
		slog.Error("failed to list deployments", "error", err)
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
			ActivePath: "/popkins/deployments",
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

// DeploymentsNew renders the new deployment wizard form
func (h *Handler) DeploymentsNew(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	// Determine current step (1-4)
	step := 1
	if s := r.URL.Query().Get("step"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed >= 1 && parsed <= 4 {
			step = parsed
		}
	}

	// Get form data from POST request (multi-step form)
	formData := pages.DeploymentFormData{}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			formData = pages.DeploymentFormData{
				Stack:       r.FormValue("stack"),
				ChainName:   r.FormValue("chain_name"),
				ChainID:     r.FormValue("chain_id"),
				L1RPC:       r.FormValue("l1_rpc"),
				L1ChainID:   r.FormValue("l1_chain_id"),
				DA:          r.FormValue("da"),
				DeployerKey: r.FormValue("deployer_key"),
				BatcherKey:  r.FormValue("batcher_key"),
				ProposerKey: r.FormValue("proposer_key"),
			}
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
			ActivePath: "/popkins/deployments/new",
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
		http.Redirect(w, r, "/popkins/deployments/new?step=4&error=Invalid+form+data", http.StatusFound)
		return
	}

	// Parse chain ID
	chainID, err := strconv.ParseInt(r.FormValue("chain_id"), 10, 64)
	if err != nil || chainID <= 0 {
		http.Redirect(w, r, "/popkins/deployments/new?step=2&error=Invalid+chain+ID", http.StatusFound)
		return
	}

	// Validate required fields
	chainName := r.FormValue("chain_name")
	stack := r.FormValue("stack")
	l1RPC := r.FormValue("l1_rpc")

	if chainName == "" || stack == "" || l1RPC == "" {
		http.Redirect(w, r, "/popkins/deployments/new?step=2&error=Missing+required+fields", http.StatusFound)
		return
	}

	// Build deployment config
	config := map[string]interface{}{
		"chain_name":   chainName,
		"chain_id":     chainID,
		"l1_rpc":       l1RPC,
		"l1_chain_id":  r.FormValue("l1_chain_id"),
		"da":           r.FormValue("da"),
		"deployer_key": r.FormValue("deployer_key"),
		"batcher_key":  r.FormValue("batcher_key"),
		"proposer_key": r.FormValue("proposer_key"),
		"org_id":       org.ID.String(),
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		http.Redirect(w, r, "/popkins/deployments/new?step=4&error=Failed+to+create+deployment", http.StatusFound)
		return
	}

	// Create deployment record
	deployment := &repository.Deployment{
		ID:      uuid.New(),
		ChainID: chainID,
		Stack:   repository.Stack(stack),
		Status:  repository.StatusPending,
		Config:  configJSON,
	}

	if err := h.deployRepo.CreateDeployment(r.Context(), deployment); err != nil {
		slog.Error("failed to create deployment", "error", err)
		http.Redirect(w, r, "/popkins/deployments/new?step=4&error=Failed+to+create+deployment", http.StatusFound)
		return
	}

	slog.Info("deployment created",
		"deployment_id", deployment.ID,
		"chain_name", chainName,
		"chain_id", chainID,
		"stack", stack,
		"org_id", org.ID,
	)

	// Redirect to deployment status page
	http.Redirect(w, r, "/popkins/deployments/"+deployment.ID.String()+"/status", http.StatusFound)
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
			ActivePath: "/popkins/deployments",
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
		http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
		return
	}

	deploymentUUID, err := uuid.Parse(deploymentID)
	if err != nil {
		http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
		return
	}

	// Get deployment from repository
	deployment, err := h.deployRepo.GetDeployment(r.Context(), deploymentUUID)
	if err != nil {
		slog.Error("failed to get deployment", "id", deploymentID, "error", err)
		http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
		return
	}

	// If completed, redirect to complete page
	if deployment.Status == repository.StatusCompleted {
		http.Redirect(w, r, "/popkins/deployments/"+deploymentID+"/complete", http.StatusFound)
		return
	}

	// Build progress data
	data := h.buildProgressData(user, org, deployment)

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

	// Build progress data
	data := h.buildProgressData(user, org, deployment)

	// Render just the progress content (partial)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pages.DeploymentProgressPartial(data).Render(r.Context(), w)
}

// buildProgressData constructs the progress page data from a deployment
func (h *Handler) buildProgressData(user *models.User, org *models.Organization, deployment *repository.Deployment) pages.DeploymentProgressData {
	// Build stage list based on stack
	stages := h.buildStagesInfo(deployment)

	// Get latest transaction if available
	var latestTx *components.TxInfo
	txs, err := h.deployRepo.GetTransactionsByDeployment(context.Background(), deployment.ID)
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
			ActivePath: "/popkins/deployments",
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

	if deployment.Stack == repository.StackOPStack {
		// OP Stack stages
		stages = []components.StageInfo{
			{Name: "Initialize Configuration", Status: "pending", TxCount: 0},
			{Name: "Validate Settings", Status: "pending", TxCount: 0},
			{Name: "Deploy L1 Contracts", Status: "pending", TxCount: 35},
			{Name: "Generate Genesis", Status: "pending", TxCount: 0},
			{Name: "Configure Sequencer", Status: "pending", TxCount: 0},
			{Name: "Finalize Deployment", Status: "pending", TxCount: 0},
		}
	} else {
		// Nitro stages
		stages = []components.StageInfo{
			{Name: "Initialize Configuration", Status: "pending", TxCount: 0},
			{Name: "Validate Settings", Status: "pending", TxCount: 0},
			{Name: "Deploy Rollup Contracts", Status: "pending", TxCount: 1},
			{Name: "Configure Validator", Status: "pending", TxCount: 0},
			{Name: "Finalize Deployment", Status: "pending", TxCount: 0},
		}
	}

	// Update stages based on deployment status
	if deployment.Status == repository.StatusCompleted {
		// All stages complete
		for i := range stages {
			stages[i].Status = "completed"
			stages[i].TxComplete = stages[i].TxCount
		}
	} else if deployment.Status == repository.StatusFailed {
		// Mark stages up to current as complete, current as failed
		foundCurrent := false
		for i := range stages {
			if !foundCurrent {
				if stages[i].Name == currentStage {
					stages[i].Status = "failed"
					foundCurrent = true
				} else if currentStage == "" {
					// No current stage means first stage
					stages[0].Status = "failed"
					foundCurrent = true
				} else {
					stages[i].Status = "completed"
					stages[i].TxComplete = stages[i].TxCount
				}
			}
		}
	} else if deployment.Status == repository.StatusRunning {
		// Mark stages up to current as complete, current as in_progress
		foundCurrent := false
		for i := range stages {
			if !foundCurrent {
				if stages[i].Name == currentStage {
					stages[i].Status = "in_progress"
					stages[i].TxComplete = stages[i].TxCount / 2 // Estimate 50% for demo
					stages[i].Details = "Processing..."
					foundCurrent = true
				} else if currentStage == "" && i == 0 {
					stages[i].Status = "in_progress"
					foundCurrent = true
				} else {
					stages[i].Status = "completed"
					stages[i].TxComplete = stages[i].TxCount
				}
			}
		}
	} else if deployment.Status == repository.StatusPending {
		// First stage is pending/waiting
		stages[0].Status = "pending"
	}

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
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleAuthError(w, r)
		return
	}

	data := layouts.PopkinsData{
		UserName:   getUserName(user),
		UserEmail:  user.Email,
		AvatarURL:  getAvatarURL(user),
		OrgName:    org.Name,
		ActivePath: "/popkins/deployments",
	}

	// Render layout with placeholder content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layouts.PopkinsWithContent("Deployment Complete", data, placeholderDeploymentComplete()).Render(r.Context(), w)
}

// DownloadBundle handles artifact bundle downloads
func (h *Handler) DownloadBundle(w http.ResponseWriter, r *http.Request) {
	// Placeholder - redirects to API endpoint
	http.Error(w, "Bundle download not yet implemented", http.StatusNotImplemented)
}

// DeploymentResume handles resuming a paused deployment
func (h *Handler) DeploymentResume(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
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
				<a href="/popkins/deployments/new" 
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
				<a href="/popkins/deployments" 
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

