package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// mockKeyService is a mock implementation of KeyService for testing.
type mockKeyService struct {
	createFunc      func(ctx context.Context, req service.CreateKeyRequest) (*models.Key, error)
	createBatchFunc func(ctx context.Context, req service.CreateBatchKeyRequest) ([]*models.Key, error)
	getFunc         func(ctx context.Context, orgID, keyID uuid.UUID) (*models.Key, error)
	listFunc        func(ctx context.Context, orgID uuid.UUID, namespaceID *uuid.UUID) ([]*models.Key, error)
	deleteFunc      func(ctx context.Context, orgID, keyID uuid.UUID) error
	signFunc        func(ctx context.Context, orgID, keyID uuid.UUID, data []byte, prehashed bool) (*service.SignKeyResponse, error)
	signBatchFunc   func(ctx context.Context, req service.SignBatchKeyRequest) ([]*service.SignKeyResponse, error)
	importFunc      func(ctx context.Context, req service.ImportKeyRequest) (*models.Key, error)
	exportFunc      func(ctx context.Context, orgID, keyID uuid.UUID) (string, error)
}

func (m *mockKeyService) Create(ctx context.Context, req service.CreateKeyRequest) (*models.Key, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockKeyService) CreateBatch(ctx context.Context, req service.CreateBatchKeyRequest) ([]*models.Key, error) {
	if m.createBatchFunc != nil {
		return m.createBatchFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockKeyService) Get(ctx context.Context, orgID, keyID uuid.UUID) (*models.Key, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, orgID, keyID)
	}
	return nil, nil
}

func (m *mockKeyService) List(ctx context.Context, orgID uuid.UUID, namespaceID *uuid.UUID) ([]*models.Key, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, orgID, namespaceID)
	}
	return nil, nil
}

func (m *mockKeyService) Delete(ctx context.Context, orgID, keyID uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, orgID, keyID)
	}
	return nil
}

func (m *mockKeyService) Sign(ctx context.Context, orgID, keyID uuid.UUID, data []byte, prehashed bool) (*service.SignKeyResponse, error) {
	if m.signFunc != nil {
		return m.signFunc(ctx, orgID, keyID, data, prehashed)
	}
	return nil, nil
}

func (m *mockKeyService) SignBatch(ctx context.Context, req service.SignBatchKeyRequest) ([]*service.SignKeyResponse, error) {
	if m.signBatchFunc != nil {
		return m.signBatchFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockKeyService) Import(ctx context.Context, req service.ImportKeyRequest) (*models.Key, error) {
	if m.importFunc != nil {
		return m.importFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockKeyService) Export(ctx context.Context, orgID, keyID uuid.UUID) (string, error) {
	if m.exportFunc != nil {
		return m.exportFunc(ctx, orgID, keyID)
	}
	return "", nil
}

// createKeyTestRequest creates a request with org ID in context
func createKeyTestRequest(t *testing.T, method, path string, body interface{}, orgID uuid.UUID) *http.Request {
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

func TestKeyHandler_Create(t *testing.T) {
	orgID := uuid.New()
	keyID := uuid.New()
	namespaceID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockKeyService
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "creates key successfully",
			body: CreateKeyHTTPRequest{
				NamespaceID: namespaceID.String(),
				Name:        "test-key",
				Exportable:  false,
			},
			mockService: &mockKeyService{
				createFunc: func(ctx context.Context, req service.CreateKeyRequest) (*models.Key, error) {
					return &models.Key{
						ID:          keyID,
						OrgID:       req.OrgID,
						NamespaceID: req.NamespaceID,
						Name:        req.Name,
						PublicKey:   []byte{0x02, 0x01, 0x02, 0x03},
						Address:     "celestia1test",
						Algorithm:   models.AlgorithmSecp256k1,
						Exportable:  req.Exportable,
						Version:     1,
						CreatedAt:   time.Now(),
					}, nil
				},
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data KeyResponse `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Data.Name != "test-key" {
					t.Errorf("Name = %v, want 'test-key'", resp.Data.Name)
				}
			},
		},
		{
			name: "rejects missing name",
			body: CreateKeyHTTPRequest{
				NamespaceID: namespaceID.String(),
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing namespace_id",
			body: CreateKeyHTTPRequest{
				Name: "test-key",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects invalid namespace_id",
			body: CreateKeyHTTPRequest{
				NamespaceID: "not-a-uuid",
				Name:        "test-key",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "rejects invalid JSON",
			body:           "not json",
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			var reqBody []byte
			if str, ok := tt.body.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/keys", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID.String())
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestKeyHandler_CreateBatch(t *testing.T) {
	orgID := uuid.New()
	namespaceID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockKeyService
		expectedStatus int
		expectedCount  int
	}{
		{
			name: "creates batch successfully",
			body: CreateBatchHTTPRequest{
				NamespaceID: namespaceID.String(),
				Prefix:      "worker",
				Count:       4,
			},
			mockService: &mockKeyService{
				createBatchFunc: func(ctx context.Context, req service.CreateBatchKeyRequest) ([]*models.Key, error) {
					keys := make([]*models.Key, req.Count)
					for i := 0; i < req.Count; i++ {
						keys[i] = &models.Key{
							ID:          uuid.New(),
							OrgID:       req.OrgID,
							NamespaceID: req.NamespaceID,
							Name:        req.Prefix + "-" + string(rune('1'+i)),
							PublicKey:   []byte{0x02, 0x01, 0x02, 0x03},
							Address:     "celestia1test",
							Algorithm:   models.AlgorithmSecp256k1,
							Version:     1,
							CreatedAt:   time.Now(),
						}
					}
					return keys, nil
				},
			},
			expectedStatus: http.StatusCreated,
			expectedCount:  4,
		},
		{
			name: "rejects count over 100",
			body: CreateBatchHTTPRequest{
				NamespaceID: namespaceID.String(),
				Prefix:      "worker",
				Count:       101,
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects count of 0",
			body: CreateBatchHTTPRequest{
				NamespaceID: namespaceID.String(),
				Prefix:      "worker",
				Count:       0,
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing prefix",
			body: CreateBatchHTTPRequest{
				NamespaceID: namespaceID.String(),
				Count:       4,
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodPost, "/v1/keys/batch", tt.body, orgID)
			rec := httptest.NewRecorder()
			handler.CreateBatch(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.expectedStatus == http.StatusCreated && tt.expectedCount > 0 {
				var resp struct {
					Data struct {
						Keys  []*KeyResponse `json:"keys"`
						Count int            `json:"count"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Data.Count != tt.expectedCount {
					t.Errorf("Count = %d, want %d", resp.Data.Count, tt.expectedCount)
				}
			}
		})
	}
}

