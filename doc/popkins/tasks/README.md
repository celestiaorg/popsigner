# POPKins Task Tracker

## Parallel Agent Work Distribution

This directory contains task definitions for parallel development of POPKins components.

---

## 📋 Task Structure

Each task file follows a standard format:

- **Clear scope** - What exactly needs to be built
- **Prerequisites** - What must exist before starting
- **Acceptance criteria** - How to verify completion
- **Files to create/modify** - Explicit paths
- **Dependencies** - Links to other tasks

---

## 🔀 Current Work Streams

### Stream 1: Shared Infrastructure

| Task                                                             | Status  | Agent         | Description                 |
| ---------------------------------------------------------------- | ------- | ------------- | --------------------------- |
| [TASK-001: Database Schema](./TASK-001-database-schema.md)       | ✅ Done | claude-opus-4 | Core tables for deployments |
| [TASK-002: Deployment Repository](./TASK-002-deployment-repo.md) | ✅ Done | claude-opus-4 | Go repository layer         |
| [TASK-003: API Handlers](./TASK-003-api-handlers.md)             | ✅ Done | claude-opus-4 | REST endpoints              |
| [TASK-004: CLI Commands](./TASK-004-cli-commands.md)             | ✅ Done | claude-opus-4 | popctl bootstrap commands   |

### Stream 2: OP Stack Deployment

| Task                                                                      | Status         | Agent     | Description                     |
| ------------------------------------------------------------------------- | -------------- | --------- | ------------------------------- |
| [TASK-010: SignerFn Implementation](./TASK-010-opstack-signerfn.md)       | ✅ Done        | Agent-OP1 | op-deployer signing integration |
| [TASK-011: StateWriter Implementation](./TASK-011-opstack-statewriter.md) | ✅ Done        | Agent-OP2 | State persistence               |
| [TASK-012: Orchestrator Core](./TASK-012-opstack-orchestrator.md)         | ✅ Done        | Agent-OP1 | Stage execution engine          |
| [TASK-013: Artifact Extraction](./TASK-013-opstack-artifacts.md)          | ✅ Done        | Agent-OP2 | Genesis, rollup.json generation |

### Stream 3: Nitro Deployment

| Task                                                                      | Status  | Agent    | Description                          |
| ------------------------------------------------------------------------- | ------- | -------- | ------------------------------------ |
| [TASK-020: Viem Account (TypeScript)](./TASK-020-nitro-viem-account.md)   | ✅ Done | Agent-N1 | Custom Viem account for mTLS signing |
| [TASK-021: Deploy Script (TypeScript)](./TASK-021-nitro-deploy-script.md) | ✅ Done | Agent-N2 | orbit-sdk deployment script          |
| [TASK-022: Go Wrapper](./TASK-022-nitro-go-wrapper.md)                    | ✅ Done | Agent-N1 | Subprocess execution                 |
| [TASK-023: Nitro Config Builder](./TASK-023-nitro-config.md)              | ✅ Done | Agent-N2 | chain-info.json, node-config.json    |

### Stream 4: Post-Deployment (Unified)

| Task                                                                   | Status  | Agent | Description                           |
| ---------------------------------------------------------------------- | ------- | ----- | ------------------------------------- |
| [TASK-030: Artifact Bundler](./TASK-030-artifact-bundler.md)           | 🔲 Open | -     | Bundle generation for **both stacks** |
| [TASK-031: Docker Compose Generator](./TASK-031-docker-compose-gen.md) | 🔲 Open | -     | Compose templates for **both stacks** |
| [TASK-032: Cloud Deploy API](./TASK-032-cloud-deploy-api.md)           | 🔲 Open | -     | One-click cloud deployment            |

---

