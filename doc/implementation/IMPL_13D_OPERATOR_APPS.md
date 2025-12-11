# Agent 13D: Application Layer Controller

## Overview

Implement API and Dashboard deployment logic. Creates Deployments, Services, HPAs, and Ingress for the Control Plane API and Web Dashboard.

> **Requires:** Agent 13B (OpenBao) and 13C (Data Layer) complete

---

## Deliverables

### 1. API Deployment Builder

```go
// internal/resources/api/deployment.go
package api

import (
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
    APIImage = "banhbaoring/control-plane"
    APIPort  = 8080
)

func Deployment(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.Deployment {
    spec := cluster.Spec.API
    name := fmt.Sprintf("%s-api", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentAPI, spec.Version)

    replicas := spec.Replicas
    if replicas == 0 {
        replicas = 2
    }

    return &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: &replicas,
            Selector: &metav1.LabelSelector{MatchLabels: labels},
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{Labels: labels},
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{{
                        Name:  "api",
                        Image: fmt.Sprintf("%s:%s", APIImage, spec.Version),
                        Ports: []corev1.ContainerPort{{ContainerPort: APIPort}},
                        Env:   buildEnv(cluster),
                        ReadinessProbe: &corev1.Probe{
                            ProbeHandler: corev1.ProbeHandler{
                                HTTPGet: &corev1.HTTPGetAction{
                                    Path: "/health",
                                    Port: intstr.FromInt(APIPort),
                                },
                            },
                            InitialDelaySeconds: 5,
                            PeriodSeconds:       10,
                        },
                        Resources: spec.Resources,
                    }},
                },
            },
        },
    }
}

func buildEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
    dbSecret := fmt.Sprintf("%s-postgres-credentials", cluster.Name)
    redisSecret := fmt.Sprintf("%s-redis-connection", cluster.Name)
    openbaoSvc := fmt.Sprintf("%s-openbao-active", cluster.Name)

    return []corev1.EnvVar{
        {Name: "DATABASE_URL", ValueFrom: secretRef(dbSecret, "url")},
        {Name: "REDIS_URL", ValueFrom: secretRef(redisSecret, "url")},
        {Name: "OPENBAO_ADDR", Value: fmt.Sprintf("https://%s:8200", openbaoSvc)},
        {Name: "OPENBAO_TOKEN", ValueFrom: secretRef(cluster.Name+"-openbao-root", "token")},
    }
}

func secretRef(name, key string) *corev1.EnvVarSource {
    return &corev1.EnvVarSource{
        SecretKeyRef: &corev1.SecretKeySelector{
            LocalObjectReference: corev1.LocalObjectReference{Name: name},
            Key:                  key,
        },
    }
}
```

### 2. Dashboard Deployment Builder

```go
// internal/resources/dashboard/deployment.go
package dashboard

import (
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
    DashboardImage = "banhbaoring/dashboard"
    DashboardPort  = 3000
)

func Deployment(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.Deployment {
    spec := cluster.Spec.Dashboard
    name := fmt.Sprintf("%s-dashboard", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentDashboard, spec.Version)

    replicas := spec.Replicas
    if replicas == 0 {
        replicas = 2
    }

    apiURL := fmt.Sprintf("http://%s-api:8080", cluster.Name)

    return &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: &replicas,
            Selector: &metav1.LabelSelector{MatchLabels: labels},
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{Labels: labels},
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{{
                        Name:  "dashboard",
                        Image: fmt.Sprintf("%s:%s", DashboardImage, spec.Version),
                        Ports: []corev1.ContainerPort{{ContainerPort: DashboardPort}},
                        Env: []corev1.EnvVar{
                            {Name: "API_URL", Value: apiURL},
                        },
                        ReadinessProbe: &corev1.Probe{
                            ProbeHandler: corev1.ProbeHandler{
                                HTTPGet: &corev1.HTTPGetAction{
                                    Path: "/health",
                                    Port: intstr.FromInt(DashboardPort),
                                },
                            },
                        },
                        Resources: spec.Resources,
                    }},
                },
            },
        },
    }
}
```

### 3. HPA Builder

