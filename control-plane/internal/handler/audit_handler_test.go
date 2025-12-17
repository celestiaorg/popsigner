package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/pkg/response"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
)

// MockAuditService is a mock implementation of service.AuditService.
type MockAuditService struct {
	mock.Mock
}

func (m *MockAuditService) Log(ctx context.Context, entry service.AuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockAuditService) Query(ctx context.Context, orgID uuid.UUID, filter service.AuditFilter) ([]*models.AuditLog, string, error) {
	args := m.Called(ctx, orgID, filter)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*models.AuditLog), args.String(1), args.Error(2)
}

func (m *MockAuditService) CleanupOldLogs(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAuditService) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuditLog), args.Error(1)
}

func (m *MockAuditService) CountForPeriod(ctx context.Context, orgID uuid.UUID, start, end time.Time) (int64, error) {
	args := m.Called(ctx, orgID, start, end)
	return args.Get(0).(int64), args.Error(1)
}

// Helper to create a request with org context
func createAuditRequest(method, path string, orgID uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID.String())
	return req.WithContext(ctx)
}

func TestAuditHandler_ListLogs(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	now := time.Now()

	logs := []*models.AuditLog{
		{
			ID:        uuid.New(),
			OrgID:     orgID,
			Event:     models.AuditEventKeyCreated,
			ActorType: models.ActorTypeUser,
			CreatedAt: now,
		},
		{
			ID:        uuid.New(),
			OrgID:     orgID,
			Event:     models.AuditEventKeySigned,
			ActorType: models.ActorTypeAPIKey,
			CreatedAt: now.Add(-time.Hour),
		},
	}

	mockService.On("Query", mock.Anything, orgID, mock.AnythingOfType("service.AuditFilter")).Return(logs, "", nil)

	req := createAuditRequest(http.MethodGet, "/logs", orgID)
	rr := httptest.NewRecorder()

	handler.ListLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp response.Response
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data)

	mockService.AssertExpectations(t)
}

func TestAuditHandler_ListLogs_WithFilters(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()

	mockService.On("Query", mock.Anything, orgID, mock.MatchedBy(func(f service.AuditFilter) bool {
		return f.Event != nil && *f.Event == models.AuditEventKeyCreated
	})).Return([]*models.AuditLog{}, "", nil)

	req := createAuditRequest(http.MethodGet, "/logs?event=key.created&limit=25", orgID)
	rr := httptest.NewRecorder()

	handler.ListLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockService.AssertExpectations(t)
}

func TestAuditHandler_ListLogs_WithTimeFilters(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	mockService.On("Query", mock.Anything, orgID, mock.MatchedBy(func(f service.AuditFilter) bool {
		return f.StartTime != nil && f.EndTime != nil
	})).Return([]*models.AuditLog{}, "", nil)

	// Use url.Values for proper URL encoding of RFC3339 timestamps
	values := url.Values{}
	values.Set("start_time", startTime.Format(time.RFC3339))
	values.Set("end_time", endTime.Format(time.RFC3339))

	req := createAuditRequest(http.MethodGet, "/logs?"+values.Encode(), orgID)
	rr := httptest.NewRecorder()

	handler.ListLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockService.AssertExpectations(t)
}

func TestAuditHandler_ListLogs_Unauthorized(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	// Request without org ID in context
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rr := httptest.NewRecorder()

	handler.ListLogs(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuditHandler_ListLogs_WithPagination(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	nextCursor := uuid.New().String()

	mockService.On("Query", mock.Anything, orgID, mock.AnythingOfType("service.AuditFilter")).Return([]*models.AuditLog{}, nextCursor, nil)

	req := createAuditRequest(http.MethodGet, "/logs", orgID)
	rr := httptest.NewRecorder()

	handler.ListLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp response.Response
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Meta)
	assert.Equal(t, nextCursor, resp.Meta.NextCursor)

	mockService.AssertExpectations(t)
}

func TestAuditHandler_GetLog(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	logID := uuid.New()
	now := time.Now()

	auditLog := &models.AuditLog{
		ID:        logID,
		OrgID:     orgID,
		Event:     models.AuditEventKeyCreated,
		ActorType: models.ActorTypeUser,
		CreatedAt: now,
	}

	mockService.On("GetByID", mock.Anything, logID).Return(auditLog, nil)

	req := createAuditRequest(http.MethodGet, "/logs/"+logID.String(), orgID)

	// Add URL param using chi
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", logID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetLog(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp response.Response
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data)

	mockService.AssertExpectations(t)
}

func TestAuditHandler_GetLog_NotFound(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	logID := uuid.New()

	mockService.On("GetByID", mock.Anything, logID).Return(nil, nil)

	req := createAuditRequest(http.MethodGet, "/logs/"+logID.String(), orgID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", logID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetLog(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mockService.AssertExpectations(t)
}

func TestAuditHandler_GetLog_WrongOrg(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()
	otherOrgID := uuid.New()
	logID := uuid.New()

	auditLog := &models.AuditLog{
		ID:    logID,
		OrgID: otherOrgID, // Different org
	}

	mockService.On("GetByID", mock.Anything, logID).Return(auditLog, nil)

	req := createAuditRequest(http.MethodGet, "/logs/"+logID.String(), orgID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", logID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetLog(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mockService.AssertExpectations(t)
}

func TestAuditHandler_GetLog_InvalidID(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	orgID := uuid.New()

	req := createAuditRequest(http.MethodGet, "/logs/invalid-uuid", orgID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	handler.GetLog(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuditHandler_Routes(t *testing.T) {
	mockService := new(MockAuditService)
	handler := NewAuditHandler(mockService)

	router := handler.Routes()
	assert.NotNil(t, router)
}

