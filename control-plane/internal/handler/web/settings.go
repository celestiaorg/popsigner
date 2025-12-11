// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
	"github.com/Bidon15/banhbaoring/control-plane/templates/layouts"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
)

// ============================================
// Settings Page Handlers Implementation
// ============================================

// SettingsProfile renders the profile settings page.
func (h *WebHandler) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get session data
	session, err := h.sessionStore.Get(r, "session")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	// Get user data
	user, err := h.authService.GetUserByID(ctx, uid)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get organization data
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)
	org, _ := h.orgService.Get(ctx, oid)

	// Build dashboard data
	dashboardData := buildDashboardData(user, org, "/settings/profile")

	// Build profile page data
	data := pages.ProfilePageData{
		DashboardData: dashboardData,
		Email:         user.Email,
		Name:          safeString(user.Name),
		AvatarURL:     safeString(user.AvatarURL),
		EmailVerified: user.EmailVerified,
		OAuthProvider: safeString(user.OAuthProvider),
	}

	// Render based on request type
	if r.Header.Get("HX-Request") == "true" {
		// HTMX partial update
		component := pages.SettingsProfilePage(data)
		templ.Handler(component).ServeHTTP(w, r)
	} else {
		component := pages.SettingsProfilePage(data)
		templ.Handler(component).ServeHTTP(w, r)
	}
}

