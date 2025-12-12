# Implementation: POPSigner Rebrand

> Refactoring BanhBaoRing to POPSigner across the codebase

---

## Parallel Agent Tasks

Fire up agents with these independent task documents:

### Stage 1: Core Library & CLI (can run in parallel)

| Agent | Document | Dependencies |
|-------|----------|--------------|
| Agent 1 | [IMPL_14A_REBRAND_CORE_LIBRARY.md](./IMPL_14A_REBRAND_CORE_LIBRARY.md) | None |
| Agent 2 | [IMPL_14B_REBRAND_CLI.md](./IMPL_14B_REBRAND_CLI.md) | None |
| Agent 3 | [IMPL_14C_REBRAND_SDK_GO.md](./IMPL_14C_REBRAND_SDK_GO.md) | None |
| Agent 4 | [IMPL_14D_REBRAND_SDK_RUST.md](./IMPL_14D_REBRAND_SDK_RUST.md) | None |
| Agent 5 | [IMPL_14E_REBRAND_PLUGIN.md](./IMPL_14E_REBRAND_PLUGIN.md) | None |

### Stage 2: Web Application (can run in parallel)

| Agent | Document | Dependencies |
|-------|----------|--------------|
| Agent 6 | [IMPL_14F_REBRAND_WEBAPP_LANDING.md](./IMPL_14F_REBRAND_WEBAPP_LANDING.md) | None |
| Agent 7 | [IMPL_14G_REBRAND_WEBAPP_DASHBOARD.md](./IMPL_14G_REBRAND_WEBAPP_DASHBOARD.md) | None |
| Agent 8 | [IMPL_14H_REBRAND_WEBAPP_CONFIG.md](./IMPL_14H_REBRAND_WEBAPP_CONFIG.md) | None |

### Stage 3: Kubernetes Operator (run after Stage 1)

| Agent | Document | Dependencies |
|-------|----------|--------------|
| Agent 9 | [IMPL_14I_REBRAND_OPERATOR.md](./IMPL_14I_REBRAND_OPERATOR.md) | Stage 1 (imports) |

---

## Release Process

After all agents complete:

1. **Verify all changes**: `grep -r "banhbao" --include="*.go" --include="*.templ" --include="*.yaml" .`
2. **Run all tests**: `make test`
3. **Update remaining docs** (see doc refactor section below)
4. **Tag release**: `git tag v1.0.0-popsigner`
5. **Build and push images**
6. **Update package registries** (Go, Rust crates)

---

## Overview

This document outlines the implementation plan for rebranding BanhBaoRing to POPSigner. The refactoring is divided into 3 stages:

| Stage | Scope | Complexity |
|-------|-------|------------|
| **Stage 1** | Core library, CLI, SDKs | Medium |
| **Stage 2** | Web application (control-plane) | Medium |
| **Stage 3** | Kubernetes Operator | High |

### Naming Conventions

| Component | Keep As-Is | Rename To |
|-----------|------------|-----------|
| `BaoClient` | ✅ Yes | — (interfaces with OpenBao) |
| `BaoKeyring` | ✅ Yes | — (interfaces with OpenBao) |
| `BaoStore` | ✅ Yes | — (interfaces with OpenBao) |
| `banhbao` CLI | ❌ | `popsigner` |
| `banhbaoring` package | ❌ | `popsigner` |
| `bbr_` API prefix | ❌ | `psk_` |
| `BanhBaoRing` branding | ❌ | `POPSigner` |

---

## Stage 1: Core Library & CLI Refactoring

### 1.1 Scope

Refactor the core library, CLI tool, and SDKs to use POPSigner naming while preserving OpenBao interface names.

### 1.2 Files to Rename

#### CLI Directory

```
cmd/banhbao/           →  cmd/popsigner/
cmd/banhbao/main.go    →  cmd/popsigner/main.go
cmd/banhbao/keys.go    →  cmd/popsigner/keys.go
cmd/banhbao/migrate.go →  cmd/popsigner/migrate.go
cmd/banhbao/commands_test.go → cmd/popsigner/commands_test.go
```

