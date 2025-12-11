# BanhBaoRing Kubernetes Operator - Technical Specification

## 1. Overview

### 1.1 Purpose
A Kubernetes operator that deploys and manages the entire BanhBaoRing stack with a single command:
- OpenBao cluster with secp256k1 plugin
- Control Plane API
- Web Dashboard
- PostgreSQL & Redis
- Monitoring stack (Prometheus, Grafana)
- Automatic TLS, backups, and scaling

### 1.2 Product Name Origin
**BanhBaoRing** - Like the reliable "ring ring!" of the bánh bao vendor, this operator brings the complete key management stack right to your Kubernetes cluster. One ring (one command), everything arrives.

### 1.3 Target Users

> **🎯 Maximum Focus:** Rollup developers and operators only.

| User Type             | Use Case                                                       |
| --------------------- | -------------------------------------------------------------- |
| **Rollup Developers** | Self-host BanhBaoRing for dev/staging rollup environments      |
| **Rollup Operators**  | Production-grade deployment for sequencer and bridge key management |

### 1.4 The Pain We Solve

Rollup operators deploying their own key infrastructure face:
- Complex OpenBao/Vault setup with 50+ config options
- Manual unseal procedures after every pod restart
- No built-in backup/restore for disaster recovery
- Separate deployments for DB, Redis, monitoring
- Weeks of DevOps work before first key is created

**BanhBaoRing Operator:** One YAML, one command, full stack. Your rollup's keys are secure in 5 minutes.

### 1.5 OpenBao Deployment Strategy

The operator leverages **OpenBao's official Helm chart** for battle-tested OpenBao deployment:

| Responsibility | Owner |
|---------------|-------|
| OpenBao StatefulSet, Raft storage, auto-unseal | OpenBao Helm chart |
| secp256k1 plugin registration | BanhBaoRing Operator |
| Tenant namespace provisioning | BanhBaoRing Operator |
| Control Plane & Dashboard integration | BanhBaoRing Operator |
| Backup/restore coordination | BanhBaoRing Operator |

This hybrid approach gives us production-grade OpenBao while maintaining full control over our customizations.

### 1.6 Scalability & High Availability

The system scales horizontally to handle increased load:

| Component | Scaling Method | Limits | Notes |
|-----------|---------------|--------|-------|
| **OpenBao** | StatefulSet replicas | 3, 5, 7 (odd) | Raft consensus |
| **Control Plane API** | HPA (CPU-based) | 2 → 100+ pods | Stateless, scales freely |
| **Web Dashboard** | Deployment replicas | 2 → 10+ pods | Stateless |
| **PostgreSQL** | Read replicas | 1 primary + N replicas | HA with streaming replication |
| **Redis** | Cluster mode | 3 → 6+ shards | Automatic sharding |

**Signing Throughput:**
- Single OpenBao node: ~1,000 signs/sec
- 3-node cluster: ~2,500 signs/sec (one leader handles writes)
- Horizontal scaling for API allows more concurrent requests

### 1.7 One-Command Deployment Goal
```bash
# Install operator
kubectl apply -f https://banhbaoring.io/install/operator.yaml

# Deploy full stack
kubectl apply -f - <<EOF
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: production
spec:
  domain: keys.mycompany.com
  plan: pro
  replicas: 3
EOF

# That's it! Full stack deployed in ~5 minutes
# Ring ring! 🔔
```

---

## 2. Custom Resource Definitions (CRDs)

### 2.1 BanhBaoRingCluster (Main Resource)

```yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: production
  namespace: banhbaoring-system
spec:
  # === BASIC CONFIGURATION ===
  domain: keys.mycompany.com
  
  # === OPENBAO CONFIGURATION ===
  openbao:
    replicas: 3
    version: "2.0.0"
    storage:
      size: 50Gi
      storageClass: fast-ssd
    resources:
      requests:
        cpu: 500m
        memory: 512Mi
      limits:
        cpu: 2
        memory: 2Gi
    autoUnseal:
      enabled: true
      provider: awskms  # or gcpkms, azurekv, transit
      keyId: alias/banhbaoring-unseal
    tls:
      issuer: letsencrypt-prod
      # or: secretName: my-tls-secret
    plugin:
      # secp256k1 plugin configuration
      version: "1.0.0"
      # Plugin is automatically registered
  
  # === CONTROL PLANE API ===
  api:
    replicas: 2
    version: "1.0.0"
    resources:
      requests:
        cpu: 250m
        memory: 256Mi
      limits:
        cpu: 1
        memory: 1Gi
    autoscaling:
      enabled: true
      minReplicas: 2
      maxReplicas: 10
      targetCPU: 70
  
  # === WEB DASHBOARD ===
  dashboard:
    replicas: 2
    version: "1.0.0"
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
  
  # === DATABASE ===
  database:
    # Option 1: Managed (operator deploys PostgreSQL)
    managed: true
    version: "16"
    storage:
      size: 20Gi
      storageClass: fast-ssd
    # High availability
    replicas: 2  # 1 primary + 1 replica
    
    # Option 2: External (use existing database)
    # managed: false
    # connectionString:
    #   secretName: external-db
    #   key: url
  
  # === REDIS ===
  redis:
    managed: true
    version: "7"
    mode: cluster  # or standalone
    replicas: 3   # For cluster mode
    storage:
      size: 5Gi
  
  # === MONITORING ===
  monitoring:
    enabled: true
    prometheus:
      retention: 15d
      storage:
        size: 50Gi
    grafana:
      enabled: true
      adminPassword:
        secretName: grafana-admin
        key: password
    alerting:
      enabled: true
      slack:
        webhookUrl:
          secretName: slack-webhook
          key: url
      pagerduty:
        routingKey:
          secretName: pagerduty
          key: routing-key
  
  # === BACKUPS ===
  backup:
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM UTC
    retention: 30  # days
    destination:
      s3:
        bucket: banhbaoring-backups
        region: us-east-1
        prefix: production/
        credentials:
          secretName: s3-credentials
      # or gcs:
      #   bucket: banhbaoring-backups
      #   credentials:
      #     secretName: gcs-credentials
  
  # === BILLING INTEGRATIONS ===
  billing:
    stripe:
      enabled: true
      secretKeyRef:
        name: stripe-secrets
        key: secret-key
      webhookSecretRef:
        name: stripe-secrets
        key: webhook-secret

status:
  phase: Running  # Pending, Initializing, Running, Degraded, Failed
  
  # Component statuses
  openbao:
    ready: true
    sealed: false
    version: "2.0.0"
    nodes: 3/3
    leader: banhbaoring-openbao-0
  
  api:
    ready: true
    version: "1.0.0"
    replicas: 2/2
  
  dashboard:
    ready: true
    version: "1.0.0"
    replicas: 2/2
  
  database:
    ready: true
    version: "16.1"
    primaryEndpoint: banhbaoring-postgres-primary:5432
  
  redis:
    ready: true
    version: "7.2"
    mode: cluster
  
  # Access endpoints
  endpoints:
    api: https://api.keys.mycompany.com
    dashboard: https://keys.mycompany.com
    openbao: https://vault.keys.mycompany.com
    grafana: https://grafana.keys.mycompany.com
  
  # Health conditions
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-01-10T12:00:00Z"
      reason: AllComponentsHealthy
    - type: TLSReady
      status: "True"
      lastTransitionTime: "2025-01-10T11:55:00Z"
    - type: DatabaseReady
      status: "True"
      lastTransitionTime: "2025-01-10T11:50:00Z"
    - type: BackupSucceeded
      status: "True"
      lastTransitionTime: "2025-01-10T02:05:00Z"
      message: "Last backup: 2025-01-10 02:00:00 UTC, size: 1.2GB"
```

---

### 2.2 BanhBaoRingTenant (Multi-tenant Provisioning)

```yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingTenant
metadata:
  name: celestia-validator-co
  namespace: banhbaoring-system
spec:
  # Reference to cluster
  clusterRef:
    name: production
  
  # Tenant configuration
  displayName: "Celestia Validator Co"
  plan: pro
  
  # Resource quotas
  quotas:
    keys: 25
    signaturesPerMonth: 500000
    namespaces: 5
    teamMembers: 10
    apiKeys: 20
  
  # Initial admin user
  admin:
    email: admin@celestia-validator.co
    # Password sent via email, or:
    # password:
    #   secretName: tenant-admin
    #   key: password
  
  # Custom settings
  settings:
    auditRetentionDays: 90
    allowExportableKeys: false
    allowedIPRanges:
      - "10.0.0.0/8"
      - "192.168.1.0/24"
    webhooks:
      - url: https://webhooks.celestia-validator.co/banhbaoring
        events: ["key.created", "key.signed"]
        secret:
          secretName: tenant-webhook
          key: secret

status:
  phase: Active  # Pending, Active, Suspended, Deleted
  
  # OpenBao namespace for this tenant
  openbaoNamespace: tenant-celestia-validator-co
  
  # Usage metrics
  usage:
    keys: 2
    signaturesThisMonth: 45231
    apiKeys: 3
    teamMembers: 2
  
  # Timestamps
  createdAt: "2025-01-10T12:00:00Z"
  lastActiveAt: "2025-01-10T14:30:00Z"
```

---

### 2.3 BanhBaoRingBackup (Manual/On-Demand Backups)

```yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingBackup
metadata:
  name: pre-upgrade-backup
  namespace: banhbaoring-system
spec:
  # Reference to cluster
  clusterRef:
    name: production
  
  # Backup type
  type: full  # or incremental
  
  # What to backup
  components:
    - openbao      # Raft snapshot
    - database     # PostgreSQL dump
    - secrets      # Kubernetes secrets
  
  # Destination (overrides cluster default)
  destination:
    s3:
      bucket: banhbaoring-backups
      prefix: manual/pre-upgrade-2025-01-10/
      credentials:
        secretName: s3-credentials

status:
  phase: Completed  # Pending, Running, Completed, Failed
  
  startedAt: "2025-01-10T12:00:00Z"
  completedAt: "2025-01-10T12:05:00Z"
  
  # Backup details
  components:
    - name: openbao
      status: Completed
      size: 500Mi
      location: s3://banhbaoring-backups/manual/pre-upgrade-2025-01-10/openbao.snap
    - name: database
      status: Completed
      size: 750Mi
      location: s3://banhbaoring-backups/manual/pre-upgrade-2025-01-10/postgres.sql.gz
    - name: secrets
      status: Completed
      size: 10Ki
      location: s3://banhbaoring-backups/manual/pre-upgrade-2025-01-10/secrets.yaml.enc
  
  totalSize: 1.2Gi
```

---

### 2.4 BanhBaoRingRestore (Disaster Recovery)

```yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingRestore
metadata:
  name: restore-from-backup
  namespace: banhbaoring-system
spec:
  # Reference to cluster
  clusterRef:
    name: production
  
  # Backup to restore from
  backupRef:
    name: pre-upgrade-backup
  
  # Or restore from specific location
  # source:
  #   s3:
  #     bucket: banhbaoring-backups
  #     prefix: manual/pre-upgrade-2025-01-10/
  
  # What to restore (default: all)
  components:
    - openbao
    - database
  
  # Safety options
  options:
    # Stop all applications before restore
    stopApplications: true
    # Verify backup integrity before restore
    verifyBackup: true

status:
  phase: Completed  # Pending, Stopping, Restoring, Starting, Completed, Failed
  
  startedAt: "2025-01-10T15:00:00Z"
  completedAt: "2025-01-10T15:10:00Z"
  
  steps:
    - name: StopApplications
      status: Completed
    - name: RestoreOpenBao
      status: Completed
    - name: RestoreDatabase
      status: Completed
    - name: StartApplications
      status: Completed
    - name: VerifyHealth
      status: Completed
```

---

