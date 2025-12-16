#!/bin/bash
# Build and push images using Docker with buildx
# Supports multi-platform builds with better layer caching
set -euo pipefail

# Configuration
REGISTRY="${REGISTRY:-rg.nl-ams.scw.cloud/banhbao}"
VERSION="${VERSION:-latest}"
PLATFORM="${PLATFORM:-linux/arm64}"
# Enable BuildKit for faster builds
export DOCKER_BUILDKIT=1

echo "🐳 Building POPSigner images using Docker"
echo "Platform: $PLATFORM"
echo "Registry: $REGISTRY"
echo "Version: $VERSION"
echo ""

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Error: 'docker' command not found"
    echo "Install Docker Desktop or Docker Engine"
    exit 1
fi

# Ensure buildx is available and set up
setup_buildx() {
    local builder_name="popsigner-builder"
    
    # Check if our builder exists
    if ! docker buildx inspect "$builder_name" &> /dev/null; then
        echo "🔧 Creating buildx builder: $builder_name"
        docker buildx create --name "$builder_name" --use --driver docker-container
    else
        docker buildx use "$builder_name"
    fi
    
    # Bootstrap the builder
    docker buildx inspect --bootstrap &> /dev/null
}

# Function to build and push an image
build_and_push() {
    local name=$1
    local dockerfile=$2
    local context=$3
    local tag="$REGISTRY/$name:$VERSION"
    
    echo "📦 Building $name..."
    echo "   Dockerfile: $dockerfile"
    echo "   Context: $context"
    echo "   Tag: $tag"
    
    # Build and push in one step with buildx
    # --push pushes after build, avoiding separate push step
    # --cache-from and --cache-to enable layer caching
    docker buildx build \
        --platform "$PLATFORM" \
        --file "$dockerfile" \
        --tag "$tag" \
        --push \
        --cache-from "type=registry,ref=$REGISTRY/$name:cache" \
        --cache-to "type=registry,ref=$REGISTRY/$name:cache,mode=max" \
        "$context"
    
    echo "✅ $name built and pushed"
    echo ""
}

# Function to build only (no push)
build_only() {
    local name=$1
    local dockerfile=$2
    local context=$3
    local tag="$REGISTRY/$name:$VERSION"
    
    echo "📦 Building $name (local only)..."
    echo "   Dockerfile: $dockerfile"
    echo "   Context: $context"
    echo "   Tag: $tag"
    
    docker buildx build \
        --platform "$PLATFORM" \
        --file "$dockerfile" \
        --tag "$tag" \
        --load \
        "$context"
    
    echo "✅ $name built locally"
    echo ""
}

# Navigate to project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "Working directory: $(pwd)"
echo ""

# Parse arguments
PUSH=true
if [[ "${1:-}" == "--no-push" ]] || [[ "${1:-}" == "-n" ]]; then
    PUSH=false
    echo "⚠️  Build only mode (no push)"
    echo ""
fi

# Setup buildx for cross-platform builds
echo "🔧 Setting up Docker buildx..."
setup_buildx
echo ""

# Login check
REGISTRY_HOST="${REGISTRY%%/*}"
echo "📋 Registry host: $REGISTRY_HOST"
if $PUSH; then
    echo "   Checking registry login..."
    if ! docker login "$REGISTRY_HOST" --get-login &> /dev/null 2>&1; then
        echo "   ⚠️  Not logged in. Run: docker login $REGISTRY_HOST"
    fi
fi
echo ""

# Choose build function based on push flag
if $PUSH; then
    build_fn=build_and_push
else
    build_fn=build_only
fi

# Build operator
$build_fn "operator" \
    "operator/Dockerfile" \
    "operator"

# Build control-plane
$build_fn "control-plane" \
    "control-plane/docker/Dockerfile" \
    "control-plane"

# Build secp256k1 plugin
$build_fn "secp256k1-plugin" \
    "plugin/Dockerfile" \
    "plugin"

echo "============================================"
echo "✅ All images built successfully!"
echo "============================================"
echo ""
echo "Images:"
echo "  - $REGISTRY/operator:$VERSION"
echo "  - $REGISTRY/control-plane:$VERSION"
echo "  - $REGISTRY/secp256k1-plugin:$VERSION"
echo ""
echo "Platform: $PLATFORM"
if $PUSH; then
    echo "Status: Pushed to registry"
else
    echo "Status: Built locally (use without --no-push to push)"
fi

