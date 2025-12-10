package banhbaoring

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// BaoStore manages local key metadata with atomic file persistence.
type BaoStore struct {
	mu    sync.RWMutex
	path  string
	data  *StoreData
	dirty bool
}

// NewBaoStore creates or opens a store at the given path.
// If the file doesn't exist, a new empty store is created.
// If the directory doesn't exist, it is created with 0700 permissions.
func NewBaoStore(path string) (*BaoStore, error) {
	store := &BaoStore{
		path: path,
		data: &StoreData{
			Version: DefaultStoreVersion,
			Keys:    make(map[string]*KeyMetadata),
		},
	}

	// Create directory with restricted permissions
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Load existing store file
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return store, nil
}

// load reads store data from disk.
// Returns os.ErrNotExist if file doesn't exist (which is not an error for new stores).
func (s *BaoStore) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Empty file is valid - treat as empty store
	if len(data) == 0 {
		return nil
	}

	var storeData StoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return fmt.Errorf("%w: %v", ErrStoreCorrupted, err)
	}

	// Version validation
	if storeData.Version > DefaultStoreVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrStoreCorrupted, storeData.Version)
	}

	// Ensure Keys map is initialized
	if storeData.Keys == nil {
		storeData.Keys = make(map[string]*KeyMetadata)
	}

	s.data = &storeData
	s.dirty = false
	return nil
}

// syncLocked writes store data atomically using temp file + rename pattern.
// Must be called with write lock held.
func (s *BaoStore) syncLocked() error {
	if !s.dirty {
		return nil
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmpPath := s.path + ".tmp"

	// Create temp file with restricted permissions
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("%w: create temp: %v", ErrStorePersist, err)
	}

	// Write data
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: write: %v", ErrStorePersist, err)
	}

	// Fsync to ensure data is on disk before rename
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: fsync: %v", ErrStorePersist, err)
	}

	// Close before rename
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: close: %v", ErrStorePersist, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: rename: %v", ErrStorePersist, err)
	}

	s.dirty = false
	return nil
}

// Sync flushes pending changes to disk.
func (s *BaoStore) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked()
}

// Close syncs any pending changes and releases resources.
func (s *BaoStore) Close() error {
	return s.Sync()
}

// Path returns the store file path.
func (s *BaoStore) Path() string {
	return s.path
}

// Save stores key metadata.
func (s *BaoStore) Save(meta *KeyMetadata) error {
	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}
	if meta.UID == "" {
		return fmt.Errorf("metadata UID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, exists := s.data.Keys[meta.UID]; exists {
		if existing.Address != meta.Address {
			return fmt.Errorf("%w: %s", ErrKeyExists, meta.UID)
		}
	}

	s.data.Keys[meta.UID] = meta
	s.dirty = true
	return s.syncLocked()
}

// Get retrieves metadata by UID.
func (s *BaoStore) Get(uid string) (*KeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, exists := s.data.Keys[uid]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, uid)
	}
	return copyMetadata(meta), nil
}

// GetByAddress retrieves metadata by address.
func (s *BaoStore) GetByAddress(address string) (*KeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, meta := range s.data.Keys {
		if meta.Address == address {
			return copyMetadata(meta), nil
		}
	}
	return nil, fmt.Errorf("%w: address %s", ErrKeyNotFound, address)
}

// List returns all metadata.
func (s *BaoStore) List() ([]*KeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*KeyMetadata, 0, len(s.data.Keys))
	for _, meta := range s.data.Keys {
		result = append(result, copyMetadata(meta))
	}
	return result, nil
}

// Delete removes metadata.
func (s *BaoStore) Delete(uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data.Keys[uid]; !exists {
		return fmt.Errorf("%w: %s", ErrKeyNotFound, uid)
	}

	delete(s.data.Keys, uid)
	s.dirty = true
	return s.syncLocked()
}

// Rename changes the UID.
func (s *BaoStore) Rename(oldUID, newUID string) error {
	if oldUID == newUID {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, exists := s.data.Keys[oldUID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrKeyNotFound, oldUID)
	}
	if _, exists := s.data.Keys[newUID]; exists {
		return fmt.Errorf("%w: %s", ErrKeyExists, newUID)
	}

	meta.UID = newUID
	meta.Name = newUID
	s.data.Keys[newUID] = meta
	delete(s.data.Keys, oldUID)
	s.dirty = true
	return s.syncLocked()
}

// Has checks existence.
func (s *BaoStore) Has(uid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Keys[uid] != nil
}

// Count returns key count.
func (s *BaoStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.Keys)
}

// ForEach iterates over all keys with a callback.
func (s *BaoStore) ForEach(fn func(uid string, meta *KeyMetadata) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for uid, meta := range s.data.Keys {
		if err := fn(uid, copyMetadata(meta)); err != nil {
			return err
		}
	}
	return nil
}

// copyMetadata creates a deep copy of KeyMetadata.
func copyMetadata(meta *KeyMetadata) *KeyMetadata {
	if meta == nil {
		return nil
	}
	cp := *meta
	if meta.PubKeyBytes != nil {
		cp.PubKeyBytes = make([]byte, len(meta.PubKeyBytes))
		copy(cp.PubKeyBytes, meta.PubKeyBytes)
	}
	return &cp
}

// newStoreForTesting creates a store for testing without file operations.
// This is used only in tests to bypass the file-based NewBaoStore.
func newStoreForTesting() *BaoStore {
	return &BaoStore{
		path: "",
		data: &StoreData{
			Version: DefaultStoreVersion,
			Keys:    make(map[string]*KeyMetadata),
		},
		dirty: false,
	}
}
