# BanhBaoRing Product Documentation

> 🔔 **BanhBaoRing** - Named after the distinctive "ring ring!" of Vietnamese bánh bao street vendors. Just as that familiar sound signals trusted, reliable service arriving at your door, BanhBaoRing signals secure, reliable key management arriving in your infrastructure.

---

## Product Overview

BanhBaoRing is the Point of Presence signing platform for Celestia and Cosmos rollups. Deploy next to your nodes. Same region, same datacenter. Built on OpenBao. Open source.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         BANHBAORING PLATFORM                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │
│  │  Web Dashboard  │  │ Control Plane   │  │  K8s Operator   │             │
│  │  (PRD_DASHBOARD)│  │ (PRD_CONTROL)   │  │  (PRD_OPERATOR) │             │
│  │                 │  │                 │  │                 │             │
│  │  User-facing UI │  │  Multi-tenant   │  │  One-command    │             │
│  │  5-min onboard  │  │  API + Billing  │  │  deployment     │             │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘             │
│           │                    │                    │                       │
│           └────────────────────┼────────────────────┘                       │
│                                │                                            │
│                                ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    CORE LIBRARY (Phases 0-4)                        │   │
│  │                                                                     │   │
│  │  ┌─────────────┐  ┌──────────────────┐  ┌───────────────────────┐  │   │
│  │  │ BaoKeyring  │  │ secp256k1 Plugin │  │ CLI (banhbaoring)     │  │   │
│  │  │ (Go lib)    │  │ (OpenBao plugin) │  │                       │  │   │
│  │  └─────────────┘  └──────────────────┘  └───────────────────────┘  │   │
│  │                                                                     │   │
│  │  Documented in: ARCHITECTURE.md, PLUGIN_DESIGN.md, API_REFERENCE   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Document Index

### Core Library (Already Built - Phases 0-4)

| Document | Description |
|----------|-------------|
| [PRD.md](./PRD.md) | Original product requirements for core library |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Technical architecture & component design |
| [PLUGIN_DESIGN.md](./PLUGIN_DESIGN.md) | OpenBao secp256k1 plugin specification |
| [API_REFERENCE.md](./API_REFERENCE.md) | Plugin API endpoints reference |
| [INTEGRATION.md](./INTEGRATION.md) | Celestia/Cosmos integration guide |
| [MIGRATION.md](./MIGRATION.md) | Key migration procedures |
| [DEPLOYMENT.md](./DEPLOYMENT.md) | Kubernetes deployment guide |

### SaaS Platform (New - Phases 5-7)

| Document | Description | Status |
|----------|-------------|--------|
| [PRD_CONTROL_PLANE.md](./PRD_CONTROL_PLANE.md) | Multi-tenant API, billing (Stripe) | 📝 PRD Ready |
| [PRD_DASHBOARD.md](./PRD_DASHBOARD.md) | Web dashboard, UX, 5-min onboarding | 📝 PRD Ready |
| [PRD_OPERATOR.md](./PRD_OPERATOR.md) | K8s operator for one-command deployment | 📝 PRD Ready |

### Design & Visual Identity

| Document | Description | Status |
|----------|-------------|--------|
| [DESIGN_SYSTEM.md](../design/DESIGN_SYSTEM.md) | Brand identity, color palette, typography, components, landing page wireframes | 📝 Ready |

---

## Platform Layers

### Layer 1: Core Library ✅ (Phases 0-4)
The foundation - a Go library implementing `keyring.Keyring` interface with OpenBao backend.

**Key Features:**
- `BaoKeyring` - Drop-in replacement for Cosmos SDK keyrings
- `secp256k1` OpenBao plugin - Native signing inside vault
- Key migration tools - Import/export between keyrings
- CLI tool - Command-line key management

**Status:** Implementation complete (17 agents across 4 phases)

---

### Layer 2: Control Plane API 📝 (Phase 5)
Multi-tenant backend API that wraps the core library.

**Key Features:**
- Multi-tenant isolation (organizations, namespaces)
- Authentication (OAuth, API keys, wallet connect)
- Role-based access control (RBAC)
- Billing (Stripe)
- Audit logging & compliance
- Webhooks

**Billing:**
- Stripe integration (cards, ACH, SEPA)

**Timeline:** ~9 weeks

---

### Layer 3: Web Dashboard 📝 (Phase 6)
User-facing web application for key management.

**Key Features:**
- 5-minute onboarding flow
- Key management UI (create, view, sign test)
- Usage analytics & audit log viewer
- Team management
- Billing (Stripe)

