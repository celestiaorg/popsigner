# Architecture Design: OpenBao Keyring Backend

## 1. Overview

This document describes the technical architecture for the OpenBao Keyring Backend, a custom `keyring.Keyring` implementation that delegates cryptographic operations to a **custom secp256k1 OpenBao plugin** for maximum security.

> **🎯 Target Users:** Rollup developers and operators building on Celestia who need Point of Presence signing - deployed next to their nodes - for their sequencers, provers, and bridge operators.

### ⚠️ CRITICAL: Celestia Fork Dependencies

This project uses **Celestia's forks** of the Cosmos SDK and Tendermint. The `keyring.Keyring` interface comes from `celestiaorg/cosmos-sdk`, NOT the upstream `cosmos/cosmos-sdk`.

```go
// go.mod MUST include these replace directives:
replace (
    github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v1.25.0-sdk-v0.50.6
    github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.41.0-tm-v0.34.29
)
```

Import paths in code remain standard (`github.com/cosmos/cosmos-sdk/...`) but resolve to Celestia's fork.

### 1.1 Key Design Decision: Native Plugin

We chose to implement a **native OpenBao plugin** for secp256k1 signing rather than a hybrid "decrypt-and-sign-locally" approach (like AWS KMS).

| Approach | Where Key Decrypts | Security Level |
|----------|-------------------|----------------|
| AWS KMS / Hybrid | App memory | Good |
| **BanhBaoRing Plugin** | **Never leaves OpenBao** | **Excellent** |

**Result:** Private keys are NEVER exposed outside OpenBao's secure boundary.

---

## 2. System Architecture

### 2.1 High-Level Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Celestia Application                         │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐  │
│  │  tx.Factory  │───▶│   Signer     │───▶│  keyring.Keyring     │  │
│  └──────────────┘    └──────────────┘    │   (interface)        │  │
│                                          └──────────┬───────────┘  │
└─────────────────────────────────────────────────────┼───────────────┘
                                                      │
                                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         BaoKeyring Module                           │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                       BaoKeyring                              │  │
│  │  ┌─────────────┐  ┌─────────────┐                            │  │
│  │  │  BaoClient  │  │  BaoStore   │   No signature conversion  │  │
│  │  │  (HTTP)     │  │  (metadata) │   needed - plugin returns  │  │
│  │  └──────┬──────┘  └──────┬──────┘   Cosmos format directly   │  │
│  └─────────┼────────────────┼───────────────────────────────────┘  │
│            │                │                                       │
└────────────┼────────────────┼───────────────────────────────────────┘
             │                │
             ▼                ▼
┌────────────────────────────────────────┐  ┌────────────────────┐
│          OpenBao Server                │  │   Local Storage    │
│  ┌──────────────────────────────────┐  │  │   (metadata.json)  │
│  │     secp256k1 Plugin             │  │  └────────────────────┘
│  │  ┌────────────┐ ┌─────────────┐  │  │
│  │  │ Key Store  │ │  Signing    │  │  │
│  │  │ (encrypted)│ │  (btcec)    │  │  │
│  │  └────────────┘ └─────────────┘  │  │
│  │                                  │  │
│  │  🔒 Private keys NEVER leave    │  │
│  │     this boundary               │  │
│  └──────────────────────────────────┘  │
│                                        │
│  ┌──────────────────────────────────┐  │
│  │     Encrypted Storage (Raft)     │  │
│  └──────────────────────────────────┘  │
└────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Component              | Responsibility                                                     |
| ---------------------- | ------------------------------------------------------------------ |
| **BaoKeyring**         | Implements `keyring.Keyring` interface, orchestrates operations    |
| **BaoClient**          | HTTP client for secp256k1 plugin API                               |
| **BaoStore**           | Manages local metadata persistence (key names, pubkeys, addresses) |
| **secp256k1 Plugin**   | Native OpenBao plugin for key storage and signing                  |
| **Migration**          | Handles key import/export between local and OpenBao keyrings       |

