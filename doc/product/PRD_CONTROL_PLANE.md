# Control Plane API - Product Requirements Document

## 1. Overview

### 1.1 Product Vision

The BanhBaoRing Control Plane is a multi-tenant API that manages the entire lifecycle of
cryptographic key operations, user authentication, billing, and tenant isolation. It
serves as the bridge between customer applications and the underlying OpenBao infrastructure.

### 1.2 Product Name Origin

**BanhBaoRing** - Named after the distinctive bell ring ("ring ring!") of Vietnamese street vendors cycling through neighborhoods selling fresh bánh bao (steamed buns). Just as that familiar ring signals trusted, reliable service arriving at your door, BanhBaoRing signals secure, reliable key management arriving in your infrastructure.

### 1.3 Target Users

> **🎯 Maximum Focus:** We serve exactly two user types. No validators. No dApp builders. Just rollups.

| User Type             | Description                                                    |
| --------------------- | -------------------------------------------------------------- |
| **Rollup Developers** | Building sovereign rollups on Celestia, need secure key signing for sequencers, provers, and bridge operators |
| **Rollup Operators**  | Running production rollups, need HSM-level security without the complexity for their DA layer keys |

### 1.4 The Pain We Solve

Rollup teams know this pain:
- Sequencer keys stored in plaintext config files
- Bridge operator keys on a single server = single point of failure  
- Manual key rotation during incidents = downtime
- No audit trail of who signed what when
- Compliance asks "where are your keys?" and you point to a `.env` file
- **Parallel workers with fee grants** need concurrent signing from multiple accounts

**BanhBaoRing:** One API call to sign. Keys never leave the vault. Full audit trail. Sleep at night.

### 1.5 Parallel Worker Support (Fee Grant Pattern)

