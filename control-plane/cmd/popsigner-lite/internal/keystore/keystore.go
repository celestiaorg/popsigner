package keystore

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"
)

// Keystore is an in-memory key storage for popsigner-lite.
// Thread-safe for concurrent access.
type Keystore struct {
	keys    map[string]*Key  // address -> key
	apiKeys map[string]*APIKey
	mu      sync.RWMutex
}

// Key represents a cryptographic key pair.
type Key struct {
	ID         string
	Name       string
	Address    string // 0x... Ethereum address
	PrivateKey *ecdsa.PrivateKey
	PublicKey  []byte
	CreatedAt  time.Time
}

// APIKey represents an API key for authentication.
type APIKey struct {
	ID        string
	Name      string
	KeyHash   string
	Scopes    []string
	CreatedAt time.Time
}

// NewKeystore creates a new in-memory keystore.
func NewKeystore() *Keystore {
	return &Keystore{
		keys:    make(map[string]*Key),
		apiKeys: make(map[string]*APIKey),
	}
}

// AddKey adds a key to the keystore.
func (k *Keystore) AddKey(key *Key) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, exists := k.keys[key.Address]; exists {
		return fmt.Errorf("key with address %s already exists", key.Address)
	}

	k.keys[key.Address] = key
	return nil
}

// GetKey retrieves a key by address.
func (k *Keystore) GetKey(address string) (*Key, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	key, exists := k.keys[address]
	if !exists {
		return nil, fmt.Errorf("key with address %s not found", address)
	}

	return key, nil
}

// GetKeyByID retrieves a key by ID.
func (k *Keystore) GetKeyByID(id string) (*Key, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	for _, key := range k.keys {
		if key.ID == id {
			return key, nil
		}
	}

	return nil, fmt.Errorf("key with ID %s not found", id)
}

// ListKeys returns all keys in the keystore.
func (k *Keystore) ListKeys() []*Key {
	k.mu.RLock()
	defer k.mu.RUnlock()

	keys := make([]*Key, 0, len(k.keys))
	for _, key := range k.keys {
		keys = append(keys, key)
	}

	return keys
}

// DeleteKey removes a key from the keystore.
func (k *Keystore) DeleteKey(address string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, exists := k.keys[address]; !exists {
		return fmt.Errorf("key with address %s not found", address)
	}

	delete(k.keys, address)
	return nil
}