## 3. Operator Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         BANHBAORING OPERATOR                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    CONTROLLER MANAGER                                │   │
│  │                                                                     │   │
│  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐             │   │
│  │  │   Cluster     │ │    Tenant     │ │    Backup     │             │   │
│  │  │  Controller   │ │  Controller   │ │  Controller   │             │   │
│  │  │               │ │               │ │               │             │   │
│  │  │ Reconciles:   │ │ Reconciles:   │ │ Reconciles:   │             │   │
│  │  │ - OpenBao     │ │ - Namespaces  │ │ - CronJobs    │             │   │
│  │  │ - API         │ │ - Policies    │ │ - Snapshots   │             │   │
│  │  │ - Dashboard   │ │ - Quotas      │ │ - S3 uploads  │             │   │
│  │  │ - Database    │ │ - Users       │ │               │             │   │
│  │  │ - Redis       │ │ - API Keys    │ │               │             │   │
│  │  │ - Monitoring  │ │               │ │               │             │   │
│  │  └───────────────┘ └───────────────┘ └───────────────┘             │   │
│  │                                                                     │   │
│  │  ┌───────────────┐ ┌───────────────┐                               │   │
│  │  │   Restore     │ │   Metrics     │                               │   │
│  │  │  Controller   │ │  Controller   │                               │   │
│  │  │               │ │               │                               │   │
│  │  │ Reconciles:   │ │ Reconciles:   │                               │   │
│  │  │ - DR restores │ │ - Usage stats │                               │   │
│  │  │ - Validation  │ │ - Billing     │                               │   │
│  │  └───────────────┘ └───────────────┘                               │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    CREATED RESOURCES                                 │   │
│  │                                                                     │   │
│  │  OpenBao:       StatefulSet, Service, Ingress, PVC, ConfigMap,     │   │
│  │                 Plugin registration, Policies                       │   │
│  │                                                                     │   │
│  │  API:           Deployment, Service, Ingress, HPA, ConfigMap       │   │
│  │                                                                     │   │
│  │  Dashboard:     Deployment, Service, Ingress, HPA                  │   │
│  │                                                                     │   │
│  │  Database:      StatefulSet, Service, PVC, ConfigMap, Secrets      │   │
│  │                                                                     │   │
│  │  Redis:         StatefulSet, Service, PVC, ConfigMap               │   │
│  │                                                                     │   │
│  │  Monitoring:    Prometheus, Grafana, AlertManager, ServiceMonitors │   │
│  │                                                                     │   │
│  │  Certificates:  Certificate (cert-manager integration)             │   │
│  │                                                                     │   │
│  │  Secrets:       Unseal keys, root token, DB creds, API keys        │   │
│  │                                                                     │   │
│  │  Networking:    NetworkPolicies, Ingress rules                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Reconciliation Logic

### 4.1 Cluster Controller (Simplified)

```go
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    // Fetch the BanhBaoRingCluster
    cluster := &banhbaoringv1.BanhBaoRingCluster{}
    if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // Phase 1: Prerequisites
    log.Info("Reconciling prerequisites")
    
    if err := r.reconcileNamespace(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Initializing", err)
    }
    
    if err := r.reconcileCertificates(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Initializing", err)
    }
    
    // Phase 2: Data layer
    log.Info("Reconciling data layer")
    
    if cluster.Spec.Database.Managed {
        if err := r.reconcilePostgreSQL(ctx, cluster); err != nil {
            return r.updateStatus(ctx, cluster, "Initializing", err)
        }
    }
    
    if cluster.Spec.Redis.Managed {
        if err := r.reconcileRedis(ctx, cluster); err != nil {
            return r.updateStatus(ctx, cluster, "Initializing", err)
        }
    }
    
    // Wait for data layer
    if !r.isDataLayerReady(ctx, cluster) {
        log.Info("Waiting for data layer to be ready")
        return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
    }
    
    // Phase 3: OpenBao
    log.Info("Reconciling OpenBao")
    
    if err := r.reconcileOpenBao(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Initializing", err)
    }
    
    if !r.isOpenBaoReady(ctx, cluster) {
        log.Info("Waiting for OpenBao to be ready")
        return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
    }
    
    // Phase 4: Initialize OpenBao (first time only)
    if !cluster.Status.OpenBao.Initialized {
        log.Info("Initializing OpenBao")
        if err := r.initializeOpenBao(ctx, cluster); err != nil {
            return r.updateStatus(ctx, cluster, "Failed", err)
        }
        if err := r.registerPlugin(ctx, cluster); err != nil {
            return r.updateStatus(ctx, cluster, "Failed", err)
        }
    }
    
    // Phase 5: Applications
    log.Info("Reconciling applications")
    
    if err := r.reconcileAPI(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Degraded", err)
    }
    
    if err := r.reconcileDashboard(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Degraded", err)
    }
    
    // Phase 6: Monitoring (non-fatal)
    if cluster.Spec.Monitoring.Enabled {
        log.Info("Reconciling monitoring")
        if err := r.reconcileMonitoring(ctx, cluster); err != nil {
            log.Error(err, "Failed to reconcile monitoring")
            // Don't fail the whole reconciliation
        }
    }
    
    // Phase 7: Backups
    if cluster.Spec.Backup.Enabled {
        log.Info("Reconciling backups")
        if err := r.reconcileBackupCronJob(ctx, cluster); err != nil {
            log.Error(err, "Failed to reconcile backups")
        }
    }
    
    // Phase 8: Ingress & networking
    if err := r.reconcileIngress(ctx, cluster); err != nil {
        return r.updateStatus(ctx, cluster, "Degraded", err)
    }
    
    if err := r.reconcileNetworkPolicies(ctx, cluster); err != nil {
        log.Error(err, "Failed to reconcile network policies")
    }
    
    // All good!
    return r.updateStatus(ctx, cluster, "Running", nil)
}
```