> **Critical for Celestia rollups:** The [Celestia client supports parallel blob submission](https://github.com/celestiaorg/celestia-node/blob/main/api/client/readme.md) with multiple worker accounts using fee grants. BanhBaoRing MUST support this pattern.

```go
// Celestia client with parallel workers
cfg := client.Config{
    SubmitConfig: client.SubmitConfig{
        TxWorkerAccounts: 4,  // 4 parallel worker accounts
    },
}
```

**What this means for BanhBaoRing:**

| Requirement | Description |
|-------------|-------------|
| **Concurrent Signing** | Multiple `/sign` requests for different keys simultaneously |
| **High Throughput** | Support 100+ sign requests/second per tenant |
| **Worker Key Management** | Easy creation of N worker keys for fee grant setup |
| **Batch Operations** | Sign multiple messages in a single API call |
| **No Head-of-Line Blocking** | One slow sign request shouldn't block others |

**Typical Rollup Setup:**
```
Fee Granter Account (main funding)
├── Worker 1 (signs blobs in parallel)
├── Worker 2 (signs blobs in parallel)
├── Worker 3 (signs blobs in parallel)
└── Worker 4 (signs blobs in parallel)
```

All 4 worker keys stored in BanhBaoRing, all signing concurrently.

---

## 2. Functional Requirements

### 2.1 Authentication & Authorization

#### 2.1.1 Authentication Methods

> **OAuth-Only:** BanhBaoRing uses OAuth exclusively - no email/password. This eliminates password storage liability, password reset flows, and credential stuffing attacks.

| Method               | Use Case            | Implementation               |
| -------------------- | ------------------- | ---------------------------- |
| **OAuth 2.0**        | User authentication | GitHub, Google               |
| **API Keys**         | Programmatic access | Scoped tokens with rotation  |

#### 2.1.2 API Key Scopes

```
api:keys:read       - List and view keys
api:keys:write      - Create, delete keys
api:keys:sign       - Sign operations
api:audit:read      - View audit logs
api:billing:read    - View invoices
api:billing:write   - Manage subscriptions
```

#### 2.1.3 Role-Based Access Control (RBAC)

| Role         | Permissions                      |
| ------------ | -------------------------------- |
| **Owner**    | Full access, billing, delete org |
| **Admin**    | Manage keys, users, view billing |
| **Operator** | Sign operations, view keys       |
| **Viewer**   | Read-only access                 |

---

### 2.2 Multi-Tenancy

#### 2.2.1 Tenant Isolation Model

```
Tenant (Organization)
├── Namespaces (Environments)
│   ├── production
│   │   ├── Keys
│   │   │   ├── validator-1
│   │   │   └── validator-2
│   │   └── Policies
│   └── staging
│       ├── Keys
│       └── Policies
├── Members
│   ├── alice@company.com (Owner)
│   ├── bob@company.com (Admin)
│   └── ci-bot (Operator)
└── Billing
    ├── Subscription: Pro
    └── Payment Method: Stripe
```

#### 2.2.2 Resource Quotas by Plan

| Resource         | Free   | Pro     | Enterprise |
| ---------------- | ------ | ------- | ---------- |
| Keys             | 3      | 25      | Unlimited  |
| Signatures/month | 10,000 | 500,000 | Unlimited  |
| Namespaces       | 1      | 5       | Unlimited  |
| Team members     | 1      | 10      | Unlimited  |
| Audit retention  | 7 days | 90 days | 1 year     |
| SLA              | None   | 99.9%   | 99.99%     |

---

### 2.3 Key Management API

#### 2.3.1 Endpoints

```
POST   /v1/keys                  Create new key
GET    /v1/keys                  List all keys
GET    /v1/keys/{id}             Get key details
DELETE /v1/keys/{id}             Delete key
POST   /v1/keys/{id}/sign        Sign data
POST   /v1/keys/{id}/rotate      Rotate key version

POST   /v1/keys/import           Import existing key (wrapped)
POST   /v1/keys/{id}/export      Export key (if exportable)

# Batch operations for parallel workers
POST   /v1/keys/batch            Create multiple keys at once
POST   /v1/sign/batch            Sign multiple messages (different keys)

GET    /v1/namespaces            List namespaces
POST   /v1/namespaces            Create namespace
DELETE /v1/namespaces/{id}       Delete namespace
```

#### 2.3.2 Batch Operations (Parallel Workers)

**Create Worker Keys (Batch):**
```json
POST /v1/keys/batch
{
  "keys": [
    {"name": "worker-1", "namespace": "production"},
    {"name": "worker-2", "namespace": "production"},
    {"name": "worker-3", "namespace": "production"},
    {"name": "worker-4", "namespace": "production"}
  ],
  "algorithm": "secp256k1",
  "exportable": false
}

Response:
{
  "keys": [
    {"id": "key_01...", "name": "worker-1", "address": "celestia1..."},
    {"id": "key_02...", "name": "worker-2", "address": "celestia1..."},
    {"id": "key_03...", "name": "worker-3", "address": "celestia1..."},
    {"id": "key_04...", "name": "worker-4", "address": "celestia1..."}
  ]
}
```

**Batch Sign (Parallel):**
```json
POST /v1/sign/batch
{
  "requests": [
    {"key_id": "key_01...", "data": "base64_tx_1"},
    {"key_id": "key_02...", "data": "base64_tx_2"},
    {"key_id": "key_03...", "data": "base64_tx_3"},
    {"key_id": "key_04...", "data": "base64_tx_4"}
  ],
  "encoding": "base64",
  "prehashed": false
}

Response:
{
  "signatures": [
    {"key_id": "key_01...", "signature": "base64_sig_1", "public_key": "..."},
    {"key_id": "key_02...", "signature": "base64_sig_2", "public_key": "..."},
    {"key_id": "key_03...", "signature": "base64_sig_3", "public_key": "..."},
    {"key_id": "key_04...", "signature": "base64_sig_4", "public_key": "..."}
  ]
}
```

> **Performance:** Batch sign requests are executed in parallel internally. Signing 4 transactions takes the same time as signing 1.

#### 2.3.2 Request/Response Examples

**Create Key:**

```json
POST /v1/keys
{
  "name": "validator-mainnet",
  "namespace": "production",
  "algorithm": "secp256k1",
  "exportable": false,
  "metadata": {
    "chain": "celestia",
    "network": "mainnet"
  }
}

Response:
{
  "id": "key_01HXYZ...",
  "name": "validator-mainnet",
  "public_key": "02abc...",
  "address": "celestia1xyz...",
  "algorithm": "secp256k1",
  "created_at": "2025-01-10T12:00:00Z"
}
```

**Sign Request:**

```json
POST /v1/keys/key_01HXYZ.../sign
{
  "data": "base64_encoded_tx_bytes",
  "encoding": "base64",
  "prehashed": false
}

Response:
{
  "signature": "base64_signature",
  "public_key": "02abc...",
  "key_version": 1
}
```

---

### 2.4 Billing System

#### 2.4.1 Subscription Tiers

| Plan           | Monthly Price | Annual (20% off) |
| -------------- | ------------- | ---------------- |
| **Free**       | $0            | $0               |
| **Pro**        | $49           | $470             |
| **Enterprise** | Custom        | Custom           |

#### 2.4.2 Usage-Based Pricing (Overage)

| Resource   | Pro Included | Overage Rate |
| ---------- | ------------ | ------------ |
| Signatures | 500,000/mo   | $0.0001/sig  |
| Keys       | 25           | $2/key/mo    |
| API calls  | 1M/mo        | $0.50/10K    |

#### 2.4.3 Payment Methods

**Phase 1: Stripe (Launch)**

- Credit/debit cards
- ACH bank transfer
- SEPA for EU
- Invoicing for Enterprise

#### 2.4.4 Billing API Endpoints

```
GET    /v1/billing/subscription     Current subscription
POST   /v1/billing/subscription     Change plan
GET    /v1/billing/usage            Usage metrics
GET    /v1/billing/invoices         Invoice history
POST   /v1/billing/payment-methods  Add payment method
```

---

### 2.5 Audit & Compliance

#### 2.5.1 Audit Log Events

```
key.created         - New key created
key.deleted         - Key deleted
key.signed          - Signing operation
key.exported        - Key exported (if allowed)
auth.login          - User login
auth.api_key_used   - API key authentication
billing.charge      - Payment processed
member.invited      - Team member added
```

#### 2.5.2 Audit Log API

```
GET /v1/audit/logs?
    start_time=2025-01-01T00:00:00Z&
    end_time=2025-01-31T23:59:59Z&
    event_type=key.signed&
    key_id=key_01HXYZ...&
    limit=100

Response:
{
  "logs": [
    {
      "id": "log_01H...",
      "event": "key.signed",
      "key_id": "key_01HXYZ...",
      "actor": "user_01H...",
      "ip_address": "1.2.3.4",
      "user_agent": "banhbaoring-sdk/1.0",
      "timestamp": "2025-01-10T12:00:00Z",
      "metadata": {
        "data_hash": "sha256:abc...",
        "sign_mode": "SIGN_MODE_DIRECT"
      }
    }
  ],
  "next_cursor": "cursor_xyz"
}
```

#### 2.5.3 Compliance Features

- SOC 2 Type II audit trail
- GDPR data export/deletion
- Export audit logs to SIEM (Splunk, Datadog)
- Webhook notifications for critical events

---

### 2.6 Webhooks

#### 2.6.1 Webhook Events

```
key.created
key.deleted
signature.completed
quota.warning       (80% usage)
quota.exceeded
payment.succeeded
payment.failed
```

#### 2.6.2 Webhook Configuration

```
POST /v1/webhooks
{
  "url": "https://myapp.com/webhooks/banhbaoring",
  "events": ["key.created", "signature.completed"],
  "secret": "whsec_..." // for signature verification
}
```

---

## 3. Non-Functional Requirements

### 3.1 Performance

| Metric                     | Target   | Notes                                    |
| -------------------------- | -------- | ---------------------------------------- |
| API latency (p99)          | < 100ms  | Non-signing operations                   |
| Sign operation (p99)       | < 200ms  | Single signature                         |
| **Batch sign (4 keys)**    | < 250ms  | Parallel execution                       |
| **Concurrent signs/tenant**| 100+/sec | For parallel worker pattern              |
| Availability               | 99.99%   |                                          |
| RPS per tenant             | 1,000    |                                          |

> **Parallel Workers:** The system MUST handle concurrent signing requests without head-of-line blocking. If worker-1 and worker-4 request signatures simultaneously, both should complete in ~200ms, not 400ms.

### 3.2 Security

- All API calls over TLS 1.3
- API keys hashed with Argon2
- Rate limiting per IP and API key
- DDoS protection (Cloudflare)
- Secrets never logged

### 3.3 Scalability

- Horizontal scaling for API pods
- Read replicas for PostgreSQL
- Redis cluster for sessions/cache
- OpenBao HA with auto-unseal

---

## 4. Technical Architecture

### 4.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         API Gateway (Kong/Envoy)                        │
│                    Rate Limiting, Auth, Routing                          │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
         ┌──────────────────────────┼──────────────────────────┐
         ▼                          ▼                          ▼
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│  Auth Service   │      │  Keys Service   │      │ Billing Service │
│  - Login/OAuth  │      │  - CRUD keys    │      │  - Stripe       │
│  - API keys     │      │  - Sign ops     │      │  - Invoices     │
│  - Sessions     │      │  - Audit log    │      │  - Usage meter  │
└────────┬────────┘      └────────┬────────┘      └────────┬────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          PostgreSQL (Primary)                            │
│              Users, Orgs, Keys metadata, Audit logs, Billing             │
└─────────────────────────────────────────────────────────────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│      Redis      │      │     OpenBao     │      │   Stripe API    │
│  Sessions/Cache │      │  Key Storage    │      │   Payments      │
└─────────────────┘      └─────────────────┘      └─────────────────┘
```

### 4.2 Database Schema (Core Tables)

```sql
-- Organizations (Tenants)
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan VARCHAR(50) DEFAULT 'free',
    stripe_customer_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    oauth_provider VARCHAR(50),
    oauth_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Organization Members
CREATE TABLE org_members (
    org_id UUID REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    role VARCHAR(50) NOT NULL,
    PRIMARY KEY (org_id, user_id)
);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    org_id UUID REFERENCES organizations(id),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL, -- bbr_key_xxxx (for display)
    scopes TEXT[] NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Keys (metadata, actual keys in OpenBao)
CREATE TABLE keys (
    id UUID PRIMARY KEY,
    org_id UUID REFERENCES organizations(id),
    namespace VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    public_key BYTEA NOT NULL,
    address VARCHAR(100) NOT NULL,
    algorithm VARCHAR(50) NOT NULL,
    bao_key_path VARCHAR(255) NOT NULL,
    exportable BOOLEAN DEFAULT FALSE,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (org_id, namespace, name)
);

-- Audit Logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    org_id UUID REFERENCES organizations(id),
    event VARCHAR(100) NOT NULL,
    actor_id UUID,
    actor_type VARCHAR(50), -- user, api_key
    resource_type VARCHAR(50),
    resource_id UUID,
    ip_address INET,
    user_agent TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_org_time ON audit_logs(org_id, created_at DESC);

-- Usage Metrics (for billing)
CREATE TABLE usage_metrics (
    id UUID PRIMARY KEY,
    org_id UUID REFERENCES organizations(id),
    metric VARCHAR(100) NOT NULL, -- signatures, api_calls, keys
    value BIGINT NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    UNIQUE (org_id, metric, period_start)
);
```

---

## 5. API SDK Requirements

### 5.1 Official SDKs

> **Note:** Celestia only has official clients in Go and Rust. We prioritize these two languages.

| Language | Package                         | Priority | Notes                           |
| -------- | ------------------------------- | -------- | ------------------------------- |
| Go       | `github.com/banhbaoring/sdk-go` | P0       | Primary - celestia-node uses Go |
| Rust     | `banhbaoring` (crate)           | P0       | celestia-node-rs uses Rust      |
| Python   | `banhbaoring`                   | P2       | Future - community request      |

### 5.2 SDK Example (Go)

```go
import banhbaoring "github.com/banhbaoring/sdk-go"

client := banhbaoring.NewClient("bbr_key_xxxxx")

// Create key
key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
    Name:      "sequencer",
    Namespace: "production",
    Algorithm: "secp256k1",
})

