# Agent 13A: Operator Foundation

## Overview

Set up the BanhBaoRing Kubernetes Operator project using Kubebuilder. Define all CRDs and controller scaffolding.

> **PRD Reference:** [`doc/product/PRD_OPERATOR.md`](../product/PRD_OPERATOR.md)

---

## Prerequisites

- Go 1.22+
- Docker (for building operator image)
- kubectl configured
- Kubebuilder v3.14+ installed

```bash
# Install kubebuilder
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
```

---

## Directory Structure

```
operator/
├── api/
│   └── v1/
│       ├── banhbaoringcluster_types.go    # Main cluster CRD
│       ├── banhbaoringtenant_types.go     # Tenant CRD
│       ├── banhbaoringbackup_types.go     # Backup CRD
│       ├── banhbaoringrestore_types.go    # Restore CRD
│       ├── groupversion_info.go           # API group info
│       └── zz_generated.deepcopy.go       # Generated
│
├── controllers/
│   ├── cluster_controller.go              # Main cluster reconciler
│   ├── tenant_controller.go               # Tenant reconciler
│   ├── backup_controller.go               # Backup reconciler
│   ├── restore_controller.go              # Restore reconciler
│   └── suite_test.go                      # Controller test suite
│
├── internal/
│   ├── resources/                         # K8s resource builders
│   │   └── common.go                      # Shared utilities
│   ├── conditions/                        # Status condition helpers
│   │   └── conditions.go
│   └── constants/
│       └── constants.go                   # Labels, annotations, etc.
│
├── config/
│   ├── crd/
│   │   └── bases/                         # Generated CRD YAMLs
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
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── main.go
```

---

## Deliverables

### 1. Initialize Kubebuilder Project

```bash
mkdir -p operator && cd operator

# Initialize project
kubebuilder init \
  --domain banhbaoring.io \
  --repo github.com/Bidon15/banhbaoring/operator \
  --project-name banhbaoring-operator

# Create CRDs
kubebuilder create api --group "" --version v1 --kind BanhBaoRingCluster --resource --controller
kubebuilder create api --group "" --version v1 --kind BanhBaoRingTenant --resource --controller
kubebuilder create api --group "" --version v1 --kind BanhBaoRingBackup --resource --controller
kubebuilder create api --group "" --version v1 --kind BanhBaoRingRestore --resource --controller
```

---

### 2. CRD Types: BanhBaoRingCluster