---

## 5. Auto-Unseal Integration

### 5.1 AWS KMS

```yaml
spec:
  openbao:
    autoUnseal:
      enabled: true
      provider: awskms
      keyId: alias/banhbaoring-unseal
      region: us-east-1
      # Credentials from:
      # - IAM Role for Service Accounts (IRSA) - recommended
      # - Or explicit credentials
      credentials:
        accessKeyId:
          secretName: aws-credentials
          key: access-key-id
        secretAccessKey:
          secretName: aws-credentials
          key: secret-access-key
```

### 5.2 GCP Cloud KMS

```yaml
spec:
  openbao:
    autoUnseal:
      enabled: true
      provider: gcpkms
      project: my-gcp-project
      location: global
      keyRing: banhbaoring
      cryptoKey: unseal
      # Uses Workload Identity by default
      # Or explicit service account:
      credentials:
        serviceAccountKey:
          secretName: gcp-credentials
          key: service-account.json
```

### 5.3 Azure Key Vault

```yaml
spec:
  openbao:
    autoUnseal:
      enabled: true
      provider: azurekv
      tenantId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
      vaultName: banhbaoring-vault
      keyName: unseal-key
      # Uses Azure AD Pod Identity / Workload Identity
      # Or explicit credentials:
      credentials:
        clientId:
          secretName: azure-credentials
          key: client-id
        clientSecret:
          secretName: azure-credentials
          key: client-secret
```

### 5.4 Transit (Another OpenBao/Vault)

```yaml
spec:
  openbao:
    autoUnseal:
      enabled: true
      provider: transit
      address: https://vault.example.com:8200
      mountPath: transit
      keyName: autounseal
      token:
        secretName: transit-token
        key: token
```

---

## 6. Installation & Usage

### 6.1 Prerequisites

```bash
# Required
- Kubernetes 1.25+
- kubectl configured
- Helm 3.x (for operator installation)

# Optional but recommended
- cert-manager (for TLS)
- External Secrets Operator (for cloud secrets)
- Ingress controller (nginx, traefik, or istio)
```

### 6.2 Install Operator

```bash
# Option 1: Helm (recommended)
helm repo add banhbaoring https://charts.banhbaoring.io
helm repo update

helm install banhbaoring-operator banhbaoring/operator \
  --namespace banhbaoring-system \
  --create-namespace \
  --set image.tag=v1.0.0

# Option 2: Direct manifest
kubectl apply -f https://banhbaoring.io/install/operator.yaml
```

### 6.3 Deploy Full Stack