#### SDK-Go Files

```
sdk-go/banhbaoring.go       →  sdk-go/popsigner.go
sdk-go/banhbaoring_test.go  →  sdk-go/popsigner_test.go
```

#### Binary Artifacts

```
banhbao                         →  popsigner
plugin/banhbaoring-secp256k1    →  plugin/popsigner-secp256k1
```

### 1.3 Files to Modify (Content Only)

#### Core Library (Keep Filenames)

These files keep their names but need content updates:

| File | Changes |
|------|---------|
| `go.mod` | Update module description/comments only |
| `Makefile` | Update binary names, targets |
| `types.go` | Update package comments |
| `errors.go` | Update error message prefixes |
| `example/main.go` | Update imports, comments |

#### Files to Keep Unchanged (Filenames Only)

These files interface with OpenBao and should retain their **filenames**, but the **package name** changes:

| File | Filename | Package |
|------|----------|---------|
| `bao_client.go` | Keep | `package popsigner` |
| `bao_client_test.go` | Keep | `package popsigner` |
| `bao_keyring.go` | Keep | `package popsigner` |
| `bao_keyring_test.go` | Keep | `package popsigner` |
| `bao_keyring_parallel_test.go` | Keep | `package popsigner` |
| `bao_store.go` | Keep | `package popsigner` |
| `bao_store_test.go` | Keep | `package popsigner` |
| `cmd/baokey/*` | Keep | `package main` |

The `bao_` prefix in filenames is fine—it indicates these components interface with OpenBao (Bao). The public package name becomes `popsigner`.

### 1.4 SDK-Go Refactoring

#### File: `sdk-go/popsigner.go` (renamed from banhbaoring.go)

```go
// Package popsigner provides a Go SDK for the POPSigner Control Plane API.
//
// POPSigner is Point-of-Presence signing infrastructure.
// Deploy inline with execution. Keys remain remote. You remain sovereign.
package popsigner

// Client is the POPSigner API client.
type Client struct {
    // ...
}

// NewClient creates a new POPSigner client with the given API key.
func NewClient(apiKey string, opts ...ClientOption) *Client {
    // ...
}
```

#### Update Imports

All files importing the SDK need updates:

```go
// Before
import "github.com/banhbaoring/sdk-go"

// After  
import "github.com/popsigner/sdk-go"
// or
import popsigner "github.com/Bidon15/banhbaoring/sdk-go"
```

### 1.5 SDK-Rust Refactoring

#### File: `sdk-rust/Cargo.toml`

```toml
[package]
name = "popsigner"
version = "0.1.0"
description = "POPSigner Rust SDK - Point-of-Presence signing infrastructure"
```

#### File: `sdk-rust/src/lib.rs`

```rust
//! POPSigner Rust SDK
//!
//! Point-of-Presence signing infrastructure.
//! Deploy inline with execution. Keys remain remote. You remain sovereign.

pub mod client;
pub mod keys;
// ...
```

### 1.6 API Key Prefix

Update the API key prefix from `bbr_` to `psk_`:

| Location | Change |
|----------|--------|
| `sdk-go/popsigner.go` | Validate `psk_` prefix only |
| `sdk-go/http.go` | Header: `X-POPSigner-API-Key` |
| `sdk-rust/src/client.rs` | Validate `psk_` prefix only |
| `control-plane/internal/middleware/auth.go` | Validate `psk_` prefix only |

**Breaking Change:** Old `bbr_` prefix will not be accepted.

### 1.7 Environment Variables

| Old (Remove) | New |
|--------------|-----|
| `BANHBAO_TOKEN` | `POPSIGNER_API_KEY` |
| `BANHBAO_ADDR` | `POPSIGNER_ADDR` |

**Breaking Change:** Old environment variables will not be supported.

### 1.8 CLI Commands

The CLI binary is renamed but commands stay the same:

```bash
# Before
banhbao keys create my-key
banhbao keys list
banhbao migrate import --from ~/.celestia-app/keyring-file

# After
popsigner keys create my-key
popsigner keys list
popsigner migrate import --from ~/.celestia-app/keyring-file
```

### 1.9 Implementation Checklist - Stage 1

```
□ Rename cmd/banhbao/ → cmd/popsigner/
□ Update cmd/popsigner/main.go (binary name, help text)
□ Update cmd/popsigner/keys.go (help text, error messages)
□ Update cmd/popsigner/migrate.go (help text)
□ Rename sdk-go/banhbaoring.go → sdk-go/popsigner.go
□ Rename sdk-go/banhbaoring_test.go → sdk-go/popsigner_test.go
□ Update sdk-go/go.mod (module path if needed)
□ Update all sdk-go/*.go files (package name, comments)
□ Update sdk-rust/Cargo.toml (package name)
□ Update sdk-rust/src/lib.rs (crate documentation)
□ Update sdk-rust/src/client.rs (API key prefix)
□ Update root Makefile (binary targets)
□ Update example/main.go (imports, comments)
□ Update scripts/build-push.sh (image names)
□ Rename plugin binary: banhbaoring-secp256k1 → popsigner-secp256k1
□ Update plugin/Dockerfile
□ Update plugin/go.mod (module path if needed)
```

---

## Stage 2: Web Application Refactoring

### 2.1 Scope

Refactor the control-plane web application including:
- Landing page templates
- Dashboard templates
- Layouts and components
- Static assets
- Configuration files

### 2.2 Landing Page Templates

#### Files to Update

| File | Changes |
|------|---------|
| `templates/components/landing/nav.templ` | Logo, brand name, links |
| `templates/components/landing/hero.templ` | Headline, copy, CTAs |
| `templates/components/landing/problems.templ` | Remove or reframe |
| `templates/components/landing/solution.templ` | New positioning |
| `templates/components/landing/how_it_works.templ` | Remove time claims |
| `templates/components/landing/features.templ` | Update feature list |
| `templates/components/landing/pricing.templ` | New tier structure |
| `templates/components/landing/cta.templ` | New CTA copy |
| `templates/components/landing/footer.templ` | Brand name, links |

#### Copy Changes (Reference DESIGN_SYSTEM.md)

**Hero:**
```
- OLD: "Ring ring! Sign where your infra lives."
- NEW: "Point-of-Presence Signing Infrastructure"

- OLD: "🔔 Point of Presence key management for sovereign rollups."
- NEW: "A distributed signing layer designed to live inline with execution—not behind an API queue."
```

**Pricing:**
```
- OLD: Free ($0), Pro ($49), Enterprise (Custom)
- NEW: Shared (€49), Priority (€499), Dedicated (€19,999)
```

#### Remove Forbidden Elements

- Bell emoji (🔔)
- "Ring ring!" tagline
- Time-based claims ("5 minutes", "30 sec")
- "Zero network hops"
- Performance marketing language

### 2.3 Dashboard Templates

#### Files to Update

| File | Changes |
|------|---------|
| `templates/layouts/base.templ` | Title, meta tags |
| `templates/layouts/dashboard.templ` | Branding, nav |
| `templates/layouts/auth.templ` | Logo, branding |
| `templates/components/sidebar.templ` | Logo |
| `templates/pages/dashboard.templ` | Branding |
| `templates/pages/login.templ` | Branding, copy |
| `templates/pages/signup.templ` | Branding, copy |
| `templates/pages/onboarding.templ` | Branding, copy |
| `templates/pages/keys_list.templ` | Add export visibility |
| `templates/pages/keys_detail.templ` | Add export action |

### 2.4 Static Assets

| File | Changes |
|------|---------|
| `static/css/input.css` | Update color variables |
| `static/img/logo.svg` | New logo (geometric, no emoji) |
| `tailwind.config.js` | Update color palette |
| `static/js/app.js` | Update any branding references |

### 2.5 Configuration Files

