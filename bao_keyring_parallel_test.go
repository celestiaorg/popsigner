package banhbaoring

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Concurrent Sign Tests
// ============================================

// TestSign_Concurrent verifies that Sign() is safe for concurrent use.
// This is critical for Celestia's parallel worker pattern.
func TestSign_Concurrent(t *testing.T) {
	var requestCount int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		// Simulate signing latency
		time.Sleep(50 * time.Millisecond)

		if strings.Contains(r.URL.Path, "/sign/") {
			sig := make([]byte, 64)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": SignResponse{
					Signature: base64.StdEncoding.EncodeToString(sig),
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create 4 worker keys
	for i := 1; i <= 4; i++ {
		uid := fmt.Sprintf("worker-%d", i)
		_ = kr.store.Save(&KeyMetadata{
			UID:         uid,
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("celestia1worker%d", i),
		})
	}

	// Sign concurrently with 4 workers
	const numWorkers = 4
	const signsPerWorker = 10

	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*signsPerWorker)

	start := time.Now()

	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uid := fmt.Sprintf("worker-%d", workerID)
			for i := 0; i < signsPerWorker; i++ {
				msg := []byte(fmt.Sprintf("message-%d-%d", workerID, i))
				_, _, err := kr.Sign(uid, msg, signing.SignMode_SIGN_MODE_DIRECT)
				if err != nil {
					errors <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errors)

	elapsed := time.Since(start)

	// Check no errors
	for err := range errors {
		t.Errorf("sign error: %v", err)
	}

	// Verify all requests were made
	assert.Equal(t, int64(numWorkers*signsPerWorker), requestCount)

	// Performance check: parallel should be faster than sequential
	// Sequential would be: 40 signs × 50ms = 2000ms
	// Parallel (4 workers) should be: ~10 batches × 50ms = ~500ms
	t.Logf("Elapsed: %v for %d signs (%d workers)", elapsed, numWorkers*signsPerWorker, numWorkers)
	assert.Less(t, elapsed, 1500*time.Millisecond,
		"parallel signing should be faster than sequential")
}

// TestSign_NoHeadOfLineBlocking verifies that slow signs don't block others.
func TestSign_NoHeadOfLineBlocking(t *testing.T) {
	// Worker 1 is slow (500ms), workers 2-4 are fast (50ms)
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "worker-1") {
			time.Sleep(500 * time.Millisecond) // Slow
		} else {
			time.Sleep(50 * time.Millisecond) // Fast
		}

		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create 4 workers
	for i := 1; i <= 4; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("worker-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	// Sign concurrently
	completionTimes := make([]time.Duration, 4)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, _ = kr.Sign(fmt.Sprintf("worker-%d", idx), []byte("msg"),
				signing.SignMode_SIGN_MODE_DIRECT)
			completionTimes[idx-1] = time.Since(start)
		}(i)
	}
	wg.Wait()

	// Fast workers (2,3,4) should complete in ~50-100ms
	// Slow worker (1) should complete in ~500ms
	// But fast workers should NOT wait for slow worker!
	for i := 1; i <= 3; i++ {
		assert.Less(t, completionTimes[i], 200*time.Millisecond,
			"worker-%d should not be blocked by slow worker-1", i+1)
	}

	t.Logf("Completion times: worker-1=%v, worker-2=%v, worker-3=%v, worker-4=%v",
		completionTimes[0], completionTimes[1], completionTimes[2], completionTimes[3])
}

// TestSign_ConcurrentWithDifferentKeys verifies multiple keys can sign concurrently.
func TestSign_ConcurrentWithDifferentKeys(t *testing.T) {
	var mu sync.Mutex
	signedKeys := make(map[string]int)

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Extract key name from path
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 5 && strings.Contains(r.URL.Path, "/sign/") {
			keyName := parts[len(parts)-1]
			mu.Lock()
			signedKeys[keyName]++
			mu.Unlock()
		}

		time.Sleep(10 * time.Millisecond)
		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create 10 keys
	for i := 0; i < 10; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("key-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	// Sign with all keys concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _, _ = kr.Sign(fmt.Sprintf("key-%d", idx), []byte(fmt.Sprintf("msg-%d", j)),
					signing.SignMode_SIGN_MODE_DIRECT)
			}
		}(i)
	}
	wg.Wait()

	// Verify each key was used 5 times
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < 10; i++ {
		assert.Equal(t, 5, signedKeys[fmt.Sprintf("key-%d", i)],
			"key-%d should have been used 5 times", i)
	}
}

// ============================================
// CreateBatch Tests
// ============================================