// SettingsProfileUpdate handles profile update.
func (h *WebHandler) SettingsProfileUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	uid, _ := uuid.Parse(userID)

	name := r.FormValue("name")
	email := r.FormValue("email")

	req := service.UpdateProfileRequest{
		Name: &name,
	}
	_, err := h.authService.UpdateProfile(ctx, uid, req)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to update profile", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Trigger", `{"toast": {"message": "Profile updated successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// SettingsTeam renders the team settings page.
func (h *WebHandler) SettingsTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	orgID, _ := session.Values["org_id"].(string)

	uid, _ := uuid.Parse(userID)
	oid, _ := uuid.Parse(orgID)

	// Get user and org
	user, _ := h.authService.GetUserByID(ctx, uid)
	org, _ := h.orgService.Get(ctx, oid)

	// Get team members
	members, _ := h.orgService.ListMembers(ctx, oid, uid)

	// Get current user's role
	currentRole := models.RoleViewer
	for _, m := range members {
		if m.UserID == uid {
			currentRole = m.Role
			break
		}
	}

	// Get pending invitations
	invitations, _ := h.orgService.ListPendingInvitations(ctx, oid, uid)

	// Get plan limits
	limits := models.GetPlanLimits(org.Plan)

	// Build display data
	var displayMembers []*pages.TeamMemberDisplay
	for _, m := range members {
		dm := &pages.TeamMemberDisplay{
			ID:            m.UserID,
			Name:          safeString(m.User.Name),
			Email:         m.User.Email,
			AvatarURL:     safeString(m.User.AvatarURL),
			Role:          m.Role,
			JoinedAt:      m.JoinedAt.Format("Jan 2, 2006"),
			IsCurrentUser: m.UserID == uid,
		}
		displayMembers = append(displayMembers, dm)
	}

	dashboardData := buildDashboardData(user, org, "/settings/team")

	data := pages.TeamPageData{
		DashboardData: dashboardData,
		Members:       displayMembers,
		Invitations:   invitations,
		CurrentRole:   currentRole,
		MemberLimit:   limits.TeamMembers,
		MemberCount:   len(members),
	}

	component := pages.SettingsTeamPage(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsTeamInvite handles team member invitation.
func (h *WebHandler) SettingsTeamInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	orgID, _ := session.Values["org_id"].(string)

	uid, _ := uuid.Parse(userID)
	oid, _ := uuid.Parse(orgID)

	email := r.FormValue("email")
	role := models.Role(r.FormValue("role"))

	if !models.ValidRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	_, err := h.orgService.InviteMember(ctx, oid, email, role, uid)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to send invitation", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Trigger", `{"toast": {"message": "Invitation sent successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// SettingsTeamRemove handles team member removal.
func (h *WebHandler) SettingsTeamRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	memberID := chi.URLParam(r, "id")
	mid, err := uuid.Parse(memberID)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	uid, _ := uuid.Parse(session.Values["user_id"].(string))
	err = h.orgService.RemoveMember(ctx, oid, mid, uid)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to remove member", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Trigger", `{"toast": {"message": "Member removed successfully", "type": "success"}}`)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// SettingsAPIKeys renders the API keys settings page.
func (h *WebHandler) SettingsAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	orgID, _ := session.Values["org_id"].(string)

	uid, _ := uuid.Parse(userID)
	oid, _ := uuid.Parse(orgID)

	user, _ := h.authService.GetUserByID(ctx, uid)
	org, _ := h.orgService.Get(ctx, oid)

	// Get API keys for the organization
	apiKeys, _ := h.apiKeyService.List(ctx, oid)

	// Check if user can create API keys
	canCreate := true // Could be based on role

	dashboardData := buildDashboardData(user, org, "/settings/api-keys")

	data := pages.APIKeysPageData{
		DashboardData: dashboardData,
		APIKeys:       apiKeys,
		CanCreate:     canCreate,
	}

	component := pages.SettingsAPIKeysPage(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsAPIKeysCreate handles API key creation.
func (h *WebHandler) SettingsAPIKeysCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	orgID, _ := session.Values["org_id"].(string)

	uid, _ := uuid.Parse(userID)
	oid, _ := uuid.Parse(orgID)

	name := r.FormValue("name")
	scopes := r.Form["scopes"]
	expires := r.FormValue("expires")

	var expiresAt *time.Time
	if expires != "" {
		var duration time.Duration
		switch expires {
		case "30d":
			duration = 30 * 24 * time.Hour
		case "90d":
			duration = 90 * 24 * time.Hour
		case "1y":
			duration = 365 * 24 * time.Hour
		}
		if duration > 0 {
			t := time.Now().Add(duration)
			expiresAt = &t
		}
	}

	req := service.CreateAPIKeyRequest{
		Name:   name,
		Scopes: scopes,
	}
	if expiresAt != nil {
		days := int(time.Until(*expiresAt).Hours() / 24)
		req.ExpiresInDays = &days
	}
	apiKey, key, err := h.apiKeyService.Create(ctx, oid, req)
	_ = uid // Unused but kept for future audit logging
	if err != nil {
		http.Error(w, "Failed to create API key", http.StatusInternalServerError)
		return
	}

	// Return the created key result partial
	component := pages.APIKeyCreatedResult(apiKey.Name, key)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsTeamInviteModal renders the invite member modal.
func (h *WebHandler) SettingsTeamInviteModal(w http.ResponseWriter, r *http.Request) {
	component := pages.InviteMemberModal()
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsTeamEditModal renders the edit member role modal.
func (h *WebHandler) SettingsTeamEditModal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	memberID := chi.URLParam(r, "id")
	mid, err := uuid.Parse(memberID)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	// Get member details
	members, _ := h.orgService.ListMembers(ctx, oid, uid)
	
	var member *pages.TeamMemberDisplay
	for _, m := range members {
		if m.UserID == mid {
			member = &pages.TeamMemberDisplay{
				ID:        m.UserID,
				Name:      safeString(m.User.Name),
				Email:     m.User.Email,
				AvatarURL: safeString(m.User.AvatarURL),
				Role:      m.Role,
				JoinedAt:  m.JoinedAt.Format("Jan 2, 2006"),
			}
			break
		}
	}

	if member == nil {
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	component := pages.EditMemberRoleModal(member)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsTeamUpdate handles team member role update.
func (h *WebHandler) SettingsTeamUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	memberID := chi.URLParam(r, "id")
	mid, err := uuid.Parse(memberID)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	role := models.Role(r.FormValue("role"))
	if !models.ValidRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	uid, _ := uuid.Parse(session.Values["user_id"].(string))
	err = h.orgService.UpdateMemberRole(ctx, oid, mid, role, uid)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to update role", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Trigger", `{"toast": {"message": "Role updated successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// SettingsAPIKeysNewModal renders the create API key modal.
func (h *WebHandler) SettingsAPIKeysNewModal(w http.ResponseWriter, r *http.Request) {
	component := pages.CreateAPIKeyModal()
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsAPIKeysDelete handles API key deletion/revocation.
func (h *WebHandler) SettingsAPIKeysDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	keyID := chi.URLParam(r, "id")
	kid, err := uuid.Parse(keyID)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	err = h.apiKeyService.Revoke(ctx, oid, kid)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to revoke API key", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Re-render the API keys list
	apiKeys, _ := h.apiKeyService.List(ctx, oid)
	component := pages.APIKeysList(apiKeys)
	templ.Handler(component).ServeHTTP(w, r)
}

// ============================================
// Helper Functions
// ============================================

// buildDashboardData creates the DashboardData from user and org.
func buildDashboardData(user *models.User, org *models.Organization, activePath string) layouts.DashboardData {
	return layouts.DashboardData{
		UserName:   safeString(user.Name),
		UserEmail:  user.Email,
		AvatarURL:  safeString(user.AvatarURL),
		OrgName:    safeOrgName(org),
		OrgPlan:    safeOrgPlan(org),
		ActivePath: activePath,
	}
}

// safeString returns the string value or empty string if nil.
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// safeOrgName returns the org name or default.
func safeOrgName(org *models.Organization) string {
	if org == nil {
		return "Personal"
	}
	return org.Name
}

// safeOrgPlan returns the org plan or default.
func safeOrgPlan(org *models.Organization) string {
	if org == nil {
		return "free"
	}
	return string(org.Plan)
}

