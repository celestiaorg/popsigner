# Multi-stage build for popsigner-lite
# Stage 1: Builder
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy entire popsigner repo (popsigner-lite is part of it)
COPY . .

# Build popsigner-lite
WORKDIR /build/control-plane/cmd/popsigner-lite
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o popsigner-lite .

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget curl

# Create non-root user
RUN addgroup -g 1000 popsigner && \
    adduser -D -u 1000 -G popsigner popsigner

# Create data directory
RUN mkdir -p /data && chown popsigner:popsigner /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/control-plane/cmd/popsigner-lite/popsigner-lite /usr/local/bin/popsigner-lite
RUN chmod +x /usr/local/bin/popsigner-lite

# Switch to non-root user
USER popsigner

# Expose ports
# 3000: REST API
# 8545: JSON-RPC (Ethereum-compatible)
EXPOSE 3000 8545

# Health check
HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
  CMD wget -qO- http://localhost:3000/health || exit 1

# Environment variables (can be overridden)
ENV JSONRPC_PORT=8545
ENV REST_API_PORT=3000

# Run popsigner-lite
CMD ["popsigner-lite"]
