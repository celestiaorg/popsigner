# Deploying BanhBaoRing to Scaleway Kubernetes

This guide walks you through deploying the complete BanhBaoRing stack to Scaleway Kapsule (managed Kubernetes).

## Prerequisites

- [Scaleway account](https://console.scaleway.com/)
- [scw CLI](https://github.com/scaleway/scaleway-cli) installed and configured
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Helm 3](https://helm.sh/docs/intro/install/) installed
- Docker (for building images)
- GitHub OAuth App credentials
- Google OAuth credentials

---

## 1. Create Scaleway Kapsule Cluster

### Via CLI

```bash
# Create a Kubernetes cluster
scw k8s cluster create \
  name=banhbaoring \
  version=1.29 \
  cni=cilium \
  pools.0.name=default \
  pools.0.node-type=DEV1-M \
  pools.0.size=3 \
  pools.0.min-size=2 \
  pools.0.max-size=5 \
  pools.0.autoscaling=true \
  pools.0.autohealing=true \
  region=fr-par

# Wait for cluster to be ready
scw k8s cluster wait <cluster-id>

# Get kubeconfig
scw k8s kubeconfig install <cluster-id>

# Verify connection
kubectl get nodes
```

### Via Console

1. Go to **Scaleway Console** → **Kubernetes** → **Create Cluster**
2. Name: `banhbaoring`
3. Region: `fr-par` (or closest to you)
4. Kubernetes version: `1.29`
5. CNI: `Cilium`
6. Node pool:
   - Name: `default`
   - Node type: `DEV1-M` (3 vCPU, 4GB RAM) - good for testing
   - Size: 3 nodes
   - Autoscaling: 2-5 nodes
7. Click **Create Cluster**

---

## 2. Create Scaleway Container Registry

```bash
# Create registry
scw registry namespace create \
  name=banhbaoring \
  region=fr-par \
  is-public=false

# Login to registry
scw registry login
```

Note your registry endpoint: `rg.fr-par.scw.cloud/banhbaoring`

---

## 3. Build and Push Images

```bash
cd /path/to/banhbaoring

# Set your registry
export REGISTRY=rg.fr-par.scw.cloud/banhbaoring
export VERSION=v0.1.0

# Build images
make docker-build

# Tag for Scaleway
docker tag ghcr.io/bidon15/banhbaoring-operator:dev $REGISTRY/operator:$VERSION
docker tag ghcr.io/bidon15/banhbaoring-control-plane:dev $REGISTRY/control-plane:$VERSION
docker tag ghcr.io/bidon15/banhbaoring-secp256k1:dev $REGISTRY/secp256k1-plugin:$VERSION

# Push to Scaleway
docker push $REGISTRY/operator:$VERSION
docker push $REGISTRY/control-plane:$VERSION
docker push $REGISTRY/secp256k1-plugin:$VERSION
```

---

## 4. Create Scaleway Block Storage (for OpenBao)

OpenBao needs persistent storage for its Raft backend:

```bash
# Create storage class for Scaleway Block Storage
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: scw-bssd
provisioner: csi.scaleway.com
parameters:
  type: b_ssd  # SSD block storage
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
EOF
```

---

## 5. Create Secrets

### OAuth & API Secrets

```bash
# Create namespace
kubectl create namespace banhbaoring

# Create secrets (replace with your values!)
kubectl create secret generic banhbaoring-config \
  --namespace banhbaoring \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --from-literal=oauth-github-id="YOUR_GITHUB_CLIENT_ID" \
  --from-literal=oauth-github-secret="YOUR_GITHUB_CLIENT_SECRET" \
  --from-literal=oauth-google-id="YOUR_GOOGLE_CLIENT_ID" \
  --from-literal=oauth-google-secret="YOUR_GOOGLE_CLIENT_SECRET"
```

### Database Secrets

```bash
# PostgreSQL credentials
kubectl create secret generic postgresql-credentials \
  --namespace banhbaoring \
  --from-literal=postgres-password="$(openssl rand -base64 24)" \
  --from-literal=password="$(openssl rand -base64 24)"

# Redis credentials
kubectl create secret generic redis-credentials \
  --namespace banhbaoring \
  --from-literal=redis-password="$(openssl rand -base64 24)"
```

---

## 6. Install BanhBaoRing Operator

```bash
# Add Helm repo (if published) or install from local
helm install banhbaoring-operator ./operator/charts/banhbaoring-operator \
  --namespace banhbaoring-system \
  --create-namespace \
  --set image.repository=$REGISTRY/operator \
  --set image.tag=$VERSION \
  --set controlPlaneImage.repository=$REGISTRY/control-plane \
  --set controlPlaneImage.tag=$VERSION \
  --set openbaoPluginImage.repository=$REGISTRY/secp256k1-plugin \
  --set openbaoPluginImage.tag=$VERSION

# Verify operator is running
kubectl get pods -n banhbaoring-system
```

---

## 7. Deploy BanhBaoRing Cluster

Create the cluster resource:

```bash
kubectl apply -f - <<EOF
apiVersion: banhbaoring.io/v1alpha1
kind: BanhBaoRingCluster
metadata:
  name: production
  namespace: banhbaoring
spec:
  # OpenBao Configuration
  openbao:
    replicas: 3
    storage:
      size: 10Gi
      storageClassName: scw-bssd
    autoUnseal:
      # For production, use cloud KMS. For testing, skip auto-unseal.
      # transit:
      #   address: https://external-vault.example.com
      #   mount: transit
      #   keyName: banhbaoring-unseal

  # PostgreSQL (Scaleway Managed Database recommended for production)
  database:
    # Option 1: In-cluster PostgreSQL (for testing)
    internal:
      enabled: true
      replicas: 1
      storage:
        size: 20Gi
        storageClassName: scw-bssd
    # Option 2: Scaleway Managed Database (recommended)
    # external:
    #   host: YOUR_MANAGED_DB.rdb.fr-par.scw.cloud
    #   port: 5432
    #   database: banhbaoring
    #   credentialsSecret: postgresql-external-credentials

  # Redis
  redis:
    internal:
      enabled: true
      replicas: 1
      storage:
        size: 5Gi
        storageClassName: scw-bssd

  # Control Plane API
  controlPlane:
    replicas: 2
    image:
      repository: $REGISTRY/control-plane
      tag: $VERSION
    config:
      secretName: banhbaoring-config
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        cpu: 500m
        memory: 512Mi

  # Web Dashboard
  dashboard:
    replicas: 2
    resources:
      requests:
        cpu: 50m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi

  # Monitoring
  monitoring:
    enabled: true
    prometheus:
      retention: 7d
      storage:
        size: 10Gi
        storageClassName: scw-bssd
    grafana:
      enabled: true
EOF
```

---

## 8. Expose via Scaleway Load Balancer

```bash
# Create LoadBalancer service
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: banhbaoring-lb
  namespace: banhbaoring
  annotations:
    # Scaleway Load Balancer annotations
    service.beta.kubernetes.io/scw-loadbalancer-type: lb-s  # Small LB
    service.beta.kubernetes.io/scw-loadbalancer-protocol-http: "true"
    service.beta.kubernetes.io/scw-loadbalancer-use-hostname: "true"
spec:
  type: LoadBalancer
  selector:
    app: banhbaoring-control-plane
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: https
      port: 443
      targetPort: 8080
EOF

# Get Load Balancer IP
kubectl get svc banhbaoring-lb -n banhbaoring -w
```

---

## 9. Configure DNS & TLS

### Option A: Scaleway DNS

```bash
# Create DNS record pointing to Load Balancer IP
scw dns record add YOUR_DOMAIN \
  name=api \
  type=A \
  data=<LOAD_BALANCER_IP> \
  ttl=300
```

### Option B: Install cert-manager for TLS

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml

# Create Let's Encrypt issuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: nginx
EOF

# Create Ingress with TLS
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: banhbaoring
  namespace: banhbaoring
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - api.banhbaoring.example.com
        - dashboard.banhbaoring.example.com
      secretName: banhbaoring-tls
  rules:
    - host: api.banhbaoring.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: banhbaoring-control-plane
                port:
                  number: 8080
    - host: dashboard.banhbaoring.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: banhbaoring-dashboard
                port:
                  number: 3000
EOF
```

---

## 10. Update OAuth Callback URLs

After getting your domain, update OAuth apps:

### GitHub

1. Go to https://github.com/settings/developers
2. Edit your OAuth App
3. Set **Authorization callback URL**: `https://api.banhbaoring.example.com/auth/github/callback`

### Google

1. Go to https://console.cloud.google.com/apis/credentials
2. Edit your OAuth 2.0 Client
3. Add **Authorized redirect URIs**: `https://api.banhbaoring.example.com/auth/google/callback`

---

## 11. Initialize OpenBao

If not using auto-unseal, manually initialize and unseal:

```bash
# Port-forward to OpenBao
kubectl port-forward svc/production-openbao -n banhbaoring 8200:8200 &

# Initialize
export VAULT_ADDR=http://localhost:8200
vault operator init -key-shares=5 -key-threshold=3

# Save the unseal keys and root token securely!

# Unseal (need 3 of 5 keys)
vault operator unseal <key1>
vault operator unseal <key2>
vault operator unseal <key3>

# Verify
vault status
```

---

## 12. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n banhbaoring

# Check services
kubectl get svc -n banhbaoring

# Test API endpoint
curl https://api.banhbaoring.example.com/health

# Test dashboard
open https://dashboard.banhbaoring.example.com
```

---

## Cost Estimation (Scaleway)

| Resource | Type | Monthly Cost (approx) |
|----------|------|----------------------|
| Kapsule Cluster | 3x DEV1-M | ~€45 |
| Block Storage | 50GB SSD | ~€5 |
| Load Balancer | LB-S | ~€10 |
| Container Registry | Private | ~€0 (usage-based) |
| **Total** | | **~€60/month** |

For production, consider:
- Managed PostgreSQL: ~€20/month
- Larger nodes (PRO2-S): ~€90/month
- Multiple availability zones

---

## Troubleshooting

### Pods not starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n banhbaoring

# Check logs
kubectl logs <pod-name> -n banhbaoring

# Check events
kubectl get events -n banhbaoring --sort-by='.lastTimestamp'
```

### OpenBao sealed

```bash
# Check seal status
kubectl exec -it production-openbao-0 -n banhbaoring -- vault status

# Unseal manually
kubectl exec -it production-openbao-0 -n banhbaoring -- vault operator unseal
```

### Database connection issues

```bash
# Test PostgreSQL connection
kubectl run pg-test --rm -it --image=postgres:15 -- \
  psql "postgresql://user:pass@production-postgresql:5432/banhbaoring"
```

---

## Cleanup

```bash
# Delete cluster resources
kubectl delete banhbaoringcluster production -n banhbaoring

# Uninstall operator
helm uninstall banhbaoring-operator -n banhbaoring-system

# Delete namespaces
kubectl delete namespace banhbaoring banhbaoring-system

# Delete Kapsule cluster (if no longer needed)
scw k8s cluster delete <cluster-id>
```

---

## Next Steps

1. **Set up monitoring alerts** - Configure Grafana alerts for key metrics
2. **Enable backups** - Set up scheduled backups to Scaleway Object Storage
3. **Production hardening** - Switch to managed PostgreSQL, enable auto-unseal with cloud KMS
4. **Scale testing** - Test with parallel worker load

