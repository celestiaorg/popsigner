package secp256k1

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory_CreatesBackendSuccessfully(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)

	require.NoError(t, err)
	require.NotNil(t, b)

	backend, ok := b.(*backend)
	require.True(t, ok, "expected *backend type")
	require.NotNil(t, backend.keyCache)
}

func TestFactory_BackendHasCorrectType(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)

	backend := b.(*backend)
	assert.Equal(t, logical.TypeLogical, backend.BackendType)
}

func TestFactory_SealWrapConfigured(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)

	backend := b.(*backend)
	require.NotNil(t, backend.PathsSpecial)
	assert.Contains(t, backend.PathsSpecial.SealWrapStorage, "keys/")
}

func TestBackend_HelpString(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)

	backend := b.(*backend)
	assert.Contains(t, backend.Help, "secp256k1")
	assert.Contains(t, backend.Help, "Cosmos")
}

func TestBackendCache_SetAndGet(t *testing.T) {
	b, _ := getTestBackend(t)

	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
		Exportable: true,
		CreatedAt:  time.Now(),
		Imported:   false,
	}

	b.setKeyInCache("test-key", entry)
	cached := b.getKeyFromCache("test-key")

	require.NotNil(t, cached)
	assert.Equal(t, entry.PublicKey, cached.PublicKey)
	assert.Equal(t, entry.Exportable, cached.Exportable)
}

func TestBackendCache_GetNonExistent(t *testing.T) {
	b, _ := getTestBackend(t)

	cached := b.getKeyFromCache("non-existent")
	assert.Nil(t, cached)
}

func TestBackendCache_Delete(t *testing.T) {
	b, _ := getTestBackend(t)

	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
	}

	b.setKeyInCache("test-key", entry)
	require.NotNil(t, b.getKeyFromCache("test-key"))

	b.deleteKeyFromCache("test-key")
	assert.Nil(t, b.getKeyFromCache("test-key"))
}

func TestBackendCache_DeleteNonExistent(t *testing.T) {
	b, _ := getTestBackend(t)

	// Should not panic
	b.deleteKeyFromCache("non-existent")
}

func TestBackendCache_Clear(t *testing.T) {
	b, _ := getTestBackend(t)

	// Add multiple keys
	for i := 0; i < 5; i++ {
		entry := &keyEntry{
			PrivateKey: []byte("test-private-key-32-bytes-long!"),
			PublicKey:  []byte("test-pub-key"),
		}
		b.setKeyInCache("test-key-"+string(rune('a'+i)), entry)
	}

	// Verify keys are cached
	assert.NotNil(t, b.getKeyFromCache("test-key-a"))

	// Clear cache
	b.clearCache()

	// Verify all keys are removed
	assert.Nil(t, b.getKeyFromCache("test-key-a"))
	assert.Nil(t, b.getKeyFromCache("test-key-b"))
}

func TestBackendCache_NilEntry(t *testing.T) {
	b, _ := getTestBackend(t)

	// Set nil entry
	b.setKeyInCache("nil-key", nil)

	// Delete should not panic
	b.deleteKeyFromCache("nil-key")
}

func TestBackend_Invalidate_AllKeys(t *testing.T) {
	b, _ := getTestBackend(t)

	// Add some keys to cache
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
	}
	b.setKeyInCache("key1", entry)
	b.setKeyInCache("key2", entry)

	// Invalidate all keys
	b.invalidate(context.Background(), "keys/")

	// Cache should be empty
	assert.Nil(t, b.getKeyFromCache("key1"))
	assert.Nil(t, b.getKeyFromCache("key2"))
}

func TestBackend_Invalidate_SpecificKey(t *testing.T) {
	b, _ := getTestBackend(t)

	// Add some keys to cache
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
	}
	b.setKeyInCache("key1", entry)
	b.setKeyInCache("key2", entry)

	// Invalidate specific key
	b.invalidate(context.Background(), "keys/key1")

	// Only key1 should be removed
	assert.Nil(t, b.getKeyFromCache("key1"))
	assert.NotNil(t, b.getKeyFromCache("key2"))
}

