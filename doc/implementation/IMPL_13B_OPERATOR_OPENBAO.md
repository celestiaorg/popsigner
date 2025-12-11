# Agent 13B: OpenBao Controller

## Overview

Implement the OpenBao deployment logic in the Cluster controller. This agent **leverages OpenBao's official Helm chart** as a subchart for battle-tested deployment, while our operator handles plugin registration, integration, and orchestration.

> **Requires:** Agent 13A (Operator Foundation) complete

---

## Deployment Strategy

### Why Use OpenBao's Helm Chart?

| Approach | Pros | Cons |
|----------|------|------|
| **Full Custom** | Total control | High maintenance, reinventing wheel |
| **OpenBao Helm Subchart** ✅ | Battle-tested, maintained by OpenBao team | Less flexibility |
| **Hybrid (Recommended)** | Best of both - use Helm for core, operator for customization | Slightly more complex |

**Our approach:** Use OpenBao Helm chart for core deployment, operator handles:
- secp256k1 plugin registration
- Integration with Control Plane  
- Tenant namespace provisioning
- Backup/restore coordination

### Scalability

The design supports horizontal scaling:

| Component | Scaling Method | Notes |
|-----------|---------------|-------|
| **OpenBao** | StatefulSet replicas (3, 5, 7) | Raft consensus requires odd numbers |
| **API** | HPA (min 2, max 10+) | CPU-based autoscaling |
| **Dashboard** | Deployment replicas | Stateless, scale freely |
| **PostgreSQL** | Read replicas | Primary + replicas for HA |
| **Redis** | Cluster mode (3+ nodes) | Sharding for high throughput |

---

## Prerequisites

- Agent 13A completed (CRDs and controller stubs exist)
- Understanding of OpenBao/Vault architecture
- Familiarity with Kubernetes StatefulSets and Helm

---

## Helm-Based Deployment Option

For production, consider using Helm SDK to deploy OpenBao's official chart:

```go
// internal/helm/openbao.go
package helm

import (
    "context"
    "fmt"

    "helm.sh/helm/v3/pkg/action"
    "helm.sh/helm/v3/pkg/chart/loader"
    
    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

const (
    OpenBaoChartRepo = "https://openbao.github.io/openbao-helm"
    OpenBaoChartName = "openbao"
)

// DeployOpenBao uses Helm SDK to deploy OpenBao's official chart
func DeployOpenBao(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    cfg := new(action.Configuration)
    // Initialize Helm action config...
    
    install := action.NewInstall(cfg)
    install.ReleaseName = fmt.Sprintf("%s-openbao", cluster.Name)
    install.Namespace = cluster.Namespace
    install.CreateNamespace = false
    
    // Build values from cluster spec
    values := buildOpenBaoValues(cluster)
    
    chart, err := loader.Load(OpenBaoChartName)
    if err != nil {
        return err
    }
    
    _, err = install.Run(chart, values)
    return err
}

func buildOpenBaoValues(cluster *banhbaoringv1.BanhBaoRingCluster) map[string]interface{} {
    spec := cluster.Spec.OpenBao
    
    return map[string]interface{}{
        "server": map[string]interface{}{
            "ha": map[string]interface{}{
                "enabled":  spec.Replicas > 1,
                "replicas": spec.Replicas,
                "raft": map[string]interface{}{
                    "enabled": true,
                },
            },
            "dataStorage": map[string]interface{}{
                "size":         spec.Storage.Size.String(),
                "storageClass": spec.Storage.StorageClass,
            },
            "extraVolumes": []map[string]interface{}{
                {"type": "emptyDir", "name": "plugins"},
            },
            "extraInitContainers": []map[string]interface{}{
                // Plugin download init container
            },
        },
        "injector": map[string]interface{}{
            "enabled": false, // We don't need the injector
        },
    }
}
```

**Decision Point:** The operator can either:
1. Use Helm SDK (above) - Recommended for production
2. Use raw K8s resources (below) - More control, useful for learning

For initial development, we'll implement raw K8s resources to understand the deployment deeply, then optionally migrate to Helm SDK.

---

## Deliverables

### 1. OpenBao Resource Builder (Raw K8s Approach)

