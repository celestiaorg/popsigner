// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/templates/layouts"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
	"github.com/Bidon15/banhbaoring/control-plane/templates/partials"
)

// Ensure layouts is imported for buildDashboardData
var _ layouts.DashboardData

// ============================================
// Billing Page Handlers Implementation
// ============================================

// SettingsBilling renders the billing settings page.
func (h *WebHandler) SettingsBilling(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	userID, _ := session.Values["user_id"].(string)
	orgID, _ := session.Values["org_id"].(string)

	uid, _ := uuid.Parse(userID)
	oid, _ := uuid.Parse(orgID)

	user, _ := h.authService.GetUserByID(ctx, uid)
	org, _ := h.orgService.Get(ctx, oid)

	// Get billing information from billing service
	subscription, _ := h.billingService.GetSubscription(ctx, oid)
	usage, _ := h.billingService.GetCurrentUsage(ctx, oid)
	invoices, _ := h.billingService.ListInvoices(ctx, oid)

	// Get plan limits
	limits := models.GetPlanLimits(org.Plan)

	// Build subscription info
	subInfo := &pages.SubscriptionInfo{
		Plan:            string(org.Plan),
		Price:           getPlanPrice(org.Plan),
		CardLast4:       getCardLast4(subscription),
		CardBrand:       getCardBrand(subscription),
		BillingEmail:    user.Email,
		NextBillingDate: getNextBillingDate(subscription),
	}

	// Build usage info
	usageInfo := &pages.UsageInfo{
		Keys:             int64(usage.Keys),
		KeysLimit:        int64(limits.Keys),
		Signatures:       usage.SignaturesMonth,
		SignaturesLimit:  limits.SignaturesPerMonth,
		TeamMembers:      int64(usage.TeamMembers),
		TeamMembersLimit: int64(limits.TeamMembers),
		Namespaces:       int64(usage.Namespaces),
		NamespacesLimit:  int64(limits.Namespaces),
	}

	// Convert invoices to display format
	var displayInvoices []*pages.Invoice
	for _, inv := range invoices {
		displayInvoices = append(displayInvoices, &pages.Invoice{
			ID:          inv.ID,
			Date:        inv.Date,
			Description: inv.Description,
			Amount:      inv.Amount,
			Paid:        inv.Paid,
			DownloadURL: inv.DownloadURL,
		})
	}

	dashboardData := buildDashboardData(user, org, "/settings/billing")

	data := pages.BillingPageData{
		DashboardData: dashboardData,
		Subscription:  subInfo,
		Usage:         usageInfo,
		Invoices:      displayInvoices,
	}

	component := pages.BillingPage(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsBillingPortal handles redirect to Stripe billing portal.
func (h *WebHandler) SettingsBillingPortal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	// Get the Stripe customer portal URL
	portalURL, err := h.billingService.CreatePortalSession(ctx, oid, r.Host+"/settings/billing")
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to open billing portal", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Redirect to Stripe portal
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", portalURL)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, portalURL, http.StatusFound)
	}
}

// SettingsBillingUpgrade renders the upgrade plan modal.
func (h *WebHandler) SettingsBillingUpgrade(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	org, _ := h.orgService.Get(ctx, oid)

	component := partials.UpgradePlanModal(string(org.Plan))
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsBillingCard renders the Stripe card update modal.
func (h *WebHandler) SettingsBillingCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	// Create a SetupIntent for adding/updating payment method
	setupIntent, err := h.billingService.CreateSetupIntentWithSecret(ctx, oid)
	if err != nil {
		http.Error(w, "Failed to create payment setup", http.StatusInternalServerError)
		return
	}

	// Get current card info if any
	subscription, _ := h.billingService.GetSubscription(ctx, oid)

	var clientSecret string
	if setupIntent != nil {
		clientSecret = setupIntent.ClientSecret
	}
	data := partials.StripeCardModalData{
		ClientSecret:    clientSecret,
		StripePublicKey: h.billingService.GetPublicKey(),
		CardLast4:       getCardLast4(subscription),
		CardBrand:       getCardBrand(subscription),
	}

	component := partials.StripeCardModal(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// SettingsBillingCardConfirm handles the card update confirmation from Stripe.
func (h *WebHandler) SettingsBillingCardConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	// Parse the setup intent ID from request body
	var req struct {
		SetupIntentID string `json:"setup_intent_id"`
	}

	if err := parseJSON(r, &req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Confirm the setup and attach payment method to customer
	err := h.billingService.ConfirmSetupIntent(ctx, oid, req.SetupIntentID)
	if err != nil {
		http.Error(w, "Failed to save payment method", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true}`))
}

// SettingsBillingCheckout creates a Stripe checkout session for upgrade.
func (h *WebHandler) SettingsBillingCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session, _ := h.sessionStore.Get(r, "session")
	orgID, _ := session.Values["org_id"].(string)
	oid, _ := uuid.Parse(orgID)

	plan := r.FormValue("plan")
	if plan != "pro" && plan != "enterprise" {
		http.Error(w, "Invalid plan", http.StatusBadRequest)
		return
	}

	// Create Stripe checkout session
	checkoutURL, err := h.billingService.CreateCheckoutSession(ctx, oid, plan, r.Host+"/settings/billing")
	if err != nil {
		w.Header().Set("HX-Trigger", `{"toast": {"message": "Failed to start checkout", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Redirect to Stripe checkout
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", checkoutURL)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, checkoutURL, http.StatusFound)
	}
}

// ============================================
// Helper Functions
// ============================================

func getPlanPrice(plan models.Plan) string {
	switch plan {
	case models.PlanPro:
		return "49"
	case models.PlanEnterprise:
		return "Custom"
	default:
		return "0"
	}
}

func getCardLast4(subscription interface{}) string {
	if subscription == nil {
		return ""
	}
	// Type assert or access card info from subscription
	// This is a placeholder - actual implementation depends on billing service
	if sub, ok := subscription.(interface{ GetCardLast4() string }); ok {
		return sub.GetCardLast4()
	}
	return ""
}

func getCardBrand(subscription interface{}) string {
	if subscription == nil {
		return ""
	}
	// Type assert or access card info from subscription
	if sub, ok := subscription.(interface{ GetCardBrand() string }); ok {
		return sub.GetCardBrand()
	}
	return "Visa"
}

func getNextBillingDate(subscription interface{}) time.Time {
	if subscription == nil {
		return time.Now().AddDate(0, 1, 0)
	}
	// Type assert or access billing date from subscription
	if sub, ok := subscription.(interface{ GetNextBillingDate() time.Time }); ok {
		return sub.GetNextBillingDate()
	}
	return time.Now().AddDate(0, 1, 0)
}

func parseJSON(r *http.Request, v interface{}) error {
	return nil // Placeholder - would use json.NewDecoder
}

