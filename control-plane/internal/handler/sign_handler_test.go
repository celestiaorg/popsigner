package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

func TestSignHandler_SignBatch(t *testing.T) {
	orgID := uuid.New()
	keyID1 := uuid.New()
	keyID2 := uuid.New()

	tests := []struct {
		name           string
		body           interface{}
		mockService    *mockKeyService
		expectedStatus int
		expectedCount  int
	}{
		{
			name: "batch signs successfully",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{KeyID: keyID1.String(), Data: base64.StdEncoding.EncodeToString([]byte("data1"))},
					{KeyID: keyID2.String(), Data: base64.StdEncoding.EncodeToString([]byte("data2"))},
				},
			},
			mockService: &mockKeyService{
				signBatchFunc: func(ctx context.Context, req service.SignBatchKeyRequest) ([]*service.SignKeyResponse, error) {
					return []*service.SignKeyResponse{
						{KeyID: keyID1, Signature: "sig1", PublicKey: "pub1"},
						{KeyID: keyID2, Signature: "sig2", PublicKey: "pub2"},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "handles partial failures",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{KeyID: keyID1.String(), Data: base64.StdEncoding.EncodeToString([]byte("data1"))},
					{KeyID: keyID2.String(), Data: base64.StdEncoding.EncodeToString([]byte("data2"))},
				},
			},
			mockService: &mockKeyService{
				signBatchFunc: func(ctx context.Context, req service.SignBatchKeyRequest) ([]*service.SignKeyResponse, error) {
					return []*service.SignKeyResponse{
						{KeyID: keyID1, Signature: "sig1", PublicKey: "pub1"},
						{KeyID: keyID2, Error: "key not found"},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "rejects empty requests",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{},
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects too many requests",
			body: func() SignBatchHTTPRequest {
				requests := make([]SignBatchItemHTTPRequest, 101)
				for i := range requests {
					requests[i] = SignBatchItemHTTPRequest{
						KeyID: uuid.New().String(),
						Data:  base64.StdEncoding.EncodeToString([]byte("data")),
					}
				}
				return SignBatchHTTPRequest{Requests: requests}
			}(),
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing key_id",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{Data: base64.StdEncoding.EncodeToString([]byte("data"))},
				},
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects invalid key_id",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{KeyID: "not-a-uuid", Data: base64.StdEncoding.EncodeToString([]byte("data"))},
				},
			},
			mockService:    &mockKeyService{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing data",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{KeyID: keyID1.String()},
				},
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
		{
			name: "handles service error",
			body: SignBatchHTTPRequest{
				Requests: []SignBatchItemHTTPRequest{
					{KeyID: keyID1.String(), Data: base64.StdEncoding.EncodeToString([]byte("data"))},
				},
			},
			mockService: &mockKeyService{
				signBatchFunc: func(ctx context.Context, req service.SignBatchKeyRequest) ([]*service.SignKeyResponse, error) {
					return nil, apierrors.ErrQuotaExceeded
				},
			},
			expectedStatus: http.StatusPaymentRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSignHandler(tt.mockService)

			var reqBody []byte
			if str, ok := tt.body.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/sign/batch", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID.String())
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.SignBatch(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d. Body: %s", rec.Code, tt.expectedStatus, rec.Body.String())
			}

			if tt.expectedStatus == http.StatusOK && tt.expectedCount > 0 {
				var resp struct {
					Data struct {
						Signatures []*service.SignKeyResponse `json:"signatures"`
						Count      int                        `json:"count"`
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

func TestSignHandler_Routes(t *testing.T) {
	mockService := &mockKeyService{}
	handler := NewSignHandler(mockService)
	router := handler.Routes()

	if router == nil {
		t.Error("Routes() returned nil router")
	}
}

func TestSignHandler_Unauthorized(t *testing.T) {
	handler := NewSignHandler(&mockKeyService{})

	body := SignBatchHTTPRequest{
		Requests: []SignBatchItemHTTPRequest{
			{KeyID: uuid.New().String(), Data: "aGVsbG8="},
		},
	}
	reqBody, _ := json.Marshal(body)

	// Request without org ID in context
	req := httptest.NewRequest(http.MethodPost, "/v1/sign/batch", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.SignBatch(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