```go
// internal/resources/openbao/statefulset.go
package openbao

import (
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
    OpenBaoImage     = "openbao/openbao"
    OpenBaoPort      = 8200
    OpenBaoClusterPort = 8201
    PluginDir        = "/vault/plugins"
)

// StatefulSet builds the OpenBao StatefulSet
func StatefulSet(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.StatefulSet {
    spec := cluster.Spec.OpenBao
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentOpenBao, spec.Version)

    replicas := spec.Replicas
    if replicas == 0 {
        replicas = 3
    }

    return &appsv1.StatefulSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: appsv1.StatefulSetSpec{
            ServiceName: name,
            Replicas:    &replicas,
            Selector: &metav1.LabelSelector{
                MatchLabels: labels,
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: labels,
                },
                Spec: corev1.PodSpec{
                    ServiceAccountName: name,
                    SecurityContext: &corev1.PodSecurityContext{
                        FSGroup: int64Ptr(1000),
                    },
                    Containers: []corev1.Container{
                        {
                            Name:  "openbao",
                            Image: fmt.Sprintf("%s:%s", OpenBaoImage, spec.Version),
                            Command: []string{
                                "bao", "server", "-config=/vault/config/config.hcl",
                            },
                            Ports: []corev1.ContainerPort{
                                {Name: "api", ContainerPort: OpenBaoPort},
                                {Name: "cluster", ContainerPort: OpenBaoClusterPort},
                            },
                            Env: buildEnv(cluster),
                            VolumeMounts: []corev1.VolumeMount{
                                {Name: "data", MountPath: "/vault/data"},
                                {Name: "config", MountPath: "/vault/config"},
                                {Name: "plugins", MountPath: PluginDir},
                            },
                            Resources: spec.Resources,
                            ReadinessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    HTTPGet: &corev1.HTTPGetAction{
                                        Path:   "/v1/sys/health?standbyok=true",
                                        Port:   intstr.FromInt(OpenBaoPort),
                                        Scheme: corev1.URISchemeHTTPS,
                                    },
                                },
                                InitialDelaySeconds: 5,
                                PeriodSeconds:       10,
                            },
                            LivenessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    HTTPGet: &corev1.HTTPGetAction{
                                        Path:   "/v1/sys/health?standbyok=true",
                                        Port:   intstr.FromInt(OpenBaoPort),
                                        Scheme: corev1.URISchemeHTTPS,
                                    },
                                },
                                InitialDelaySeconds: 30,
                                PeriodSeconds:       30,
                            },
                            SecurityContext: &corev1.SecurityContext{
                                Capabilities: &corev1.Capabilities{
                                    Add: []corev1.Capability{"IPC_LOCK"},
                                },
                            },
                        },
                    },
                    Volumes: []corev1.Volume{
                        {
                            Name: "config",
                            VolumeSource: corev1.VolumeSource{
                                ConfigMap: &corev1.ConfigMapVolumeSource{
                                    LocalObjectReference: corev1.LocalObjectReference{
                                        Name: name + "-config",
                                    },
                                },
                            },
                        },
                        {
                            Name: "plugins",
                            VolumeSource: corev1.VolumeSource{
                                EmptyDir: &corev1.EmptyDirVolumeSource{},
                            },
                        },
                    },
                },
            },
            VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
                {
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "data",
                    },
                    Spec: corev1.PersistentVolumeClaimSpec{
                        AccessModes: []corev1.PersistentVolumeAccessMode{
                            corev1.ReadWriteOnce,
                        },
                        Resources: corev1.VolumeResourceRequirements{
                            Requests: corev1.ResourceList{
                                corev1.ResourceStorage: spec.Storage.Size,
                            },
                        },
                        StorageClassName: storageClassPtr(spec.Storage.StorageClass),
                    },
                },
            },
        },
    }
}

func buildEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    
    env := []corev1.EnvVar{
        {Name: "VAULT_ADDR", Value: "https://127.0.0.1:8200"},
        {Name: "VAULT_CLUSTER_ADDR", Value: fmt.Sprintf("https://$(HOSTNAME).%s:8201", name)},
        {Name: "VAULT_API_ADDR", Value: fmt.Sprintf("https://$(HOSTNAME).%s:8200", name)},
        {Name: "VAULT_SKIP_VERIFY", Value: "true"},
        {
            Name: "HOSTNAME",
            ValueFrom: &corev1.EnvVarSource{
                FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
            },
        },
    }

    // Add auto-unseal env vars
    if cluster.Spec.OpenBao.AutoUnseal.Enabled {
        env = append(env, autoUnsealEnv(cluster)...)
    }

    return env
}

func autoUnsealEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
    unseal := cluster.Spec.OpenBao.AutoUnseal
    var env []corev1.EnvVar

    switch unseal.Provider {
    case "awskms":
        if unseal.AWSKMS != nil {
            env = append(env,
                corev1.EnvVar{Name: "AWS_REGION", Value: unseal.AWSKMS.Region},
            )
            if unseal.AWSKMS.Credentials != nil {
                env = append(env,
                    envFromSecret("AWS_ACCESS_KEY_ID", unseal.AWSKMS.Credentials.Name, "access-key-id"),
                    envFromSecret("AWS_SECRET_ACCESS_KEY", unseal.AWSKMS.Credentials.Name, "secret-access-key"),
                )
            }
        }
    case "gcpkms":
        if unseal.GCPKMS != nil {
            env = append(env,
                corev1.EnvVar{Name: "GOOGLE_PROJECT", Value: unseal.GCPKMS.Project},
            )
        }
    case "azurekv":
        if unseal.AzureKV != nil {
            env = append(env,
                corev1.EnvVar{Name: "AZURE_TENANT_ID", Value: unseal.AzureKV.TenantID},
            )
        }
    }

    return env
}

func envFromSecret(name, secretName, key string) corev1.EnvVar {
    return corev1.EnvVar{
        Name: name,
        ValueFrom: &corev1.EnvVarSource{
            SecretKeyRef: &corev1.SecretKeySelector{
                LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
                Key:                  key,
            },
        },
    }
}

func int64Ptr(i int64) *int64 { return &i }
func storageClassPtr(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

---

### 2. OpenBao ConfigMap

```go
// internal/resources/openbao/configmap.go
package openbao