// TestCreateBatch_Success tests batch key creation.
func TestCreateBatch_Success(t *testing.T) {
	var keyCount int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/keys/") {
			idx := atomic.AddInt64(&keyCount, 1)
			pubKeyHex := fmt.Sprintf("02%064d", idx)
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       r.URL.Path,
					"public_key": pubKeyHex,
					"address":    fmt.Sprintf("addr%d", idx),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	result, err := kr.CreateBatch(context.Background(), CreateBatchOptions{
		Prefix: "blob-worker",
		Count:  4,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Keys, 4)

	// Verify all keys were created
	for i, key := range result.Keys {
		require.NotNil(t, key, "key %d should not be nil", i)
		assert.Contains(t, key.Name, "blob-worker")
	}

	// Verify no errors
	for i, err := range result.Errors {
		assert.Nil(t, err, "key %d should have no error", i)
	}
}

// TestCreateBatch_InvalidCount tests batch creation with invalid count.
func TestCreateBatch_InvalidCount(t *testing.T) {
	kr, server := setupTestKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	tests := []struct {
		name  string
		count int
	}{
		{"zero count", 0},
		{"negative count", -1},
		{"too large count", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kr.CreateBatch(context.Background(), CreateBatchOptions{
				Prefix: "test",
				Count:  tt.count,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "count must be between 1 and 100")
		})
	}
}

// TestCreateBatch_EmptyPrefix tests batch creation with empty prefix.
func TestCreateBatch_EmptyPrefix(t *testing.T) {
	kr, server := setupTestKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	_, err := kr.CreateBatch(context.Background(), CreateBatchOptions{
		Prefix: "",
		Count:  4,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix is required")
}

// TestCreateBatch_PartialFailure tests batch creation with some failures.
func TestCreateBatch_PartialFailure(t *testing.T) {
	var callCount int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/keys/") {
			idx := atomic.AddInt64(&callCount, 1)
			// Fail on key 2
			if idx == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"simulated failure"},
				})
				return
			}
			pubKeyHex := fmt.Sprintf("02%064d", idx)
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       fmt.Sprintf("partial-%d", idx),
					"public_key": pubKeyHex,
					"address":    fmt.Sprintf("addr%d", idx),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	result, err := kr.CreateBatch(context.Background(), CreateBatchOptions{
		Prefix: "partial",
		Count:  4,
	})

	// Should return partial result with error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch create partial failure")
	assert.NotNil(t, result)

	// Count successes and failures
	var successes, failures int
	for i := range result.Keys {
		if result.Errors[i] == nil && result.Keys[i] != nil {
			successes++
		} else {
			failures++
		}
	}
	assert.Equal(t, 3, successes, "should have 3 successful keys")
	assert.Equal(t, 1, failures, "should have 1 failed key")
}

// TestCreateBatch_Performance tests batch creation is parallel.
func TestCreateBatch_Performance(t *testing.T) {
	const createLatency = 50 * time.Millisecond

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/keys/") {
			time.Sleep(createLatency)
			pubKeyHex := "02" + strings.Repeat("00", 32)
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "test",
					"public_key": pubKeyHex,
					"address":    "addr",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	start := time.Now()
	_, err := kr.CreateBatch(context.Background(), CreateBatchOptions{
		Prefix: "perf",
		Count:  4,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)

	// Sequential would be: 4 × 50ms = 200ms
	// Parallel should be: ~50-100ms
	t.Logf("Batch create of 4 keys took: %v", elapsed)
	assert.Less(t, elapsed, 150*time.Millisecond,
		"batch creation should execute in parallel")
}

// ============================================
// SignBatch Tests
// ============================================

// TestSignBatch_Success tests batch signing.
func TestSignBatch_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create 4 workers
	for i := 1; i <= 4; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("worker-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	requests := []BatchSignRequest{
		{UID: "worker-1", Msg: []byte("tx1")},
		{UID: "worker-2", Msg: []byte("tx2")},
		{UID: "worker-3", Msg: []byte("tx3")},
		{UID: "worker-4", Msg: []byte("tx4")},
	}

	results := kr.SignBatch(context.Background(), requests)

	assert.Len(t, results, 4)
	for i, r := range results {
		assert.Nil(t, r.Error, "request %d should succeed", i)
		assert.Len(t, r.Signature, 64, "signature %d should be 64 bytes", i)
		assert.Equal(t, fmt.Sprintf("worker-%d", i+1), r.UID)
	}
}

// TestSignBatch_Empty tests batch signing with empty input.
func TestSignBatch_Empty(t *testing.T) {
	kr, server := setupTestKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	results := kr.SignBatch(context.Background(), nil)
	assert.Nil(t, results)

	results = kr.SignBatch(context.Background(), []BatchSignRequest{})
	assert.Nil(t, results)
}