**Note:** No `SignatureConverter` needed - the plugin returns signatures in Cosmos format directly.

---

## 3. Component Design

### 3.1 BaoKeyring

The main struct implementing the `keyring.Keyring` interface.

```go
// BaoKeyring implements keyring.Keyring using OpenBao Transit for signing
type BaoKeyring struct {
    client       *BaoClient
    store        *BaoStore
    addressCodec address.Codec
    cdc          codec.Codec
}

// Config for BaoKeyring initialization
type Config struct {
    // OpenBao connection settings
    BaoAddr      string        // OpenBao server address (e.g., "https://bao.example.com:8200")
    BaoToken     string        // OpenBao authentication token
    BaoNamespace string        // Optional: OpenBao namespace
    TransitPath  string        // Transit engine mount path (default: "transit")

    // Storage settings
    StorePath    string        // Path to metadata store file

    // Network settings
    HTTPTimeout  time.Duration // HTTP request timeout
    TLSConfig    *tls.Config   // Optional: custom TLS configuration
}
```

#### 3.1.1 Interface Methods

```go
// Key retrieves a key's Record by uid
func (k *BaoKeyring) Key(uid string) (*keyring.Record, error)

// List returns all keys in the keyring
func (k *BaoKeyring) List() ([]*keyring.Record, error)

// NewAccount creates a new key in OpenBao and stores metadata locally
func (k *BaoKeyring) NewAccount(
    uid string,
    mnemonic string,
    bip39Passphrase string,
    hdPath string,
    algo keyring.SignatureAlgo,
) (*keyring.Record, error)

// Sign signs the given bytes using the key identified by uid
func (k *BaoKeyring) Sign(
    uid string,
    msg []byte,
    signMode signing.SignMode,
) ([]byte, cryptotypes.PubKey, error)

// SignByAddress signs using the key matching the given address
func (k *BaoKeyring) SignByAddress(
    address sdk.Address,
    msg []byte,
    signMode signing.SignMode,
) ([]byte, cryptotypes.PubKey, error)

// Delete removes a key from both OpenBao and local store
func (k *BaoKeyring) Delete(uid string) error

// Rename changes the uid of a key
func (k *BaoKeyring) Rename(oldUID, newUID string) error
```

### 3.2 BaoClient

HTTP wrapper for OpenBao Transit API interactions.

```go
// BaoClient handles HTTP communication with OpenBao Transit engine
type BaoClient struct {
    httpClient  *http.Client
    baseURL     string
    token       string
    namespace   string
    transitPath string
}

// TransitKeyResponse represents OpenBao's key read response
type TransitKeyResponse struct {
    Data struct {
        Name             string            `json:"name"`
        Type             string            `json:"type"`
        Keys             map[string]KeyVersion `json:"keys"`
        LatestVersion    int               `json:"latest_version"`
        MinDecryptionVer int               `json:"min_decryption_version"`
        MinEncryptionVer int               `json:"min_encryption_version"`
    } `json:"data"`
}

// KeyVersion contains version-specific key data
type KeyVersion struct {
    PublicKey    string    `json:"public_key"`
    CreationTime time.Time `json:"creation_time"`
}

// SignResponse represents OpenBao's sign endpoint response
type SignResponse struct {
    Data struct {
        Signature  string `json:"signature"`
        KeyVersion int    `json:"key_version"`
    } `json:"data"`
}
```

#### 3.2.1 Client Methods

```go
// CreateKey creates a new ECDSA key in Transit
func (c *BaoClient) CreateKey(name string, keyType string) error

// GetPublicKey retrieves the public key for a Transit key
func (c *BaoClient) GetPublicKey(name string) ([]byte, error)

// Sign signs data using the specified Transit key
func (c *BaoClient) Sign(keyName string, data []byte, prehashed bool) ([]byte, error)

// DeleteKey removes a key from Transit (if allowed by policy)
func (c *BaoClient) DeleteKey(name string) error

// ListKeys lists all keys in the Transit engine
func (c *BaoClient) ListKeys() ([]string, error)
```

