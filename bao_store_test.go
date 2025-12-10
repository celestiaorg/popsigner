package banhbaoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBaoStore_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "subdir", "nested", "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Verify directory was created with correct permissions
	info, err := os.Stat(filepath.Dir(storePath))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify store path is set correctly
	assert.Equal(t, storePath, store.Path())
}

func TestNewBaoStore_InitializesEmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Verify internal state
	assert.NotNil(t, store.data)
	assert.Equal(t, DefaultStoreVersion, store.data.Version)
	assert.NotNil(t, store.data.Keys)
	assert.Len(t, store.data.Keys, 0)
}

func TestNewBaoStore_LoadsExistingStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Create an existing store file
	storeData := StoreData{
		Version: DefaultStoreVersion,
		Keys: map[string]*KeyMetadata{
			"test-key": {
				UID:         "test-key",
				Name:        "test-key",
				PubKeyBytes: []byte{0x02, 0x03},
				Address:     "cosmos1abc",
				Algorithm:   AlgorithmSecp256k1,
				CreatedAt:   time.Now(),
				Source:      SourceGenerated,
			},
		},
	}

	data, err := json.MarshalIndent(storeData, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(storePath, data, 0600)
	require.NoError(t, err)

	// Open the store
	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Verify data was loaded
	assert.Equal(t, DefaultStoreVersion, store.data.Version)
	assert.Len(t, store.data.Keys, 1)
	assert.Contains(t, store.data.Keys, "test-key")
	assert.Equal(t, "cosmos1abc", store.data.Keys["test-key"].Address)
}

func TestNewBaoStore_HandlesEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Create empty file
	err := os.WriteFile(storePath, []byte{}, 0600)
	require.NoError(t, err)

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Should initialize with defaults
	assert.Equal(t, DefaultStoreVersion, store.data.Version)
	assert.NotNil(t, store.data.Keys)
}

func TestNewBaoStore_DetectsCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Write invalid JSON
	err := os.WriteFile(storePath, []byte("not valid json{{{"), 0600)
	require.NoError(t, err)

	store, err := NewBaoStore(storePath)
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.ErrorIs(t, err, ErrStoreCorrupted)
}

func TestNewBaoStore_RejectsUnsupportedVersion(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Create store with future version
	storeData := StoreData{
		Version: DefaultStoreVersion + 100, // Future version
		Keys:    make(map[string]*KeyMetadata),
	}
	data, err := json.Marshal(storeData)
	require.NoError(t, err)
	err = os.WriteFile(storePath, data, 0600)
	require.NoError(t, err)

	store, err := NewBaoStore(storePath)
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.ErrorIs(t, err, ErrStoreCorrupted)
}

func TestNewBaoStore_InitializesNilKeysMap(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Create store with null keys
	err := os.WriteFile(storePath, []byte(`{"version":1,"keys":null}`), 0600)
	require.NoError(t, err)

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Keys should be initialized to empty map
	assert.NotNil(t, store.data.Keys)
}

func TestBaoStore_Sync_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Mark as dirty and add data
	store.data.Keys["test"] = &KeyMetadata{
		UID:     "test",
		Name:    "test",
		Address: "cosmos1xyz",
	}
	store.dirty = true

	// Sync to disk
	err = store.Sync()
	require.NoError(t, err)

	// Verify file exists with correct data
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var loaded StoreData
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Contains(t, loaded.Keys, "test")

	// Verify no temp file left behind
	_, err = os.Stat(storePath + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should be removed after sync")
}

func TestBaoStore_Sync_SkipsWhenNotDirty(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Store is not dirty (default)
	assert.False(t, store.dirty)

	// Sync should succeed without writing file
	err = store.Sync()
	require.NoError(t, err)

	// File should not exist (never written because not dirty)
	_, err = os.Stat(storePath)
	assert.True(t, os.IsNotExist(err))
}

func TestBaoStore_Sync_ClearsDirtyFlag(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	store.dirty = true
	err = store.Sync()
	require.NoError(t, err)

	assert.False(t, store.dirty)
}

func TestBaoStore_Close_SyncsData(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Add data and mark dirty
	store.data.Keys["close-test"] = &KeyMetadata{
		UID:     "close-test",
		Name:    "close-test",
		Address: "cosmos1close",
	}
	store.dirty = true

	// Close should sync
	err = store.Close()
	require.NoError(t, err)

	// Verify data was persisted
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var loaded StoreData
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Contains(t, loaded.Keys, "close-test")
}

func TestBaoStore_Path_ReturnsCorrectPath(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "my-store.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	assert.Equal(t, storePath, store.Path())
}