```go
// internal/resources/api/hpa.go
package api

import (
    "fmt"

    autoscalingv2 "k8s.io/api/autoscaling/v2"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
    "github.com/Bidon15/banhbaoring/operator/internal/constants"
)

func HPA(cluster *banhbaoringv1.BanhBaoRingCluster) *autoscalingv2.HorizontalPodAutoscaler {
    spec := cluster.Spec.API.Autoscaling
    name := fmt.Sprintf("%s-api", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentAPI, cluster.Spec.API.Version)

    minReplicas := spec.MinReplicas
    if minReplicas == 0 {
        minReplicas = 2
    }
    maxReplicas := spec.MaxReplicas
    if maxReplicas == 0 {
        maxReplicas = 10
    }
    targetCPU := spec.TargetCPU
    if targetCPU == 0 {
        targetCPU = 70
    }

    return &autoscalingv2.HorizontalPodAutoscaler{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: cluster.Namespace,
            Labels:    labels,
        },
        Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
            ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
                APIVersion: "apps/v1",
                Kind:       "Deployment",
                Name:       name,
            },
            MinReplicas: &minReplicas,
            MaxReplicas: maxReplicas,
            Metrics: []autoscalingv2.MetricSpec{{
                Type: autoscalingv2.ResourceMetricSourceType,
                Resource: &autoscalingv2.ResourceMetricSource{
                    Name: corev1.ResourceCPU,
                    Target: autoscalingv2.MetricTarget{
                        Type:               autoscalingv2.UtilizationMetricType,
                        AverageUtilization: &targetCPU,
                    },
                },
            }},
        },
    }
}
```

### 4. Services

```go
// internal/resources/api/service.go
package api

func Service(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-api", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentAPI, cluster.Spec.API.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: cluster.Namespace, Labels: labels},
        Spec: corev1.ServiceSpec{
            Type:     corev1.ServiceTypeClusterIP,
            Selector: labels,
            Ports:    []corev1.ServicePort{{Port: APIPort, TargetPort: intstr.FromInt(APIPort)}},
        },
    }
}

// internal/resources/dashboard/service.go
func Service(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
    name := fmt.Sprintf("%s-dashboard", cluster.Name)
    labels := constants.Labels(cluster.Name, constants.ComponentDashboard, cluster.Spec.Dashboard.Version)

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: cluster.Namespace, Labels: labels},
        Spec: corev1.ServiceSpec{
            Type:     corev1.ServiceTypeClusterIP,
            Selector: labels,
            Ports:    []corev1.ServicePort{{Port: DashboardPort, TargetPort: intstr.FromInt(DashboardPort)}},
        },
    }
}
```

### 5. Controller Reconciliation

```go
// controllers/cluster_apps.go
package controllers

func (r *ClusterReconciler) reconcileAPI(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)
    log.Info("Reconciling API")

    // Deployment
    if err := r.createOrUpdate(ctx, cluster, api.Deployment(cluster)); err != nil {
        return fmt.Errorf("failed to reconcile API deployment: %w", err)
    }

    // Service
    if err := r.createOrUpdate(ctx, cluster, api.Service(cluster)); err != nil {
        return fmt.Errorf("failed to reconcile API service: %w", err)
    }

    // HPA (if autoscaling enabled)
    if cluster.Spec.API.Autoscaling.Enabled {
        if err := r.createOrUpdate(ctx, cluster, api.HPA(cluster)); err != nil {
            return fmt.Errorf("failed to reconcile API HPA: %w", err)
        }
    }

    return nil
}

func (r *ClusterReconciler) reconcileDashboard(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    log := log.FromContext(ctx)
    log.Info("Reconciling Dashboard")

    if err := r.createOrUpdate(ctx, cluster, dashboard.Deployment(cluster)); err != nil {
        return fmt.Errorf("failed to reconcile Dashboard deployment: %w", err)
    }

    if err := r.createOrUpdate(ctx, cluster, dashboard.Service(cluster)); err != nil {
        return fmt.Errorf("failed to reconcile Dashboard service: %w", err)
    }

    return nil
}
```

---

## Test Commands

```bash
cd operator
go build ./...
go test ./internal/resources/api/... -v
go test ./internal/resources/dashboard/... -v
```

---

## Acceptance Criteria

- [ ] API Deployment builder
- [ ] Dashboard Deployment builder
- [ ] HPA for API autoscaling
- [ ] Services for both components
- [ ] Environment variable injection
- [ ] Controller reconciliation
- [ ] Status updates