### 3.3 BaoStore

Local metadata storage for key information.

> **Thread Safety:** BaoStore uses `sync.RWMutex` to support concurrent access from parallel workers. Multiple goroutines can read metadata simultaneously, while writes are serialized.

```go
// BaoStore manages local key metadata persistence
type BaoStore struct {
    path     string
    metadata map[string]*KeyMetadata
    mu       sync.RWMutex  // Thread-safe for parallel worker access
}

// KeyMetadata contains locally stored key information
type KeyMetadata struct {
    UID         string    `json:"uid"`
    Name        string    `json:"name"`
    PubKeyBytes []byte    `json:"pub_key"`
    PubKeyType  string    `json:"pub_key_type"`
    Address     string    `json:"address"`
    BaoKeyPath  string    `json:"bao_key_path"`
    Algorithm   string    `json:"algorithm"`
    CreatedAt   time.Time `json:"created_at"`
}
```

#### 3.3.1 Store Methods

```go
// Save persists a key's metadata
func (s *BaoStore) Save(meta *KeyMetadata) error

// Get retrieves metadata by uid
func (s *BaoStore) Get(uid string) (*KeyMetadata, error)

// GetByAddress retrieves metadata by address
func (s *BaoStore) GetByAddress(addr string) (*KeyMetadata, error)

// List returns all stored metadata
func (s *BaoStore) List() ([]*KeyMetadata, error)

// Delete removes metadata by uid
func (s *BaoStore) Delete(uid string) error

// Rename updates the uid of stored metadata
func (s *BaoStore) Rename(oldUID, newUID string) error

// Sync persists in-memory state to disk
func (s *BaoStore) Sync() error
```

#### 3.3.2 Storage Format

The metadata store uses a simple JSON file format:

```json
{
  "version": 1,
  "keys": {
    "my-key": {
      "uid": "my-key",
      "name": "my-key",
      "pub_key": "A1B2C3...",
      "pub_key_type": "secp256k1",
      "address": "celestia1abc123...",
      "bao_key_path": "transit/keys/my-key",
      "algorithm": "secp256k1",
      "created_at": "2025-01-10T12:00:00Z"
    }
  }
}
```

---

## 4. Signing Flow

### 4.1 Sequence Diagram (Plugin Architecture)

```
┌──────────┐     ┌───────────┐     ┌───────────┐     ┌─────────────────┐
│ Celestia │     │ BaoKeyring│     │ BaoClient │     │ OpenBao +       │
│  Client  │     │           │     │           │     │ secp256k1 Plugin│
└────┬─────┘     └─────┬─────┘     └─────┬─────┘     └────────┬────────┘
     │                 │                 │                    │
     │ Sign(uid, msg)  │                 │                    │
     │────────────────▶│                 │                    │
     │                 │                 │                    │
     │                 │ Get metadata    │                    │
     │                 │ (pubkey cached) │                    │
     │                 │◀───────────────▶│                    │
     │                 │                 │                    │
     │                 │ SHA-256(msg)    │                    │
     │                 │─────────┐       │                    │
     │                 │◀────────┘       │                    │
     │                 │                 │                    │
     │                 │ Sign(key, hash) │                    │
     │                 │────────────────▶│                    │
     │                 │                 │                    │
     │                 │                 │ POST /secp256k1/   │
     │                 │                 │ sign/<key>         │
     │                 │                 │───────────────────▶│
     │                 │                 │                    │
     │                 │                 │    ┌───────────────┤
     │                 │                 │    │ Sign inside   │
     │                 │                 │    │ OpenBao with  │
     │                 │                 │    │ btcec         │
     │                 │                 │    │               │
     │                 │                 │    │ 🔒 Key NEVER  │
     │                 │                 │    │ leaves here   │
     │                 │                 │    └───────────────┤
     │                 │                 │                    │
     │                 │                 │ Cosmos signature   │
     │                 │                 │ (R||S, 64 bytes)   │
     │                 │                 │◀───────────────────│
     │                 │                 │                    │
     │                 │ signature       │                    │
     │                 │◀────────────────│                    │
     │                 │                 │                    │
     │ (sig, pubkey)   │                 │                    │
     │◀────────────────│                 │                    │
     │                 │                 │                    │
```