```go
// api/v1/banhbaoringcluster_types.go
package v1

import (
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BanhBaoRingClusterSpec defines the desired state
type BanhBaoRingClusterSpec struct {
    // Domain for the cluster endpoints (e.g., keys.mycompany.com)
    // +kubebuilder:validation:Required
    Domain string `json:"domain"`

    // OpenBao configuration
    // +kubebuilder:default={}
    OpenBao OpenBaoSpec `json:"openbao,omitempty"`

    // Control Plane API configuration
    // +kubebuilder:default={}
    API APISpec `json:"api,omitempty"`

    // Web Dashboard configuration
    // +kubebuilder:default={}
    Dashboard DashboardSpec `json:"dashboard,omitempty"`

    // Database configuration
    // +kubebuilder:default={}
    Database DatabaseSpec `json:"database,omitempty"`

    // Redis configuration
    // +kubebuilder:default={}
    Redis RedisSpec `json:"redis,omitempty"`

    // Monitoring configuration
    // +kubebuilder:default={}
    Monitoring MonitoringSpec `json:"monitoring,omitempty"`

    // Backup configuration
    // +kubebuilder:default={}
    Backup BackupSpec `json:"backup,omitempty"`

    // Billing configuration
    // +kubebuilder:default={}
    Billing BillingSpec `json:"billing,omitempty"`
}

// OpenBaoSpec configures the OpenBao cluster
type OpenBaoSpec struct {
    // +kubebuilder:default=3
    // +kubebuilder:validation:Minimum=1
    Replicas int32 `json:"replicas,omitempty"`

    // +kubebuilder:default="2.0.0"
    Version string `json:"version,omitempty"`

    // Storage configuration
    Storage StorageSpec `json:"storage,omitempty"`

    // Resource requirements
    Resources corev1.ResourceRequirements `json:"resources,omitempty"`

    // Auto-unseal configuration
    AutoUnseal AutoUnsealSpec `json:"autoUnseal,omitempty"`

    // TLS configuration
    TLS TLSSpec `json:"tls,omitempty"`

    // Plugin configuration
    Plugin PluginSpec `json:"plugin,omitempty"`
}

// AutoUnsealSpec configures auto-unseal
type AutoUnsealSpec struct {
    // Enable auto-unseal
    // +kubebuilder:default=false
    Enabled bool `json:"enabled,omitempty"`

    // Provider type: awskms, gcpkms, azurekv, transit
    // +kubebuilder:validation:Enum=awskms;gcpkms;azurekv;transit
    Provider string `json:"provider,omitempty"`

    // AWS KMS configuration
    AWSKMS *AWSKMSSpec `json:"awskms,omitempty"`

    // GCP Cloud KMS configuration
    GCPKMS *GCPKMSSpec `json:"gcpkms,omitempty"`

    // Azure Key Vault configuration
    AzureKV *AzureKVSpec `json:"azurekv,omitempty"`

    // Transit (another Vault) configuration
    Transit *TransitSpec `json:"transit,omitempty"`
}

type AWSKMSSpec struct {
    KeyID  string `json:"keyId"`
    Region string `json:"region,omitempty"`
    // Reference to secret containing access credentials
    Credentials *SecretKeyRef `json:"credentials,omitempty"`
}

type GCPKMSSpec struct {
    Project   string `json:"project"`
    Location  string `json:"location"`
    KeyRing   string `json:"keyRing"`
    CryptoKey string `json:"cryptoKey"`
    // Reference to secret containing service account key
    Credentials *SecretKeyRef `json:"credentials,omitempty"`
}

type AzureKVSpec struct {
    TenantID  string `json:"tenantId"`
    VaultName string `json:"vaultName"`
    KeyName   string `json:"keyName"`
    // Reference to secret containing Azure credentials
    Credentials *SecretKeyRef `json:"credentials,omitempty"`
}

type TransitSpec struct {
    Address   string       `json:"address"`
    MountPath string       `json:"mountPath,omitempty"`
    KeyName   string       `json:"keyName"`
    Token     SecretKeyRef `json:"token"`
}

type TLSSpec struct {
    // cert-manager issuer name
    Issuer string `json:"issuer,omitempty"`
    // Or use existing secret
    SecretName string `json:"secretName,omitempty"`
}

type PluginSpec struct {
    // +kubebuilder:default="1.0.0"
    Version string `json:"version,omitempty"`
}

type StorageSpec struct {
    // +kubebuilder:default="10Gi"
    Size resource.Quantity `json:"size,omitempty"`
    // StorageClass name
    StorageClass string `json:"storageClass,omitempty"`
}

// APISpec configures the Control Plane API
type APISpec struct {
    // +kubebuilder:default=2
    Replicas int32 `json:"replicas,omitempty"`

    // +kubebuilder:default="1.0.0"
    Version string `json:"version,omitempty"`

    Resources corev1.ResourceRequirements `json:"resources,omitempty"`

    Autoscaling AutoscalingSpec `json:"autoscaling,omitempty"`
}

type AutoscalingSpec struct {
    Enabled     bool  `json:"enabled,omitempty"`
    MinReplicas int32 `json:"minReplicas,omitempty"`
    MaxReplicas int32 `json:"maxReplicas,omitempty"`
    TargetCPU   int32 `json:"targetCPU,omitempty"`
}

// DashboardSpec configures the Web Dashboard
type DashboardSpec struct {
    // +kubebuilder:default=2
    Replicas int32 `json:"replicas,omitempty"`

    // +kubebuilder:default="1.0.0"
    Version string `json:"version,omitempty"`

    Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// DatabaseSpec configures PostgreSQL
type DatabaseSpec struct {
    // Deploy managed PostgreSQL (true) or use external (false)
    // +kubebuilder:default=true
    Managed bool `json:"managed,omitempty"`

    // +kubebuilder:default="16"
    Version string `json:"version,omitempty"`

    // +kubebuilder:default=1
    Replicas int32 `json:"replicas,omitempty"`

    Storage StorageSpec `json:"storage,omitempty"`

    // External database connection string
    ConnectionString *SecretKeyRef `json:"connectionString,omitempty"`
}

// RedisSpec configures Redis
type RedisSpec struct {
    // +kubebuilder:default=true
    Managed bool `json:"managed,omitempty"`

    // +kubebuilder:default="7"
    Version string `json:"version,omitempty"`

    // standalone or cluster
    // +kubebuilder:default="standalone"
    // +kubebuilder:validation:Enum=standalone;cluster
    Mode string `json:"mode,omitempty"`

    // +kubebuilder:default=1
    Replicas int32 `json:"replicas,omitempty"`

    Storage StorageSpec `json:"storage,omitempty"`
}

// MonitoringSpec configures observability stack
type MonitoringSpec struct {
    Enabled bool `json:"enabled,omitempty"`

    Prometheus PrometheusSpec `json:"prometheus,omitempty"`
    Grafana    GrafanaSpec    `json:"grafana,omitempty"`
    Alerting   AlertingSpec   `json:"alerting,omitempty"`
}

type PrometheusSpec struct {
    // +kubebuilder:default="15d"
    Retention string      `json:"retention,omitempty"`
    Storage   StorageSpec `json:"storage,omitempty"`
}

type GrafanaSpec struct {
    Enabled       bool          `json:"enabled,omitempty"`
    AdminPassword *SecretKeyRef `json:"adminPassword,omitempty"`
}

type AlertingSpec struct {
    Enabled   bool              `json:"enabled,omitempty"`
    Slack     *SlackAlertSpec   `json:"slack,omitempty"`
    PagerDuty *PagerDutySpec    `json:"pagerduty,omitempty"`
}

type SlackAlertSpec struct {
    WebhookURL SecretKeyRef `json:"webhookUrl"`
}

type PagerDutySpec struct {
    RoutingKey SecretKeyRef `json:"routingKey"`
}

// BackupSpec configures automatic backups
type BackupSpec struct {
    Enabled bool `json:"enabled,omitempty"`

    // Cron schedule (default: daily at 2 AM UTC)
    // +kubebuilder:default="0 2 * * *"
    Schedule string `json:"schedule,omitempty"`

    // Retention in days
    // +kubebuilder:default=30
    Retention int32 `json:"retention,omitempty"`

    Destination BackupDestination `json:"destination,omitempty"`
}

type BackupDestination struct {
    S3  *S3Destination  `json:"s3,omitempty"`
    GCS *GCSDestination `json:"gcs,omitempty"`
}

type S3Destination struct {
    Bucket      string       `json:"bucket"`
    Region      string       `json:"region,omitempty"`
    Prefix      string       `json:"prefix,omitempty"`
    Credentials SecretKeyRef `json:"credentials"`
}

type GCSDestination struct {
    Bucket      string       `json:"bucket"`
    Prefix      string       `json:"prefix,omitempty"`
    Credentials SecretKeyRef `json:"credentials"`
}

// BillingSpec configures payment integrations
type BillingSpec struct {
    Stripe StripeSpec `json:"stripe,omitempty"`
}

type StripeSpec struct {
    Enabled          bool         `json:"enabled,omitempty"`
    SecretKeyRef     SecretKeyRef `json:"secretKeyRef,omitempty"`
    WebhookSecretRef SecretKeyRef `json:"webhookSecretRef,omitempty"`
}

// SecretKeyRef references a key in a Secret
type SecretKeyRef struct {
    Name string `json:"name"`
    Key  string `json:"key"`
}

// BanhBaoRingClusterStatus defines the observed state
type BanhBaoRingClusterStatus struct {
    // Current phase: Pending, Initializing, Running, Degraded, Failed
    // +kubebuilder:default="Pending"
    Phase string `json:"phase,omitempty"`

    // Component statuses
    OpenBao   ComponentStatus `json:"openbao,omitempty"`
    API       ComponentStatus `json:"api,omitempty"`
    Dashboard ComponentStatus `json:"dashboard,omitempty"`
    Database  ComponentStatus `json:"database,omitempty"`
    Redis     ComponentStatus `json:"redis,omitempty"`

    // Access endpoints
    Endpoints EndpointsStatus `json:"endpoints,omitempty"`

    // Conditions
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // ObservedGeneration
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type ComponentStatus struct {
    Ready   bool   `json:"ready,omitempty"`
    Version string `json:"version,omitempty"`
    Message string `json:"message,omitempty"`
    // For StatefulSets: "2/3" format
    Replicas string `json:"replicas,omitempty"`
}

type EndpointsStatus struct {
    API       string `json:"api,omitempty"`
    Dashboard string `json:"dashboard,omitempty"`
    OpenBao   string `json:"openbao,omitempty"`
    Grafana   string `json:"grafana,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="OpenBao",type=string,JSONPath=`.status.openbao.replicas`