| File | Changes |
|------|---------|
| `control-plane/config.yaml` | Update app name |
| `control-plane/config/config.example.yaml` | Update app name |
| `control-plane/go.mod` | Module description |
| `control-plane/Makefile` | Binary names |
| `control-plane/docker/Dockerfile` | Image name |
| `control-plane/docker/docker-compose.yml` | Service names |

### 2.6 OAuth Configuration

Update OAuth app names and callback URLs:

```yaml
# Before
oauth:
  github:
    callback_url: /auth/github/callback
    # App name: BanhBaoRing

# After
oauth:
  github:
    callback_url: /auth/github/callback
    # App name: POPSigner
```

### 2.7 Implementation Checklist - Stage 2

```
Landing Page:
□ Update templates/components/landing/nav.templ
□ Update templates/components/landing/hero.templ
□ Update templates/components/landing/problems.templ → "what_it_is.templ"
□ Update templates/components/landing/solution.templ
□ Update templates/components/landing/how_it_works.templ
□ Update templates/components/landing/features.templ
□ Update templates/components/landing/pricing.templ
□ Update templates/components/landing/cta.templ
□ Update templates/components/landing/footer.templ
□ Run templ generate to regenerate *_templ.go files

Layouts:
□ Update templates/layouts/base.templ (title, meta)
□ Update templates/layouts/dashboard.templ
□ Update templates/layouts/auth.templ
□ Update templates/layouts/landing.templ

Dashboard Pages:
□ Update templates/pages/dashboard.templ
□ Update templates/pages/login.templ
□ Update templates/pages/signup.templ
□ Update templates/pages/onboarding.templ
□ Update templates/pages/keys_list.templ
□ Update templates/pages/keys_detail.templ
□ Update templates/components/sidebar.templ

Static Assets:
□ Update static/css/input.css (colors)
□ Create new static/img/logo.svg
□ Update tailwind.config.js
□ Run CSS build

Configuration:
□ Update control-plane/config.yaml
□ Update control-plane/config/config.example.yaml
□ Update control-plane/go.mod
□ Update control-plane/Makefile
□ Update control-plane/docker/Dockerfile
□ Update control-plane/docker/docker-compose.yml
```

---

## Stage 3: Kubernetes Operator Refactoring

### 3.1 Scope

Refactor the Kubernetes operator including:
- Custom Resource Definitions (CRDs)
- Helm charts
- Controller code
- RBAC configuration

### 3.2 CRD Naming

#### API Group Change

```
- OLD: banhbaoring.io
- NEW: popsigner.io
```

#### CRD Names

| Old | New |
|-----|-----|
| `banhbaoringclusters.banhbaoring.io` | `popsignerclusters.popsigner.io` |
| `banhbaoringtenants.banhbaoring.io` | `popsignertenants.popsigner.io` |
| `banhbaoringbackups.banhbaoring.io` | `popsignerbackups.popsigner.io` |
| `banhbaoringrestores.banhbaoring.io` | `popsignerrestores.popsigner.io` |

### 3.3 Files to Rename

```
operator/api/v1/banhbaoringcluster_types.go  →  operator/api/v1/popsignercluster_types.go
operator/api/v1/banhbaoringtenant_types.go   →  operator/api/v1/popsignertenant_types.go
operator/api/v1/banhbaoringbackup_types.go   →  operator/api/v1/popsignerbackup_types.go
operator/api/v1/banhbaoringrestore_types.go  →  operator/api/v1/popsignerrestore_types.go

operator/charts/banhbaoring-operator/        →  operator/charts/popsigner-operator/
```

### 3.4 CRD Files to Update

| File | Changes |
|------|---------|
| `config/crd/bases/banhbaoring.io_banhbaoringclusters.yaml` | Rename, update group |
| `config/crd/bases/banhbaoring.io_banhbaoringtenants.yaml` | Rename, update group |
| `config/crd/bases/banhbaoring.io_banhbaoringbackups.yaml` | Rename, update group |
| `config/crd/bases/banhbaoring.io_banhbaoringrestores.yaml` | Rename, update group |

