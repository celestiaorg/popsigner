package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"

	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// mockBillingService is a mock implementation of BillingService for testing.
type mockBillingService struct {
	createCustomerFunc              func(ctx context.Context, orgID uuid.UUID, email, name string) error
	getCustomerFunc                 func(ctx context.Context, orgID uuid.UUID) (*stripe.Customer, error)
	getSubscriptionFunc             func(ctx context.Context, orgID uuid.UUID) (*service.SubscriptionInfo, error)
	createSubscriptionFunc          func(ctx context.Context, orgID uuid.UUID, priceID string) (*service.SubscriptionInfo, error)
	changePlanFunc                  func(ctx context.Context, orgID uuid.UUID, newPriceID string) (*service.SubscriptionInfo, error)
	cancelSubscriptionFunc          func(ctx context.Context, orgID uuid.UUID) error
	reactivateSubscriptionFunc      func(ctx context.Context, orgID uuid.UUID) error
	createSetupIntentFunc           func(ctx context.Context, orgID uuid.UUID) (string, error)
	confirmSetupIntentFunc          func(ctx context.Context, orgID uuid.UUID, setupIntentID string) error
	listPaymentMethodsFunc          func(ctx context.Context, orgID uuid.UUID) ([]*stripe.PaymentMethod, error)
	setDefaultPaymentMethodFunc     func(ctx context.Context, orgID uuid.UUID, paymentMethodID string) error
	listInvoicesFunc                func(ctx context.Context, orgID uuid.UUID) ([]*stripe.Invoice, error)
	getCurrentUsageFunc             func(ctx context.Context, orgID uuid.UUID) (*service.UsageInfo, error)
	reportUsageFunc                 func(ctx context.Context, orgID uuid.UUID, metric string, quantity int64) error
	handleWebhookFunc               func(ctx context.Context, payload []byte, signature string) error
	createPortalSessionFunc         func(ctx context.Context, orgID uuid.UUID, returnURL string) (string, error)
	createCheckoutSessionFunc       func(ctx context.Context, orgID uuid.UUID, plan, returnURL string) (string, error)
	getPublicKeyFunc                func() string
	createSetupIntentWithSecretFunc func(ctx context.Context, orgID uuid.UUID) (*service.SetupIntentInfo, error)
	getSignaturesTimeSeriesFunc     func(ctx context.Context, orgID uuid.UUID, start, end time.Time) ([]service.TimeSeriesPoint, error)
	getAPICallsTimeSeriesFunc       func(ctx context.Context, orgID uuid.UUID, start, end time.Time) ([]service.TimeSeriesPoint, error)
}

func (m *mockBillingService) CreateCustomer(ctx context.Context, orgID uuid.UUID, email, name string) error {
	if m.createCustomerFunc != nil {
		return m.createCustomerFunc(ctx, orgID, email, name)
	}
	return nil
}