// +kubebuilder:printcolumn:name="API",type=string,JSONPath=`.status.api.replicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BanhBaoRingCluster is the Schema for the banhbaoringclusters API
type BanhBaoRingCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   BanhBaoRingClusterSpec   `json:"spec,omitempty"`
    Status BanhBaoRingClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BanhBaoRingClusterList contains a list of BanhBaoRingCluster
type BanhBaoRingClusterList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []BanhBaoRingCluster `json:"items"`
}

func init() {
    SchemeBuilder.Register(&BanhBaoRingCluster{}, &BanhBaoRingClusterList{})
}
```

---

### 3. CRD Types: BanhBaoRingTenant

```go
// api/v1/banhbaoringtenant_types.go
package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BanhBaoRingTenantSpec defines the desired state
type BanhBaoRingTenantSpec struct {
    // Reference to the parent cluster
    // +kubebuilder:validation:Required
    ClusterRef ClusterReference `json:"clusterRef"`

    // Display name for the tenant
    DisplayName string `json:"displayName,omitempty"`

    // Plan: free, starter, pro, enterprise
    // +kubebuilder:validation:Enum=free;starter;pro;enterprise
    // +kubebuilder:default="free"
    Plan string `json:"plan,omitempty"`

    // Resource quotas
    Quotas TenantQuotas `json:"quotas,omitempty"`

    // Initial admin user
    Admin TenantAdmin `json:"admin,omitempty"`

    // Custom settings
    Settings TenantSettings `json:"settings,omitempty"`
}

