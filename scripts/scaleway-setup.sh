#!/bin/bash
# BanhBaoRing Scaleway Quick Setup Script
# Prerequisites: scw CLI configured, kubectl, helm

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-banhbaoring}"
REGION="${REGION:-fr-par}"
NODE_TYPE="${NODE_TYPE:-DEV1-M}"
NODE_COUNT="${NODE_COUNT:-3}"
K8S_VERSION="${K8S_VERSION:-1.29}"

echo -e "${GREEN}🔔 BanhBaoRing Scaleway Setup${NC}"
echo "================================="

# Check prerequisites
check_prerequisites() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"
    
    if ! command -v scw &> /dev/null; then
        echo -e "${RED}✗ scw CLI not found. Install from: https://github.com/scaleway/scaleway-cli${NC}"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}✗ kubectl not found. Install from: https://kubernetes.io/docs/tasks/tools/${NC}"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        echo -e "${RED}✗ helm not found. Install from: https://helm.sh/docs/intro/install/${NC}"
        exit 1
    fi
    
    # Check if scw is configured
    if ! scw account project get &> /dev/null; then
        echo -e "${RED}✗ scw CLI not configured. Run: scw init${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ All prerequisites met${NC}"
}

# Create Kubernetes cluster
create_cluster() {
    echo -e "\n${YELLOW}Creating Kubernetes cluster...${NC}"
    
    # Check if cluster already exists
    if scw k8s cluster list -o json | jq -e ".[] | select(.name == \"$CLUSTER_NAME\")" &> /dev/null; then
        echo -e "${YELLOW}Cluster '$CLUSTER_NAME' already exists${NC}"
        CLUSTER_ID=$(scw k8s cluster list -o json | jq -r ".[] | select(.name == \"$CLUSTER_NAME\") | .id")
    else
        echo "Creating cluster '$CLUSTER_NAME' in $REGION..."
        CLUSTER_ID=$(scw k8s cluster create \
            name="$CLUSTER_NAME" \
            version="$K8S_VERSION" \
            cni=cilium \
            region="$REGION" \
            pools.0.name=default \
            pools.0.node-type="$NODE_TYPE" \
            pools.0.size="$NODE_COUNT" \
            pools.0.min-size=2 \
            pools.0.max-size=5 \
            pools.0.autoscaling=true \
            pools.0.autohealing=true \
            -o json | jq -r '.id')
        
        echo "Waiting for cluster to be ready..."
        scw k8s cluster wait "$CLUSTER_ID" region="$REGION"
    fi
    
    echo -e "${GREEN}✓ Cluster ready: $CLUSTER_ID${NC}"
    
    # Install kubeconfig
    echo "Installing kubeconfig..."
    scw k8s kubeconfig install "$CLUSTER_ID" region="$REGION"
    
    # Verify connection
    kubectl get nodes
}

# Create Container Registry
create_registry() {
    echo -e "\n${YELLOW}Creating Container Registry...${NC}"
    
    if scw registry namespace list -o json | jq -e ".[] | select(.name == \"$CLUSTER_NAME\")" &> /dev/null; then
        echo -e "${YELLOW}Registry namespace '$CLUSTER_NAME' already exists${NC}"
    else
        scw registry namespace create name="$CLUSTER_NAME" region="$REGION" is-public=false
    fi
    
    # Login to registry
    scw registry login
    
    REGISTRY="rg.$REGION.scw.cloud/$CLUSTER_NAME"
    echo -e "${GREEN}✓ Registry ready: $REGISTRY${NC}"
    echo ""
    echo "Export this to use in build commands:"
    echo "  export REGISTRY=$REGISTRY"
}

