# Build stage for Go
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Force fresh download of patched optimism fork (bypass module cache and checksum server)
# This ensures we always get the latest version of the patched fork
RUN GONOSUMDB=github.com/Bidon15/optimism GOPROXY=direct go mod download

# Copy source code
COPY . .

# Build the binary WITHOUT cache mounts to ensure fresh module resolution
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

# Runtime stage - use Debian for glibc compatibility (Foundry binaries need glibc)
FROM debian:bookworm-slim

LABEL org.opencontainers.image.title="POPSigner Control Plane"
LABEL org.opencontainers.image.description="POPSigner - Point-of-Presence signing infrastructure"
LABEL org.opencontainers.image.vendor="POPSigner"

WORKDIR /app

# Install runtime dependencies
# - ca-certificates, wget: for health checks and HTTPS
# - curl: for Foundry installation
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget curl \
    && rm -rf /var/lib/apt/lists/*

# Install Foundry (anvil) for pop-bundle deployments
# Foundry provides anvil (local Ethereum node) needed for local devnet bundles
# Download pre-built binaries directly from GitHub releases
ENV FOUNDRY_DIR=/usr/local/foundry
RUN mkdir -p ${FOUNDRY_DIR}/bin && \
    LATEST=$(curl -sL https://api.github.com/repos/foundry-rs/foundry/releases/latest | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4) && \
    curl -L "https://github.com/foundry-rs/foundry/releases/download/${LATEST}/foundry_${LATEST}_linux_amd64.tar.gz" | tar -xzC ${FOUNDRY_DIR}/bin
ENV PATH="${FOUNDRY_DIR}/bin:${PATH}"

# Copy binary from builder
COPY --from=builder /server /app/server

# Copy static files for web dashboard
COPY --from=builder /app/static /app/static

# Create non-root user
RUN useradd -r -s /bin/false appuser && \
    chown -R appuser:appuser /app
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Run the server
ENTRYPOINT ["/app/server"]