### 3.5 Helm Chart Updates

| File | Changes |
|------|---------|
| `charts/popsigner-operator/Chart.yaml` | Name, description |
| `charts/popsigner-operator/values.yaml` | Image names, labels |
| `charts/popsigner-operator/README.md` | Documentation |
| `charts/popsigner-operator/templates/*.yaml` | Labels, names |
| `charts/popsigner-operator/templates/crds/*.yaml` | CRD definitions |

### 3.6 Controller Updates

| File | Changes |
|------|---------|
| `operator/main.go` | Manager setup, imports |
| `operator/controllers/cluster_controller.go` | Type references |
| `operator/controllers/tenant_controller.go` | Type references |
| `operator/controllers/backup_controller.go` | Type references |
| `operator/controllers/restore_controller.go` | Type references |
| `operator/internal/constants/constants.go` | Labels, annotations |

### 3.7 Type Definitions

```go
// Before
type BanhBaoRingCluster struct {
    // ...
}

// After
type POPSignerCluster struct {
    // ...
}
```

### 3.8 Labels and Annotations

```yaml
# Before
labels:
  app.kubernetes.io/name: banhbaoring
  app.kubernetes.io/component: control-plane
  banhbaoring.io/tenant: default

# After
labels:
  app.kubernetes.io/name: popsigner
  app.kubernetes.io/component: control-plane
  popsigner.io/tenant: default
```

### 3.9 Implementation Checklist - Stage 3

```
API Types:
□ Rename operator/api/v1/banhbaoringcluster_types.go → popsignercluster_types.go
□ Rename operator/api/v1/banhbaoringtenant_types.go → popsignertenant_types.go
□ Rename operator/api/v1/banhbaoringbackup_types.go → popsignerbackup_types.go
□ Rename operator/api/v1/banhbaoringrestore_types.go → popsignerrestore_types.go
□ Update operator/api/v1/groupversion_info.go (group name)
□ Update operator/api/v1/zz_generated.deepcopy.go (regenerate)

CRDs:
□ Rename config/crd/bases/*.yaml files
□ Update CRD contents (group, names)
□ Regenerate with controller-gen

Controllers:
□ Update operator/controllers/cluster_controller.go
□ Update operator/controllers/tenant_controller.go
□ Update operator/controllers/backup_controller.go
□ Update operator/controllers/restore_controller.go
□ Update operator/controllers/suite_test.go

Helm Chart:
□ Rename charts/banhbaoring-operator/ → charts/popsigner-operator/
□ Update Chart.yaml
□ Update values.yaml
□ Update all templates/*.yaml
□ Update templates/crds/*.yaml
□ Update README.md
□ Update _helpers.tpl

Configuration:
□ Update config/manager/manager.yaml
□ Update config/rbac/*.yaml
□ Update config/samples/*.yaml

Internal:
□ Update operator/internal/constants/constants.go
□ Update all operator/internal/resources/*/*.go files
□ Update operator/main.go
□ Update operator/Makefile
□ Update operator/Dockerfile
□ Update operator/go.mod
```

---

## Migration Notes

### Breaking Change Release

This is a **clean break** with no backward compatibility. Users must:

1. **Regenerate API keys** with new `psk_` prefix
2. **Update environment variables** to new names
3. **Update CLI** from `banhbao` to `popsigner`
4. **Update imports** in Go/Rust code
5. **Redeploy CRDs** with new API group

### Breaking Changes Summary

| Component | Old | New |
|-----------|-----|-----|
| CLI binary | `banhbao` | `popsigner` |
| API key prefix | `bbr_` | `psk_` |
| Env var | `BANHBAO_TOKEN` | `POPSIGNER_API_KEY` |
| Go package | `banhbaoring` | `popsigner` |
| Rust crate | `banhbaoring` | `popsigner` |
| CRD API group | `banhbaoring.io` | `popsigner.io` |
| Helm chart | `banhbaoring-operator` | `popsigner-operator` |

