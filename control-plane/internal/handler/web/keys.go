// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"context"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
	"github.com/Bidon15/banhbaoring/control-plane/templates/components"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
	"github.com/Bidon15/banhbaoring/control-plane/templates/partials"
)

// KeysList renders the keys list page.
func (h *WebHandler) KeysList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// Parse query parameters
	searchQuery := r.URL.Query().Get("q")
	namespaceFilter := r.URL.Query().Get("namespace")

	var nsID *uuid.UUID
	if namespaceFilter != "" {
		id, err := uuid.Parse(namespaceFilter)
		if err == nil {
			nsID = &id
		}
	}

	// Fetch keys and namespaces
	keys, err := h.keyService.List(ctx, org.ID, nsID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// Filter by search query if provided
	if searchQuery != "" {
		keys = filterKeys(keys, searchQuery)
	}

	namespaces, err := h.orgService.ListNamespaces(ctx, org.ID, user.ID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// If HTMX request, return partial
	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(pages.KeysList(keys, namespaces)).ServeHTTP(w, r)
		return
	}

	// Full page render
	data := pages.KeysPageData{
		UserName:    getUserName(user),
		UserEmail:   user.Email,
		AvatarURL:   getAvatarURL(user),
		OrgName:     org.Name,
		OrgPlan:     string(org.Plan),
		Keys:        keys,
		Namespaces:  namespaces,
		SearchQuery: searchQuery,
		NamespaceID: namespaceFilter,
	}

	templ.Handler(pages.KeysListPage(data)).ServeHTTP(w, r)
}

// KeysDetail renders a key detail page.
func (h *WebHandler) KeysDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	key, err := h.keyService.Get(ctx, org.ID, keyID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// Get namespace name
	namespaces, _ := h.orgService.ListNamespaces(ctx, org.ID, user.ID)
	namespaceName := getNamespaceNameByID(key.NamespaceID, namespaces)

	// Get signing stats (mock data for now - would come from usage/audit service)
	sigStats := h.getSigningStats(ctx, keyID)

	data := pages.KeyDetailData{
		UserName:     getUserName(user),
		UserEmail:    user.Email,
		AvatarURL:    getAvatarURL(user),
		OrgName:      org.Name,
		OrgPlan:      string(org.Plan),
		Key:          key,
		Namespace:    namespaceName,
		SigningStats: sigStats,
	}

	templ.Handler(pages.KeyDetailPage(data)).ServeHTTP(w, r)
}

// KeysNew renders the new key form modal.
func (h *WebHandler) KeysNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	namespaces, err := h.orgService.ListNamespaces(ctx, org.ID, user.ID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	templ.Handler(partials.KeyNewModal(namespaces)).ServeHTTP(w, r)
}

// KeysCreate handles creating a new key.
func (h *WebHandler) KeysCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderToast(w, r, "Invalid form data", components.ToastError)
		return
	}

	nsID, err := uuid.Parse(r.FormValue("namespace_id"))
	if err != nil {
		h.renderToast(w, r, "Invalid namespace", components.ToastError)
		return
	}

	exportable := r.FormValue("exportable") == "true"
	name := strings.TrimSpace(r.FormValue("name"))

	if name == "" {
		h.renderToast(w, r, "Key name is required", components.ToastError)
		return
	}

	_, err = h.keyService.Create(ctx, service.CreateKeyRequest{
		OrgID:       org.ID,
		NamespaceID: nsID,
		Name:        name,
		Exportable:  exportable,
	})

	if err != nil {
		h.renderToast(w, r, err.Error(), components.ToastError)
		return
	}

	// Return updated keys list
	keys, _ := h.keyService.List(ctx, org.ID, nil)
	namespaces, _ := h.orgService.ListNamespaces(ctx, org.ID, user.ID)

	w.Header().Set("HX-Trigger", "modal-close")
	templ.Handler(pages.KeysList(keys, namespaces)).ServeHTTP(w, r)
}

