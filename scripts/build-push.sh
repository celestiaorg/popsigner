#!/bin/bash
# Build and push images for amd64 (Scaleway K8s)
set -euo pipefail

# Configuration
REGISTRY="${REGISTRY:-rg.nl-ams.scw.cloud/banhbao}"
VERSION="${VERSION:-latest}"
PLATFORM="linux/amd64"

echo "🔔 Building BanhBaoRing images for $PLATFORM"
echo "Registry: $REGISTRY"
echo "Version: $VERSION"
echo ""

# Ensure buildx is available
docker buildx create --name banhbao-builder --use 2>/dev/null || docker buildx use banhbao-builder

# Build and push operator
echo "📦 Building operator..."
docker buildx build \
    --platform $PLATFORM \
    --tag $REGISTRY/operator:$VERSION \
    --push \
    ./operator

# Build and push control-plane
echo "📦 Building control-plane..."
docker buildx build \
    --platform $PLATFORM \
    --tag $REGISTRY/control-plane:$VERSION \
    --push \
    -f ./control-plane/docker/Dockerfile \
    ./control-plane

# Build and push secp256k1 plugin
echo "📦 Building secp256k1 plugin..."
docker buildx build \
    --platform $PLATFORM \
    --tag $REGISTRY/secp256k1-plugin:$VERSION \
    --push \
    ./plugin

echo ""
echo "✅ Done! Images pushed to $REGISTRY"
echo ""
echo "Images:"
echo "  - $REGISTRY/operator:$VERSION"
echo "  - $REGISTRY/control-plane:$VERSION"
echo "  - $REGISTRY/secp256k1-plugin:$VERSION"