type ClusterReference struct {
    Name string `json:"name"`
}

type TenantQuotas struct {
    // +kubebuilder:default=5
    Keys int32 `json:"keys,omitempty"`

    // +kubebuilder:default=10000
    SignaturesPerMonth int64 `json:"signaturesPerMonth,omitempty"`

    // +kubebuilder:default=1
    Namespaces int32 `json:"namespaces,omitempty"`

    // +kubebuilder:default=1
    TeamMembers int32 `json:"teamMembers,omitempty"`

    // +kubebuilder:default=2
    APIKeys int32 `json:"apiKeys,omitempty"`
}

type TenantAdmin struct {
    Email    string        `json:"email"`
    Password *SecretKeyRef `json:"password,omitempty"`
}

type TenantSettings struct {
    // +kubebuilder:default=30
    AuditRetentionDays int32 `json:"auditRetentionDays,omitempty"`

    AllowExportableKeys bool     `json:"allowExportableKeys,omitempty"`
    AllowedIPRanges     []string `json:"allowedIPRanges,omitempty"`

    Webhooks []WebhookConfig `json:"webhooks,omitempty"`
}

type WebhookConfig struct {
    URL    string       `json:"url"`
    Events []string     `json:"events,omitempty"`
    Secret SecretKeyRef `json:"secret"`
}

// BanhBaoRingTenantStatus defines the observed state
type BanhBaoRingTenantStatus struct {
    // Phase: Pending, Active, Suspended, Deleted
    // +kubebuilder:default="Pending"
    Phase string `json:"phase,omitempty"`

    // OpenBao namespace for this tenant
    OpenBaoNamespace string `json:"openbaoNamespace,omitempty"`

    // Current usage
    Usage TenantUsage `json:"usage,omitempty"`

    CreatedAt    *metav1.Time `json:"createdAt,omitempty"`
    LastActiveAt *metav1.Time `json:"lastActiveAt,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type TenantUsage struct {
    Keys                int32 `json:"keys,omitempty"`
    SignaturesThisMonth int64 `json:"signaturesThisMonth,omitempty"`
    APIKeys             int32 `json:"apiKeys,omitempty"`
    TeamMembers         int32 `json:"teamMembers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Keys",type=integer,JSONPath=`.status.usage.keys`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type BanhBaoRingTenant struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   BanhBaoRingTenantSpec   `json:"spec,omitempty"`
    Status BanhBaoRingTenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type BanhBaoRingTenantList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []BanhBaoRingTenant `json:"items"`
}

func init() {
    SchemeBuilder.Register(&BanhBaoRingTenant{}, &BanhBaoRingTenantList{})
}
```

---

### 4. CRD Types: BanhBaoRingBackup & BanhBaoRingRestore

```go
// api/v1/banhbaoringbackup_types.go
package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BanhBaoRingBackupSpec struct {
    // +kubebuilder:validation:Required
    ClusterRef ClusterReference `json:"clusterRef"`

    // full or incremental
    // +kubebuilder:validation:Enum=full;incremental
    // +kubebuilder:default="full"
    Type string `json:"type,omitempty"`

    // Components to backup
    // +kubebuilder:default={"openbao","database","secrets"}
    Components []string `json:"components,omitempty"`

    // Override cluster backup destination
    Destination *BackupDestination `json:"destination,omitempty"`
}

