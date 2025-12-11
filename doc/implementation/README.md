# Implementation Guide

This directory contains implementation specifications for **41 agents** across 5 products:

- **Core Library** (Phases 0-5): 19 agents
- **Control Plane API** (Phase 6): 8 agents
- **SDKs** (Phase 7): 2 agents
- **Web Dashboard** (Phase 8): 4 agents
- **Kubernetes Operator** (Phase 9): 8 agents

---

## ⚠️ CRITICAL: Celestia Fork Dependencies

This project **MUST** use Celestia's forks of cosmos-sdk and tendermint. Do **NOT** use upstream versions!

### Minimum Version Requirements

| Package         | Minimum Version | Notes                                 |
| --------------- | --------------- | ------------------------------------- |
| `celestia-app`  | **v6.4.0**      | Required for latest keyring interface |
| `celestia-node` | **v0.28.4**     | Required for DA layer integration     |

### Required go.mod Replace Directives

```go
replace (
    // Celestia's cosmos-sdk fork (REQUIRED)
    github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v1.25.0-sdk-v0.50.6

    // Celestia's tendermint fork (celestia-core)
    github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.41.0-tm-v0.34.29
    github.com/cometbft/cometbft => github.com/celestiaorg/celestia-core v1.41.0-tm-v0.34.29

    // Celestia's IBC fork
    github.com/cosmos/ibc-go/v8 => github.com/celestiaorg/ibc-go/v8 v8.5.1

    // Required transitive dependencies
    github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
    github.com/syndtr/goleveldb => github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
)
```

### Why This Matters

| Standard Import Path               | Replaced By                 | Reason                                            |
| ---------------------------------- | --------------------------- | ------------------------------------------------- |
| `github.com/cosmos/cosmos-sdk`     | `celestiaorg/cosmos-sdk`    | Celestia-specific keyring interface modifications |
| `github.com/tendermint/tendermint` | `celestiaorg/celestia-core` | Celestia's consensus layer                        |
| `github.com/cometbft/cometbft`     | `celestiaorg/celestia-core` | Alternative tendermint fork                       |

**Import paths in code stay the same** (e.g., `github.com/cosmos/cosmos-sdk/crypto/keyring`), but Go resolves them to Celestia's forks via the `replace` directives.

---

## ⚠️ CRITICAL: Agent 00 Must Run First!

**Agent 00 (Skeleton)** creates the project structure and must complete before any other agent starts.

---

## Agent Overview

### Agent 00: Skeleton (BLOCKING)

| ID     | Component           | Files Created                         |
| ------ | ------------------- | ------------------------------------- |
| **00** | Project Scaffolding | All directories, `go.mod`, stub files |

**This agent MUST complete first. All other agents are blocked until 00 finishes.**

---

### Agent 01: Foundation Layer (3 sub-agents)

| ID      | Component         | Skills    | Files           |
| ------- | ----------------- | --------- | --------------- |
| **01A** | Types & Constants | Go types  | `types.go`      |
| **01B** | Error Definitions | Go errors | `errors.go`     |
| **01C** | BaoClient HTTP    | HTTP, TLS | `bao_client.go` |

### Agent 02: Storage Layer (2 sub-agents)

| ID      | Component         | Skills      | Files                 |
| ------- | ----------------- | ----------- | --------------------- |
| **02A** | Store Core CRUD   | Concurrency | `bao_store.go` (CRUD) |
| **02B** | Store Persistence | File I/O    | `bao_store.go` (I/O)  |

### Agent 03: OpenBao Plugin (5 sub-agents)

| ID      | Component       | Skills        | Files                              |
| ------- | --------------- | ------------- | ---------------------------------- |
| **03A** | Backend Factory | OpenBao SDK   | `backend.go`, `main.go`            |
| **03B** | Key Paths       | OpenBao paths | `path_keys.go`, `types.go`         |
| **03C** | Sign/Verify     | ECDSA         | `path_sign.go`, `path_verify.go`   |
| **03D** | Import/Export   | RSA, wrapping | `path_import.go`, `path_export.go` |
| **03E** | Crypto Helpers  | btcec         | `crypto.go`                        |

### Agent 04: BaoKeyring (3 sub-agents)

| ID      | Component       | Skills            | Files                     |
| ------- | --------------- | ----------------- | ------------------------- |
| **04A** | Keyring Core    | Cosmos SDK        | `bao_keyring.go` (struct) |
| **04B** | Key Operations  | keyring interface | `bao_keyring.go` (keys)   |
| **04C** | Sign Operations | SHA-256           | `bao_keyring.go` (sign)   |