func TestBaoStore_PersistenceAcrossRestart(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "persist-test.json")

	// Create and populate store
	store1, err := NewBaoStore(storePath)
	require.NoError(t, err)

	testKey := &KeyMetadata{
		UID:         "persist-key",
		Name:        "persist-key",
		PubKeyBytes: []byte{0x02, 0x03, 0x04},
		PubKeyType:  "secp256k1",
		Address:     "cosmos1persist",
		BaoKeyPath:  "secp256k1/keys/persist-key",
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  true,
		CreatedAt:   time.Now().Truncate(time.Millisecond), // Truncate for comparison
		Source:      SourceImported,
	}
	store1.data.Keys[testKey.UID] = testKey
	store1.dirty = true

	err = store1.Close()
	require.NoError(t, err)

	// "Restart" - open new store instance
	store2, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Verify data persisted
	assert.Len(t, store2.data.Keys, 1)
	loaded := store2.data.Keys["persist-key"]
	require.NotNil(t, loaded)
	assert.Equal(t, testKey.UID, loaded.UID)
	assert.Equal(t, testKey.Name, loaded.Name)
	assert.Equal(t, testKey.Address, loaded.Address)
	assert.Equal(t, testKey.Algorithm, loaded.Algorithm)
	assert.Equal(t, testKey.Exportable, loaded.Exportable)
	assert.Equal(t, testKey.Source, loaded.Source)
}

func TestBaoStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "perms-test.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Write data
	store.data.Keys["test"] = &KeyMetadata{UID: "test"}
	store.dirty = true
	err = store.Sync()
	require.NoError(t, err)

	// Check file permissions (should be 0600)
	info, err := os.Stat(storePath)
	require.NoError(t, err)

	// On Unix, check that group and other have no permissions
	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), perm, "file should have 0600 permissions")
}

func TestBaoStore_ConcurrentSync(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "concurrent.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Add initial data
	store.data.Keys["initial"] = &KeyMetadata{UID: "initial"}
	store.dirty = true

	// Run concurrent syncs
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Mark dirty and sync
			store.mu.Lock()
			store.dirty = true
			store.mu.Unlock()

			if err := store.Sync(); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		t.Errorf("concurrent sync error: %v", err)
	}

	// Verify no temp file left
	_, err = os.Stat(storePath + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestBaoStore_SyncLocked_NoTempFileOnError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create read-only directory to cause write error
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.MkdirAll(readOnlyDir, 0500)
	require.NoError(t, err)

	storePath := filepath.Join(readOnlyDir, "store.json")

	// Create store but it won't be able to write
	store := &BaoStore{
		path: storePath,
		data: &StoreData{
			Version: DefaultStoreVersion,
			Keys:    make(map[string]*KeyMetadata),
		},
		dirty: true,
	}

	// Sync should fail
	err = store.Sync()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrStorePersist)

	// Verify no temp file left behind
	_, err = os.Stat(storePath + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestBaoStore_Load_MissingVersionField(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "no-version.json")

	// Create file without version (defaults to 0)
	err := os.WriteFile(storePath, []byte(`{"keys":{}}`), 0600)
	require.NoError(t, err)

	// Should load successfully (version 0 <= DefaultStoreVersion)
	store, err := NewBaoStore(storePath)
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.Equal(t, 0, store.data.Version)
}

func TestBaoStore_MultipleCloseIsSafe(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "multi-close.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Multiple closes should be safe
	err = store.Close()
	assert.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err)
}

func TestBaoStore_JSONMarshalIndent(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "indent.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	store.data.Keys["test"] = &KeyMetadata{
		UID:     "test",
		Name:    "test",
		Address: "cosmos1test",
	}
	store.dirty = true

	err = store.Sync()
	require.NoError(t, err)

	// Read raw file and verify it's indented
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	// Should contain newlines (indented format)
	assert.Contains(t, string(data), "\n")
	assert.Contains(t, string(data), "  ") // Two-space indent
}

func TestNewBaoStore_DirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "new-dir", "store.json")

	_, err := NewBaoStore(nestedPath)
	require.NoError(t, err)

	// Check directory permissions (should be 0700)
	info, err := os.Stat(filepath.Join(tmpDir, "new-dir"))
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0700), perm, "directory should have 0700 permissions")
}

