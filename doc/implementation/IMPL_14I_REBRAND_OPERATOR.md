# Agent Task: Rebrand Kubernetes Operator

> **Parallel Execution:** ⚠️ Run after Stage 1 complete (imports)
> **Dependencies:** IMPL_14A (core library) should complete first
> **Estimated Time:** 3-4 hours

---

## Objective

Rename the Kubernetes operator from `banhbaoring` to `popsigner`, including CRDs, Helm charts, and controllers.

---

## Scope

### API Group Change

```
banhbaoring.io  →  popsigner.io
```

### Files to Rename

```
operator/api/v1/banhbaoringcluster_types.go    →  popsignercluster_types.go
operator/api/v1/banhbaoringtenant_types.go     →  popsignertenant_types.go
operator/api/v1/banhbaoringbackup_types.go     →  popsignerbackup_types.go
operator/api/v1/banhbaoringrestore_types.go    →  popsignerrestore_types.go

operator/charts/banhbaoring-operator/          →  popsigner-operator/

config/crd/bases/banhbaoring.io_*.yaml         →  popsigner.io_*.yaml
```

---

## Implementation

### Part 1: API Types

#### Rename Type Files

```bash
cd operator/api/v1
mv banhbaoringcluster_types.go popsignercluster_types.go
mv banhbaoringtenant_types.go popsignertenant_types.go
mv banhbaoringbackup_types.go popsignerbackup_types.go
mv banhbaoringrestore_types.go popsignerrestore_types.go
```

#### Update groupversion_info.go

```go
// Before
package v1

import (
    "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
    GroupVersion = schema.GroupVersion{Group: "banhbaoring.io", Version: "v1"}
    SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
)

// After
package v1

var (
    GroupVersion = schema.GroupVersion{Group: "popsigner.io", Version: "v1"}
    SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
)
```

#### Update Type Names in popsignercluster_types.go

```go
// Before
type BanhBaoRingCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   BanhBaoRingClusterSpec   `json:"spec,omitempty"`
    Status BanhBaoRingClusterStatus `json:"status,omitempty"`
}

type BanhBaoRingClusterList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []BanhBaoRingCluster `json:"items"`
}

// After
type POPSignerCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   POPSignerClusterSpec   `json:"spec,omitempty"`
    Status POPSignerClusterStatus `json:"status,omitempty"`
}

type POPSignerClusterList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []POPSignerCluster `json:"items"`
}
```

Apply same pattern to:
- `popsignertenant_types.go`
- `popsignerbackup_types.go`
- `popsignerrestore_types.go`

### Part 2: Regenerate CRDs

```bash
cd operator

# Regenerate deepcopy
make generate

# Regenerate CRDs
make manifests
```

This will create new CRD files in `config/crd/bases/`.

### Part 3: Update Controllers

#### cluster_controller.go

```go
// Before
func (r *BanhBaoRingClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var cluster v1.BanhBaoRingCluster
    // ...
}

// After
func (r *POPSignerClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var cluster v1.POPSignerCluster
    // ...
}
```

Apply to all controllers:
- `tenant_controller.go`
- `backup_controller.go`
- `restore_controller.go`
- `cluster_openbao.go`
- `cluster_datalayer.go`
- `cluster_apps.go`
- `cluster_monitoring.go`

### Part 4: Update Constants

#### internal/constants/constants.go

```go
// Before
const (
    LabelApp       = "app.kubernetes.io/name"
    LabelAppValue  = "banhbaoring"
    LabelComponent = "app.kubernetes.io/component"
    
    AnnotationTenant = "banhbaoring.io/tenant"
)

// After
const (
    LabelApp       = "app.kubernetes.io/name"
    LabelAppValue  = "popsigner"
    LabelComponent = "app.kubernetes.io/component"
    
    AnnotationTenant = "popsigner.io/tenant"
)
```

### Part 5: Rename Helm Chart

```bash
cd operator/charts
mv banhbaoring-operator popsigner-operator
```

#### Update Chart.yaml

```yaml
# Before
apiVersion: v2
name: banhbaoring-operator
description: BanhBaoRing Kubernetes Operator

# After
apiVersion: v2
name: popsigner-operator
description: POPSigner Kubernetes Operator - Point-of-Presence signing infrastructure
version: 1.0.0
appVersion: "1.0.0"
```

#### Update values.yaml

```yaml
# Before
image:
  repository: ghcr.io/bidon15/banhbaoring-operator
  
controlPlane:
  image:
    repository: ghcr.io/bidon15/banhbaoring-control-plane

# After
image:
  repository: ghcr.io/bidon15/popsigner-operator
  
controlPlane:
  image:
    repository: ghcr.io/bidon15/popsigner-control-plane
```

#### Update All templates/*.yaml

Replace all occurrences:
- `banhbaoring` → `popsigner`
- `BanhBaoRing` → `POPSigner`
- `banhbaoring.io` → `popsigner.io`

### Part 6: Update Config Files

#### config/manager/manager.yaml

```yaml
# Update image and labels
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: popsigner-operator
```

#### config/rbac/*.yaml

Update all role names and labels.

#### config/samples/*.yaml

```yaml
# Before
apiVersion: banhbaoring.io/v1alpha1
kind: BanhBaoRingCluster

# After
apiVersion: popsigner.io/v1alpha1
kind: POPSignerCluster
```

### Part 7: Update main.go

```go
// Before
mgr.GetScheme().AddKnownTypes(v1.GroupVersion, &v1.BanhBaoRingCluster{})

// After
mgr.GetScheme().AddKnownTypes(v1.GroupVersion, &v1.POPSignerCluster{})
```

### Part 8: Update Makefile

```makefile
# Before
IMG ?= ghcr.io/bidon15/banhbaoring-operator:dev

# After
IMG ?= ghcr.io/bidon15/popsigner-operator:dev
```

---

## Verification

```bash
cd operator

# Generate code
make generate

# Generate manifests
make manifests

# Build
make build

# Run tests
make test

# Check for remaining references
grep -r "banhbaoring" . --include="*.go" --include="*.yaml"
grep -r "BanhBaoRing" . --include="*.go" --include="*.yaml"
```

---

## Checklist

```
API Types:
□ Rename popsignercluster_types.go
□ Rename popsignertenant_types.go
□ Rename popsignerbackup_types.go
□ Rename popsignerrestore_types.go
□ Update groupversion_info.go (API group)
□ Update all type names (BanhBaoRing* → POPSigner*)
□ Run make generate

Controllers:
□ Update cluster_controller.go
□ Update tenant_controller.go
□ Update backup_controller.go
□ Update restore_controller.go
□ Update cluster_*.go files
□ Update suite_test.go

Internal:
□ Update constants/constants.go
□ Update all internal/resources/*/*.go
□ Update main.go

Helm Chart:
□ Rename charts/banhbaoring-operator → popsigner-operator
□ Update Chart.yaml
□ Update values.yaml
□ Update all templates/*.yaml
□ Update templates/crds/*.yaml
□ Update README.md
□ Update _helpers.tpl
□ Update NOTES.txt

Config:
□ Rename/update config/crd/bases/*.yaml
□ Update config/manager/manager.yaml
□ Update config/rbac/*.yaml
□ Update config/samples/*.yaml

Build:
□ Update operator/Makefile
□ Update operator/Dockerfile
□ Update operator/go.mod

Verification:
□ make generate passes
□ make manifests passes
□ make build passes
□ make test passes
□ No remaining "banhbaoring" references
```

---

## Output

After completion, the operator is fully rebranded to POPSigner with new API group `popsigner.io`.