### Agent 05: Migration & CLI (4 sub-agents)

| ID      | Component        | Skills   | Files                    |
| ------- | ---------------- | -------- | ------------------------ |
| **05A** | Migration Import | RSA-OAEP | `migration/import.go`    |
| **05B** | Migration Export | Security | `migration/export.go`    |
| **05C** | CLI Keys         | Cobra    | `cmd/banhbao/keys.go`    |
| **05D** | CLI Migration    | Cobra    | `cmd/banhbao/migrate.go` |

### Agent 06: Parallel Workers (1 sub-agent)

| ID     | Component        | Skills            | Files                                                        |
| ------ | ---------------- | ----------------- | ------------------------------------------------------------ |
| **06** | Batch Operations | Concurrency, sync | `bao_keyring.go`, `types.go`, `bao_keyring_parallel_test.go` |

> **Reference:** [Celestia Parallel Workers Pattern](https://github.com/celestiaorg/celestia-node/blob/main/api/client/readme.md)

---

## Phase 6: Control Plane API (SaaS Platform)

> **PRD:** [`doc/product/PRD_CONTROL_PLANE.md`](../product/PRD_CONTROL_PLANE.md)

### Agent 07: Foundation (BLOCKING)

| ID     | Component     | Skills          | Files                     |
| ------ | ------------- | --------------- | ------------------------- |
| **07** | CP Foundation | PostgreSQL, Chi | `control-plane/` scaffold |

**This agent MUST complete before Phase 6.1 agents can start.**

### Phase 6.1: Auth Layer (3 parallel)

| ID      | Component        | Skills           | Files                                |
| ------- | ---------------- | ---------------- | ------------------------------------ |
| **08A** | Users & Sessions | bcrypt, sessions | `auth_service.go`, `auth_handler.go` |
| **08B** | OAuth            | OAuth 2.0        | `oauth_service.go`                   |
| **08C** | API Keys         | Argon2, scopes   | `api_key_service.go`                 |

### Phase 6.2: Core Services (2 parallel)

| ID      | Component          | Skills          | Files                       |
| ------- | ------------------ | --------------- | --------------------------- |
| **09A** | Orgs & Namespaces  | Multi-tenancy   | `org_service.go`, RBAC      |
| **09B** | Key Management API | BaoKeyring wrap | `key_service.go`, batch ops |

### Phase 6.3: Supporting Services (2 parallel)

| ID      | Component        | Skills     | Files                                    |
| ------- | ---------------- | ---------- | ---------------------------------------- |
| **10A** | Audit & Webhooks | HMAC, HTTP | `audit_service.go`, `webhook_service.go` |
| **10B** | Billing (Stripe) | Stripe API | `billing_service.go`                     |

---

## Phase 7: SDKs

> **Can run in parallel with Phase 6.3 or after.**

### Phase 7.1: Official SDKs (2 parallel)

| ID      | Component | Skills               | Files       |
| ------- | --------- | -------------------- | ----------- |
| **11A** | Go SDK    | Go, HTTP             | `sdk-go/`   |
| **11B** | Rust SDK  | Rust, async, reqwest | `sdk-rust/` |

> **Why Go + Rust?** Celestia only has official clients in Go and Rust. No TypeScript/JS client exists.

---

## Phase 8: Web Dashboard

> **PRD:** [`doc/product/PRD_DASHBOARD.md`](../product/PRD_DASHBOARD.md)

### Agent 12A: Dashboard Foundation (BLOCKING)

| ID      | Component            | Skills                | Files                           |
| ------- | -------------------- | --------------------- | ------------------------------- |
| **12A** | Dashboard Foundation | templ, Tailwind, HTMX | `templates/`, `static/`, routes |

**This agent MUST complete before Phase 8.1 agents can start.**

### Phase 8.1: Auth & Pages (3 parallel)

| ID      | Component      | Skills          | Files                           |
| ------- | -------------- | --------------- | ------------------------------- |
| **12B** | Auth Pages     | OAuth, sessions | `templates/pages/login.templ`   |
| **12C** | Keys Pages     | HTMX, forms     | `templates/pages/keys_*.templ`  |
| **12D** | Settings Pages | Billing, audit  | `templates/pages/billing.templ` |

> **Tech Stack:** Go + templ + HTMX + Alpine.js + Tailwind CSS (NO React, NO Node.js)

---

## Execution Order

```
┌─────────────────────────────────────────────────────────────────────┐
│                    PHASE 0: SKELETON (BLOCKING)                      │
│                                                                     │
│                          ┌──────────┐                               │
│                          │    00    │                               │
│                          │ Skeleton │                               │
│                          └────┬─────┘                               │
│                               │                                     │
│                    Creates all directories & stubs                  │
│                               │                                     │
│                               ▼                                     │
│              ════════════════════════════════════                   │
│                     ALL OTHER AGENTS UNBLOCKED                      │
└─────────────────────────────────────────────────────────────────────┘

Phase 1 - Foundation (5 parallel):
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   01A    │  │   01B    │  │   02A    │  │   02B    │  │   03E    │
│  Types   │  │  Errors  │  │  Store   │  │  Store   │  │  Crypto  │
│          │  │          │  │  Core    │  │  Persist │  │  Helpers │
└──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘

Phase 2 - Components (5 parallel):
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   01C    │  │   03A    │  │   03B    │  │   03C    │  │   03D    │
│  Client  │  │ Backend  │  │  Keys    │  │  Sign    │  │ Import/  │
│   HTTP   │  │ Factory  │  │  Paths   │  │  Verify  │  │  Export  │
└──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘

Phase 3 - Integration (3 parallel):
┌──────────┐  ┌──────────┐  ┌──────────┐
│   04A    │  │   04B    │  │   04C    │
│ Keyring  │  │ Keyring  │  │ Keyring  │
│  Core    │  │  Keys    │  │  Sign    │
└──────────┘  └──────────┘  └──────────┘

Phase 4 - User-Facing (4 parallel):
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   05A    │  │   05B    │  │   05C    │  │   05D    │
│ Import   │  │ Export   │  │  CLI     │  │  CLI     │
│ Logic    │  │ Logic    │  │  Keys    │  │ Migrate  │
└──────────┘  └──────────┘  └──────────┘  └──────────┘

Phase 5 - Parallel Workers (1 agent, can run after Phase 3):
                        ┌──────────┐
                        │    06    │
                        │ Parallel │
                        │ Workers  │
                        └──────────┘

════════════════════════════════════════════════════════════════════════
                    CONTROL PLANE API (SAAS PLATFORM)
════════════════════════════════════════════════════════════════════════

Phase 6.0 - CP Foundation (BLOCKING):
                        ┌──────────┐
                        │    07    │
                        │Foundation│
                        │ DB + API │
                        └──────────┘

Phase 6.1 - Auth Layer (3 parallel):
┌──────────┐  ┌──────────┐  ┌──────────┐
│   08A    │  │   08B    │  │   08C    │
│  Users   │  │  OAuth   │  │ API Keys │
│ Sessions │  │ GitHub/G │  │ Scopes   │
└──────────┘  └──────────┘  └──────────┘

Phase 6.2 - Core Services (2 parallel):
┌──────────────┐  ┌──────────────┐
│     09A      │  │     09B      │
│ Orgs + RBAC  │  │ Key Mgmt API │
│ Namespaces   │  │ Batch Sign   │
└──────────────┘  └──────────────┘

Phase 6.3 - Supporting Services (2 parallel):
┌──────────────┐  ┌──────────────┐
│     10A      │  │     10B      │
│ Audit Logs   │  │   Stripe     │
│ Webhooks     │  │   Billing    │
└──────────────┘  └──────────────┘

════════════════════════════════════════════════════════════════════════
                         SDKS (CAN RUN PARALLEL)
════════════════════════════════════════════════════════════════════════

Phase 7 - Official SDKs (2 parallel):
┌──────────────┐  ┌──────────────┐
│     11A      │  │     11B      │
│   Go SDK     │  │  Rust SDK    │
│              │  │              │
└──────────────┘  └──────────────┘

════════════════════════════════════════════════════════════════════════
                      WEB DASHBOARD (HTMX + TEMPL)
════════════════════════════════════════════════════════════════════════

Phase 8.0 - Dashboard Foundation (BLOCKING):
                        ┌──────────┐
                        │   12A    │
                        │ Foundation│
                        │ templ+CSS│
                        └──────────┘

Phase 8.1 - Dashboard Pages (3 parallel):
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│     12B      │  │     12C      │  │     12D      │
│ Auth Pages   │  │ Keys Pages   │  │ Settings     │
│ Onboarding   │  │ HTMX         │  │ Billing      │
└──────────────┘  └──────────────┘  └──────────────┘

════════════════════════════════════════════════════════════════════════
                     KUBERNETES OPERATOR (PHASE 9)
════════════════════════════════════════════════════════════════════════

Phase 9.0 - Operator Foundation (BLOCKING):
                        ┌──────────┐
                        │   13A    │
                        │Foundation│
                        │ CRDs     │
                        └──────────┘

Phase 9.1 - Core Controllers (2 parallel):
┌──────────────┐  ┌──────────────┐
│     13B      │  │     13C      │
│   OpenBao    │  │ Data Layer  │
│  Controller  │  │ PG + Redis  │
└──────────────┘  └──────────────┘

Phase 9.2 - App & Tenant (2 parallel):
┌──────────────┐  ┌──────────────┐
│     13D      │  │     13E      │
│   App Layer  │  │   Tenant    │
│ API+Dashboard│  │ Controller  │
└──────────────┘  └──────────────┘

Phase 9.3 - Supporting (2 parallel):
┌──────────────┐  ┌──────────────┐
│     13F      │  │     13G      │
│   Backup     │  │ Monitoring  │
│   Restore    │  │ Prometheus  │
└──────────────┘  └──────────────┘

Phase 9.4 - Release:
                        ┌──────────┐
                        │   13H    │
                        │  Helm    │
                        │  Chart   │
                        └──────────┘
```

---

## Parallelization Summary

### Core Library (Phases 0-5)

| Phase | Agents                       | Description             |
| ----- | ---------------------------- | ----------------------- |
| **0** | 1 (blocking)                 | Agent 00 - Skeleton     |
| **1** | 5 parallel                   | 01A, 01B, 02A, 02B, 03E |
| **2** | 5 parallel                   | 01C, 03A, 03B, 03C, 03D |
| **3** | 3 parallel                   | 04A, 04B, 04C           |
| **4** | 4 parallel                   | 05A, 05B, 05C, 05D      |
| **5** | 1 (can overlap with Phase 4) | 06 - Parallel Workers   |

**Subtotal: 19 agents**

### Control Plane API (Phase 6)

| Phase   | Agents       | Description                    |
| ------- | ------------ | ------------------------------ |
| **6.0** | 1 (blocking) | Agent 07 - CP Foundation       |
| **6.1** | 3 parallel   | 08A, 08B, 08C - Auth Layer     |
| **6.2** | 2 parallel   | 09A, 09B - Core Services       |
| **6.3** | 2 parallel   | 10A, 10B - Supporting Services |

**Subtotal: 8 agents**

### SDKs (Phase 7)

| Phase | Agents     | Description        |
| ----- | ---------- | ------------------ |
| **7** | 2 parallel | 11A, 11B - Go/Rust |

**Subtotal: 2 agents**

### Web Dashboard (Phase 8)

| Phase   | Agents       | Description            |
| ------- | ------------ | ---------------------- |
| **8.0** | 1 (blocking) | Agent 12A - Foundation |
| **8.1** | 3 parallel   | 12B, 12C, 12D - Pages  |

**Subtotal: 4 agents**

### Kubernetes Operator (Phase 9)

| Phase   | Agents       | Description                       |
| ------- | ------------ | --------------------------------- |
| **9.0** | 1 (blocking) | Agent 13A - Operator Foundation   |
| **9.1** | 2 parallel   | 13B, 13C - OpenBao + Data Layer   |
| **9.2** | 2 parallel   | 13D, 13E - Apps + Tenant          |
| **9.3** | 2 parallel   | 13F, 13G - Backup + Monitoring    |
| **9.4** | 1            | 13H - Helm Chart                  |

**Subtotal: 8 agents**

---

**Total: 41 agents** across Core Library + Control Plane + SDKs + Dashboard + Operator

---

## File → Agent Mapping

| File                              | Agent(s)              |
| --------------------------------- | --------------------- |
| `types.go`                        | 01A, **06**           |
| `errors.go`                       | 01B                   |
| `bao_client.go`                   | 01C                   |
| `bao_store.go`                    | 02A, 02B              |
| `bao_keyring.go`                  | 04A, 04B, 04C, **06** |
| `bao_keyring_parallel_test.go`    | **06**                |
| `migration/types.go`              | 05A                   |
| `migration/import.go`             | 05A                   |
| `migration/export.go`             | 05B                   |
| `cmd/banhbao/main.go`             | 05C                   |
| `cmd/banhbao/keys.go`             | 05C                   |
| `cmd/banhbao/migrate.go`          | 05D                   |
| `plugin/secp256k1/backend.go`     | 03A                   |
| `plugin/secp256k1/types.go`       | 03B                   |
| `plugin/secp256k1/path_keys.go`   | 03B                   |
| `plugin/secp256k1/path_sign.go`   | 03C                   |
| `plugin/secp256k1/path_verify.go` | 03C                   |
| `plugin/secp256k1/path_import.go` | 03D                   |
| `plugin/secp256k1/path_export.go` | 03D                   |
| `plugin/secp256k1/crypto.go`      | 03E                   |
| `plugin/cmd/plugin/main.go`       | 03A                   |

---

## Documents

### Agent 00: Skeleton

- [IMPL_00_SKELETON.md](./IMPL_00_SKELETON.md) - **START HERE**

### Agent 01: Foundation

- [IMPL_01A_TYPES.md](./IMPL_01A_TYPES.md) - Types & Constants
- [IMPL_01B_ERRORS.md](./IMPL_01B_ERRORS.md) - Error Definitions
- [IMPL_01C_CLIENT.md](./IMPL_01C_CLIENT.md) - BaoClient HTTP

### Agent 02: Storage

- [IMPL_02A_STORE_CORE.md](./IMPL_02A_STORE_CORE.md) - Store CRUD
- [IMPL_02B_STORE_PERSIST.md](./IMPL_02B_STORE_PERSIST.md) - Store Persistence

### Agent 03: Plugin

- [IMPL_03A_PLUGIN_BACKEND.md](./IMPL_03A_PLUGIN_BACKEND.md) - Backend Factory
- [IMPL_03B_PLUGIN_KEYS.md](./IMPL_03B_PLUGIN_KEYS.md) - Key Paths
- [IMPL_03C_PLUGIN_SIGN.md](./IMPL_03C_PLUGIN_SIGN.md) - Sign/Verify
- [IMPL_03D_PLUGIN_IMPORT_EXPORT.md](./IMPL_03D_PLUGIN_IMPORT_EXPORT.md) - Import/Export
- [IMPL_03E_PLUGIN_CRYPTO.md](./IMPL_03E_PLUGIN_CRYPTO.md) - Crypto Helpers

### Agent 04: BaoKeyring

- [IMPL_04A_KEYRING_CORE.md](./IMPL_04A_KEYRING_CORE.md) - Core Struct
- [IMPL_04B_KEYRING_KEYS.md](./IMPL_04B_KEYRING_KEYS.md) - Key Operations
- [IMPL_04C_KEYRING_SIGN.md](./IMPL_04C_KEYRING_SIGN.md) - Sign Operations

### Agent 05: Migration & CLI

- [IMPL_05A_MIGRATION_IMPORT.md](./IMPL_05A_MIGRATION_IMPORT.md) - Import Logic
- [IMPL_05B_MIGRATION_EXPORT.md](./IMPL_05B_MIGRATION_EXPORT.md) - Export Logic
- [IMPL_05C_CLI_KEYS.md](./IMPL_05C_CLI_KEYS.md) - CLI Keys Commands
- [IMPL_05D_CLI_MIGRATE.md](./IMPL_05D_CLI_MIGRATE.md) - CLI Migration Commands

### Agent 06: Parallel Workers

- [IMPL_06_PARALLEL_WORKERS.md](./IMPL_06_PARALLEL_WORKERS.md) - Batch Operations & Concurrency

---

## Control Plane API Documents

### Agent 07: CP Foundation

- [IMPL_07_CONTROL_PLANE_FOUNDATION.md](./IMPL_07_CONTROL_PLANE_FOUNDATION.md) - Project scaffold, DB schema

### Phase 6.1: Auth Layer

- [IMPL_08A_AUTH_USERS.md](./IMPL_08A_AUTH_USERS.md) - Users & Sessions
- [IMPL_08B_AUTH_OAUTH.md](./IMPL_08B_AUTH_OAUTH.md) - OAuth (GitHub, Google, Discord)
- [IMPL_08C_AUTH_APIKEYS.md](./IMPL_08C_AUTH_APIKEYS.md) - API Keys & Scopes

### Phase 6.2: Core Services

- [IMPL_09A_ORGS_NAMESPACES.md](./IMPL_09A_ORGS_NAMESPACES.md) - Organizations & RBAC
- [IMPL_09B_KEY_MANAGEMENT_API.md](./IMPL_09B_KEY_MANAGEMENT_API.md) - Key Management API

### Phase 6.3: Supporting Services

- [IMPL_10A_AUDIT_WEBHOOKS.md](./IMPL_10A_AUDIT_WEBHOOKS.md) - Audit Logs & Webhooks
- [IMPL_10B_BILLING_STRIPE.md](./IMPL_10B_BILLING_STRIPE.md) - Stripe Billing

### Phase 7: SDKs

- [IMPL_11A_SDK_GO.md](./IMPL_11A_SDK_GO.md) - Go SDK
- [IMPL_11B_SDK_RUST.md](./IMPL_11B_SDK_RUST.md) - Rust SDK

---

## Web Dashboard Documents

### Agent 12A: Dashboard Foundation

- [IMPL_12A_DASHBOARD_FOUNDATION.md](./IMPL_12A_DASHBOARD_FOUNDATION.md) - templ, Tailwind, layouts

### Phase 8.1: Dashboard Pages

- [IMPL_12B_DASHBOARD_AUTH.md](./IMPL_12B_DASHBOARD_AUTH.md) - Auth & Onboarding
- [IMPL_12C_DASHBOARD_KEYS.md](./IMPL_12C_DASHBOARD_KEYS.md) - Keys Management
- [IMPL_12D_DASHBOARD_SETTINGS.md](./IMPL_12D_DASHBOARD_SETTINGS.md) - Settings & Billing

---

## Kubernetes Operator Documents

> **PRD:** [`doc/product/PRD_OPERATOR.md`](../product/PRD_OPERATOR.md)

### Agent 13A: Operator Foundation

- [IMPL_13A_OPERATOR_FOUNDATION.md](./IMPL_13A_OPERATOR_FOUNDATION.md) - CRDs, controller stubs, Kubebuilder

### Phase 9.1: Core Controllers

- [IMPL_13B_OPERATOR_OPENBAO.md](./IMPL_13B_OPERATOR_OPENBAO.md) - OpenBao deployment & auto-unseal
- [IMPL_13C_OPERATOR_DATALAYER.md](./IMPL_13C_OPERATOR_DATALAYER.md) - PostgreSQL & Redis

### Phase 9.2: App & Tenant Controllers

- [IMPL_13D_OPERATOR_APPS.md](./IMPL_13D_OPERATOR_APPS.md) - API & Dashboard deployments
- [IMPL_13E_OPERATOR_TENANT.md](./IMPL_13E_OPERATOR_TENANT.md) - Multi-tenant provisioning

### Phase 9.3: Supporting Controllers

- [IMPL_13F_OPERATOR_BACKUP.md](./IMPL_13F_OPERATOR_BACKUP.md) - Backup & Restore
- [IMPL_13G_OPERATOR_MONITORING.md](./IMPL_13G_OPERATOR_MONITORING.md) - Prometheus & Grafana

### Phase 9.4: Release

- [IMPL_13H_OPERATOR_HELM.md](./IMPL_13H_OPERATOR_HELM.md) - Helm chart & CI/CD

---

### Legacy (Consolidated Agent Docs)

- [IMPL_01_TYPES_ERRORS_CLIENT.md](./IMPL_01_TYPES_ERRORS_CLIENT.md)
- [IMPL_02_BAO_STORE.md](./IMPL_02_BAO_STORE.md)
- [IMPL_03_PLUGIN.md](./IMPL_03_PLUGIN.md)
- [IMPL_04_BAO_KEYRING.md](./IMPL_04_BAO_KEYRING.md)
- [IMPL_05_MIGRATION_CLI.md](./IMPL_05_MIGRATION_CLI.md)

---

## Test Requirements

Each sub-agent MUST deliver:

1. ✅ Replace `panic("TODO...")` with implementation
2. ✅ Unit tests (>80% coverage)
3. ✅ `go test ./...` passing
4. ✅ `golangci-lint run` clean

---

## Quick Start

```bash
# 1. Agent 00 runs first
./scripts/scaffold.sh

# 2. Verify structure
go build ./...
cd plugin && go build ./...

# 3. Deploy remaining agents (phases 1-4)
```
