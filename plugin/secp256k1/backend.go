package secp256k1

import (
	"context"
	"strings"
	"sync"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

// Factory creates a new secp256k1 secrets engine backend.
// This is the entry point called by OpenBao when the plugin is loaded.
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := &backend{
		keyCache: make(map[string]*keyEntry),
	}

	b.Backend = &framework.Backend{
		Help:        strings.TrimSpace(backendHelp),
		BackendType: logical.TypeLogical,
		Paths: framework.PathAppend(
			pathKeys(b),
			pathSign(b),
			pathVerify(b),
			pathImport(b),
			pathExport(b),
		),
		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{"keys/"},
		},
		Invalidate: b.invalidate,
		Clean:      b.cleanup,
	}

	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}

	return b, nil
}

// backend is the secp256k1 secrets engine backend.
// It manages secp256k1 private keys and provides signing operations.
type backend struct {
	*framework.Backend
	cacheMu  sync.RWMutex
	keyCache map[string]*keyEntry
}

// invalidate is called when a watched key is modified.
// This clears the key cache when storage is modified externally.
func (b *backend) invalidate(ctx context.Context, key string) {
	if strings.HasPrefix(key, "keys/") {
		b.cacheMu.Lock()
		defer b.cacheMu.Unlock()
		// Clear the specific key from cache, or all keys if it's just "keys/"
		if key == "keys/" {
			b.keyCache = make(map[string]*keyEntry)
		} else {
			// Extract key name from path "keys/name"
			keyName := strings.TrimPrefix(key, "keys/")
			delete(b.keyCache, keyName)
		}
	}
}

// cleanup is called when the backend is being shut down.
// It clears all cached keys from memory securely.
func (b *backend) cleanup(ctx context.Context) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()

	// Securely wipe all cached private keys
	for name, entry := range b.keyCache {
		if entry != nil && entry.PrivateKey != nil {
			secureZero(entry.PrivateKey)
		}
		delete(b.keyCache, name)
	}
	b.keyCache = make(map[string]*keyEntry)
}

// getKeyFromCache retrieves a key from the cache.
// Returns nil if the key is not cached.
func (b *backend) getKeyFromCache(name string) *keyEntry {
	b.cacheMu.RLock()
	defer b.cacheMu.RUnlock()
	return b.keyCache[name]
}

// setKeyInCache stores a key in the cache.
func (b *backend) setKeyInCache(name string, entry *keyEntry) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	b.keyCache[name] = entry
}

// deleteKeyFromCache removes a key from the cache.
func (b *backend) deleteKeyFromCache(name string) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	if entry, ok := b.keyCache[name]; ok {
		if entry != nil && entry.PrivateKey != nil {
			secureZero(entry.PrivateKey)
		}
		delete(b.keyCache, name)
	}
}

// clearCache removes all keys from the cache.
func (b *backend) clearCache() {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	for name, entry := range b.keyCache {
		if entry != nil && entry.PrivateKey != nil {
			secureZero(entry.PrivateKey)
		}
		delete(b.keyCache, name)
	}
	b.keyCache = make(map[string]*keyEntry)
}

// getKey retrieves a key from cache or storage.
// This is used by other path handlers to access key entries.
func (b *backend) getKey(ctx context.Context, storage logical.Storage, name string) (*keyEntry, error) {
	// Check cache first
	if entry := b.getKeyFromCache(name); entry != nil {
		return entry, nil
	}

	// Load from storage
	raw, err := storage.Get(ctx, "keys/"+name)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var entry keyEntry
	if err := raw.DecodeJSON(&entry); err != nil {
		return nil, err
	}

	// Cache the entry
	b.setKeyInCache(name, &entry)

	return &entry, nil
}

const backendHelp = `
The secp256k1 secrets engine provides native secp256k1 key management
and signing for Cosmos/Celestia blockchain applications.

Private keys are generated and stored securely within OpenBao, never
leaving the secure boundary. All cryptographic operations are performed
server-side.

Features:
  - Generate secp256k1 keypairs
  - Sign messages with ECDSA (Cosmos-compatible R||S format)
  - Verify signatures
  - Import/export keys (when marked exportable)
  - Derive Cosmos addresses from public keys

Paths:
  keys/           - List all keys
  keys/:name      - Create, read, or delete a named key
  sign/:name      - Sign data with a named key
  verify/:name    - Verify a signature with a named key
  import/:name    - Import an external key
  export/:name    - Export a key (if marked exportable)
`