func (m *mockBillingService) GetCustomer(ctx context.Context, orgID uuid.UUID) (*stripe.Customer, error) {
	if m.getCustomerFunc != nil {
		return m.getCustomerFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) GetSubscription(ctx context.Context, orgID uuid.UUID) (*service.SubscriptionInfo, error) {
	if m.getSubscriptionFunc != nil {
		return m.getSubscriptionFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) CreateSubscription(ctx context.Context, orgID uuid.UUID, priceID string) (*service.SubscriptionInfo, error) {
	if m.createSubscriptionFunc != nil {
		return m.createSubscriptionFunc(ctx, orgID, priceID)
	}
	return nil, nil
}

func (m *mockBillingService) ChangePlan(ctx context.Context, orgID uuid.UUID, newPriceID string) (*service.SubscriptionInfo, error) {
	if m.changePlanFunc != nil {
		return m.changePlanFunc(ctx, orgID, newPriceID)
	}
	return nil, nil
}

func (m *mockBillingService) CancelSubscription(ctx context.Context, orgID uuid.UUID) error {
	if m.cancelSubscriptionFunc != nil {
		return m.cancelSubscriptionFunc(ctx, orgID)
	}
	return nil
}

func (m *mockBillingService) ReactivateSubscription(ctx context.Context, orgID uuid.UUID) error {
	if m.reactivateSubscriptionFunc != nil {
		return m.reactivateSubscriptionFunc(ctx, orgID)
	}
	return nil
}

func (m *mockBillingService) CreateSetupIntent(ctx context.Context, orgID uuid.UUID) (string, error) {
	if m.createSetupIntentFunc != nil {
		return m.createSetupIntentFunc(ctx, orgID)
	}
	return "", nil
}

func (m *mockBillingService) ConfirmSetupIntent(ctx context.Context, orgID uuid.UUID, setupIntentID string) error {
	if m.confirmSetupIntentFunc != nil {
		return m.confirmSetupIntentFunc(ctx, orgID, setupIntentID)
	}
	return nil
}

func (m *mockBillingService) ListPaymentMethods(ctx context.Context, orgID uuid.UUID) ([]*stripe.PaymentMethod, error) {
	if m.listPaymentMethodsFunc != nil {
		return m.listPaymentMethodsFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) SetDefaultPaymentMethod(ctx context.Context, orgID uuid.UUID, paymentMethodID string) error {
	if m.setDefaultPaymentMethodFunc != nil {
		return m.setDefaultPaymentMethodFunc(ctx, orgID, paymentMethodID)
	}
	return nil
}

func (m *mockBillingService) ListInvoices(ctx context.Context, orgID uuid.UUID) ([]*stripe.Invoice, error) {
	if m.listInvoicesFunc != nil {
		return m.listInvoicesFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) GetCurrentUsage(ctx context.Context, orgID uuid.UUID) (*service.UsageInfo, error) {
	if m.getCurrentUsageFunc != nil {
		return m.getCurrentUsageFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) ReportUsage(ctx context.Context, orgID uuid.UUID, metric string, quantity int64) error {
	if m.reportUsageFunc != nil {
		return m.reportUsageFunc(ctx, orgID, metric, quantity)
	}
	return nil
}

func (m *mockBillingService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if m.handleWebhookFunc != nil {
		return m.handleWebhookFunc(ctx, payload, signature)
	}
	return nil
}

func (m *mockBillingService) CreatePortalSession(ctx context.Context, orgID uuid.UUID, returnURL string) (string, error) {
	if m.createPortalSessionFunc != nil {
		return m.createPortalSessionFunc(ctx, orgID, returnURL)
	}
	return "", nil
}

func (m *mockBillingService) CreateCheckoutSession(ctx context.Context, orgID uuid.UUID, plan, returnURL string) (string, error) {
	if m.createCheckoutSessionFunc != nil {
		return m.createCheckoutSessionFunc(ctx, orgID, plan, returnURL)
	}
	return "", nil
}

func (m *mockBillingService) GetPublicKey() string {
	if m.getPublicKeyFunc != nil {
		return m.getPublicKeyFunc()
	}
	return ""
}

func (m *mockBillingService) CreateSetupIntentWithSecret(ctx context.Context, orgID uuid.UUID) (*service.SetupIntentInfo, error) {
	if m.createSetupIntentWithSecretFunc != nil {
		return m.createSetupIntentWithSecretFunc(ctx, orgID)
	}
	return nil, nil
}

func (m *mockBillingService) GetSignaturesTimeSeries(ctx context.Context, orgID uuid.UUID, start, end time.Time) ([]service.TimeSeriesPoint, error) {
	if m.getSignaturesTimeSeriesFunc != nil {
		return m.getSignaturesTimeSeriesFunc(ctx, orgID, start, end)
	}
	return nil, nil
}

func (m *mockBillingService) GetAPICallsTimeSeries(ctx context.Context, orgID uuid.UUID, start, end time.Time) ([]service.TimeSeriesPoint, error) {
	if m.getAPICallsTimeSeriesFunc != nil {
		return m.getAPICallsTimeSeriesFunc(ctx, orgID, start, end)
	}
	return nil, nil
}

// createBillingTestRequest creates a request with org ID in context
func createBillingTestRequest(t *testing.T, method, path string, body interface{}, orgID uuid.UUID) *http.Request {
	t.Helper()

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Add org ID to context
	ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID.String())
	return req.WithContext(ctx)
}

func TestBillingHandler_GetSubscription(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "returns subscription successfully",
			mockService: &mockBillingService{
				getSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) (*service.SubscriptionInfo, error) {
					return &service.SubscriptionInfo{
						ID:                 "sub_test123",
						Plan:               "pro",
						Status:             "active",
						CurrentPeriodStart: time.Now(),
						CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data service.SubscriptionInfo `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Data.Plan != "pro" {
					t.Errorf("Plan = %v, want 'pro'", resp.Data.Plan)
				}
			},
		},
		{
			name: "returns free plan for no subscription",
			mockService: &mockBillingService{
				getSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) (*service.SubscriptionInfo, error) {
					return &service.SubscriptionInfo{
						Plan:   "free",
						Status: "active",
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "returns 404 for org not found",
			mockService: &mockBillingService{
				getSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) (*service.SubscriptionInfo, error) {
					return nil, apierrors.NewNotFoundError("Organization")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodGet, "/v1/billing/subscription", nil, orgID)
			rec := httptest.NewRecorder()
			handler.GetSubscription(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestBillingHandler_CreateSubscription(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name: "creates subscription successfully",
			body: CreateSubscriptionRequest{
				PriceID: "price_pro_monthly",
			},
			mockService: &mockBillingService{
				createSubscriptionFunc: func(ctx context.Context, oID uuid.UUID, priceID string) (*service.SubscriptionInfo, error) {
					return &service.SubscriptionInfo{
						ID:     "sub_test123",
						Plan:   "pro",
						Status: "incomplete",
					}, nil
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "rejects missing price_id",
			body:           CreateSubscriptionRequest{},
			mockService:    &mockBillingService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "returns 409 when subscription already exists",
			body: CreateSubscriptionRequest{
				PriceID: "price_pro_monthly",
			},
			mockService: &mockBillingService{
				createSubscriptionFunc: func(ctx context.Context, oID uuid.UUID, priceID string) (*service.SubscriptionInfo, error) {
					return nil, apierrors.NewConflictError("Already has subscription")
				},
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodPost, "/v1/billing/subscription", tt.body, orgID)
			rec := httptest.NewRecorder()
			handler.CreateSubscription(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestBillingHandler_ChangePlan(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name: "changes plan successfully",
			body: ChangePlanRequest{
				PriceID: "price_enterprise_monthly",
			},
			mockService: &mockBillingService{
				changePlanFunc: func(ctx context.Context, oID uuid.UUID, newPriceID string) (*service.SubscriptionInfo, error) {
					return &service.SubscriptionInfo{
						ID:     "sub_test123",
						Plan:   "enterprise",
						Status: "active",
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "rejects missing price_id",
			body:           ChangePlanRequest{},
			mockService:    &mockBillingService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "returns 404 when no subscription",
			body: ChangePlanRequest{
				PriceID: "price_pro_monthly",
			},
			mockService: &mockBillingService{
				changePlanFunc: func(ctx context.Context, oID uuid.UUID, newPriceID string) (*service.SubscriptionInfo, error) {
					return nil, apierrors.NewNotFoundError("Subscription")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodPatch, "/v1/billing/subscription", tt.body, orgID)
			rec := httptest.NewRecorder()
			handler.ChangePlan(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestBillingHandler_CancelSubscription(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name: "cancels subscription successfully",
			mockService: &mockBillingService{
				cancelSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) error {
					return nil
				},
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "returns 404 when no subscription",
			mockService: &mockBillingService{
				cancelSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) error {
					return apierrors.NewNotFoundError("Subscription")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodDelete, "/v1/billing/subscription", nil, orgID)
			rec := httptest.NewRecorder()
			handler.CancelSubscription(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestBillingHandler_ReactivateSubscription(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name: "reactivates subscription successfully",
			mockService: &mockBillingService{
				reactivateSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) error {
					return nil
				},
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "returns 404 when no subscription",
			mockService: &mockBillingService{
				reactivateSubscriptionFunc: func(ctx context.Context, oID uuid.UUID) error {
					return apierrors.NewNotFoundError("Subscription")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodPost, "/v1/billing/subscription/reactivate", nil, orgID)
			rec := httptest.NewRecorder()
			handler.ReactivateSubscription(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestBillingHandler_GetUsage(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "returns usage successfully",
			mockService: &mockBillingService{
				getCurrentUsageFunc: func(ctx context.Context, oID uuid.UUID) (*service.UsageInfo, error) {
					return &service.UsageInfo{
						Signatures:      500,
						SignaturesLimit: 10000,
						Keys:            2,
						KeysLimit:       3,
						PeriodStart:     time.Now(),
						PeriodEnd:       time.Now().AddDate(0, 1, 0),
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data service.UsageInfo `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Data.Signatures != 500 {
					t.Errorf("Signatures = %d, want 500", resp.Data.Signatures)
				}
			},
		},
		{
			name: "returns 404 for org not found",
			mockService: &mockBillingService{
				getCurrentUsageFunc: func(ctx context.Context, oID uuid.UUID) (*service.UsageInfo, error) {
					return nil, apierrors.NewNotFoundError("Organization")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodGet, "/v1/billing/usage", nil, orgID)
			rec := httptest.NewRecorder()
			handler.GetUsage(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestBillingHandler_ListInvoices(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "returns invoices successfully",
			mockService: &mockBillingService{
				listInvoicesFunc: func(ctx context.Context, oID uuid.UUID) ([]*stripe.Invoice, error) {
					return []*stripe.Invoice{
						{
							ID:        "in_test123",
							Number:    "INV-001",
							Status:    stripe.InvoiceStatusPaid,
							AmountDue: 4900,
							Currency:  stripe.CurrencyUSD,
						},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data []InvoiceResponse `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(resp.Data) != 1 {
					t.Errorf("Invoice count = %d, want 1", len(resp.Data))
				}
			},
		},
		{
			name: "returns empty list when no invoices",
			mockService: &mockBillingService{
				listInvoicesFunc: func(ctx context.Context, oID uuid.UUID) ([]*stripe.Invoice, error) {
					return nil, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodGet, "/v1/billing/invoices", nil, orgID)
			rec := httptest.NewRecorder()
			handler.ListInvoices(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestBillingHandler_CreateSetupIntent(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "creates setup intent successfully",
			mockService: &mockBillingService{
				createSetupIntentFunc: func(ctx context.Context, oID uuid.UUID) (string, error) {
					return "seti_secret_xxx", nil
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data map[string]string `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Data["client_secret"] != "seti_secret_xxx" {
					t.Errorf("client_secret = %v, want 'seti_secret_xxx'", resp.Data["client_secret"])
				}
			},
		},
		{
			name: "returns 404 for org not found",
			mockService: &mockBillingService{
				createSetupIntentFunc: func(ctx context.Context, oID uuid.UUID) (string, error) {
					return "", apierrors.NewNotFoundError("Organization")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodPost, "/v1/billing/setup-intent", nil, orgID)
			rec := httptest.NewRecorder()
			handler.CreateSetupIntent(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestBillingHandler_ListPaymentMethods(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		mockService    *mockBillingService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "returns payment methods successfully",
			mockService: &mockBillingService{
				listPaymentMethodsFunc: func(ctx context.Context, oID uuid.UUID) ([]*stripe.PaymentMethod, error) {
					return []*stripe.PaymentMethod{
						{
							ID:   "pm_test123",
							Type: stripe.PaymentMethodTypeCard,
							Card: &stripe.PaymentMethodCard{
								Brand:    stripe.PaymentMethodCardBrandVisa,
								Last4:    "4242",
								ExpMonth: 12,
								ExpYear:  2025,
							},
						},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data []PaymentMethodResponse `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(resp.Data) != 1 {
					t.Errorf("Payment method count = %d, want 1", len(resp.Data))
				}
				if resp.Data[0].Card.Last4 != "4242" {
					t.Errorf("Last4 = %v, want '4242'", resp.Data[0].Card.Last4)
				}
			},
		},
		{
			name: "returns empty list when no payment methods",
			mockService: &mockBillingService{
				listPaymentMethodsFunc: func(ctx context.Context, oID uuid.UUID) ([]*stripe.PaymentMethod, error) {
					return nil, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodGet, "/v1/billing/payment-methods", nil, orgID)
			rec := httptest.NewRecorder()
			handler.ListPaymentMethods(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestBillingHandler_SetDefaultPaymentMethod(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name: "sets default payment method successfully",
			body: SetDefaultPaymentMethodRequest{
				PaymentMethodID: "pm_test123",
			},
			mockService: &mockBillingService{
				setDefaultPaymentMethodFunc: func(ctx context.Context, oID uuid.UUID, pmID string) error {
					return nil
				},
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "rejects missing payment_method_id",
			body:           SetDefaultPaymentMethodRequest{},
			mockService:    &mockBillingService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "returns 404 when no customer",
			body: SetDefaultPaymentMethodRequest{
				PaymentMethodID: "pm_test123",
			},
			mockService: &mockBillingService{
				setDefaultPaymentMethodFunc: func(ctx context.Context, oID uuid.UUID, pmID string) error {
					return apierrors.NewNotFoundError("Customer")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := createBillingTestRequest(t, http.MethodPost, "/v1/billing/payment-methods/default", tt.body, orgID)
			rec := httptest.NewRecorder()
			handler.SetDefaultPaymentMethod(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestBillingHandler_WebhookHandler(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		signature      string
		mockService    *mockBillingService
		expectedStatus int
	}{
		{
			name:      "handles valid webhook",
			body:      `{"type": "customer.subscription.updated"}`,
			signature: "valid_signature",
			mockService: &mockBillingService{
				handleWebhookFunc: func(ctx context.Context, payload []byte, signature string) error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "rejects invalid signature",
			body:      `{"type": "customer.subscription.updated"}`,
			signature: "invalid_signature",
			mockService: &mockBillingService{
				handleWebhookFunc: func(ctx context.Context, payload []byte, signature string) error {
					return apierrors.ErrUnauthorized
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "rejects missing signature",
			body:           `{"type": "customer.subscription.updated"}`,
			signature:      "",
			mockService:    &mockBillingService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewBillingHandler(tt.mockService)

			req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Stripe-Signature", tt.signature)

			rec := httptest.NewRecorder()
			handler.WebhookHandler(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestBillingHandler_Routes(t *testing.T) {
	mockService := &mockBillingService{}
	handler := NewBillingHandler(mockService)
	router := handler.Routes()

	if router == nil {
		t.Error("Routes() returned nil router")
	}
}

func TestBillingHandler_Unauthorized(t *testing.T) {
	handler := NewBillingHandler(&mockBillingService{})

	// Request without org ID in context
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/subscription", nil)
	rec := httptest.NewRecorder()

	handler.GetSubscription(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBillingHandler_InvalidJSON(t *testing.T) {
	handler := NewBillingHandler(&mockBillingService{})
	orgID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscription", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID.String())
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.CreateSubscription(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