func TestBaoStore_Load_PreservesAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "full-fields.json")

	// Create store with all fields populated
	now := time.Now().Truncate(time.Second)
	original := StoreData{
		Version: DefaultStoreVersion,
		Keys: map[string]*KeyMetadata{
			"full-key": {
				UID:         "full-key",
				Name:        "my-key-name",
				PubKeyBytes: []byte{0x02, 0x03, 0x04, 0x05},
				PubKeyType:  "secp256k1",
				Address:     "cosmos1fulladdr",
				BaoKeyPath:  "secp256k1/keys/my-key-name",
				Algorithm:   AlgorithmSecp256k1,
				Exportable:  true,
				CreatedAt:   now,
				Source:      SourceGenerated,
			},
		},
	}

	data, err := json.MarshalIndent(original, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(storePath, data, 0600)
	require.NoError(t, err)

	// Load store
	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Verify all fields
	loaded := store.data.Keys["full-key"]
	require.NotNil(t, loaded)
	assert.Equal(t, "full-key", loaded.UID)
	assert.Equal(t, "my-key-name", loaded.Name)
	assert.Equal(t, []byte{0x02, 0x03, 0x04, 0x05}, loaded.PubKeyBytes)
	assert.Equal(t, "secp256k1", loaded.PubKeyType)
	assert.Equal(t, "cosmos1fulladdr", loaded.Address)
	assert.Equal(t, "secp256k1/keys/my-key-name", loaded.BaoKeyPath)
	assert.Equal(t, AlgorithmSecp256k1, loaded.Algorithm)
	assert.True(t, loaded.Exportable)
	assert.Equal(t, SourceGenerated, loaded.Source)
}

func TestBaoStore_Load_NotDirtyAfterLoad(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dirty-test.json")

	// Create existing store file
	storeData := StoreData{
		Version: DefaultStoreVersion,
		Keys: map[string]*KeyMetadata{
			"test-key": {
				UID:     "test-key",
				Address: "cosmos1test",
			},
		},
	}
	data, err := json.Marshal(storeData)
	require.NoError(t, err)
	err = os.WriteFile(storePath, data, 0600)
	require.NoError(t, err)

	// Load store
	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Should not be dirty after load
	assert.False(t, store.dirty)
}

func TestBaoStore_Sync_WritesVersionCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "version-test.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	store.dirty = true
	err = store.Sync()
	require.NoError(t, err)

	// Read and verify version
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var loaded StoreData
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Equal(t, DefaultStoreVersion, loaded.Version)
}

func TestBaoStore_ConcurrentReadsDuringSync(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "concurrent-read.json")

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Pre-populate with data
	for i := 0; i < 10; i++ {
		uid := "key" + string(rune('0'+i))
		store.data.Keys[uid] = &KeyMetadata{
			UID:     uid,
			Address: "addr" + string(rune('0'+i)),
		}
	}
	store.dirty = true

	var wg sync.WaitGroup

	// Start multiple sync goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				store.mu.Lock()
				store.dirty = true
				store.mu.Unlock()
				_ = store.Sync()
			}
		}()
	}

	// Start multiple read goroutines (simulating data access)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				store.mu.RLock()
				_ = len(store.data.Keys)
				store.mu.RUnlock()
			}
		}()
	}

	wg.Wait()

	// Final sync
	store.mu.Lock()
	store.dirty = true
	store.mu.Unlock()
	err = store.Sync()
	require.NoError(t, err)

	// Verify file is valid
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var loaded StoreData
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Equal(t, 10, len(loaded.Keys))
}

func TestBaoStore_NestedDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "store.json")

	store, err := NewBaoStore(deepPath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Verify all directories were created
	_, err = os.Stat(filepath.Dir(deepPath))
	require.NoError(t, err)
}

func TestBaoStore_ExistingDirectoryOK(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "store.json")

	// Create store twice - second time directory exists
	store1, err := NewBaoStore(storePath)
	require.NoError(t, err)
	store1.dirty = true
	err = store1.Close()
	require.NoError(t, err)

	// Second creation with existing directory should work
	store2, err := NewBaoStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store2)
}

// ============================================
// Agent 02A: CRUD Operation Tests
// ============================================

// testMetadata creates test metadata with given uid and address
func testMetadata(uid, address string) *KeyMetadata {
	return &KeyMetadata{
		UID:         uid,
		Name:        uid,
		PubKeyBytes: []byte("test-pubkey-" + uid),
		PubKeyType:  "secp256k1",
		Address:     address,
		BaoKeyPath:  "secp256k1/keys/" + uid,
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  true,
		CreatedAt:   time.Now(),
		Source:      SourceGenerated,
	}
}

// TestSave_Valid tests saving valid metadata
func TestSave_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")

	err = store.Save(meta)
	require.NoError(t, err)

	assert.Equal(t, 1, store.Count())
}

// TestSave_NilMetadata tests saving nil metadata returns error
func TestSave_NilMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	err = store.Save(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata cannot be nil")
}