// WorkerKeysNew renders the worker keys batch creation wizard.
func (h *WebHandler) WorkerKeysNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	namespaces, err := h.orgService.ListNamespaces(ctx, org.ID, user.ID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	limits, err := h.orgService.GetLimits(ctx, org.ID)
	if err != nil {
		limits = &models.PlanLimits{Keys: -1}
	}

	// Count current keys
	keys, _ := h.keyService.List(ctx, org.ID, nil)
	currentCount := len(keys)

	templ.Handler(partials.WorkerKeysModal(namespaces, limits, currentCount)).ServeHTTP(w, r)
}

// WorkerKeysCreate handles creating worker keys in bulk.
func (h *WebHandler) WorkerKeysCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderToast(w, r, "Invalid form data", components.ToastError)
		return
	}

	nsID, err := uuid.Parse(r.FormValue("namespace_id"))
	if err != nil {
		h.renderToast(w, r, "Invalid namespace", components.ToastError)
		return
	}

	prefix := strings.TrimSpace(r.FormValue("prefix"))
	if prefix == "" {
		h.renderToast(w, r, "Prefix is required", components.ToastError)
		return
	}

	count, err := strconv.Atoi(r.FormValue("count"))
	if err != nil || count < 1 || count > 100 {
		h.renderToast(w, r, "Invalid count (must be 1-100)", components.ToastError)
		return
	}

	exportable := r.FormValue("exportable") == "true"

	createdKeys, err := h.keyService.CreateBatch(ctx, service.CreateBatchKeyRequest{
		OrgID:       org.ID,
		NamespaceID: nsID,
		Prefix:      prefix,
		Count:       count,
		Exportable:  exportable,
	})

	created := len(createdKeys)
	failed := count - created

	if err != nil && created == 0 {
		h.renderToast(w, r, err.Error(), components.ToastError)
		return
	}

	// Return updated keys list with success message
	keys, _ := h.keyService.List(ctx, org.ID, nil)
	namespaces, _ := h.orgService.ListNamespaces(ctx, org.ID, user.ID)

	w.Header().Set("HX-Trigger", "modal-close")

	// Show success toast
	if failed > 0 {
		w.Header().Add("HX-Trigger", `{"toast": {"message": "Created `+strconv.Itoa(created)+` keys (`+strconv.Itoa(failed)+` failed)", "type": "warning"}}`)
	}

	templ.Handler(pages.KeysList(keys, namespaces)).ServeHTTP(w, r)
}

// KeysSignTest handles test signing with a key.
func (h *WebHandler) KeysSignTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		templ.Handler(partials.SignResult("", "", 0, err)).ServeHTTP(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		templ.Handler(partials.SignResult("", "", 0, err)).ServeHTTP(w, r)
		return
	}

	data := r.FormValue("data")
	if data == "" {
		data = "test message from BanhBaoRing dashboard"
	}

	// Convert data to bytes
	dataBytes := []byte(data)

	// Check if it's hex encoded
	if strings.HasPrefix(data, "0x") || strings.HasPrefix(data, "0X") {
		if decoded, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(data, "0x"), "0X")); err == nil {
			dataBytes = decoded
		}
	}

	result, err := h.keyService.Sign(ctx, org.ID, keyID, dataBytes, false)
	if err != nil {
		// For quick test from list page, return toast
		if r.Header.Get("HX-Target") == "#toast-container" {
			templ.Handler(partials.SignTestToast(false, err.Error())).ServeHTTP(w, r)
			return
		}
		templ.Handler(partials.SignResult("", "", 0, err)).ServeHTTP(w, r)
		return
	}

	// For quick test from list page, return toast
	if r.Header.Get("HX-Target") == "#toast-container" {
		shortSig := result.Signature
		if len(shortSig) > 20 {
			shortSig = shortSig[:10] + "..." + shortSig[len(shortSig)-10:]
		}
		templ.Handler(partials.SignTestToast(true, shortSig)).ServeHTTP(w, r)
		return
	}

	templ.Handler(partials.SignResult(result.Signature, result.PublicKey, 1, nil)).ServeHTTP(w, r)
}