## 🔗 Dependency Graph

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                   SHARED INFRASTRUCTURE                      │
                    │                                                              │
                    │     TASK-001 ──► TASK-002 ──► TASK-003 ──► TASK-004         │
                    │     (Schema)     (Repo)       (API)        (CLI)            │
                    └──────────────────────────────┬───────────────────────────────┘
                                                   │
                    ┌──────────────────────────────┼──────────────────────────────┐
                    │                              │                               │
                    ▼                              ▼                               │
    ┌────────────────────────────┐  ┌────────────────────────────┐                │
    │        OP STACK            │  │          NITRO             │                │
    │                            │  │                            │                │
    │   TASK-010 (SignerFn)      │  │   TASK-020 (Viem Account)  │                │
    │       │                    │  │       │                    │                │
    │       ▼                    │  │       ▼                    │                │
    │   TASK-011 (StateWriter)   │  │   TASK-021 (Deploy Script) │                │
    │       │                    │  │       │                    │                │
    │       ▼                    │  │       ▼                    │                │
    │   TASK-012 (Orchestrator)  │  │   TASK-022 (Go Wrapper)    │                │
    │       │                    │  │       │                    │                │
    │       ▼                    │  │       ▼                    │                │
    │   TASK-013 (Artifacts) ────┼──┼── TASK-023 (Config)        │                │
    └────────────────────────────┘  └────────────────────────────┘                │
                    │                              │                               │
                    │    ┌─────────────────────────┘                               │
                    │    │                                                         │
                    ▼    ▼                                                         │
    ┌─────────────────────────────────────────────────────────────────────────────┘
    │
    │        POST-DEPLOYMENT (requires BOTH OP Stack & Nitro artifacts)
    │
    │     ┌───────────────────────────────────────────────────────────┐
    │     │                                                           │
    │     │   TASK-031 (Docker Compose) ◄── generates templates for   │
    │     │       │                         BOTH OP Stack & Nitro     │
    │     │       │                                                   │
    │     │       ▼                                                   │
    │     │   TASK-030 (Artifact Bundler) ◄── bundles configs for     │
    │     │       │                           BOTH OP Stack & Nitro   │
    │     │       │                                                   │
    │     │       ▼                                                   │
    │     │   TASK-032 (Cloud Deploy)                                 │
    │     │                                                           │
    │     └───────────────────────────────────────────────────────────┘
    │
    └─────────────────────────────────────────────────────────────────
```

---

## 🚦 Key Dependencies

| Task         | Depends On                   | Why                                              |
| ------------ | ---------------------------- | ------------------------------------------------ |
| **TASK-030** | TASK-013, TASK-023, TASK-031 | Needs artifacts from both stacks + compose files |
| **TASK-031** | TASK-013, TASK-023           | Needs to know artifact structure for both stacks |
| **TASK-013** | TASK-010, TASK-011, TASK-012 | OP Stack must deploy before extracting artifacts |
| **TASK-023** | TASK-020, TASK-021, TASK-022 | Nitro must deploy before generating configs      |

---

## 🔄 OP Stack vs Nitro: Key Differences

Understanding these differences is critical for implementation:

| Aspect               | OP Stack                     | Nitro                             |
| -------------------- | ---------------------------- | --------------------------------- |
| **Transactions**     | ~35 transactions             | 1 transaction (atomic)            |
| **Tool**             | op-deployer (Go native)      | orbit-sdk (TypeScript subprocess) |
| **POPSigner Auth**   | API Key (`X-API-Key` header) | mTLS (client certificates)        |
| **Signing Roles**    | Batcher + Proposer           | Batch Poster + Validator          |
| **Config Artifacts** | genesis.json, rollup.json    | chain-info.json, node-config.json |
| **Bundle Size**      | Large (~50MB genesis)        | Small (~1MB)                      |
| **Credentials**      | API key in .env              | Certificates in ./certs/          |

---

## 📝 Task Template

When creating new tasks, use [TEMPLATE.md](./TEMPLATE.md).

---

## 🏷️ Status Legend

| Status         | Meaning               |
| -------------- | --------------------- |
| 🔲 Open        | Not started           |
| 🟡 In Progress | Agent working on it   |
| 🔵 In Review   | PR submitted          |
| ✅ Done        | Merged                |
| ⛔ Blocked     | Waiting on dependency |

---

## 🚀 Quick Start for Agents

1. Pick an **Open** task with no blockers (check prerequisites in the task file)
2. Update status to **In Progress** and add your agent ID
3. **Read the task file completely** - includes implementation details
4. Implement according to acceptance criteria
5. Run tests if specified
6. Update status to **In Review** when done

### Parallel Work Guidelines

- **Streams 2 & 3 (OP Stack & Nitro) can run in parallel** - they're independent
- **Stream 4 requires both Stream 2 & 3 complete** - it bundles artifacts from both
- **Within a stream**, tasks are sequential (follow dependency chain)
