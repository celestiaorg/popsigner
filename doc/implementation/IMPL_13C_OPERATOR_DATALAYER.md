# Agent 13C: Data Layer Controller

## Overview

Implement PostgreSQL and Redis deployment logic in the Cluster controller. Supports both managed (operator-deployed) and external (bring-your-own) database configurations.

> **Requires:** Agent 13A (Operator Foundation) complete

---

## Prerequisites

- Agent 13A completed (CRDs and controller stubs exist)
- Understanding of PostgreSQL and Redis in Kubernetes
- Familiarity with StatefulSets and persistent storage

---

## Deliverables

### 1. PostgreSQL Resource Builder

```go
// internal/resources/database/postgresql.go
package database

import (
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
    PostgresImage = "postgres"
    PostgresPort  = 5432
)

// StatefulSet builds the PostgreSQL StatefulSet
func StatefulSet(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.StatefulSet {
    spec := cluster.Spec.Database
    name := fmt.Sprintf("%s-postgres", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentPostgres, spec.Version)

    replicas := spec.Replicas
    if replicas == 0 {
        replicas = 1
    }

    version := spec.Version
    if version == "" {
        version = "16"
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
                    Containers: []corev1.Container{
                        {
                            Name:  "postgres",
                            Image: fmt.Sprintf("%s:%s", PostgresImage, version),
                            Ports: []corev1.ContainerPort{
                                {Name: "postgres", ContainerPort: PostgresPort},
                            },
                            Env: []corev1.EnvVar{
                                {
                                    Name: "POSTGRES_USER",
                                    ValueFrom: &corev1.EnvVarSource{
                                        SecretKeyRef: &corev1.SecretKeySelector{
                                            LocalObjectReference: corev1.LocalObjectReference{Name: name + "-credentials"},
                                            Key:                  "username",
                                        },
                                    },
                                },
                                {
                                    Name: "POSTGRES_PASSWORD",
                                    ValueFrom: &corev1.EnvVarSource{
                                        SecretKeyRef: &corev1.SecretKeySelector{
                                            LocalObjectReference: corev1.LocalObjectReference{Name: name + "-credentials"},
                                            Key:                  "password",
                                        },
                                    },
                                },
                                {
                                    Name:  "POSTGRES_DB",
                                    Value: "banhbaoring",
                                },
                                {
                                    Name:  "PGDATA",
                                    Value: "/var/lib/postgresql/data/pgdata",
                                },
                            },
                            VolumeMounts: []corev1.VolumeMount{
                                {Name: "data", MountPath: "/var/lib/postgresql/data"},
                                {Name: "init-scripts", MountPath: "/docker-entrypoint-initdb.d"},
                            },
                            ReadinessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    Exec: &corev1.ExecAction{
                                        Command: []string{
                                            "pg_isready",
                                            "-U", "$(POSTGRES_USER)",
                                            "-d", "banhbaoring",
                                        },
                                    },
                                },
                                InitialDelaySeconds: 5,
                                PeriodSeconds:       10,
                            },
                            LivenessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    Exec: &corev1.ExecAction{
                                        Command: []string{
                                            "pg_isready",
                                            "-U", "$(POSTGRES_USER)",
                                            "-d", "banhbaoring",
                                        },
                                    },
                                },
                                InitialDelaySeconds: 30,
                                PeriodSeconds:       30,
                            },
                            Resources: corev1.ResourceRequirements{
                                Requests: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("250m"),
                                    corev1.ResourceMemory: resource.MustParse("256Mi"),
                                },
                                Limits: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("1"),
                                    corev1.ResourceMemory: resource.MustParse("1Gi"),
                                },
                            },
                        },
                    },
                    Volumes: []corev1.Volume{
                        {
                            Name: "init-scripts",
                            VolumeSource: corev1.VolumeSource{
                                ConfigMap: &corev1.ConfigMapVolumeSource{
                                    LocalObjectReference: corev1.LocalObjectReference{
                                        Name: name + "-init",
                                    },
                                },
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

// Service builds the PostgreSQL Service
func Service(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-postgres", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentPostgres, cluster.Spec.Database.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: corev1.ServiceSpec{
            Type:     corev1.ServiceTypeClusterIP,
            Selector: labels,
            Ports: []corev1.ServicePort{
                {Name: "postgres", Port: PostgresPort, TargetPort: intstr.FromInt(PostgresPort)},
            },
        },
    }
}

// CredentialsSecret builds the PostgreSQL credentials Secret
func CredentialsSecret(cluster *banhbaoringv1.BanhBaoRingCluster, password string) *corev1.Secret {
    name := fmt.Sprintf("%s-postgres", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentPostgres, cluster.Spec.Database.Version)

    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name + "-credentials",
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Type: corev1.SecretTypeOpaque,
        StringData: map[string]string{
            "username": "banhbaoring",
            "password": password,
            "database": "banhbaoring",
            "url": fmt.Sprintf("postgres://banhbaoring:%s@%s:5432/banhbaoring?sslmode=disable", password, name),
        },
    }
}

// InitConfigMap builds the init scripts ConfigMap
func InitConfigMap(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.ConfigMap {
    name := fmt.Sprintf("%s-postgres", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentPostgres, cluster.Spec.Database.Version)

    // Schema from control-plane migrations
    initSQL := `