import (
    "bytes"
    "fmt"
    "text/template"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const configTemplate = `
ui = true
disable_mlock = false
plugin_directory = "{{ .PluginDir }}"

listener "tcp" {
  address         = "0.0.0.0:8200"
  cluster_address = "0.0.0.0:8201"
  tls_cert_file   = "/vault/tls/tls.crt"
  tls_key_file    = "/vault/tls/tls.key"
}

storage "raft" {
  path = "/vault/data"
  node_id = "${HOSTNAME}"
  
  {{ range $i := .PeerIndices }}
  retry_join {
    leader_api_addr = "https://{{ $.StatefulSetName }}-{{ $i }}.{{ $.StatefulSetName }}:8200"
    leader_ca_cert_file = "/vault/tls/ca.crt"
  }
  {{ end }}
}

api_addr     = "https://${HOSTNAME}.{{ .StatefulSetName }}:8200"
cluster_addr = "https://${HOSTNAME}.{{ .StatefulSetName }}:8201"

{{ if .AutoUnseal }}
{{ .SealConfig }}
{{ end }}

telemetry {
  prometheus_retention_time = "30s"
  disable_hostname = true
}
`

type ConfigData struct {
    PluginDir       string
    StatefulSetName string
    PeerIndices     []int
    AutoUnseal      bool
    SealConfig      string
}

// ConfigMap builds the OpenBao configuration ConfigMap
func ConfigMap(cluster *banhbaoringv1.BanhBaoRingCluster) (*corev1.ConfigMap, error) {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)

    replicas := int(cluster.Spec.OpenBao.Replicas)
    if replicas == 0 {
        replicas = 3
    }

    peerIndices := make([]int, replicas)
    for i := 0; i < replicas; i++ {
        peerIndices[i] = i
    }

    data := ConfigData{
        PluginDir:       PluginDir,
        StatefulSetName: name,
        PeerIndices:     peerIndices,
        AutoUnseal:      cluster.Spec.OpenBao.AutoUnseal.Enabled,
        SealConfig:      buildSealConfig(cluster),
    }

    tmpl, err := template.New("config").Parse(configTemplate)
    if err != nil {
        return nil, err
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return nil, err
    }

    return &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name + "-config",
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Data: map[string]string{
            "config.hcl": buf.String(),
        },
    }, nil
}