// Sign transaction
sig, err := client.Keys.Sign(ctx, key.ID, txBytes)
```

### 5.3 SDK Example: Parallel Workers (Go)

```go
import (
    "sync"
    banhbaoring "github.com/banhbaoring/sdk-go"
)

client := banhbaoring.NewClient("bbr_key_xxxxx")

// Create 4 worker keys at once
workers, err := client.Keys.CreateBatch(ctx, banhbaoring.CreateBatchRequest{
    Prefix:    "blob-worker",
    Count:     4,
    Namespace: "production",
})
// Creates: blob-worker-1, blob-worker-2, blob-worker-3, blob-worker-4

// Sign 4 transactions in parallel (no blocking!)
var wg sync.WaitGroup
sigs := make([][]byte, 4)

for i, worker := range workers {
    wg.Add(1)
    go func(idx int, keyID string, tx []byte) {
        defer wg.Done()
        sig, _, _ := client.Keys.Sign(ctx, keyID, tx)
        sigs[idx] = sig
    }(i, worker.ID, txBytes[i])
}
wg.Wait()  // All 4 complete in ~200ms, not 800ms!

// Or use batch sign API (even simpler):
results, err := client.Keys.SignBatch(ctx, banhbaoring.SignBatchRequest{
    Requests: []banhbaoring.SignRequest{
        {KeyID: workers[0].ID, Data: txBytes[0]},
        {KeyID: workers[1].ID, Data: txBytes[1]},
        {KeyID: workers[2].ID, Data: txBytes[2]},
        {KeyID: workers[3].ID, Data: txBytes[3]},
    },
})
```

---

## 6. Success Metrics

| Metric             | Target (Month 3) | Target (Month 12) |
| ------------------ | ---------------- | ----------------- |
| Registered orgs    | 100              | 1,000             |
| Monthly signatures | 1M               | 50M               |
| MRR                | $5,000           | $50,000           |
| API uptime         | 99.9%            | 99.99%            |
| Avg sign latency   | <200ms           | <100ms            |

---

## 7. Timeline

| Phase   | Deliverables                          | Duration |
| ------- | ------------------------------------- | -------- |
| **5.1** | Auth, Orgs, Users, API Keys           | 2 weeks  |
| **5.2** | Key Management API (wraps BaoKeyring) | 2 weeks  |
| **5.3** | Billing (Stripe integration)          | 2 weeks  |
| **5.4** | Audit logs, Webhooks                  | 1 week   |
| **5.5** | SDKs (Go, Rust)                       | 2 weeks  |

---

## 8. Future Enhancements

| Enhancement                   | Description                            | Priority |
| ----------------------------- | -------------------------------------- | -------- |
| **Multi-Chain Keys**          | Support for Ethereum, Solana keys      | Medium   |
| **Key Ceremony**              | Multi-party key generation             | Medium   |
| **Hardware Security Modules** | HSM-backed key storage                 | High     |
| **Geo-Redundancy**            | Multi-region deployment                | Medium   |
