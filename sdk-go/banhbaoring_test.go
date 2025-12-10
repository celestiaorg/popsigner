package banhbaoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key")

	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got %q", client.apiKey)
	}

	if client.baseURL != DefaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultBaseURL, client.baseURL)
	}

	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, client.httpClient.Timeout)
	}

	if client.Keys == nil {
		t.Error("expected Keys service to be initialized")
	}
	if client.Sign == nil {
		t.Error("expected Sign service to be initialized")
	}
	if client.Orgs == nil {
		t.Error("expected Orgs service to be initialized")
	}
	if client.Audit == nil {
		t.Error("expected Audit service to be initialized")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}
	customURL := "https://custom.api.io"

	client := NewClient("test-key",
		WithBaseURL(customURL),
		WithHTTPClient(customClient),
	)

	if client.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, client.baseURL)
	}

	if client.httpClient != customClient {
		t.Error("expected custom HTTP client to be set")
	}
}

func TestNewClient_WithTimeout(t *testing.T) {
	client := NewClient("test-key", WithTimeout(5*time.Second))

	if client.httpClient.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", client.httpClient.Timeout)
	}
}

func TestClient_BaseURL(t *testing.T) {
	client := NewClient("test-key", WithBaseURL("https://test.api.io"))
	if client.BaseURL() != "https://test.api.io" {
		t.Errorf("expected BaseURL() to return custom URL")
	}
}

// newTestServer creates a test server and client for testing.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	client := NewClient("test-api-key", WithBaseURL(server.URL))
	t.Cleanup(server.Close)
	return server, client
}