type BanhBaoRingBackupStatus struct {
    // Pending, Running, Completed, Failed
    Phase string `json:"phase,omitempty"`

    StartedAt   *metav1.Time `json:"startedAt,omitempty"`
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    Components []BackupComponentStatus `json:"components,omitempty"`
    TotalSize  string                  `json:"totalSize,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type BackupComponentStatus struct {
    Name     string `json:"name"`
    Status   string `json:"status"`
    Size     string `json:"size,omitempty"`
    Location string `json:"location,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.status.totalSize`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type BanhBaoRingBackup struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   BanhBaoRingBackupSpec   `json:"spec,omitempty"`
    Status BanhBaoRingBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type BanhBaoRingBackupList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []BanhBaoRingBackup `json:"items"`
}

func init() {
    SchemeBuilder.Register(&BanhBaoRingBackup{}, &BanhBaoRingBackupList{})
}
```

```go
// api/v1/banhbaoringrestore_types.go
package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BanhBaoRingRestoreSpec struct {
    // +kubebuilder:validation:Required
    ClusterRef ClusterReference `json:"clusterRef"`

    // Reference to backup resource
    BackupRef *BackupReference `json:"backupRef,omitempty"`

    // Or restore from specific location
    Source *BackupDestination `json:"source,omitempty"`

    // Components to restore (default: all from backup)
    Components []string `json:"components,omitempty"`

    Options RestoreOptions `json:"options,omitempty"`
}

type BackupReference struct {
    Name string `json:"name"`
}

type RestoreOptions struct {
    // +kubebuilder:default=true
    StopApplications bool `json:"stopApplications,omitempty"`

    // +kubebuilder:default=true
    VerifyBackup bool `json:"verifyBackup,omitempty"`
}