func TestBackend_Invalidate_UnrelatedPath(t *testing.T) {
	b, _ := getTestBackend(t)

	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
	}
	b.setKeyInCache("key1", entry)

	// Invalidate unrelated path
	b.invalidate(context.Background(), "config/")

	// Key should still be cached
	assert.NotNil(t, b.getKeyFromCache("key1"))
}

func TestBackend_Cleanup(t *testing.T) {
	b, _ := getTestBackend(t)

	// Add keys with private data
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key"),
	}
	b.setKeyInCache("key1", entry)
	b.setKeyInCache("key2", entry)

	// Cleanup
	b.cleanup(context.Background())

	// Cache should be empty
	assert.Nil(t, b.getKeyFromCache("key1"))
	assert.Nil(t, b.getKeyFromCache("key2"))
}

func TestBackend_Cleanup_NilEntries(t *testing.T) {
	b, _ := getTestBackend(t)

	b.setKeyInCache("nil-key", nil)

	// Should not panic
	b.cleanup(context.Background())
}

func TestBackend_Cleanup_NilPrivateKey(t *testing.T) {
	b, _ := getTestBackend(t)

	entry := &keyEntry{
		PrivateKey: nil,
		PublicKey:  []byte("test-pub-key"),
	}
	b.setKeyInCache("no-priv-key", entry)

	// Should not panic
	b.cleanup(context.Background())
}

func TestBackend_ConcurrentAccess(t *testing.T) {
	b, _ := getTestBackend(t)

	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			entry := &keyEntry{
				PrivateKey: []byte("test-private-key-32-bytes-long!"),
				PublicKey:  []byte("test-pub-key"),
			}
			b.setKeyInCache("concurrent-key", entry)
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = b.getKeyFromCache("concurrent-key")
		}
	}()

	// Deleter goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			b.deleteKeyFromCache("concurrent-key")
		}
	}()

	wg.Wait()
}

func TestBackend_PathsRegistered(t *testing.T) {
	b, _ := getTestBackend(t)

	// Backend should have paths registered (even if they're empty stubs)
	assert.NotNil(t, b.Backend)
}

func TestBackend_KeyEntryFields(t *testing.T) {
	b, _ := getTestBackend(t)

	now := time.Now()
	entry := &keyEntry{
		PrivateKey: []byte("32-byte-private-key-for-testing!"),
		PublicKey:  []byte{0x02, 0x03, 0x04},
		Exportable: true,
		CreatedAt:  now,
		Imported:   true,
	}

	b.setKeyInCache("full-entry", entry)
	cached := b.getKeyFromCache("full-entry")

	require.NotNil(t, cached)
	assert.Equal(t, entry.PrivateKey, cached.PrivateKey)
	assert.Equal(t, entry.PublicKey, cached.PublicKey)
	assert.Equal(t, entry.Exportable, cached.Exportable)
	assert.Equal(t, entry.CreatedAt, cached.CreatedAt)
	assert.Equal(t, entry.Imported, cached.Imported)
}

func TestBackend_CacheOverwrite(t *testing.T) {
	b, _ := getTestBackend(t)

	entry1 := &keyEntry{
		PrivateKey: []byte("first-private-key-32-bytes-long!"),
		PublicKey:  []byte("first-pub"),
		Exportable: false,
	}
	entry2 := &keyEntry{
		PrivateKey: []byte("second-private-key-32-bytes-lon!"),
		PublicKey:  []byte("second-pub"),
		Exportable: true,
	}

	b.setKeyInCache("key", entry1)
	assert.Equal(t, []byte("first-pub"), b.getKeyFromCache("key").PublicKey)

	b.setKeyInCache("key", entry2)
	cached := b.getKeyFromCache("key")
	assert.Equal(t, []byte("second-pub"), cached.PublicKey)
	assert.True(t, cached.Exportable)
}

func TestBackend_ClearEmptyCache(t *testing.T) {
	b, _ := getTestBackend(t)

	// Clear empty cache should not panic
	b.clearCache()

	// Cache should still be usable
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub"),
	}
	b.setKeyInCache("new-key", entry)
	assert.NotNil(t, b.getKeyFromCache("new-key"))
}

