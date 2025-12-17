// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
	"github.com/Bidon15/popsigner/control-plane/templates/components"
	"github.com/Bidon15/popsigner/control-plane/templates/layouts"
	"github.com/Bidon15/popsigner/control-plane/templates/pages"
)

// ============================================
// Settings Page Handlers Implementation
// ============================================

// SettingsProfile renders the profile settings page.
func (h *WebHandler) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

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

	user, _, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	name := r.FormValue("name")

	req := service.UpdateProfileRequest{
		Name: &name,
	}
	_, err = h.authService.UpdateProfile(ctx, user.ID, req)
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Get team members
	members, _ := h.orgService.ListMembers(ctx, org.ID, user.ID)

	// Get current user's role
	currentRole := models.RoleViewer
	for _, m := range members {
		if m.UserID == user.ID {
			currentRole = m.Role
			break
		}
	}

	// Get pending invitations
	invitations, _ := h.orgService.ListPendingInvitations(ctx, org.ID, user.ID)

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
			IsCurrentUser: m.UserID == user.ID,
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	email := r.FormValue("email")
	role := models.Role(r.FormValue("role"))

	if !models.ValidRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	_ = user // Unused but kept for future audit logging
	_, err = h.orgService.InviteMember(ctx, org.ID, email, role, user.ID)
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = h.orgService.RemoveMember(ctx, org.ID, mid, user.ID)
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Get API keys for the organization
	apiKeys, _ := h.apiKeyService.List(ctx, org.ID)

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

	// Get user and org properly
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		errMsg := "Session expired. Please refresh and try again."
		if err == ErrNoOrganization {
			errMsg = "No organization found. Please complete onboarding first."
		}
		component := pages.APIKeyCreateError(errMsg)
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	name := r.FormValue("name")
	scopes := r.Form["scopes"]
	expires := r.FormValue("expires")

	// Validate required fields
	if name == "" {
		component := pages.APIKeyCreateError("Name is required")
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	if len(scopes) == 0 {
		component := pages.APIKeyCreateError("Please select at least one permission")
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

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
	apiKey, key, err := h.apiKeyService.Create(ctx, org.ID, req)
	_ = user // Unused but kept for future audit logging
	if err != nil {
		component := pages.APIKeyCreateError("Failed to create API key: " + err.Error())
		templ.Handler(component).ServeHTTP(w, r)
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	// Get member details
	members, _ := h.orgService.ListMembers(ctx, org.ID, user.ID)
	
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

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	role := models.Role(r.FormValue("role"))
	if !models.ValidRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	err = h.orgService.UpdateMemberRole(ctx, org.ID, mid, role, user.ID)
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
	// Verify user has access before showing modal
	_, _, err := h.getUserAndOrg(r)
	if err != nil {
		http.Error(w, "Session expired or no organization. Please refresh the page.", http.StatusUnauthorized)
		return
	}
	
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

	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = h.apiKeyService.Revoke(ctx, org.ID, kid)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to revoke API key", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Re-render the API keys list
	apiKeys, _ := h.apiKeyService.List(ctx, org.ID)
	component := pages.APIKeysList(apiKeys)
	templ.Handler(component).ServeHTTP(w, r)
}

// ============================================
// Certificate Settings Handlers
// ============================================

// SettingsCertificates renders the certificates settings page.
func (h *WebHandler) SettingsCertificates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Get certificates for the organization
	var certs []models.Certificate
	if h.certService != nil {
		certList, err := h.certService.List(ctx, org.ID.String(), repository.CertificateFilterAll)
		if err == nil && certList != nil {
			certs = certList.Certificates
		}
	}

	dashboardData := buildDashboardData(user, org, "/settings/certificates")

	data := pages.CertificatesPageData{
		DashboardData: dashboardData,
		Certificates:  certs,
		Total:         len(certs),
	}

	// Handle HTMX partial request for list refresh
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "certificates-list" {
		component := pages.CertificatesList(certs)
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	component := pages.CertificatesPage(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsCertificatesNewModal renders the create certificate modal.
func (h *WebHandler) SettingsCertificatesNewModal(w http.ResponseWriter, r *http.Request) {
	_, _, err := h.getUserAndOrg(r)
	if err != nil {
		http.Error(w, "Session expired or no organization. Please refresh the page.", http.StatusUnauthorized)
		return
	}

	component := pages.CreateCertificateModal()
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsCertificatesCreate handles certificate creation.
func (h *WebHandler) SettingsCertificatesCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		errMsg := "Session expired. Please refresh and try again."
		if err == ErrNoOrganization {
			errMsg = "No organization found. Please complete onboarding first."
		}
		component := pages.CertificateCreateError(errMsg)
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	if h.certService == nil {
		component := pages.CertificateCreateError("Certificate service not available")
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	name := r.FormValue("name")
	validityPeriod := r.FormValue("validity_period")

	if name == "" {
		component := pages.CertificateCreateError("Certificate name is required")
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	// Parse validity period
	duration := models.DefaultValidityPeriod
	if validityPeriod != "" {
		d, err := time.ParseDuration(validityPeriod)
		if err == nil {
			duration = d
		}
	}

	req := &models.CreateCertificateRequest{
		OrgID:          org.ID,
		Name:           name,
		ValidityPeriod: duration,
	}

	bundle, err := h.certService.Issue(ctx, req)
	if err != nil {
		component := pages.CertificateCreateError("Failed to generate certificate: " + err.Error())
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	// Return the download modal with the certificate bundle
	component := components.CertDownloadModal(bundle)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsCertificatesRevoke handles certificate revocation.
func (h *WebHandler) SettingsCertificatesRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	certID := chi.URLParam(r, "id")
	if certID == "" {
		http.Error(w, "Invalid certificate ID", http.StatusBadRequest)
		return
	}

	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if h.certService == nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Certificate service not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.certService.Revoke(ctx, org.ID.String(), certID, "User requested revocation")
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to revoke certificate", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Re-render the certificates list
	certList, _ := h.certService.List(ctx, org.ID.String(), repository.CertificateFilterAll)
	var certs []models.Certificate
	if certList != nil {
		certs = certList.Certificates
	}
	component := pages.CertificatesList(certs)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsCertificatesDelete handles certificate deletion.
func (h *WebHandler) SettingsCertificatesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	certID := chi.URLParam(r, "id")
	if certID == "" {
		http.Error(w, "Invalid certificate ID", http.StatusBadRequest)
		return
	}

	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Session expired", "type": "error"}}`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if h.certService == nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Certificate service not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.certService.Delete(ctx, org.ID.String(), certID)
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to delete certificate", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Re-render the certificates list
	certList, _ := h.certService.List(ctx, org.ID.String(), repository.CertificateFilterAll)
	var certs []models.Certificate
	if certList != nil {
		certs = certList.Certificates
	}
	component := pages.CertificatesList(certs)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsCertificatesDownloadCA serves the CA certificate for download.
func (h *WebHandler) SettingsCertificatesDownloadCA(w http.ResponseWriter, r *http.Request) {
	_, _, err := h.getUserAndOrg(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.certService == nil {
		http.Error(w, "Certificate service not available", http.StatusInternalServerError)
		return
	}

	caCert, err := h.certService.GetCACertificate(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve CA certificate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=popsigner-ca.crt")
	w.Write(caCert)
}

// SettingsCertificatesDownload serves a client certificate for download.
func (h *WebHandler) SettingsCertificatesDownload(w http.ResponseWriter, r *http.Request) {
	certID := chi.URLParam(r, "id")
	if certID == "" {
		http.Error(w, "Invalid certificate ID", http.StatusBadRequest)
		return
	}

	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if h.certService == nil {
		http.Error(w, "Certificate service not available", http.StatusInternalServerError)
		return
	}

	bundle, err := h.certService.DownloadBundle(r.Context(), org.ID.String(), certID)
	if err != nil {
		http.Error(w, "Certificate not found: "+err.Error(), http.StatusNotFound)
		return
	}

	if len(bundle.ClientCert) == 0 {
		http.Error(w, "Certificate data not available (legacy certificate)", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=client.crt")
	w.Write(bundle.ClientCert)
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