# Create namespaces and storage class
setup_kubernetes() {
    echo -e "\n${YELLOW}Setting up Kubernetes resources...${NC}"
    
    # Create namespaces
    kubectl create namespace banhbaoring --dry-run=client -o yaml | kubectl apply -f -
    kubectl create namespace banhbaoring-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Create storage class for Scaleway Block Storage
    kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: scw-bssd
provisioner: csi.scaleway.com
parameters:
  type: b_ssd
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
EOF
    
    echo -e "${GREEN}✓ Kubernetes resources created${NC}"
}

# Interactive secret creation
create_secrets() {
    echo -e "\n${YELLOW}Creating secrets...${NC}"
    echo ""
    echo "You'll need the following credentials:"
    echo "  - GitHub OAuth: https://github.com/settings/developers"
    echo "  - Google OAuth: https://console.cloud.google.com/apis/credentials"
    echo ""
    
    read -p "Enter GitHub OAuth Client ID: " GITHUB_ID
    read -sp "Enter GitHub OAuth Client Secret: " GITHUB_SECRET
    echo ""
    read -p "Enter Google OAuth Client ID: " GOOGLE_ID
    read -sp "Enter Google OAuth Client Secret: " GOOGLE_SECRET
    echo ""
    
    # Generate JWT secret
    JWT_SECRET=$(openssl rand -base64 32)
    
    # Create config secret
    kubectl create secret generic banhbaoring-config \
        --namespace banhbaoring \
        --from-literal=jwt-secret="$JWT_SECRET" \
        --from-literal=oauth-github-id="$GITHUB_ID" \
        --from-literal=oauth-github-secret="$GITHUB_SECRET" \
        --from-literal=oauth-google-id="$GOOGLE_ID" \
        --from-literal=oauth-google-secret="$GOOGLE_SECRET" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Create database secrets
    kubectl create secret generic postgresql-credentials \
        --namespace banhbaoring \
        --from-literal=postgres-password="$(openssl rand -base64 24)" \
        --from-literal=password="$(openssl rand -base64 24)" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    kubectl create secret generic redis-credentials \
        --namespace banhbaoring \
        --from-literal=redis-password="$(openssl rand -base64 24)" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo -e "${GREEN}✓ Secrets created${NC}"
}

# Print next steps
print_next_steps() {
    echo -e "\n${GREEN}✓ Setup complete!${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Build and push images:"
    echo "   cd /path/to/banhbaoring"
    echo "   export REGISTRY=rg.$REGION.scw.cloud/$CLUSTER_NAME"
    echo "   export VERSION=v0.1.0"
    echo "   make docker-build"
    echo "   docker tag ghcr.io/bidon15/banhbaoring-operator:dev \$REGISTRY/operator:\$VERSION"
    echo "   docker tag ghcr.io/bidon15/banhbaoring-control-plane:dev \$REGISTRY/control-plane:\$VERSION"
    echo "   docker tag ghcr.io/bidon15/banhbaoring-secp256k1:dev \$REGISTRY/secp256k1-plugin:\$VERSION"
    echo "   docker push \$REGISTRY/operator:\$VERSION"
    echo "   docker push \$REGISTRY/control-plane:\$VERSION"
    echo "   docker push \$REGISTRY/secp256k1-plugin:\$VERSION"
    echo ""
    echo "2. Install operator:"
    echo "   helm install banhbaoring-operator ./operator/charts/banhbaoring-operator \\"
    echo "     --namespace banhbaoring-system \\"
    echo "     --set image.repository=\$REGISTRY/operator \\"
    echo "     --set image.tag=\$VERSION"
    echo ""
    echo "3. Deploy cluster (see doc/deployment/SCALEWAY.md for example)"
    echo ""
    echo "4. Update OAuth callback URLs with your final domain"
}

# Main
main() {
    check_prerequisites
    create_cluster
    create_registry
    setup_kubernetes
    
    echo ""
    read -p "Do you want to create secrets now? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        create_secrets
    else
        echo -e "${YELLOW}Skipping secrets. Create them manually later.${NC}"
    fi
    
    print_next_steps
}

main "$@"