type BanhBaoRingRestoreStatus struct {
    // Pending, Stopping, Restoring, Starting, Completed, Failed
    Phase string `json:"phase,omitempty"`

    StartedAt   *metav1.Time `json:"startedAt,omitempty"`
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    Steps []RestoreStep `json:"steps,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type RestoreStep struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type BanhBaoRingRestore struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   BanhBaoRingRestoreSpec   `json:"spec,omitempty"`
    Status BanhBaoRingRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type BanhBaoRingRestoreList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []BanhBaoRingRestore `json:"items"`
}

func init() {
    SchemeBuilder.Register(&BanhBaoRingRestore{}, &BanhBaoRingRestoreList{})
}
```

---

### 5. Controller Stub: Cluster Controller

```go
// controllers/cluster_controller.go
package controllers

import (
    "context"
    "time"

    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

// ClusterReconciler reconciles a BanhBaoRingCluster object
type ClusterReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    log.Info("Reconciling BanhBaoRingCluster", "name", req.Name)

    // Fetch the cluster
    cluster := &banhbaoringv1.BanhBaoRingCluster{}
    if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // TODO: Implement reconciliation phases
    // Phase 1: Prerequisites (namespace, certificates)
    // Phase 2: Data layer (PostgreSQL, Redis)
    // Phase 3: OpenBao (StatefulSet, unseal, plugin)
    // Phase 4: Applications (API, Dashboard)
    // Phase 5: Monitoring (optional)
    // Phase 6: Ingress & networking

    // Requeue after 30s for periodic reconciliation
    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&banhbaoringv1.BanhBaoRingCluster{}).
        Complete(r)
}
```

---

### 6. Constants & Utilities

```go
// internal/constants/constants.go
package constants

const (
    // Labels
    LabelApp       = "app.kubernetes.io/name"
    LabelInstance  = "app.kubernetes.io/instance"
    LabelVersion   = "app.kubernetes.io/version"
    LabelComponent = "app.kubernetes.io/component"
    LabelManagedBy = "app.kubernetes.io/managed-by"

    // Component names
    ComponentOpenBao   = "openbao"
    ComponentAPI       = "api"
    ComponentDashboard = "dashboard"
    ComponentPostgres  = "postgres"
    ComponentRedis     = "redis"

    // Phases
    PhasePending      = "Pending"
    PhaseInitializing = "Initializing"
    PhaseRunning      = "Running"
    PhaseDegraded     = "Degraded"
    PhaseFailed       = "Failed"

    // Finalizer
    Finalizer = "banhbaoring.io/finalizer"

    // Manager name
    ManagedBy = "banhbaoring-operator"
)

// Labels returns standard labels for a component
func Labels(clusterName, component, version string) map[string]string {
    return map[string]string{
        LabelApp:       "banhbaoring",
        LabelInstance:  clusterName,
        LabelComponent: component,
        LabelVersion:   version,
        LabelManagedBy: ManagedBy,
    }
}
```

```go
// internal/conditions/conditions.go
package conditions

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
    TypeReady         = "Ready"
    TypeTLSReady      = "TLSReady"
    TypeDatabaseReady = "DatabaseReady"
    TypeOpenBaoReady  = "OpenBaoReady"
    TypeAPIReady      = "APIReady"
    TypeBackupSuccess = "BackupSucceeded"
)

// SetCondition adds or updates a condition
func SetCondition(conditions *[]metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string) {
    now := metav1.Now()
    
    for i, c := range *conditions {
        if c.Type == condType {
            if c.Status != status {
                (*conditions)[i].Status = status
                (*conditions)[i].LastTransitionTime = now
            }
            (*conditions)[i].Reason = reason
            (*conditions)[i].Message = message
            return
        }
    }

    *conditions = append(*conditions, metav1.Condition{
        Type:               condType,
        Status:             status,
        LastTransitionTime: now,
        Reason:             reason,
        Message:            message,
    })
}
```

---

### 7. Makefile Additions

```makefile
# Makefile (append to generated)

.PHONY: generate manifests install

# Generate code (deepcopy, CRDs)
generate:
	go generate ./...
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Generate CRD manifests
manifests:
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Install CRDs into cluster
install: manifests
	kubectl apply -f config/crd/bases

# Run operator locally
run: generate
	go run ./main.go

# Build operator image
docker-build:
	docker build -t banhbaoring-operator:latest .

# Deploy to cluster
deploy: manifests
	kubectl apply -k config/default
```

---

### 8. Sample CRs

```yaml
# config/samples/cluster_minimal.yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: dev
  namespace: banhbaoring
spec:
  domain: keys.local
  openbao:
    replicas: 1
    storage:
      size: 1Gi
  database:
    managed: true
    storage:
      size: 1Gi
  redis:
    managed: true
    mode: standalone
```

```yaml
# config/samples/cluster_production.yaml
apiVersion: banhbaoring.io/v1
kind: BanhBaoRingCluster
metadata:
  name: production
  namespace: banhbaoring
spec:
  domain: keys.mycompany.com
  
  openbao:
    replicas: 3
    version: "2.0.0"
    autoUnseal:
      enabled: true
      provider: awskms
      awskms:
        keyId: alias/banhbaoring-unseal
        region: us-east-1
    storage:
      size: 50Gi
      storageClass: gp3
  
  api:
    replicas: 2
    autoscaling:
      enabled: true
      minReplicas: 2
      maxReplicas: 10
      targetCPU: 70
  
  dashboard:
    replicas: 2
  
  database:
    managed: true
    replicas: 2
    storage:
      size: 20Gi
      storageClass: gp3
  
  redis:
    managed: true
    mode: cluster
    replicas: 3
  
  monitoring:
    enabled: true
    prometheus:
      retention: 15d
      storage:
        size: 50Gi
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
          name: s3-credentials
          key: config
  
  billing:
    stripe:
      enabled: true
      secretKeyRef:
        name: stripe-secrets
        key: secret-key
```

```yaml
# config/samples/tenant.yaml
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
    namespaces: 5
    teamMembers: 10
  admin:
    email: admin@acme.com
```

---

## Test Commands

```bash
cd operator

# Generate code and manifests
make generate manifests

# Build
go build ./...

# Run tests
go test ./... -v

# Install CRDs locally (requires kubectl + cluster)
make install

# Run operator locally
make run
```

---

## Acceptance Criteria

- [ ] `kubebuilder init` completed successfully
- [ ] All 4 CRDs defined with proper markers
- [ ] Controller stubs created for all CRDs
- [ ] `make generate manifests` succeeds
- [ ] `go build ./...` succeeds
- [ ] Sample CRs created (minimal + production)
- [ ] Constants and condition helpers implemented

