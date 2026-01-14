# Signing Compatibility with POPSigner Cloud

## Overview

`popsigner-lite` maintains API compatibility with POPSigner Cloud by implementing the same signing behavior. This document tracks the shared logic to ensure changes to the original popsigner are reflected in popsigner-lite.

## Shared Signing API Contract

### REST API: `POST /v1/keys/:id/sign`

**Request Format:**
```json
{
  "data": "0x48656c6c6f",     // Hex-encoded data to sign
  "prehashed": false          // Optional: true if data is already hashed
}
```

**Response Format:**
```json
{
  "signature": "0x..."  // Hex-encoded 65-byte signature [R || S || V]
}
```

### Signing Behavior (Must Match POPSigner Cloud)

The signing logic follows the original `BaoKeyring.Sign()` method from [popsigner/bao_keyring.go:228](../../bao_keyring.go#L228):

| Parameter | Hashing Applied | Use Case |
|-----------|----------------|----------|
| `prehashed=false` (default) | **SHA-256** | Celestia/Cosmos SDK signing |
| `prehashed=true` | **None** (data must be 32 bytes) | Pre-hashed data |

**Reference Implementation:**
- **Original**: `popsigner/bao_keyring.go` - `BaoKeyring.Sign()` (always applies SHA-256)
- **Lite**: `cmd/popsigner-lite/internal/api/sign.go` - `SignHandler.Sign()` (conditionally applies SHA-256)

## Code Mapping

| Component | POPSigner Cloud | POPSigner-Lite |
|-----------|----------------|----------------|
| **API Types** | `control-plane/internal/handler/key_handler.go:271-274` | `cmd/popsigner-lite/internal/api/types.go:19-23` |
| **Request Handling** | `control-plane/internal/handler/key_handler.go:276-314` | `cmd/popsigner-lite/internal/api/sign.go:29-115` |
| **Signing Logic** | `popsigner/bao_keyring.go:228-248` (SHA-256 always) | `cmd/popsigner-lite/internal/api/sign.go:81-101` (SHA-256 conditional) |
| **Service Layer** | `control-plane/internal/service/key_service.go:367-399` | N/A (direct keystore) |

## Maintenance Checklist

When the original popsigner changes, check if popsigner-lite needs updates:

- [ ] **API Contract Changes**: If `SignHTTPRequest` struct changes in `internal/handler/key_handler.go`, update `types.go`
- [ ] **Hashing Algorithm**: If `BaoKeyring.Sign()` changes hashing (unlikely), update `sign.go`
- [ ] **Response Format**: If `SignKeyResponse` changes, update `types.go`
- [ ] **Batch Signing**: If batch signing logic changes, update `BatchSignItem` handling

## Testing

The test script [scripts_popsignerlight/1-test-popsigner-lite.sh](../../../../scripts_popsignerlight/1-test-popsigner-lite.sh) validates:

1. **Ethereum signing** (JSON-RPC `eth_signTransaction`)
2. **Celestia signing** (REST API with `prehashed=false`, SHA-256 applied)

Run tests after any changes:
```bash
./scripts_popsignerlight/1-test-popsigner-lite.sh
```

## Why Not Import Shared Packages?

We considered creating a shared signing package, but decided against it because:

1. **Module Complexity**: POPSigner uses Cosmos SDK dependencies (~100+ packages), popsigner-lite is intentionally minimal
2. **Different Architectures**:
   - POPSigner Cloud: Chi router, middleware, UUID keys, PostgreSQL, OpenBao
   - POPSigner-Lite: Gin router, no auth, string keys, in-memory, local ECDSA
3. **Maintenance Trade-off**: ~30 lines of duplicated hashing logic is simpler than managing module dependencies

The current approach duplicates minimal glue code (~30 lines) while reusing the core ECDSA signing (`signer.EthereumSigner`).

## Key Insight

The **API contract** (request/response format with `prehashed` parameter) is the critical compatibility layer. The **implementation** can differ as long as the behavior matches:

- **POPSigner Cloud**: Always hashes with SHA-256, sends to OpenBao
- **POPSigner-Lite**: Conditionally hashes with SHA-256, signs locally

Both produce the same signatures for Celestia/Cosmos SDK usage.