-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    plan VARCHAR(50) DEFAULT 'free',
    stripe_customer_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(255),
    email_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Organization members
CREATE TABLE IF NOT EXISTS org_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, user_id)
);

-- Namespaces
CREATE TABLE IF NOT EXISTS namespaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(org_id, name)
);

-- API Keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    scopes TEXT[],
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Audit logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    metadata JSONB,
    ip_address INET,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_id ON audit_logs(org_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix);
`

    return &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name + "-init",
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Data: map[string]string{
            "01-schema.sql": initSQL,
        },
    }
}

func storageClassPtr(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

---

### 2. Redis Resource Builder

```go
// internal/resources/redis/redis.go
package redis

import (
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
    RedisImage = "redis"
    RedisPort  = 6379
)

// StatefulSet builds the Redis StatefulSet (standalone mode)
func StatefulSet(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.StatefulSet {
    spec := cluster.Spec.Redis
    name := fmt.Sprintf("%s-redis", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentRedis, spec.Version)

    replicas := spec.Replicas
    if replicas == 0 {
        replicas = 1
    }

    version := spec.Version
    if version == "" {
        version = "7"
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
                    Containers: []corev1.Container{
                        {
                            Name:  "redis",
                            Image: fmt.Sprintf("%s:%s-alpine", RedisImage, version),
                            Command: []string{
                                "redis-server",
                                "--appendonly", "yes",
                                "--maxmemory", "256mb",
                                "--maxmemory-policy", "allkeys-lru",
                            },
                            Ports: []corev1.ContainerPort{
                                {Name: "redis", ContainerPort: RedisPort},
                            },
                            VolumeMounts: []corev1.VolumeMount{
                                {Name: "data", MountPath: "/data"},
                            },
                            ReadinessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    Exec: &corev1.ExecAction{
                                        Command: []string{"redis-cli", "ping"},
                                    },
                                },
                                InitialDelaySeconds: 5,
                                PeriodSeconds:       10,
                            },
                            LivenessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    Exec: &corev1.ExecAction{
                                        Command: []string{"redis-cli", "ping"},
                                    },
                                },
                                InitialDelaySeconds: 15,
                                PeriodSeconds:       20,
                            },
                            Resources: corev1.ResourceRequirements{
                                Requests: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("100m"),
                                    corev1.ResourceMemory: resource.MustParse("128Mi"),
                                },
                                Limits: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("500m"),
                                    corev1.ResourceMemory: resource.MustParse("512Mi"),
                                },
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