func TestKeysService_Create(t *testing.T) {
	namespaceID := uuid.New()
	keyID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/keys" {
			t.Errorf("expected /v1/keys, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("expected API key header")
		}

		// Return response
		resp := map[string]interface{}{
			"id":           keyID.String(),
			"namespace_id": namespaceID.String(),
			"name":         "test-key",
			"public_key":   "0x1234",
			"address":      "0xabcd",
			"algorithm":    "secp256k1",
			"exportable":   true,
			"version":      1,
			"created_at":   "2024-01-01T00:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	key, err := client.Keys.Create(ctx, CreateKeyRequest{
		Name:        "test-key",
		NamespaceID: namespaceID,
		Algorithm:   "secp256k1",
		Exportable:  true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key.ID != keyID {
		t.Errorf("expected key ID %s, got %s", keyID, key.ID)
	}
	if key.Name != "test-key" {
		t.Errorf("expected name 'test-key', got %q", key.Name)
	}
	if key.Algorithm != AlgorithmSecp256k1 {
		t.Errorf("expected algorithm secp256k1, got %s", key.Algorithm)
	}
}

func TestKeysService_CreateBatch(t *testing.T) {
	namespaceID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/keys/batch" {
			t.Errorf("expected /v1/keys/batch, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"id":           uuid.New().String(),
					"namespace_id": namespaceID.String(),
					"name":         "worker-1",
					"public_key":   "0x1111",
					"address":      "0xaaaa",
					"algorithm":    "secp256k1",
					"version":      1,
					"created_at":   "2024-01-01T00:00:00Z",
				},
				{
					"id":           uuid.New().String(),
					"namespace_id": namespaceID.String(),
					"name":         "worker-2",
					"public_key":   "0x2222",
					"address":      "0xbbbb",
					"algorithm":    "secp256k1",
					"version":      1,
					"created_at":   "2024-01-01T00:00:00Z",
				},
			},
			"count": 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	keys, err := client.Keys.CreateBatch(ctx, CreateBatchRequest{
		Prefix:      "worker",
		Count:       2,
		NamespaceID: namespaceID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestKeysService_Get(t *testing.T) {
	keyID := uuid.New()
	namespaceID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/v1/keys/%s", keyID)
		if r.URL.Path != expectedPath {
			t.Errorf("expected %s, got %s", expectedPath, r.URL.Path)
		}

		resp := map[string]interface{}{
			"id":           keyID.String(),
			"namespace_id": namespaceID.String(),
			"name":         "my-key",
			"public_key":   "0x1234",
			"address":      "0xabcd",
			"algorithm":    "secp256k1",
			"version":      1,
			"created_at":   "2024-01-01T00:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	key, err := client.Keys.Get(ctx, keyID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key.ID != keyID {
		t.Errorf("expected key ID %s, got %s", keyID, key.ID)
	}
}

func TestKeysService_List(t *testing.T) {
	namespaceID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/keys" {
			t.Errorf("expected /v1/keys, got %s", r.URL.Path)
		}

		resp := []map[string]interface{}{
			{
				"id":           uuid.New().String(),
				"namespace_id": namespaceID.String(),
				"name":         "key-1",
				"public_key":   "0x1111",
				"address":      "0xaaaa",
				"algorithm":    "secp256k1",
				"version":      1,
				"created_at":   "2024-01-01T00:00:00Z",
			},
			{
				"id":           uuid.New().String(),
				"namespace_id": namespaceID.String(),
				"name":         "key-2",
				"public_key":   "0x2222",
				"address":      "0xbbbb",
				"algorithm":    "secp256k1",
				"version":      1,
				"created_at":   "2024-01-01T00:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	keys, err := client.Keys.List(ctx, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestKeysService_Delete(t *testing.T) {
	keyID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		expectedPath := fmt.Sprintf("/v1/keys/%s", keyID)
		if r.URL.Path != expectedPath {
			t.Errorf("expected %s, got %s", expectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := context.Background()
	err := client.Keys.Delete(ctx, keyID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSignService_Sign(t *testing.T) {
	keyID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/v1/keys/%s/sign", keyID)
		if r.URL.Path != expectedPath {
			t.Errorf("expected %s, got %s", expectedPath, r.URL.Path)
		}

		resp := map[string]interface{}{
			"signature":   "c2lnbmF0dXJl", // base64 "signature"
			"public_key":  "0x1234",
			"key_version": 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	result, err := client.Sign.Sign(ctx, keyID, []byte("test message"), false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.KeyID != keyID {
		t.Errorf("expected key ID %s, got %s", keyID, result.KeyID)
	}
	if len(result.Signature) == 0 {
		t.Error("expected non-empty signature")
	}
}

func TestSignService_SignBatch(t *testing.T) {
	key1 := uuid.New()
	key2 := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sign/batch" {
			t.Errorf("expected /v1/sign/batch, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"signatures": []map[string]interface{}{
				{
					"key_id":      key1.String(),
					"signature":   "c2lnMQ==",
					"public_key":  "0x1111",
					"key_version": 1,
				},
				{
					"key_id":      key2.String(),
					"signature":   "c2lnMg==",
					"public_key":  "0x2222",
					"key_version": 1,
				},
			},
			"count": 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	results, err := client.Sign.SignBatch(ctx, BatchSignRequest{
		Requests: []SignRequest{
			{KeyID: key1, Data: []byte("msg1")},
			{KeyID: key2, Data: []byte("msg2")},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestOrgsService_Create(t *testing.T) {
	orgID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organizations" {
			t.Errorf("expected /v1/organizations, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"id":         orgID.String(),
			"name":       "Test Org",
			"plan":       "pro",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	org, err := client.Orgs.Create(ctx, CreateOrgRequest{Name: "Test Org"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if org.ID != orgID {
		t.Errorf("expected org ID %s, got %s", orgID, org.ID)
	}
}

func TestAuditService_List(t *testing.T) {
	logID := uuid.New()
	orgID := uuid.New()

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audit/logs" {
			t.Errorf("expected /v1/audit/logs, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":         logID.String(),
					"org_id":     orgID.String(),
					"event":      "key.created",
					"actor_type": "user",
					"created_at": "2024-01-01T00:00:00Z",
				},
			},
			"meta": map[string]interface{}{
				"next_cursor": "abc123",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	result, err := client.Audit.List(ctx, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(result.Logs))
	}
	if result.NextCursor != "abc123" {
		t.Errorf("expected cursor 'abc123', got %q", result.NextCursor)
	}
}

func TestError_Handling(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		resp := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "not_found",
				"message": "Key not found",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	ctx := context.Background()
	_, err := client.Keys.Get(ctx, uuid.New())

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := IsAPIError(err)
	if !ok {
		t.Fatal("expected API error")
	}

	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() to return true")
	}
	if apiErr.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %q", apiErr.Code)
	}
}

func TestError_Unauthorized(t *testing.T) {
	err := &Error{StatusCode: 401, Code: "unauthorized", Message: "Invalid API key"}

	if !err.IsUnauthorized() {
		t.Error("expected IsUnauthorized() to return true")
	}
	if err.IsNotFound() {
		t.Error("expected IsNotFound() to return false")
	}
}

func TestError_RateLimited(t *testing.T) {
	err := &Error{StatusCode: 429, Code: "rate_limited", Message: "Too many requests"}

	if !err.IsRateLimited() {
		t.Error("expected IsRateLimited() to return true")
	}
}

func TestError_ErrorString(t *testing.T) {
	err := &Error{Code: "test_error", Message: "Test message"}
	expected := "test_error: Test message"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestPtr(t *testing.T) {
	event := AuditEventKeySigned
	ptr := Ptr(event)
	if *ptr != event {
		t.Errorf("expected %v, got %v", event, *ptr)
	}
}