```bash
# 1. Create namespace
kubectl create namespace banhbaoring

# 2. Create required secrets
kubectl create secret generic stripe-secrets \
  --namespace banhbaoring \
  --from-literal=secret-key=sk_live_xxx \
  --from-literal=webhook-secret=whsec_xxx

kubectl create secret generic s3-credentials \
  --namespace banhbaoring \
  --from-literal=access-key-id=AKIA... \
  --from-literal=secret-access-key=xxx

kubectl create secret generic grafana-admin \
  --namespace banhbaoring \
  --from-literal=password=$(openssl rand -base64 32)

# 3. Deploy cluster
cat <<EOF | kubectl apply -f -
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: production
  namespace: banhbaoring
spec:
  domain: keys.mycompany.com
  
  openbao:
    replicas: 3
    autoUnseal:
      enabled: true
      provider: awskms
      keyId: alias/banhbaoring-unseal
    storage:
      size: 50Gi
      storageClass: gp3
  
  api:
    replicas: 2
    autoscaling:
      enabled: true
      maxReplicas: 10
  
  dashboard:
    replicas: 2
  
  database:
    managed: true
    storage:
      size: 20Gi
  
  redis:
    managed: true
    mode: cluster
  
  monitoring:
    enabled: true
    prometheus:
      retention: 15d
    grafana:
      enabled: true
  
  backup:
    enabled: true
    schedule: "0 2 * * *"
    retention: 30
    destination:
      s3:
        bucket: banhbaoring-backups
        region: us-east-1
        credentials:
          secretName: s3-credentials
  
  billing:
    stripe:
      enabled: true
      secretKeyRef:
        name: stripe-secrets
        key: secret-key
EOF
```

### 6.4 Check Status

```bash
# Watch deployment progress
kubectl get banhbaoringcluster production -n banhbaoring -w

# Detailed status
kubectl describe banhbaoringcluster production -n banhbaoring

# Get endpoints
kubectl get banhbaoringcluster production -n banhbaoring \
  -o jsonpath='{.status.endpoints}' | jq

# Output:
# {
#   "api": "https://api.keys.mycompany.com",
#   "dashboard": "https://keys.mycompany.com",
#   "openbao": "https://vault.keys.mycompany.com",
#   "grafana": "https://grafana.keys.mycompany.com"
# }
```

### 6.5 Create Tenants

```bash
kubectl apply -f - <<EOF
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingTenant
metadata:
  name: acme-corp
  namespace: banhbaoring
spec:
  clusterRef:
    name: production
  displayName: "Acme Corporation"
  plan: pro
  quotas:
    keys: 25
    signaturesPerMonth: 500000
  admin:
    email: admin@acme.com
EOF
```

---

## 7. Upgrade Strategy

### 7.1 Operator Upgrades

```bash
# Operator upgrades are automatic with Helm
helm upgrade banhbaoring-operator banhbaoring/operator \
  --namespace banhbaoring-system \
  --set image.tag=v1.1.0
```

### 7.2 Cluster Component Upgrades

```bash
# Update cluster spec
kubectl patch banhbaoringcluster production -n banhbaoring \
  --type merge \
  -p '{
    "spec": {
      "openbao": {"version": "2.1.0"},
      "api": {"version": "1.1.0"},
      "dashboard": {"version": "1.1.0"}
    }
  }'

# Operator performs rolling upgrade automatically
# Watch progress:
kubectl get pods -n banhbaoring -w
```

### 7.3 Pre-Upgrade Checklist

- [ ] Take manual backup: `kubectl apply -f backup-pre-upgrade.yaml`
- [ ] Review release notes for breaking changes
- [ ] Test upgrade in staging environment
- [ ] Verify unseal keys are available (if using Shamir)
- [ ] Schedule maintenance window
- [ ] Notify dependent applications
- [ ] Scale down non-critical workloads

---

## 8. Directory Structure