**Key difference from hybrid approach:** No DER parsing, no low-S normalization in the client - the plugin handles everything and returns a ready-to-use Cosmos signature.

### 4.2 Parallel Signing (Fee Grant Workers)

> **Reference:** [Celestia Client Parallel Workers](https://github.com/celestiaorg/celestia-node/blob/main/api/client/readme.md)

Rollup operators use multiple worker accounts with fee grants for parallel blob submission:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    PARALLEL SIGNING (4 Workers)                              │
└──────────────────────────────────────────────────────────────────────────────┘

     Worker 1          Worker 2          Worker 3          Worker 4
         │                 │                 │                 │
         ▼                 ▼                 ▼                 ▼
    Sign(tx1)         Sign(tx2)         Sign(tx3)         Sign(tx4)
         │                 │                 │                 │
         └─────────────────┼─────────────────┼─────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ BaoKeyring  │  ← Thread-safe, no blocking
                    │ (RWMutex)   │
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   ┌───────────┐    ┌───────────┐    ┌───────────┐
   │ OpenBao   │    │ OpenBao   │    │ OpenBao   │  ← HTTP connection pool
   │ Request 1 │    │ Request 2 │    │ Request 3 │    (parallel execution)
   └───────────┘    └───────────┘    └───────────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           │
                           ▼
              All 4 signatures in ~200ms
              (not 4 × 200ms = 800ms!)
```

**Design Principles:**
- `BaoStore.mu` is `sync.RWMutex` - reads don't block each other
- `BaoClient` uses HTTP connection pooling for parallel OpenBao requests
- No global locks during signing - each key operation is independent
- OpenBao plugin handles concurrent requests natively

### 4.3 Signature Conversion

#### 4.2.1 DER to Compact Format

OpenBao Transit returns ECDSA signatures in DER format:

```
SEQUENCE {
    INTEGER r,  -- variable length, may have leading zero
    INTEGER s   -- variable length, may have leading zero
}
```

Cosmos expects compact format: `R || S` (64 bytes total)

```go
// ConvertDERToCompact converts a DER-encoded ECDSA signature to compact format
func ConvertDERToCompact(derSig []byte) ([]byte, error) {
    // Parse DER structure
    r, s, err := parseDER(derSig)
    if err != nil {
        return nil, err
    }

    // Normalize S to low-S form (BIP-62)
    s = normalizeLowS(s)

    // Pad R and S to 32 bytes each
    compact := make([]byte, 64)
    rBytes := r.Bytes()
    sBytes := s.Bytes()

    copy(compact[32-len(rBytes):32], rBytes)
    copy(compact[64-len(sBytes):64], sBytes)

    return compact, nil
}
```

#### 4.2.2 Low-S Normalization

Bitcoin/Cosmos require "low S" values to prevent signature malleability:

```go
// secp256k1 curve order N
var curveN = secp256k1.S256().Params().N

// halfN = N / 2
var halfN = new(big.Int).Rsh(curveN, 1)

// normalizeLowS ensures S <= N/2
func normalizeLowS(s *big.Int) *big.Int {
    if s.Cmp(halfN) > 0 {
        return new(big.Int).Sub(curveN, s)
    }
    return s
}
```

---

## 5. Data Flow

### 5.1 Key Creation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                       NewAccount() Flow                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ Validate inputs │
                    │ (uid, algo)     │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ BaoClient:      │
                    │ CreateKey()     │
                    │ POST /transit/  │
                    │ keys/<uid>      │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ BaoClient:      │
                    │ GetPublicKey()  │
                    │ GET /transit/   │
                    │ keys/<uid>      │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Parse public    │
                    │ key bytes       │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Derive Cosmos   │
                    │ address (bech32)│
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ BaoStore:       │
                    │ Save metadata   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Return          │
                    │ keyring.Record  │
                    └─────────────────┘
```

### 5.2 Transaction Signing Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Transaction Signing Flow                      │
└─────────────────────────────────────────────────────────────────┘

┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Celestia    │    │  BaoKeyring  │    │  OpenBao     │
│  tx.Factory  │    │              │    │  Transit     │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       │ BuildUnsignedTx() │                   │
       │───────────────────│                   │
       │                   │                   │
       │ GetSignBytes()    │                   │
       │───────────────────│                   │
       │                   │                   │
       │ Sign(signBytes)   │                   │
       │──────────────────▶│                   │
       │                   │                   │
       │                   │ hash = SHA256()  │
       │                   │──────────┐        │
       │                   │◀─────────┘        │
       │                   │                   │
       │                   │ POST /sign        │
       │                   │──────────────────▶│
       │                   │                   │
       │                   │ DER signature     │
       │                   │◀──────────────────│
       │                   │                   │
       │                   │ Convert to        │
       │                   │ compact (R||S)    │
       │                   │──────────┐        │
       │                   │◀─────────┘        │
       │                   │                   │
       │ (sig, pubkey)     │                   │
       │◀──────────────────│                   │
       │                   │                   │
       │ SetSignatures()   │                   │
       │───────────────────│                   │
       │                   │                   │
       │ Broadcast()       │                   │
       │───────────────────│                   │
```

---

## 6. Error Handling

### 6.1 Error Types

```go
var (
    // ErrKeyNotFound indicates the requested key doesn't exist
    ErrKeyNotFound = errors.New("key not found")

    // ErrKeyExists indicates a key with the given name already exists
    ErrKeyExists = errors.New("key already exists")

    // ErrBaoConnection indicates OpenBao connectivity issues
    ErrBaoConnection = errors.New("failed to connect to OpenBao")

    // ErrBaoAuth indicates OpenBao authentication failure
    ErrBaoAuth = errors.New("OpenBao authentication failed")

    // ErrBaoSign indicates a signing operation failed
    ErrBaoSign = errors.New("OpenBao signing failed")

    // ErrInvalidSignature indicates signature parsing failed
    ErrInvalidSignature = errors.New("invalid signature format")

    // ErrUnsupportedAlgo indicates unsupported signing algorithm
    ErrUnsupportedAlgo = errors.New("unsupported algorithm")

    // ErrStorePersist indicates metadata persistence failure
    ErrStorePersist = errors.New("failed to persist metadata")
)
```

### 6.2 Error Wrapping

All errors are wrapped with context:

```go
func (k *BaoKeyring) Sign(uid string, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
    meta, err := k.store.Get(uid)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get key %q: %w", uid, err)
    }

    sig, err := k.client.Sign(meta.BaoKeyPath, msg, true)
    if err != nil {
        return nil, nil, fmt.Errorf("OpenBao signing failed for key %q: %w", uid, err)
    }

    // ...
}
```

---

## 7. Security Considerations

### 7.1 Token Management

```go
// BaoClient should support multiple token sources
type TokenSource interface {
    GetToken() (string, error)
}