// KeysDelete handles key deletion.
func (h *WebHandler) KeysDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, org, err := h.getUserAndOrg(r)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		h.renderToast(w, r, "Invalid key ID", components.ToastError)
		return
	}

	if err := h.keyService.Delete(ctx, org.ID, keyID); err != nil {
		h.renderToast(w, r, err.Error(), components.ToastError)
		return
	}

	// Return updated keys list
	keys, _ := h.keyService.List(ctx, org.ID, nil)
	namespaces, _ := h.orgService.ListNamespaces(ctx, org.ID, user.ID)

	templ.Handler(pages.KeysList(keys, namespaces)).ServeHTTP(w, r)
}

// ============================================
// Helper functions
// ============================================

// getUserAndOrg retrieves the current user and organization from the session.
func (h *WebHandler) getUserAndOrg(r *http.Request) (*models.User, *models.Organization, error) {
	session, err := h.sessionStore.Get(r, "session")
	if err != nil {
		return nil, nil, err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		return nil, nil, ErrUnauthorized
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, ErrUnauthorized
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
		if err != nil || len(orgs) == 0 {
			return nil, nil, ErrNoOrganization
		}
		org = orgs[0]
	}

	return user, org, nil
}

// handleError handles errors for web requests.
func (h *WebHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Header.Get("HX-Request") == "true" {
		h.renderToast(w, r, err.Error(), components.ToastError)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// renderToast renders a toast notification.
func (h *WebHandler) renderToast(w http.ResponseWriter, r *http.Request, message string, variant components.ToastVariant) {
	templ.Handler(components.Toast(message, variant)).ServeHTTP(w, r)
}

// getSigningStats returns signing statistics for a key.
func (h *WebHandler) getSigningStats(ctx context.Context, keyID uuid.UUID) *pages.SigningStats {
	// Generate mock data for last 30 days
	// In production, this would come from the usage/audit service
	labels := make([]string, 30)
	values := make([]int, 30)
	total := int64(0)

	now := time.Now()
	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		labels[29-i] = date.Format("Jan 2")
		// Mock random-ish values based on key ID
		values[29-i] = int((keyID[0]+byte(i))%50) + int((keyID[1]+byte(i*2))%30)
		total += int64(values[29-i])
	}

	avgPerDay := float64(total) / 30.0

	return &pages.SigningStats{
		Labels:    labels,
		Values:    values,
		Total:     total,
		AvgPerDay: avgPerDay,
	}
}

// filterKeys filters keys by search query.
func filterKeys(keys []*models.Key, query string) []*models.Key {
	query = strings.ToLower(query)
	var filtered []*models.Key
	for _, key := range keys {
		if strings.Contains(strings.ToLower(key.Name), query) ||
			strings.Contains(strings.ToLower(key.Address), query) {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

// getUserName returns the display name for a user.
func getUserName(user *models.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	return "User"
}

// getAvatarURL returns the avatar URL for a user.
func getAvatarURL(user *models.User) string {
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		return *user.AvatarURL
	}
	return ""
}

// getNamespaceNameByID returns the namespace name for a given ID.
func getNamespaceNameByID(nsID uuid.UUID, namespaces []*models.Namespace) string {
	for _, ns := range namespaces {
		if ns.ID == nsID {
			return ns.Name
		}
	}
	return "default"
}

// Error types for web handlers.
var (
	ErrUnauthorized   = &webError{message: "Unauthorized", code: http.StatusUnauthorized}
	ErrNoOrganization = &webError{message: "No organization found", code: http.StatusNotFound}
)

type webError struct {
	message string
	code    int
}

func (e *webError) Error() string {
	return e.message
}