func TestBackend_InvalidateEmptyCache(t *testing.T) {
	b, _ := getTestBackend(t)

	// Invalidating empty cache should not panic
	b.invalidate(context.Background(), "keys/")
	b.invalidate(context.Background(), "keys/some-key")
}

func TestBackend_MultipleInvalidations(t *testing.T) {
	b, _ := getTestBackend(t)

	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub"),
	}

	// Add keys
	b.setKeyInCache("key1", entry)
	b.setKeyInCache("key2", entry)
	b.setKeyInCache("key3", entry)

	// Multiple sequential invalidations
	b.invalidate(context.Background(), "keys/key1")
	assert.Nil(t, b.getKeyFromCache("key1"))
	assert.NotNil(t, b.getKeyFromCache("key2"))

	b.invalidate(context.Background(), "keys/key2")
	assert.Nil(t, b.getKeyFromCache("key2"))
	assert.NotNil(t, b.getKeyFromCache("key3"))

	b.invalidate(context.Background(), "keys/")
	assert.Nil(t, b.getKeyFromCache("key3"))
}

func TestBackend_SecureZeroOnDelete(t *testing.T) {
	b, _ := getTestBackend(t)

	// Create a key with known private key bytes
	privKey := make([]byte, 32)
	for i := range privKey {
		privKey[i] = byte(i)
	}

	entry := &keyEntry{
		PrivateKey: privKey,
		PublicKey:  []byte("test-pub"),
	}

	b.setKeyInCache("secure-delete", entry)

	// Capture reference to private key before delete
	cached := b.getKeyFromCache("secure-delete")
	require.NotNil(t, cached)

	// Delete the key
	b.deleteKeyFromCache("secure-delete")

	// The original entry's private key should be zeroed
	// (checking the entry we retrieved, which is the same reference)
	allZeros := true
	for _, b := range cached.PrivateKey {
		if b != 0 {
			allZeros = false
			break
		}
	}
	assert.True(t, allZeros, "private key should be zeroed after delete")
}

func TestBackend_CleanHandler(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)

	backend := b.(*backend)
	assert.NotNil(t, backend.Clean, "backend should have Clean handler")
}

func TestBackend_InvalidateHandler(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)

	backend := b.(*backend)
	assert.NotNil(t, backend.Invalidate, "backend should have Invalidate handler")
}

func TestBackend_GetKey_FromStorage(t *testing.T) {
	b, storage := getTestBackend(t)
	ctx := context.Background()

	// Create a key directly in storage
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("test-pub-key-33-bytes-compressed"),
		Exportable: true,
		CreatedAt:  time.Now(),
		Imported:   false,
	}

	storageEntry, err := logical.StorageEntryJSON("keys/storage-test", entry)
	require.NoError(t, err)
	require.NoError(t, storage.Put(ctx, storageEntry))

	// Get key should load from storage
	retrieved, err := b.getKey(ctx, storage, "storage-test")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, entry.PublicKey, retrieved.PublicKey)
	assert.Equal(t, entry.Exportable, retrieved.Exportable)

	// Second get should use cache
	cached := b.getKeyFromCache("storage-test")
	require.NotNil(t, cached)
	assert.Equal(t, entry.PublicKey, cached.PublicKey)
}

func TestBackend_GetKey_NotFound(t *testing.T) {
	b, storage := getTestBackend(t)
	ctx := context.Background()

	// Get non-existent key
	retrieved, err := b.getKey(ctx, storage, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestBackend_GetKey_FromCache(t *testing.T) {
	b, storage := getTestBackend(t)
	ctx := context.Background()

	// Put key in cache
	entry := &keyEntry{
		PrivateKey: []byte("test-private-key-32-bytes-long!"),
		PublicKey:  []byte("cached-pub-key"),
		Exportable: false,
	}
	b.setKeyInCache("cached-key", entry)

	// Get key should return from cache
	retrieved, err := b.getKey(ctx, storage, "cached-key")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, entry.PublicKey, retrieved.PublicKey)
}