// EnvTokenSource reads token from environment variable
type EnvTokenSource struct {
    EnvVar string // e.g., "BAO_TOKEN"
}

// FileTokenSource reads token from a file (e.g., Kubernetes secret mount)
type FileTokenSource struct {
    Path string
}

// RenewableTokenSource wraps a token with auto-renewal
type RenewableTokenSource struct {
    client   *BaoClient
    token    string
    expiry   time.Time
    renewTTL time.Duration
}
```

### 7.2 TLS Configuration

```go
// Production TLS configuration
func ProductionTLSConfig(caPath string) (*tls.Config, error) {
    caCert, err := os.ReadFile(caPath)
    if err != nil {
        return nil, err
    }

    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    return &tls.Config{
        RootCAs:    caCertPool,
        MinVersion: tls.VersionTLS12,
    }, nil
}
```

### 7.3 Audit Logging

The implementation relies on OpenBao's built-in audit logging:

- All Transit operations are logged by OpenBao
- Key access patterns are traceable
- Signing operations include request metadata

---

## 8. Migration Architecture

### 8.1 Migration Component Design

The migration package handles key transfer between local Cosmos SDK keyrings and BaoKeyring.

```go
// migration/types.go

// Migrator handles key migration between keyring backends
type Migrator struct {
    baoClient *BaoClient
    logger    *slog.Logger
}