func buildSealConfig(cluster *banhbaoringv1.BanhBaoRingCluster) string {
    unseal := cluster.Spec.OpenBao.AutoUnseal
    if !unseal.Enabled {
        return ""
    }

    switch unseal.Provider {
    case "awskms":
        if unseal.AWSKMS != nil {
            return fmt.Sprintf(`seal "awskms" {
  region     = "%s"
  kms_key_id = "%s"
}`, unseal.AWSKMS.Region, unseal.AWSKMS.KeyID)
        }

    case "gcpkms":
        if unseal.GCPKMS != nil {
            return fmt.Sprintf(`seal "gcpckms" {
  project     = "%s"
  region      = "%s"
  key_ring    = "%s"
  crypto_key  = "%s"
}`, unseal.GCPKMS.Project, unseal.GCPKMS.Location, unseal.GCPKMS.KeyRing, unseal.GCPKMS.CryptoKey)
        }

    case "azurekv":
        if unseal.AzureKV != nil {
            return fmt.Sprintf(`seal "azurekeyvault" {
  tenant_id  = "%s"
  vault_name = "%s"
  key_name   = "%s"
}`, unseal.AzureKV.TenantID, unseal.AzureKV.VaultName, unseal.AzureKV.KeyName)
        }

    case "transit":
        if unseal.Transit != nil {
            return fmt.Sprintf(`seal "transit" {
  address    = "%s"
  mount_path = "%s"
  key_name   = "%s"
}`, unseal.Transit.Address, unseal.Transit.MountPath, unseal.Transit.KeyName)
        }
    }

    return ""
}
```

---

### 3. OpenBao Service

```go
// internal/resources/openbao/service.go
package openbao

import (
    "fmt"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

// HeadlessService builds the headless service for StatefulSet DNS
func HeadlessService(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: corev1.ServiceSpec{
            ClusterIP: corev1.ClusterIPNone,
            Selector:  labels,
            Ports: []corev1.ServicePort{
                {Name: "api", Port: OpenBaoPort, TargetPort: intstr.FromInt(OpenBaoPort)},
                {Name: "cluster", Port: OpenBaoClusterPort, TargetPort: intstr.FromInt(OpenBaoClusterPort)},
            },
        },
    }
}

// ActiveService builds the service that routes to the active leader
func ActiveService(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name + "-active",
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: corev1.ServiceSpec{
            Type:     corev1.ServiceTypeClusterIP,
            Selector: labels,
            Ports: []corev1.ServicePort{
                {Name: "api", Port: OpenBaoPort, TargetPort: intstr.FromInt(OpenBaoPort)},
            },
        },
    }
}
```

---

### 4. Plugin Registration

```go
// internal/resources/openbao/plugin.go
package openbao

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

const (
    PluginName     = "secp256k1"
    PluginBinaryName = "banhbaoring-secp256k1"
)

// PluginDownloadURL returns the URL for downloading the plugin binary
func PluginDownloadURL(version string, os, arch string) string {
    return fmt.Sprintf(
        "https://github.com/Bidon15/banhbaoring/releases/download/v%s/%s_%s_%s",
        version, PluginBinaryName, os, arch,
    )
}

// PluginInfo contains information needed to register the plugin
type PluginInfo struct {
    Name    string
    Command string
    SHA256  string
    Version string
}

// GetPluginInfo returns plugin registration info
func GetPluginInfo(cluster *banhbaoringv1.BanhBaoRingCluster) PluginInfo {
    return PluginInfo{
        Name:    PluginName,
        Command: PluginBinaryName,
        Version: cluster.Spec.OpenBao.Plugin.Version,
    }
}

// InitContainer returns an init container that downloads the plugin
func InitContainer(cluster *banhbaoringv1.BanhBaoRingCluster) corev1.Container {
    version := cluster.Spec.OpenBao.Plugin.Version
    if version == "" {
        version = "1.0.0"
    }

    downloadScript := fmt.Sprintf(`#!/bin/sh
set -e
PLUGIN_URL="%s"
PLUGIN_PATH="/vault/plugins/%s"

# Download plugin
wget -O "$PLUGIN_PATH" "$PLUGIN_URL"
chmod +x "$PLUGIN_PATH"

# Calculate SHA256
sha256sum "$PLUGIN_PATH" | cut -d' ' -f1 > /vault/plugins/sha256.txt

echo "Plugin downloaded: $(sha256sum $PLUGIN_PATH)"
`, PluginDownloadURL(version, "linux", "amd64"), PluginBinaryName)

    return corev1.Container{
        Name:  "download-plugin",
        Image: "alpine:latest",
        Command: []string{"/bin/sh", "-c", downloadScript},
        VolumeMounts: []corev1.VolumeMount{
            {Name: "plugins", MountPath: PluginDir},
        },
    }
}