```
operator/
├── api/
│   └── v1/
│       ├── banhbaoringcluster_types.go
│       ├── banhbaoringtenant_types.go
│       ├── banhbaoringbackup_types.go
│       ├── banhbaoringrestore_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
│
├── controllers/
│   ├── cluster_controller.go
│   ├── cluster_controller_test.go
│   ├── tenant_controller.go
│   ├── tenant_controller_test.go
│   ├── backup_controller.go
│   ├── restore_controller.go
│   └── suite_test.go
│
├── internal/
│   ├── resources/
│   │   ├── openbao/
│   │   │   ├── statefulset.go
│   │   │   ├── service.go
│   │   │   ├── configmap.go
│   │   │   └── plugin.go
│   │   ├── api/
│   │   │   ├── deployment.go
│   │   │   ├── service.go
│   │   │   └── hpa.go
│   │   ├── dashboard/
│   │   │   ├── deployment.go
│   │   │   └── service.go
│   │   ├── database/
│   │   │   ├── postgresql.go
│   │   │   └── migrations.go
│   │   ├── redis/
│   │   │   └── cluster.go
│   │   ├── monitoring/
│   │   │   ├── prometheus.go
│   │   │   ├── grafana.go
│   │   │   └── alertmanager.go
│   │   ├── networking/
│   │   │   ├── ingress.go
│   │   │   └── networkpolicy.go
│   │   └── certificates/
│   │       └── certmanager.go
│   │
│   ├── unseal/
│   │   ├── interface.go
│   │   ├── awskms.go
│   │   ├── gcpkms.go
│   │   ├── azurekv.go
│   │   └── transit.go
│   │
│   ├── backup/
│   │   ├── interface.go
│   │   ├── openbao.go
│   │   ├── postgresql.go
│   │   ├── s3.go
│   │   └── gcs.go
│   │
│   └── health/
│       ├── checker.go
│       └── probes.go
│
├── config/
│   ├── crd/
│   │   └── bases/
│   │       ├── banhbaoring.io_banhbaoringclusters.yaml
│   │       ├── banhbaoring.io_banhbaoringtenants.yaml
│   │       ├── banhbaoring.io_banhbaoringbackups.yaml
│   │       └── banhbaoring.io_banhbaoringrestores.yaml
│   ├── rbac/
│   │   ├── role.yaml
│   │   ├── role_binding.yaml
│   │   └── service_account.yaml
│   ├── manager/
│   │   └── manager.yaml
│   └── samples/
│       ├── cluster_minimal.yaml
│       ├── cluster_production.yaml
│       └── tenant.yaml
│
├── charts/
│   └── operator/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── templates/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── serviceaccount.yaml
│       │   ├── clusterrole.yaml
│       │   ├── clusterrolebinding.yaml
│       │   └── crds/
│       └── README.md
│
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── main.go
```

---

## 9. Monitoring & Observability

### 9.1 Built-in Metrics

The operator exposes Prometheus metrics at `:8080/metrics`:

```
# Cluster health
banhbaoring_cluster_status{cluster="production", component="openbao"} 1
banhbaoring_cluster_status{cluster="production", component="api"} 1
banhbaoring_cluster_status{cluster="production", component="dashboard"} 1

# Reconciliation metrics
banhbaoring_reconcile_total{controller="cluster"} 150
banhbaoring_reconcile_errors_total{controller="cluster"} 2
banhbaoring_reconcile_duration_seconds{controller="cluster"} 0.5

# Tenant metrics
banhbaoring_tenant_count{cluster="production"} 25
banhbaoring_tenant_keys_total{tenant="acme-corp"} 5
banhbaoring_tenant_signatures_total{tenant="acme-corp"} 45231

# Backup metrics
banhbaoring_backup_last_success_timestamp{cluster="production"} 1704855600
banhbaoring_backup_size_bytes{cluster="production"} 1288490188
```

### 9.2 Pre-built Grafana Dashboards

The operator includes Grafana dashboards for:
- Cluster overview
- OpenBao metrics (requests, latency, storage)
- API metrics (requests, errors, latency)
- Tenant usage
- Backup status

---

## 10. Timeline

| Phase | Deliverables | Duration |
|-------|--------------|----------|
| **7.1** | CRDs, basic cluster controller | 1 week |
| **7.2** | OpenBao deployment + auto-unseal | 1 week |
| **7.3** | Database + Redis deployment | 1 week |
| **7.4** | API + Dashboard deployment | 1 week |
| **7.5** | Tenant controller, multi-tenancy | 1 week |
| **7.6** | Backup/restore controllers | 1 week |
| **7.7** | Monitoring integration | 1 week |
| **7.8** | Helm chart, testing, docs | 1 week |

**Total: 8 weeks**

---

## 11. Future Enhancements

| Enhancement | Description | Priority |
|-------------|-------------|----------|
| **Multi-cluster** | Federated deployments across regions | High |
| **GitOps** | ArgoCD/Flux integration | Medium |
| **Service Mesh** | Istio/Linkerd integration | Medium |
| **Cost Optimization** | Spot instances, autoscaling policies | Medium |
| **Chaos Engineering** | Built-in chaos testing | Low |
| **Operator Hub** | Publish to OperatorHub.io | Medium |