// ImportConfig configures a key import operation
type ImportConfig struct {
    SourceKeyring     keyring.Keyring  // Source: local keyring
    DestKeyring       *BaoKeyring      // Destination: OpenBao
    KeyName           string           // Key to migrate
    NewKeyName        string           // Optional: new name in destination
    DeleteAfterImport bool             // Delete from source after success
    Exportable        bool             // Allow future export from OpenBao
    VerifyAfterImport bool             // Sign test data to verify
}

// ExportConfig configures a key export operation
type ExportConfig struct {
    SourceKeyring     *BaoKeyring      // Source: OpenBao
    DestKeyring       keyring.Keyring  // Destination: local keyring
    KeyName           string           // Key to export
    NewKeyName        string           // Optional: new name in destination
    DeleteAfterExport bool             // Delete from OpenBao after success
    VerifyAfterExport bool             // Sign test data to verify
    UserConfirmed     bool             // User acknowledged security implications
}
```

### 8.2 Import Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Key Import Architecture                        │
└─────────────────────────────────────────────────────────────────────┘

┌───────────────┐                              ┌───────────────────────┐
│ Local Keyring │                              │      OpenBao          │
│   (source)    │                              │   Transit Engine      │
└───────┬───────┘                              └───────────┬───────────┘
        │                                                  │
        ▼                                                  │
┌───────────────────────────────────────────┐             │
│              Migrator.Import()            │             │
├───────────────────────────────────────────┤             │
│                                           │             │
│  1. sourceKr.ExportPrivKey(keyName)       │             │
│     └─▶ privateKeyBytes                   │             │
│                                           │             │
│  2. baoClient.GetWrappingKey()            │             │
│     ◀────────────────────────────────────────────────────┤
│     └─▶ wrappingPubKey (RSA 4096)         │             │
│                                           │             │
│  3. WrapKey(privateKeyBytes, wrappingKey) │             │
│     └─▶ wrappedKey                        │             │
│                                           │             │
│  4. baoClient.ImportKey(name, wrapped)    │             │
│     ────────────────────────────────────────────────────▶│
│                                           │             │
│  5. baoClient.GetPublicKey(name)          │             │
│     ◀────────────────────────────────────────────────────┤
│     └─▶ publicKey                         │             │
│                                           │             │
│  6. store.Save(metadata)                  │             │
│                                           │             │
│  7. verify: Sign & compare addresses      │             │
│                                           │             │
│  8. if deleteAfterImport:                 │             │
│       sourceKr.Delete(keyName)            │             │
│                                           │             │
└───────────────────────────────────────────┘             │
```

### 8.3 Export Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Key Export Architecture                        │
└─────────────────────────────────────────────────────────────────────┘

┌───────────────────────┐                     ┌───────────────────────┐
│      OpenBao          │                     │     Local Keyring     │
│   Transit Engine      │                     │     (destination)     │
└───────────┬───────────┘                     └───────────┬───────────┘
            │                                             │
            │                                             │