// TestSave_EmptyUID tests saving metadata with empty UID
func TestSave_EmptyUID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("", "addr1")

	err = store.Save(meta)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata UID is required")
}

// TestSave_DuplicateKey tests saving duplicate key with different address
func TestSave_DuplicateKey(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta1 := testMetadata("key1", "addr1")
	meta2 := testMetadata("key1", "addr2") // Same UID, different address

	err = store.Save(meta1)
	require.NoError(t, err)

	err = store.Save(meta2)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyExists)
}

// TestSave_SameAddressAllowed tests that saving same key with same address is allowed (idempotent)
func TestSave_SameAddressAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")
	err = store.Save(meta)
	require.NoError(t, err)

	// Save again with same address should succeed
	meta2 := testMetadata("key1", "addr1")
	err = store.Save(meta2)
	require.NoError(t, err)
}

// TestGet_Existing tests retrieving existing key
func TestGet_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")
	_ = store.Save(meta)

	retrieved, err := store.Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "key1", retrieved.UID)
	assert.Equal(t, "addr1", retrieved.Address)
}

// TestGet_NotFound tests retrieving non-existing key
func TestGet_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_, err = store.Get("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

// TestGet_ReturnsCopy tests that Get returns a copy, not reference
func TestGet_ReturnsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")
	_ = store.Save(meta)

	retrieved1, _ := store.Get("key1")
	retrieved1.Name = "modified"
	retrieved1.PubKeyBytes[0] = 0xFF // Modify the byte slice

	retrieved2, _ := store.Get("key1")
	assert.NotEqual(t, "modified", retrieved2.Name)
	assert.NotEqual(t, byte(0xFF), retrieved2.PubKeyBytes[0])
}

// TestGetByAddress_Existing tests retrieving by address
func TestGetByAddress_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")
	_ = store.Save(meta)

	retrieved, err := store.GetByAddress("addr1")
	require.NoError(t, err)
	assert.Equal(t, "key1", retrieved.UID)
}

// TestGetByAddress_NotFound tests retrieving by non-existing address
func TestGetByAddress_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_, err = store.GetByAddress("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

// TestGetByAddress_ReturnsCopy tests that GetByAddress returns a copy
func TestGetByAddress_ReturnsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	meta := testMetadata("key1", "addr1")
	_ = store.Save(meta)

	retrieved1, _ := store.GetByAddress("addr1")
	retrieved1.Name = "modified"

	retrieved2, _ := store.GetByAddress("addr1")
	assert.NotEqual(t, "modified", retrieved2.Name)
}

// TestList_Empty tests listing empty store
func TestList_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	list, err := store.List()
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

// TestList_Multiple tests listing multiple keys
func TestList_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))
	_ = store.Save(testMetadata("key2", "addr2"))
	_ = store.Save(testMetadata("key3", "addr3"))

	list, err := store.List()
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

// TestList_ReturnsCopies tests that List returns copies
func TestList_ReturnsCopies(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))

	list1, _ := store.List()
	list1[0].Name = "modified"

	list2, _ := store.List()
	assert.NotEqual(t, "modified", list2[0].Name)
}

// TestDelete_Existing tests deleting existing key
func TestDelete_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))

	err = store.Delete("key1")
	require.NoError(t, err)

	assert.False(t, store.Has("key1"))
	assert.Equal(t, 0, store.Count())
}

// TestDelete_NotFound tests deleting non-existing key
func TestDelete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	err = store.Delete("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

// TestRename_Success tests successful rename
func TestRename_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("oldkey", "addr1"))

	err = store.Rename("oldkey", "newkey")
	require.NoError(t, err)

	assert.False(t, store.Has("oldkey"))
	assert.True(t, store.Has("newkey"))

	meta, _ := store.Get("newkey")
	assert.Equal(t, "newkey", meta.UID)
	assert.Equal(t, "newkey", meta.Name)
}

// TestRename_SameUID tests renaming to same UID (no-op)
func TestRename_SameUID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))

	err = store.Rename("key1", "key1")
	require.NoError(t, err)
}

// TestRename_SourceNotFound tests renaming non-existing key
func TestRename_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	err = store.Rename("nonexistent", "newkey")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

// TestRename_TargetExists tests renaming to existing UID
func TestRename_TargetExists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))
	_ = store.Save(testMetadata("key2", "addr2"))

	err = store.Rename("key1", "key2")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyExists)
}

// TestHas tests existence check
func TestHas(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))

	assert.True(t, store.Has("key1"))
	assert.False(t, store.Has("nonexistent"))
}