// RegisterPluginScript returns a script to register the plugin with OpenBao
func RegisterPluginScript() string {
    return fmt.Sprintf(`#!/bin/sh
set -e

VAULT_ADDR="https://127.0.0.1:8200"
VAULT_TOKEN="$VAULT_ROOT_TOKEN"

# Wait for Vault to be ready
until vault status 2>/dev/null; do
    echo "Waiting for Vault..."
    sleep 2
done

# Get plugin SHA256
PLUGIN_SHA=$(cat /vault/plugins/sha256.txt)

# Register the plugin
vault plugin register -sha256="$PLUGIN_SHA" secret %s

# Enable the secrets engine
vault secrets enable -path=keys %s

echo "Plugin registered successfully"
`, PluginName, PluginName)
}
```

---

### 5. Controller Reconciliation Logic

```go
// controllers/cluster_openbao.go
package controllers

import (
    "context"
    "fmt"
    "time"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/log"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/conditions"
    "github.com/Bidon15/banhbaoring/operator/internal/resources/openbao"
)

// reconcileOpenBao handles all OpenBao resources
func (r *ClusterReconciler) reconcileOpenBao(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)
    log.Info("Reconciling OpenBao")

    // 1. Create/update ConfigMap
    configMap, err := openbao.ConfigMap(cluster)
    if err != nil {
        return fmt.Errorf("failed to build configmap: %w", err)
    }
    if err := r.createOrUpdate(ctx, cluster, configMap); err != nil {
        return fmt.Errorf("failed to reconcile configmap: %w", err)
    }

    // 2. Create/update headless Service
    headlessSvc := openbao.HeadlessService(cluster)
    if err := r.createOrUpdate(ctx, cluster, headlessSvc); err != nil {
        return fmt.Errorf("failed to reconcile headless service: %w", err)
    }

    // 3. Create/update active Service
    activeSvc := openbao.ActiveService(cluster)
    if err := r.createOrUpdate(ctx, cluster, activeSvc); err != nil {
        return fmt.Errorf("failed to reconcile active service: %w", err)
    }

    // 4. Create/update StatefulSet
    sts := openbao.StatefulSet(cluster)
    // Add init container for plugin download
    sts.Spec.Template.Spec.InitContainers = append(
        sts.Spec.Template.Spec.InitContainers,
        openbao.InitContainer(cluster),
    )
    if err := r.createOrUpdate(ctx, cluster, sts); err != nil {
        return fmt.Errorf("failed to reconcile statefulset: %w", err)
    }

    return nil
}

// isOpenBaoReady checks if OpenBao pods are ready
func (r *ClusterReconciler) isOpenBaoReady(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) bool {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    
    sts := &appsv1.StatefulSet{}
    if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
        return false
    }

    expectedReplicas := cluster.Spec.OpenBao.Replicas
    if expectedReplicas == 0 {
        expectedReplicas = 3
    }

    return sts.Status.ReadyReplicas >= expectedReplicas
}

// updateOpenBaoStatus updates the cluster status with OpenBao info
func (r *ClusterReconciler) updateOpenBaoStatus(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    name := fmt.Sprintf("%s-openbao", cluster.Name)
    
    sts := &appsv1.StatefulSet{}
    if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
        if errors.IsNotFound(err) {
            cluster.Status.OpenBao = banhbaoringv1.ComponentStatus{
                Ready:   false,
                Message: "StatefulSet not found",
            }
            return nil
        }
        return err
    }

    ready := sts.Status.ReadyReplicas >= *sts.Spec.Replicas
    
    cluster.Status.OpenBao = banhbaoringv1.ComponentStatus{
        Ready:    ready,
        Version:  cluster.Spec.OpenBao.Version,
        Replicas: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
    }

    condStatus := metav1.ConditionFalse
    reason := "NotReady"
    message := fmt.Sprintf("Waiting for OpenBao pods: %d/%d ready", sts.Status.ReadyReplicas, *sts.Spec.Replicas)
    
    if ready {
        condStatus = metav1.ConditionTrue
        reason = "Ready"
        message = "OpenBao cluster is ready"
    }

    conditions.SetCondition(&cluster.Status.Conditions, conditions.TypeOpenBaoReady, condStatus, reason, message)

    return nil
}

// initializeOpenBao performs first-time initialization
func (r *ClusterReconciler) initializeOpenBao(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)
    log.Info("Initializing OpenBao cluster")

    // This would typically be done via a Job that:
    // 1. Calls `vault operator init` on the first pod
    // 2. Stores the root token and unseal keys in a Secret
    // 3. Unseals all pods (if not using auto-unseal)
    // 4. Registers the secp256k1 plugin

    // For auto-unseal, the process is simpler as pods self-unseal

    // TODO: Implement actual initialization logic
    // This requires creating a Job that runs the init script

    return nil
}

