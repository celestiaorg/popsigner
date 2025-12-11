# Agent 13E: Tenant Controller

## Overview

Implement the Tenant controller for multi-tenant provisioning. Creates OpenBao namespaces, policies, and quota enforcement for each tenant.

> **Requires:** Agent 13B (OpenBao) complete

---

## Deliverables

### 1. Tenant Controller

```go
// controllers/tenant_controller.go
package controllers

import (
    "context"
    "fmt"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"

    banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

type TenantReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringtenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringtenants/status,verbs=get;update;patch

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    tenant := &banhbaoringv1.BanhBaoRingTenant{}
    if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Get parent cluster
    cluster := &banhbaoringv1.BanhBaoRingCluster{}
    clusterKey := client.ObjectKey{
        Name:      tenant.Spec.ClusterRef.Name,
        Namespace: tenant.Namespace,
    }
    if err := r.Get(ctx, clusterKey, cluster); err != nil {
        log.Error(err, "Failed to get parent cluster")
        return r.updateTenantStatus(ctx, tenant, "Failed", "Parent cluster not found")
    }

    // Check cluster is ready
    if cluster.Status.Phase != "Running" {
        log.Info("Waiting for cluster to be ready")
        return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
    }

    // Reconcile tenant resources
    if err := r.reconcileTenantNamespace(ctx, tenant, cluster); err != nil {
        return r.updateTenantStatus(ctx, tenant, "Failed", err.Error())
    }

    if err := r.reconcileTenantPolicies(ctx, tenant, cluster); err != nil {
        return r.updateTenantStatus(ctx, tenant, "Failed", err.Error())
    }

    if err := r.reconcileTenantQuotas(ctx, tenant, cluster); err != nil {
        log.Error(err, "Failed to reconcile quotas")
    }

    // Create admin user in Control Plane
    if err := r.reconcileTenantAdmin(ctx, tenant, cluster); err != nil {
        log.Error(err, "Failed to create admin user")
    }

    return r.updateTenantStatus(ctx, tenant, "Active", "")
}

func (r *TenantReconciler) reconcileTenantNamespace(ctx context.Context, tenant *banhbaoringv1.BanhBaoRingTenant, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    // Create OpenBao namespace for tenant isolation
    namespaceName := fmt.Sprintf("tenant-%s", tenant.Name)
    tenant.Status.OpenBaoNamespace = namespaceName

    // TODO: Call OpenBao API to create namespace
    // POST /v1/sys/namespaces/{namespaceName}

    return nil
}

func (r *TenantReconciler) reconcileTenantPolicies(ctx context.Context, tenant *banhbaoringv1.BanhBaoRingTenant, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    // Create OpenBao policies for tenant
    policy := fmt.Sprintf(`
# Tenant policy for %s
path "keys/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "keys/sign/*" {
  capabilities = ["create", "update"]
}

path "keys/verify/*" {
  capabilities = ["create", "update"]
}
`, tenant.Name)

    // TODO: Call OpenBao API to create policy
    // PUT /v1/sys/policies/acl/tenant-{name}

    _ = policy
    return nil
}

func (r *TenantReconciler) reconcileTenantQuotas(ctx context.Context, tenant *banhbaoringv1.BanhBaoRingTenant, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    // Apply quotas based on plan
    quotas := tenant.Spec.Quotas

    // Store quotas in Control Plane database
    // The API enforces these limits at runtime

    _ = quotas
    return nil
}

func (r *TenantReconciler) reconcileTenantAdmin(ctx context.Context, tenant *banhbaoringv1.BanhBaoRingTenant, cluster *banhbaoringv1.BanhBaoRingCluster) error {
    if tenant.Spec.Admin.Email == "" {
        return nil
    }

    // TODO: Create admin user via Control Plane API
    // POST /api/v1/internal/tenants/{id}/admin

    return nil
}

func (r *TenantReconciler) updateTenantStatus(ctx context.Context, tenant *banhbaoringv1.BanhBaoRingTenant, phase, message string) (ctrl.Result, error) {
    tenant.Status.Phase = phase
    now := metav1.Now()

    if phase == "Active" {
        tenant.Status.LastActiveAt = &now
        if tenant.Status.CreatedAt == nil {
            tenant.Status.CreatedAt = &now
        }
    }

    if err := r.Status().Update(ctx, tenant); err != nil {
        return ctrl.Result{}, err
    }

    if phase == "Failed" {
        return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
    }

    return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&banhbaoringv1.BanhBaoRingTenant{}).
        Complete(r)
}
```

### 2. OpenBao Client

```go
// internal/openbao/client.go
package openbao

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type Client struct {
    addr   string
    token  string
    client *http.Client
}

func NewClient(addr, token string) *Client {
    return &Client{
        addr:   addr,
        token:  token,
        client: &http.Client{},
    }
}

func (c *Client) CreateNamespace(ctx context.Context, name string) error {
    req, err := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("%s/v1/sys/namespaces/%s", c.addr, name), nil)
    if err != nil {
        return err
    }
    req.Header.Set("X-Vault-Token", c.token)

    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to create namespace: %s", body)
    }

    return nil
}

func (c *Client) CreatePolicy(ctx context.Context, name, policy string) error {
    payload := map[string]string{"policy": policy}
    body, _ := json.Marshal(payload)

    req, err := http.NewRequestWithContext(ctx, "PUT",
        fmt.Sprintf("%s/v1/sys/policies/acl/%s", c.addr, name),
        bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("X-Vault-Token", c.token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to create policy: %s", respBody)
    }

    return nil
}
```

### 3. Plan Quotas

```go
// internal/plans/plans.go
package plans

type PlanQuotas struct {
    Keys               int32
    SignaturesPerMonth int64
    Namespaces         int32
    TeamMembers        int32
    APIKeys            int32
}

var Plans = map[string]PlanQuotas{
    "free": {
        Keys:               5,
        SignaturesPerMonth: 10000,
        Namespaces:         1,
        TeamMembers:        1,
        APIKeys:            2,
    },
    "starter": {
        Keys:               10,
        SignaturesPerMonth: 100000,
        Namespaces:         2,
        TeamMembers:        3,
        APIKeys:            5,
    },
    "pro": {
        Keys:               25,
        SignaturesPerMonth: 500000,
        Namespaces:         5,
        TeamMembers:        10,
        APIKeys:            20,
    },
    "enterprise": {
        Keys:               -1, // unlimited
        SignaturesPerMonth: -1,
        Namespaces:         -1,
        TeamMembers:        -1,
        APIKeys:            -1,
    },
}

func GetPlanQuotas(plan string) PlanQuotas {
    if q, ok := Plans[plan]; ok {
        return q
    }
    return Plans["free"]
}
```

---

## Test Commands

```bash
cd operator
go build ./...
go test ./controllers/... -v -run TestTenant
go test ./internal/plans/... -v
```

---

## Acceptance Criteria

- [ ] Tenant controller reconciliation
- [ ] OpenBao namespace creation per tenant
- [ ] Policy creation for tenant isolation
- [ ] Quota management based on plan
- [ ] Admin user provisioning
- [ ] Status updates and conditions