func TestKeyHandler_List(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name           string
		queryParams    string
		mockService    *mockKeyService
		expectedStatus int
		expectedCount  int
	}{
		{
			name: "lists keys successfully",
			mockService: &mockKeyService{
				listFunc: func(ctx context.Context, oID uuid.UUID, nsID *uuid.UUID) ([]*models.Key, error) {
					return []*models.Key{
						{ID: uuid.New(), Name: "key-1", PublicKey: []byte{0x02}, CreatedAt: time.Now()},
						{ID: uuid.New(), Name: "key-2", PublicKey: []byte{0x02}, CreatedAt: time.Now()},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "returns empty list",
			mockService: &mockKeyService{
				listFunc: func(ctx context.Context, oID uuid.UUID, nsID *uuid.UUID) ([]*models.Key, error) {
					return []*models.Key{}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name: "handles service error",
			mockService: &mockKeyService{
				listFunc: func(ctx context.Context, oID uuid.UUID, nsID *uuid.UUID) ([]*models.Key, error) {
					return nil, apierrors.ErrInternal
				},
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodGet, "/v1/keys"+tt.queryParams, nil, orgID)
			rec := httptest.NewRecorder()
			handler.List(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp struct {
					Data []*KeyResponse `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(resp.Data) != tt.expectedCount {
					t.Errorf("Response count = %d, want %d", len(resp.Data), tt.expectedCount)
				}
			}
		})
	}
}

func TestKeyHandler_Get(t *testing.T) {
	orgID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		mockService    *mockKeyService
		expectedStatus int
	}{
		{
			name:       "gets key successfully",
			keyIDParam: keyID.String(),
			mockService: &mockKeyService{
				getFunc: func(ctx context.Context, oID, kID uuid.UUID) (*models.Key, error) {
					return &models.Key{
						ID:        kID,
						OrgID:     oID,
						Name:      "test-key",
						PublicKey: []byte{0x02, 0x01, 0x02, 0x03},
						Address:   "celestia1test",
						Algorithm: models.AlgorithmSecp256k1,
						Version:   1,
						CreatedAt: time.Now(),
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "returns 404 for nonexistent key",
			keyIDParam: uuid.New().String(),
			mockService: &mockKeyService{
				getFunc: func(ctx context.Context, oID, kID uuid.UUID) (*models.Key, error) {
					return nil, apierrors.NewNotFoundError("Key")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "rejects invalid UUID",
			keyIDParam:     "not-a-uuid",
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodGet, "/v1/keys/"+tt.keyIDParam, nil, orgID)

			// Use chi router to get URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.keyIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			handler.Get(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestKeyHandler_Delete(t *testing.T) {
	orgID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		mockService    *mockKeyService
		expectedStatus int
	}{
		{
			name:       "deletes key successfully",
			keyIDParam: keyID.String(),
			mockService: &mockKeyService{
				deleteFunc: func(ctx context.Context, oID, kID uuid.UUID) error {
					return nil
				},
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:       "returns 404 for nonexistent key",
			keyIDParam: uuid.New().String(),
			mockService: &mockKeyService{
				deleteFunc: func(ctx context.Context, oID, kID uuid.UUID) error {
					return apierrors.NewNotFoundError("Key")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "rejects invalid UUID",
			keyIDParam:     "not-a-uuid",
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodDelete, "/v1/keys/"+tt.keyIDParam, nil, orgID)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.keyIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			handler.Delete(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestKeyHandler_Sign(t *testing.T) {
	orgID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		body           interface{}
		mockService    *mockKeyService
		expectedStatus int
	}{
		{
			name:       "signs data successfully",
			keyIDParam: keyID.String(),
			body: SignHTTPRequest{
				Data: base64.StdEncoding.EncodeToString([]byte("hello world")),
			},
			mockService: &mockKeyService{
				signFunc: func(ctx context.Context, oID, kID uuid.UUID, data []byte, prehashed bool) (*service.SignKeyResponse, error) {
					return &service.SignKeyResponse{
						KeyID:     kID,
						Signature: base64.StdEncoding.EncodeToString([]byte("mock_signature")),
						PublicKey: "0201020304",
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "rejects missing data",
			keyIDParam: keyID.String(),
			body: SignHTTPRequest{
				Data: "",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects invalid base64",
			keyIDParam: keyID.String(),
			body: SignHTTPRequest{
				Data: "not-valid-base64!!!",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 404 for nonexistent key",
			keyIDParam: uuid.New().String(),
			body: SignHTTPRequest{
				Data: base64.StdEncoding.EncodeToString([]byte("hello world")),
			},
			mockService: &mockKeyService{
				signFunc: func(ctx context.Context, oID, kID uuid.UUID, data []byte, prehashed bool) (*service.SignKeyResponse, error) {
					return nil, apierrors.NewNotFoundError("Key")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "rejects invalid key UUID",
			keyIDParam:     "not-a-uuid",
			body:           SignHTTPRequest{Data: "aGVsbG8="},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodPost, "/v1/keys/"+tt.keyIDParam+"/sign", tt.body, orgID)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.keyIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			handler.Sign(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestKeyHandler_Import(t *testing.T) {
	orgID := uuid.New()
	namespaceID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockKeyService
		expectedStatus int
	}{
		{
			name: "imports key successfully",
			body: ImportHTTPRequest{
				NamespaceID: namespaceID.String(),
				Name:        "imported-key",
				PrivateKey:  base64.StdEncoding.EncodeToString([]byte("mock_private_key")),
				Exportable:  true,
			},
			mockService: &mockKeyService{
				importFunc: func(ctx context.Context, req service.ImportKeyRequest) (*models.Key, error) {
					return &models.Key{
						ID:          keyID,
						OrgID:       req.OrgID,
						NamespaceID: req.NamespaceID,
						Name:        req.Name,
						PublicKey:   []byte{0x02, 0x01, 0x02, 0x03},
						Address:     "celestia1test",
						Algorithm:   models.AlgorithmSecp256k1,
						Exportable:  req.Exportable,
						Version:     1,
						CreatedAt:   time.Now(),
					}, nil
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "rejects missing name",
			body: ImportHTTPRequest{
				NamespaceID: namespaceID.String(),
				PrivateKey:  base64.StdEncoding.EncodeToString([]byte("mock_private_key")),
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing private_key",
			body: ImportHTTPRequest{
				NamespaceID: namespaceID.String(),
				Name:        "imported-key",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects invalid base64 private_key",
			body: ImportHTTPRequest{
				NamespaceID: namespaceID.String(),
				Name:        "imported-key",
				PrivateKey:  "not-valid-base64!!!",
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodPost, "/v1/keys/import", tt.body, orgID)
			rec := httptest.NewRecorder()
			handler.Import(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestKeyHandler_Export(t *testing.T) {
	orgID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		mockService    *mockKeyService
		expectedStatus int
	}{
		{
			name:       "exports key successfully",
			keyIDParam: keyID.String(),
			mockService: &mockKeyService{
				exportFunc: func(ctx context.Context, oID, kID uuid.UUID) (string, error) {
					return base64.StdEncoding.EncodeToString([]byte("mock_private_key")), nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "returns 404 for nonexistent key",
			keyIDParam: uuid.New().String(),
			mockService: &mockKeyService{
				exportFunc: func(ctx context.Context, oID, kID uuid.UUID) (string, error) {
					return "", apierrors.NewNotFoundError("Key")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "returns 403 for non-exportable key",
			keyIDParam: keyID.String(),
			mockService: &mockKeyService{
				exportFunc: func(ctx context.Context, oID, kID uuid.UUID) (string, error) {
					return "", apierrors.ErrForbidden.WithMessage("Key is not exportable")
				},
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKeyHandler(tt.mockService)

			req := createKeyTestRequest(t, http.MethodPost, "/v1/keys/"+tt.keyIDParam+"/export", nil, orgID)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.keyIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			handler.Export(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

func TestKeyHandler_Routes(t *testing.T) {
	mockService := &mockKeyService{}
	handler := NewKeyHandler(mockService)
	router := handler.Routes()

	if router == nil {
		t.Error("Routes() returned nil router")
	}
}

func TestKeyHandler_Unauthorized(t *testing.T) {
	handler := NewKeyHandler(&mockKeyService{})

	// Request without org ID in context
	req := httptest.NewRequest(http.MethodGet, "/v1/keys", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