// TestCount tests count functionality
func TestCount(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	assert.Equal(t, 0, store.Count())

	_ = store.Save(testMetadata("key1", "addr1"))
	assert.Equal(t, 1, store.Count())

	_ = store.Save(testMetadata("key2", "addr2"))
	assert.Equal(t, 2, store.Count())

	_ = store.Delete("key1")
	assert.Equal(t, 1, store.Count())
}

// TestForEach tests iteration over all keys
func TestForEach(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))
	_ = store.Save(testMetadata("key2", "addr2"))

	visited := make(map[string]bool)
	err = store.ForEach(func(uid string, meta *KeyMetadata) error {
		visited[uid] = true
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, visited, 2)
	assert.True(t, visited["key1"])
	assert.True(t, visited["key2"])
}

// TestForEach_Error tests ForEach stops on error
func TestForEach_Error(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))
	_ = store.Save(testMetadata("key2", "addr2"))

	testErr := assert.AnError
	err = store.ForEach(func(uid string, meta *KeyMetadata) error {
		return testErr
	})

	assert.Error(t, err)
}

// TestForEach_ReturnsCopies tests that ForEach passes copies
func TestForEach_ReturnsCopies(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	_ = store.Save(testMetadata("key1", "addr1"))

	_ = store.ForEach(func(uid string, meta *KeyMetadata) error {
		meta.Name = "modified"
		return nil
	})

	retrieved, _ := store.Get("key1")
	assert.NotEqual(t, "modified", retrieved.Name)
}

// TestConcurrentCRUD tests thread-safety of CRUD operations
func TestConcurrentCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			uid := "key" + string(rune('A'+i%26)) + string(rune('0'+i/26))
			addr := "addr" + string(rune('A'+i%26)) + string(rune('0'+i/26))
			meta := testMetadata(uid, addr)
			_ = store.Save(meta)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.List()
			_ = store.Count()
		}()
	}
	wg.Wait()

	// Store should be in consistent state
	count := store.Count()
	list, _ := store.List()
	assert.Equal(t, count, len(list))
}

// TestConcurrentReadWrite tests concurrent reads and writes
func TestConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewBaoStore(filepath.Join(tmpDir, "store.json"))
	require.NoError(t, err)

	// Pre-populate with some keys
	for i := 0; i < 10; i++ {
		uid := "prekey" + string(rune('0'+i))
		addr := "preaddr" + string(rune('0'+i))
		_ = store.Save(testMetadata(uid, addr))
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Start readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = store.Get("prekey0")
					_ = store.Has("prekey1")
					_, _ = store.List()
				}
			}
		}()
	}

	// Start writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				select {
				case <-done:
					return
				default:
					uid := "newkey" + string(rune('0'+i)) + string(rune('0'+j))
					addr := "newaddr" + string(rune('0'+i)) + string(rune('0'+j))
					_ = store.Save(testMetadata(uid, addr))
				}
			}
		}(i)
	}

	// Let it run for a short time
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	// Verify store is in consistent state
	count := store.Count()
	list, _ := store.List()
	assert.Equal(t, count, len(list))
}

// TestCopyMetadata_Nil tests copyMetadata with nil input
func TestCopyMetadata_Nil(t *testing.T) {
	result := copyMetadata(nil)
	assert.Nil(t, result)
}

// TestCopyMetadata_NilPubKeyBytes tests copyMetadata with nil PubKeyBytes
func TestCopyMetadata_NilPubKeyBytes(t *testing.T) {
	meta := &KeyMetadata{
		UID:         "key1",
		Name:        "key1",
		PubKeyBytes: nil,
		Address:     "addr1",
	}

	copied := copyMetadata(meta)
	assert.Nil(t, copied.PubKeyBytes)
}

// TestCopyMetadata_DeepCopy tests that PubKeyBytes is deep copied
func TestCopyMetadata_DeepCopy(t *testing.T) {
	meta := &KeyMetadata{
		UID:         "key1",
		Name:        "key1",
		PubKeyBytes: []byte{0x01, 0x02, 0x03},
		Address:     "addr1",
	}

	copied := copyMetadata(meta)

	// Modify original
	meta.PubKeyBytes[0] = 0xFF

	// Copied should be unchanged
	assert.NotEqual(t, byte(0xFF), copied.PubKeyBytes[0])
}

// TestNewStoreForTesting tests the test helper function
func TestNewStoreForTesting(t *testing.T) {
	store := newStoreForTesting()

	require.NotNil(t, store)
	require.NotNil(t, store.data)
	require.NotNil(t, store.data.Keys)
	assert.Equal(t, DefaultStoreVersion, store.data.Version)
	assert.False(t, store.dirty)
}