**USPs:**
- 📍 **Point of Presence** - Deploy next to your nodes. Same region, same datacenter. Zero network hops.
- 🚀 **Deploy in minutes** - Sign up → Create key → First signature in under 5 minutes.
- 🔓 **No vendor lock-in** - 100% open source. Built on OpenBao. Self-host or use our cloud.
- 🧩 **Plugin architecture** - secp256k1 today, your algorithm tomorrow.

**Timeline:** ~7 weeks

---

### Layer 4: Kubernetes Operator 📝 (Phase 7)
One-command deployment of the entire stack.

**Key Features:**
- Single CRD deploys everything
- Auto-unseal (AWS KMS, GCP KMS, Azure KV)
- Built-in PostgreSQL & Redis
- Monitoring stack (Prometheus, Grafana)
- Automated backups to S3/GCS
- Tenant provisioning

**One-Command Deploy:**
```yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: production
spec:
  domain: keys.mycompany.com
  openbao:
    replicas: 3
    autoUnseal:
      provider: awskms
      keyId: alias/banhbaoring-unseal
```

**Timeline:** ~8 weeks

---

## Timeline Summary

| Phase | Component | Agents | Duration |
|-------|-----------|--------|----------|
| 0-4 | Core Library | 18 | ✅ Complete |
| 5 | Control Plane API | ~6 | 9 weeks |
| 6 | Web Dashboard | ~6 | 7 weeks |
| 7 | K8s Operator | ~4 | 8 weeks |
| **Total** | **Full Platform** | **~34** | **~24 weeks** |

---

## Target Users

> **🎯 Maximum Focus:** We serve exactly two user types. No validators. No dApp builders. Just rollups.

| User Segment          | The Pain                                              | BanhBaoRing Solution                    |
|-----------------------|-------------------------------------------------------|----------------------------------------|
| **Rollup Developers** | Remote signers are remote, vendor lock-in, no secp256k1 | POP deployment, same datacenter, plugins |
| **Rollup Operators**  | Tedious local setup, vault far from nodes, config hell  | Deploy in minutes, next to your infra    |

### The Pain We Solve

Rollup teams know this pain:
- 🔒 **Vendor lock-in** - Stuck with AWS KMS or HashiCorp Vault enterprise pricing
- 🧩 **No customizability** - Need secp256k1 for Cosmos? "Sorry, not supported."
- 🐢 **Low performance** - Existing solutions can't handle 100+ signs/sec for parallel blob workers
- 😫 **Tedious local setup** - Config files, passphrases, backup keys somewhere safe... every time

**BanhBaoRing:** Open source. Plugin architecture. 100+ signs/sec. 5-minute setup. No lock-in. Your keys, your rules.

### Parallel Worker Support (Critical for Celestia)

> **Reference:** [Celestia Client Parallel Workers](https://github.com/celestiaorg/celestia-node/blob/main/api/client/readme.md)

Celestia rollups use parallel blob submission with multiple worker accounts:

```go
cfg := client.Config{
    SubmitConfig: client.SubmitConfig{
        TxWorkerAccounts: 4,  // 4 parallel workers
    },
}
```

**BanhBaoRing supports:**
- ⚡ Concurrent signing from multiple worker keys
- 📦 Batch key creation (create 4 workers in one call)
- 🚀 No head-of-line blocking (100+ signs/second)
- 🔧 Easy worker key management in dashboard

---

## Pricing Model

| Plan | Monthly | Keys | Signatures | Use Case |
|------|---------|------|------------|----------|
| **Free** | $0 | 3 | 10K/mo | Testing, small projects |
| **Pro** | $49 | 25 | 500K/mo | Production validators |
| **Enterprise** | Custom | Unlimited | Unlimited | Large teams, SLA |

**Payment Options:**
- 💳 Credit/debit cards (Stripe)
- 🏦 Bank transfer (ACH, SEPA)

---

## Next Steps

1. **Review PRDs** - Control Plane, Dashboard, Operator
2. **Prioritize** - Which layer to build first?
3. **Create Implementation Docs** - Break down into agent tasks
4. **Build** - Execute with parallel agents

---

## Quick Links

- **Core Library Implementation:** [`../implementation/README.md`](../implementation/README.md)
- **Design System:** [`../design/DESIGN_SYSTEM.md`](../design/DESIGN_SYSTEM.md)
- **Dashboard Implementation:** [`../implementation/IMPL_11_DASHBOARD.md`](../implementation/IMPL_11_DASHBOARD.md)
- **Repository:** `github.com/Bidon15/banhbaoring`

