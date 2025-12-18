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

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/bundle"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
	"github.com/Bidon15/popsigner/control-plane/templates/layouts"
	"github.com/Bidon15/popsigner/control-plane/templates/pages"
)

// Session constants (shared with main web handler)
const (
	SessionCookieName = "banhbao_session"
)

// Handler handles POPKins-specific HTTP requests.
type Handler struct {
	authService  service.AuthService
	orgService   service.OrgService
	deployRepo   repository.Repository
	bundler      *bundle.Bundler
	sessionStore sessions.Store
}

// NewHandler creates a new POPKins handler.
func NewHandler(
	authService service.AuthService,
	orgService service.OrgService,
	deployRepo repository.Repository,
	bundler *bundle.Bundler,
	sessionStore sessions.Store,
) *Handler {
	return &Handler{
		authService:  authService,
		orgService:   orgService,
		deployRepo:   deployRepo,
		bundler:      bundler,
		sessionStore: sessionStore,
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

// DeploymentsNew renders the new deployment form (TASK-042)
func (h *Handler) DeploymentsNew(w http.ResponseWriter, r *http.Request) {
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
		ActivePath: "/popkins/deployments/new",
	}

	// Render layout with placeholder content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layouts.PopkinsWithContent("Deploy New Chain", data, placeholderDeploymentsNew()).Render(r.Context(), w)
}

// DeploymentsCreate handles the deployment creation form submission (TASK-042)
func (h *Handler) DeploymentsCreate(w http.ResponseWriter, r *http.Request) {
	// Placeholder until TASK-042 is implemented
	http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
}

// DeploymentDetail renders a specific deployment detail page (TASK-043)
func (h *Handler) DeploymentDetail(w http.ResponseWriter, r *http.Request) {
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
	layouts.PopkinsWithContent("Deployment Detail", data, placeholderDeploymentDetail()).Render(r.Context(), w)
}

// DeploymentStatus renders the deployment status/progress page (TASK-044)
func (h *Handler) DeploymentStatus(w http.ResponseWriter, r *http.Request) {
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
	layouts.PopkinsWithContent("Deployment Progress", data, placeholderDeploymentProgress()).Render(r.Context(), w)
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
func (h *Handler) getUserAndOrg(r *http.Request) (*models.User, *models.Organization, error) {
	session, err := h.sessionStore.Get(r, SessionCookieName)
	if err != nil {
		return nil, nil, err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		return nil, nil, errors.New("unauthorized")
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, errors.New("unauthorized")
	}

	// Get user from auth service
	user, err := h.authService.GetUserByID(r.Context(), uid)
	if err != nil {
		return nil, nil, err
	}

	// Get current org from session or first org
	orgID, ok := session.Values["org_id"].(string)
	var org *models.Organization

	if ok {
		oid, err := uuid.Parse(orgID)
		if err == nil {
			org, _ = h.orgService.Get(r.Context(), oid)
		}
	}

	if org == nil {
		// Get first org for user
		orgs, err := h.orgService.ListUserOrgs(r.Context(), uid)
		if err == nil && len(orgs) > 0 {
			org = orgs[0]
		}
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

func placeholderDeploymentDetail() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`
		<div class="max-w-4xl mx-auto p-8">
			<div class="border border-dashed border-[#333300] p-8 text-center text-[#666600]">
				<p class="text-sm uppercase">Deployment detail page - TASK-043</p>
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

