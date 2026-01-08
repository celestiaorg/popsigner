#!/bin/bash
# Build and push images for amd64 (Scaleway K8s)
# Uses parallel builds for faster execution
set -euo pipefail

# Configuration
REGISTRY="${REGISTRY:-rg.nl-ams.scw.cloud/banhbao}"
VERSION="${VERSION:-latest}"
PLATFORM="linux/arm64"

echo "🔔 Building POPSigner images for $PLATFORM"
echo "Registry: $REGISTRY"
echo "Version: $VERSION"
echo ""

# Ensure buildx is available
docker buildx create --name banhbao-builder --use 2>/dev/null || docker buildx use banhbao-builder

# Create temp directory for build logs
LOG_DIR=$(mktemp -d)
trap "rm -rf $LOG_DIR" EXIT

echo "📦 Building all images in parallel..."
echo ""

# Build and push operator (background)
(
    echo "[operator] Starting build..."
    docker buildx build \
        --platform $PLATFORM \
        --tag $REGISTRY/operator:$VERSION \
        --push \
        ./operator \
        > "$LOG_DIR/operator.log" 2>&1 \
    && echo "[operator] ✅ Complete" \
    || { echo "[operator] ❌ Failed"; cat "$LOG_DIR/operator.log"; exit 1; }
) &
PID_OPERATOR=$!

# Build and push control-plane (background)
(
    echo "[control-plane] Starting build..."
    docker buildx build \
        --platform $PLATFORM \
        --tag $REGISTRY/control-plane:$VERSION \
        --push \
        -f ./control-plane/docker/Dockerfile \
        ./control-plane \
        > "$LOG_DIR/control-plane.log" 2>&1 \
    && echo "[control-plane] ✅ Complete" \
    || { echo "[control-plane] ❌ Failed"; cat "$LOG_DIR/control-plane.log"; exit 1; }
) &
PID_CONTROL_PLANE=$!

# Build and push secp256k1 plugin (background)
(
    echo "[plugin] Starting build..."
    docker buildx build \
        --platform $PLATFORM \
        --tag $REGISTRY/secp256k1-plugin:$VERSION \
        --push \
        ./plugin \
        > "$LOG_DIR/plugin.log" 2>&1 \
    && echo "[plugin] ✅ Complete" \
    || { echo "[plugin] ❌ Failed"; cat "$LOG_DIR/plugin.log"; exit 1; }
) &
PID_PLUGIN=$!

# Wait for all builds to complete
echo "Waiting for builds to complete..."
FAILED=0

wait $PID_OPERATOR || FAILED=1
wait $PID_CONTROL_PLANE || FAILED=1
wait $PID_PLUGIN || FAILED=1

echo ""

if [ $FAILED -eq 1 ]; then
    echo "❌ Some builds failed! Check logs above."
    exit 1
fi

echo "============================================"
echo "✅ All images built and pushed successfully!"
echo "============================================"
echo ""
echo "Images:"
echo "  - $REGISTRY/operator:$VERSION"
echo "  - $REGISTRY/control-plane:$VERSION"
echo "  - $REGISTRY/secp256k1-plugin:$VERSION"