// TestSignBatch_PartialFailure tests batch signing with some failures.
func TestSignBatch_PartialFailure(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Fail for worker-2
		if strings.Contains(r.URL.Path, "worker-2") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"key not found"},
			})
			return
		}

		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create workers (worker-2 will fail at signing because of mock)
	for i := 1; i <= 4; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("worker-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	requests := []BatchSignRequest{
		{UID: "worker-1", Msg: []byte("tx1")},
		{UID: "worker-2", Msg: []byte("tx2")}, // Will fail
		{UID: "worker-3", Msg: []byte("tx3")},
		{UID: "worker-4", Msg: []byte("tx4")},
	}

	results := kr.SignBatch(context.Background(), requests)

	assert.Len(t, results, 4)

	// worker-1, worker-3, worker-4 should succeed
	assert.Nil(t, results[0].Error)
	assert.NotNil(t, results[1].Error) // worker-2 fails
	assert.Nil(t, results[2].Error)
	assert.Nil(t, results[3].Error)
}

// TestSignBatch_Performance verifies batch is faster than sequential.
func TestSignBatch_Performance(t *testing.T) {
	const signLatency = 50 * time.Millisecond

	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(signLatency)
		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create workers
	for i := 1; i <= 4; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("worker-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	requests := make([]BatchSignRequest, 4)
	for i := 0; i < 4; i++ {
		requests[i] = BatchSignRequest{
			UID: fmt.Sprintf("worker-%d", i+1),
			Msg: []byte(fmt.Sprintf("tx%d", i)),
		}
	}

	start := time.Now()
	results := kr.SignBatch(context.Background(), requests)
	elapsed := time.Since(start)

	// All should succeed
	for _, r := range results {
		assert.Nil(t, r.Error)
	}

	// Sequential would be: 4 × 50ms = 200ms
	// Parallel should be: ~50-100ms (all execute concurrently)
	t.Logf("Batch sign of 4 requests took: %v", elapsed)
	assert.Less(t, elapsed, 150*time.Millisecond,
		"batch signing should execute in parallel")
}

// TestSignBatch_LargeCount tests batch signing with many requests.
func TestSignBatch_LargeCount(t *testing.T) {
	var signCount int64

	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&signCount, 1)
		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create 20 workers
	for i := 1; i <= 20; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("worker-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	requests := make([]BatchSignRequest, 20)
	for i := 0; i < 20; i++ {
		requests[i] = BatchSignRequest{
			UID: fmt.Sprintf("worker-%d", i+1),
			Msg: []byte(fmt.Sprintf("tx%d", i)),
		}
	}

	results := kr.SignBatch(context.Background(), requests)

	assert.Len(t, results, 20)
	for i, r := range results {
		assert.Nil(t, r.Error, "request %d should succeed", i)
		assert.Len(t, r.Signature, 64)
	}
	assert.Equal(t, int64(20), signCount)
}

// ============================================
// Race Condition Tests
// ============================================

// TestSign_RaceCondition tests for race conditions with concurrent Sign calls.
// Run with: go test -race -run TestSign_RaceCondition
func TestSign_RaceCondition(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		sig := make([]byte, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(sig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Create a single key
	_ = kr.store.Save(&KeyMetadata{
		UID:         "race-test",
		PubKeyBytes: testPubKeyBytes(),
		Address:     "cosmos1race",
	})

	// Hammer the same key from many goroutines
	const goroutines = 50
	const signsPerGoroutine = 10

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < signsPerGoroutine; j++ {
				msg := []byte(fmt.Sprintf("msg-%d-%d", id, j))
				_, _, err := kr.Sign("race-test", msg, signing.SignMode_SIGN_MODE_DIRECT)
				if err != nil {
					t.Errorf("sign error in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}
	wg.Wait()
}

// TestStore_RaceCondition tests for race conditions with concurrent store access.
// Run with: go test -race -run TestStore_RaceCondition
func TestStore_RaceCondition(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Pre-populate with keys
	for i := 0; i < 10; i++ {
		_ = kr.store.Save(&KeyMetadata{
			UID:         fmt.Sprintf("key-%d", i),
			PubKeyBytes: testPubKeyBytes(),
			Address:     fmt.Sprintf("addr%d", i),
		})
	}

	// Concurrent reads and writes
	var wg sync.WaitGroup

	// Readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = kr.store.List()
				_, _ = kr.store.Get(fmt.Sprintf("key-%d", j%10))
			}
		}()
	}

	// Writers (renames - involves read + delete + write)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// Just test Has() which is a common concurrent operation
				_ = kr.store.Has(fmt.Sprintf("key-%d", j))
			}
		}(i)
	}

	wg.Wait()
}

// TestCreateBatch_RaceCondition tests for race conditions in batch creation.
func TestCreateBatch_RaceCondition(t *testing.T) {
	var createCount int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/keys/") {
			idx := atomic.AddInt64(&createCount, 1)
			pubKeyHex := fmt.Sprintf("02%064d", idx)
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       fmt.Sprintf("key-%d", idx),
					"public_key": pubKeyHex,
					"address":    fmt.Sprintf("addr%d", idx),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Run multiple batch creates concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()
			_, _ = kr.CreateBatch(context.Background(), CreateBatchOptions{
				Prefix: fmt.Sprintf("batch-%d", batchID),
				Count:  4,
			})
		}(i)
	}
	wg.Wait()
}