// Service builds the Redis Service
func Service(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-redis", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentRedis, cluster.Spec.Redis.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: corev1.ServiceSpec{
            Type:     corev1.ServiceTypeClusterIP,
            Selector: labels,
            Ports: []corev1.ServicePort{
                {Name: "redis", Port: RedisPort, TargetPort: intstr.FromInt(RedisPort)},
            },
        },
    }
}

// ConnectionSecret builds the Redis connection Secret
func ConnectionSecret(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Secret {
    name := fmt.Sprintf("%s-redis", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentRedis, cluster.Spec.Redis.Version)

    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name + "-connection",
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Type: corev1.SecretTypeOpaque,
        StringData: map[string]string{
            "url": fmt.Sprintf("redis://%s:6379", name),
        },
    }
}

func storageClassPtr(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

---

### 3. Controller Reconciliation Logic

```go
// controllers/cluster_datalayer.go
package controllers

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/log"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/conditions"
    "github.com/Bidon15/banhbaoring/operator/internal/resources/database"
    "github.com/Bidon15/banhbaoring/operator/internal/resources/redis"
)

// reconcilePostgreSQL handles PostgreSQL resources
func (r *ClusterReconciler) reconcilePostgreSQL(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)

    if !cluster.Spec.Database.Managed {
        log.Info("Using external database, skipping PostgreSQL deployment")
        return nil
    }

    log.Info("Reconciling PostgreSQL")

    name := fmt.Sprintf("%s-postgres", cluster.Name)

    // 1. Ensure credentials secret exists
    credSecret := &corev1.Secret{}
    if err := r.Get(ctx, types.NamespacedName{Name: name + "-credentials", Namespace: cluster.Namespace}, credSecret); err != nil {
        if errors.IsNotFound(err) {
            // Generate new password
            password, err := generatePassword(32)
            if err != nil {
                return fmt.Errorf("failed to generate password: %w", err)
            }

            credSecret = database.CredentialsSecret(cluster, password)
            if err := r.createOrUpdate(ctx, cluster, credSecret); err != nil {
                return fmt.Errorf("failed to create credentials secret: %w", err)
            }
        } else {
            return err
        }
    }

    // 2. Create init ConfigMap
    initCM := database.InitConfigMap(cluster)
    if err := r.createOrUpdate(ctx, cluster, initCM); err != nil {
        return fmt.Errorf("failed to reconcile init configmap: %w", err)
    }

    // 3. Create Service
    svc := database.Service(cluster)
    if err := r.createOrUpdate(ctx, cluster, svc); err != nil {
        return fmt.Errorf("failed to reconcile service: %w", err)
    }

    // 4. Create StatefulSet
    sts := database.StatefulSet(cluster)
    if err := r.createOrUpdate(ctx, cluster, sts); err != nil {
        return fmt.Errorf("failed to reconcile statefulset: %w", err)
    }

    return nil
}

// reconcileRedis handles Redis resources
func (r *ClusterReconciler) reconcileRedis(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)

    if !cluster.Spec.Redis.Managed {
        log.Info("Using external Redis, skipping deployment")
        return nil
    }

    log.Info("Reconciling Redis")

    // 1. Create connection Secret
    connSecret := redis.ConnectionSecret(cluster)
    if err := r.createOrUpdate(ctx, cluster, connSecret); err != nil {
        return fmt.Errorf("failed to reconcile connection secret: %w", err)
    }

    // 2. Create Service
    svc := redis.Service(cluster)
    if err := r.createOrUpdate(ctx, cluster, svc); err != nil {
        return fmt.Errorf("failed to reconcile service: %w", err)
    }

    // 3. Create StatefulSet
    if cluster.Spec.Redis.Mode == "cluster" {
        // TODO: Implement Redis Cluster mode
        log.Info("Redis Cluster mode not yet implemented, falling back to standalone")
    }

    sts := redis.StatefulSet(cluster)
    if err := r.createOrUpdate(ctx, cluster, sts); err != nil {
        return fmt.Errorf("failed to reconcile statefulset: %w", err)
    }

    return nil
}

// isDataLayerReady checks if PostgreSQL and Redis are ready
func (r *ClusterReconciler) isDataLayerReady(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) bool {
    // Check PostgreSQL
    if cluster.Spec.Database.Managed {
        name := fmt.Sprintf("%s-postgres", cluster.Name)
        sts := &appsv1.StatefulSet{}
        if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
            return false
        }
        if sts.Status.ReadyReplicas < 1 {
            return false
        }
    }

    // Check Redis
    if cluster.Spec.Redis.Managed {
        name := fmt.Sprintf("%s-redis", cluster.Name)
        sts := &appsv1.StatefulSet{}
        if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
            return false
        }
        if sts.Status.ReadyReplicas < 1 {
            return false
        }
    }

    return true
}

// updateDatabaseStatus updates the cluster status with database info
func (r *ClusterReconciler) updateDatabaseStatus(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    // PostgreSQL status
    if cluster.Spec.Database.Managed {
        name := fmt.Sprintf("%s-postgres", cluster.Name)
        sts := &appsv1.StatefulSet{}
        if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
            if !errors.IsNotFound(err) {
                return err
            }
            cluster.Status.Database = banhbaoringv1.ComponentStatus{
                Ready:   false,
                Message: "StatefulSet not found",
            }
        } else {
            ready := sts.Status.ReadyReplicas >= 1
            cluster.Status.Database = banhbaoringv1.ComponentStatus{
                Ready:    ready,
                Version:  cluster.Spec.Database.Version,
                Replicas: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
            }
        }
    } else {
        cluster.Status.Database = banhbaoringv1.ComponentStatus{
            Ready:   true,
            Message: "Using external database",
        }
    }

    // Redis status
    if cluster.Spec.Redis.Managed {
        name := fmt.Sprintf("%s-redis", cluster.Name)
        sts := &appsv1.StatefulSet{}
        if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
            if !errors.IsNotFound(err) {
                return err
            }
            cluster.Status.Redis = banhbaoringv1.ComponentStatus{
                Ready:   false,
                Message: "StatefulSet not found",
            }
        } else {
            ready := sts.Status.ReadyReplicas >= 1
            cluster.Status.Redis = banhbaoringv1.ComponentStatus{
                Ready:    ready,
                Version:  cluster.Spec.Redis.Version,
                Replicas: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
            }
        }
    } else {
        cluster.Status.Redis = banhbaoringv1.ComponentStatus{
            Ready:   true,
            Message: "Using external Redis",
        }
    }

    // Update condition
    dbReady := cluster.Status.Database.Ready && cluster.Status.Redis.Ready
    condStatus := metav1.ConditionFalse
    reason := "NotReady"
    message := "Waiting for data layer"

    if dbReady {
        condStatus = metav1.ConditionTrue
        reason = "Ready"
        message = "Database and Redis are ready"
    }

    conditions.SetCondition(&cluster.Status.Conditions, conditions.TypeDatabaseReady, condStatus, reason, message)

    return nil
}

// generatePassword creates a secure random password
func generatePassword(length int) (string, error) {
    bytes := make([]byte, length/2)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return hex.EncodeToString(bytes), nil
}
```

---

### 4. External Database Support

```go
// internal/resources/database/external.go
package database

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/client"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

// ExternalConfig holds external database configuration
type ExternalConfig struct {
    Host     string
    Port     int
    Database string
    Username string
    Password string
    SSLMode  string
}

// GetExternalConfig retrieves external database config from secret
func GetExternalConfig(ctx context.Context, c client.Client, cluster *banhbaoringv1.BanhBaoRingCluster) (*ExternalConfig, error) {
    if cluster.Spec.Database.Managed {
        return nil, fmt.Errorf("database is managed, not external")
    }

    connRef := cluster.Spec.Database.ConnectionString
    if connRef == nil {
        return nil, fmt.Errorf("connectionString not configured for external database")
    }

    secret := &corev1.Secret{}
    if err := c.Get(ctx, types.NamespacedName{
        Name:      connRef.Name,
        Namespace: cluster.Namespace,
    }, secret); err != nil {
        return nil, fmt.Errorf("failed to get connection string secret: %w", err)
    }

    connString, ok := secret.Data[connRef.Key]
    if !ok {
        return nil, fmt.Errorf("key %q not found in secret", connRef.Key)
    }

    // Parse connection string
    // Format: postgres://user:password@host:port/database?sslmode=disable
    // For simplicity, just return the raw URL
    return &ExternalConfig{
        // In production, properly parse the URL
    }, nil
}

// ConnectionStringSecret returns the connection string for applications
func ConnectionStringSecret(cluster *banhbaoringv1.BanhBaoRingCluster, connString string) *corev1.Secret {
    name := fmt.Sprintf("%s-database-url", cluster.Name)

    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
        },
        Type: corev1.SecretTypeOpaque,
        StringData: map[string]string{
            "url": connString,
        },
    }
}
```

---

### 5. Database Migration Job

```go
// internal/resources/database/migrations.go
package database

import (
    "fmt"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

// MigrationJob creates a Job to run database migrations
func MigrationJob(cluster *banhbaoringv1.BanhBaoRingCluster, apiVersion string) *batchv1.Job {
    name := fmt.Sprintf("%s-migrate", cluster.Name)
    labels := constants.Labels(cluster.Name, "migration", apiVersion)

    backoffLimit := int32(3)
    ttlSeconds := int32(300)

    dbSecretName := fmt.Sprintf("%s-postgres-credentials", cluster.Name)

    return &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: batchv1.JobSpec{
            BackoffLimit:            &backoffLimit,
            TTLSecondsAfterFinished: &ttlSeconds,
            Template: corev1.PodTemplateSpec{
                Spec: corev1.PodSpec{
                    RestartPolicy: corev1.RestartPolicyNever,
                    Containers: []corev1.Container{
                        {
                            Name:  "migrate",
                            Image: fmt.Sprintf("banhbaoring/control-plane:%s", apiVersion),
                            Command: []string{
                                "/app/control-plane",
                                "migrate",
                                "--up",
                            },
                            Env: []corev1.EnvVar{
                                {
                                    Name: "DATABASE_URL",
                                    ValueFrom: &corev1.EnvVarSource{
                                        SecretKeyRef: &corev1.SecretKeySelector{
                                            LocalObjectReference: corev1.LocalObjectReference{
                                                Name: dbSecretName,
                                            },
                                            Key: "url",
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
    }
}
```

---

## Test Commands

```bash
cd operator

# Build
go build ./...

# Run tests
go test ./internal/resources/database/... -v
go test ./internal/resources/redis/... -v

# Test against kind cluster
kind create cluster --name banhbaoring-test
make install
kubectl apply -f config/samples/cluster_minimal.yaml

# Verify PostgreSQL
kubectl get sts -n banhbaoring
kubectl get pods -n banhbaoring -l app.kubernetes.io/component=postgres
kubectl logs -n banhbaoring -l app.kubernetes.io/component=postgres

# Verify Redis
kubectl get pods -n banhbaoring -l app.kubernetes.io/component=redis
```

---

## Acceptance Criteria

- [ ] PostgreSQL StatefulSet builder implemented
- [ ] PostgreSQL Service and credentials Secret
- [ ] Init ConfigMap with schema SQL
- [ ] Redis StatefulSet builder (standalone mode)
- [ ] Redis Service and connection Secret
- [ ] External database support
- [ ] Controller reconciliation for data layer
- [ ] Ready checks and status updates
- [ ] Secure password generation
- [ ] Unit tests passing