┌───────────────────────────────────────────┐             │
│              Migrator.Export()            │             │
├───────────────────────────────────────────┤             │
│                                           │             │
│  0. Verify key is exportable              │             │
│     └─▶ if !exportable: return error      │             │
│                                           │             │
│  1. baoClient.ExportKey(keyName)          │             │
│     ◀─────────────────────────────────────┤             │
│     └─▶ wrappedKeyMaterial                │             │
│                                           │             │
│  2. UnwrapKey(wrappedKeyMaterial)         │             │
│     └─▶ privateKeyBytes                   │             │
│                                           │             │
│  3. destKr.ImportPrivKey(name, privKey)   │             │
│     ─────────────────────────────────────────────────────▶
│                                           │             │
│  4. verify: Sign & compare addresses      │             │
│                                           │             │
│  5. SecureZero(privateKeyBytes)           │             │
│     └─▶ Wipe key from memory              │             │
│                                           │             │
│  6. if deleteAfterExport:                 │             │
│       baoClient.DeleteKey(keyName)        │             │
│       store.Delete(keyName)               │             │
│                                           │             │
└───────────────────────────────────────────┘             │
```

### 8.4 Key Wrapping for Import

OpenBao Transit requires keys to be wrapped before import:

```go
// WrapKeyForImport encrypts a private key using OpenBao's wrapping key
func WrapKeyForImport(privKey []byte, wrappingPubKey *rsa.PublicKey) ([]byte, error) {
    // OpenBao uses RSA-OAEP with SHA-256
    wrapped, err := rsa.EncryptOAEP(
        sha256.New(),
        rand.Reader,
        wrappingPubKey,
        privKey,
        nil, // label
    )
    if err != nil {
        return nil, fmt.Errorf("failed to wrap key: %w", err)
    }

    return wrapped, nil
}

// Format for import API
type ImportKeyRequest struct {
    Ciphertext string `json:"ciphertext"` // Base64-encoded wrapped key
    Type       string `json:"type"`       // "ecdsa-p256" or similar
    Exportable bool   `json:"exportable"` // Allow future export
}
```

### 8.5 Security Measures

```go
// SecureZero overwrites sensitive data in memory
func SecureZero(b []byte) {
    for i := range b {
        b[i] = 0
    }
    // Prevent compiler optimization from removing the zeroing
    runtime.KeepAlive(b)
}

// Import with security measures
func (m *Migrator) Import(ctx context.Context, cfg ImportConfig) (*ImportResult, error) {
    // Extract private key from source
    privKey, err := exportPrivateKey(cfg.SourceKeyring, cfg.KeyName)
    if err != nil {
        return nil, err
    }

    // CRITICAL: Ensure key is zeroed on exit
    defer SecureZero(privKey)

    // Wrap and import...
}
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

- Mock `BaoClient` for isolated `BaoKeyring` testing
- Test signature conversion with known test vectors
- Test metadata store operations

### 9.2 Integration Tests

- Use OpenBao dev mode for local testing
- Test full key lifecycle (create, sign, delete)
- Verify signature compatibility with Cosmos SDK

### 9.3 Test Vectors

```go
var testVectors = []struct {
    name     string
    message  []byte
    derSig   []byte
    compact  []byte
}{
    {
        name:    "standard signature",
        message: []byte("test message"),
        derSig:  hexDecode("3045022100..."),
        compact: hexDecode("...64 bytes..."),
    },
    // Additional test vectors...
}
```

---

## 10. Future Enhancements

| Enhancement       | Description                       | Priority |
| ----------------- | --------------------------------- | -------- |
| HSM Support       | Optional HSM backend for Transit  | Low      |
| Key Rotation      | Automatic key version rotation    | Low      |
| Multi-Key Signing | Batch signing operations          | Low      |
| Metrics           | Prometheus metrics for operations | Medium   |
| Caching           | Public key caching with TTL       | Low      |