// registerPlugin registers the secp256k1 plugin
func (r *ClusterReconciler) registerPlugin(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)
    log.Info("Registering secp256k1 plugin")

    // TODO: Create a Job that runs the plugin registration script
    // The Job needs access to the root token

    return nil
}
```

---

### 6. Unseal Interface

```go
// internal/unseal/interface.go
package unseal

import (
    "context"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

// Provider defines the interface for auto-unseal providers
type Provider interface {
    // Name returns the provider name
    Name() string
    
    // Validate checks if the configuration is valid
    Validate(spec *banhbaoringv1.AutoUnsealSpec) error
    
    // GetConfig returns the HCL configuration for the seal stanza
    GetConfig(spec *banhbaoringv1.AutoUnsealSpec) string
    
    // GetEnvVars returns environment variables needed by the provider
    GetEnvVars(ctx context.Context, spec *banhbaoringv1.AutoUnsealSpec, namespace string) ([]corev1.EnvVar, error)
}

// NewProvider creates the appropriate unseal provider
func NewProvider(providerType string) Provider {
    switch providerType {
    case "awskms":
        return &AWSKMSProvider{}
    case "gcpkms":
        return &GCPKMSProvider{}
    case "azurekv":
        return &AzureKVProvider{}
    case "transit":
        return &TransitProvider{}
    default:
        return nil
    }
}
```

```go
// internal/unseal/awskms.go
package unseal

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    
    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

type AWSKMSProvider struct{}

func (p *AWSKMSProvider) Name() string {
    return "awskms"
}

func (p *AWSKMSProvider) Validate(spec *banhbaoringv1.AutoUnsealSpec) error {
    if spec.AWSKMS == nil {
        return fmt.Errorf("awskms configuration required")
    }
    if spec.AWSKMS.KeyID == "" {
        return fmt.Errorf("awskms.keyId required")
    }
    return nil
}

func (p *AWSKMSProvider) GetConfig(spec *banhbaoringv1.AutoUnsealSpec) string {
    if spec.AWSKMS == nil {
        return ""
    }
    
    region := spec.AWSKMS.Region
    if region == "" {
        region = "us-east-1"
    }
    
    return fmt.Sprintf(`seal "awskms" {
  region     = "%s"
  kms_key_id = "%s"
}`, region, spec.AWSKMS.KeyID)
}

func (p *AWSKMSProvider) GetEnvVars(ctx context.Context, spec *banhbaoringv1.AutoUnsealSpec, namespace string) ([]corev1.EnvVar, error) {
    var envVars []corev1.EnvVar
    
    if spec.AWSKMS == nil {
        return envVars, nil
    }

    if spec.AWSKMS.Region != "" {
        envVars = append(envVars, corev1.EnvVar{
            Name:  "AWS_REGION",
            Value: spec.AWSKMS.Region,
        })
    }

    // If credentials secret is specified, add env vars from secret
    if spec.AWSKMS.Credentials != nil {
        envVars = append(envVars,
            corev1.EnvVar{
                Name: "AWS_ACCESS_KEY_ID",
                ValueFrom: &corev1.EnvVarSource{
                    SecretKeyRef: &corev1.SecretKeySelector{
                        LocalObjectReference: corev1.LocalObjectReference{
                            Name: spec.AWSKMS.Credentials.Name,
                        },
                        Key: "access-key-id",
                    },
                },
            },
            corev1.EnvVar{
                Name: "AWS_SECRET_ACCESS_KEY",
                ValueFrom: &corev1.EnvVarSource{
                    SecretKeyRef: &corev1.SecretKeySelector{
                        LocalObjectReference: corev1.LocalObjectReference{
                            Name: spec.AWSKMS.Credentials.Name,
                        },
                        Key: "secret-access-key",
                    },
                },
            },
        )
    }

    return envVars, nil
}
```

---

## Test Commands

```bash
cd operator

# Build
go build ./...

# Run tests
go test ./internal/resources/openbao/... -v

# Generate manifests
make manifests

# Test against kind cluster (optional)
kind create cluster --name banhbaoring-test
make install
kubectl apply -f config/samples/cluster_minimal.yaml
```

---

## Acceptance Criteria

- [ ] OpenBao StatefulSet builder implemented
- [ ] ConfigMap with Raft storage and auto-unseal config
- [ ] Headless and active Services created
- [ ] Plugin download init container
- [ ] Auto-unseal providers (AWS KMS at minimum)
- [ ] Controller reconciliation for OpenBao resources
- [ ] Health check and status updates
- [ ] Unit tests passing