### Rollout Strategy

1. **Phase 1**: Update documentation (DONE)
2. **Phase 2**: Implement Stage 1 + Stage 2 + Stage 3
3. **Phase 3**: Tag release as v1.0.0-popsigner (breaking)
4. **Phase 4**: Deploy to production
5. **Phase 5**: Users redeploy with new configuration

### User Migration Guide

```bash
# 1. Update CLI
go install github.com/Bidon15/banhbaoring/cmd/popsigner@latest

# 2. Update environment variables
export POPSIGNER_API_KEY="psk_xxx"  # Get new key from dashboard
export POPSIGNER_ADDR="https://api.popsigner.io"

# 3. Update Go imports
# Before: import "github.com/banhbaoring/sdk-go"
# After:  import "github.com/popsigner/sdk-go"

# 4. Update Rust dependencies
# [dependencies]
# popsigner = "1.0"

# 5. Redeploy Kubernetes resources
kubectl delete -f old-cluster.yaml
kubectl apply -f new-popsigner-cluster.yaml
```

---

## Testing

### Stage 1 Tests

```bash
# Build new CLI
go build -o popsigner ./cmd/popsigner

# Test CLI commands
./popsigner keys list
./popsigner health

# Test SDK-Go
cd sdk-go && go test ./...

# Test SDK-Rust
cd sdk-rust && cargo test
```

### Stage 2 Tests

```bash
# Generate templates
cd control-plane && templ generate

# Build CSS
npx tailwindcss -i static/css/input.css -o static/css/output.css

# Run server
go run ./cmd/server

# Visual inspection of landing page
open http://localhost:8080
```

### Stage 3 Tests

```bash
# Regenerate CRDs
cd operator && make manifests

# Run controller tests
make test

# Install CRDs in test cluster
make install

# Deploy operator
make deploy
```

---

## Estimated Effort

| Stage | Tasks | Estimated Time |
|-------|-------|----------------|
| Stage 1 | Core library, CLI, SDKs | 2-3 days |
| Stage 2 | Web application | 2-3 days |
| Stage 3 | Kubernetes operator | 3-4 days |
| **Total** | | **7-10 days** |

---

## Post-Agent Doc Refactor

After all agents complete, update remaining documentation files:

### Implementation Docs to Update

These docs reference "banhbaoring" and need updates:

```bash
# Find all docs with old branding
grep -l "banhbao" doc/implementation/*.md
```

| File | Status |
|------|--------|
| `IMPL_00_SKELETON.md` | Update module paths |
| `IMPL_01*` through `IMPL_13*` | Update references |

### Quick Find/Replace for Docs

```bash
# In doc/implementation/
find doc/implementation -name "*.md" -exec sed -i '' \
  -e 's/banhbaoring/popsigner/g' \
  -e 's/BanhBaoRing/POPSigner/g' \
  -e 's/banhbao/popsigner/g' \
  -e 's/bbr_/psk_/g' \
  {} \;
```

### Web App Internal Docs

```bash
# control-plane README
grep -l "banhbao" control-plane/*.md
```

---

## Final Verification Checklist

```bash
# No remaining old references
grep -r "banhbaoring" --include="*.go" . | wc -l  # Should be 0
grep -r "BanhBaoRing" --include="*.go" . | wc -l  # Should be 0
grep -r "banhbao" --include="*.templ" . | wc -l   # Should be 0
grep -r "bbr_" --include="*.go" . | wc -l         # Should be 0

# All tests pass
make test
cd control-plane && go test ./...
cd sdk-go && go test ./...
cd sdk-rust && cargo test
cd operator && make test

# All builds pass
make build
cd control-plane && make build
cd operator && make build
```

---

## References

- [DESIGN_SYSTEM.md](../design/DESIGN_SYSTEM.md) - Brand guidelines and copy
- [PRD_DASHBOARD.md](../product/PRD_DASHBOARD.md) - Dashboard requirements
- Agent task docs: `IMPL_14A` through `IMPL_14I`

