#!/bin/bash
# test-nitro.sh - Quick test script for Arbitrum Nitro mTLS integration
#
# This script simulates how Arbitrum Nitro components interact with the RPC Gateway
# using mTLS (mutual TLS) authentication with client certificates.
#
# Usage:
#   ./test-nitro.sh <mtls_url> <ca_cert> <client_cert> <client_key> <signer_address> [chain_id]
#
# Example:
#   ./test-nitro.sh https://rpc.popsigner.com:8546 \
#     /path/to/popsigner-ca.crt \
#     /path/to/client.crt \
#     /path/to/client.key \
#     0x742d35Cc... \
#     42161
#
# Using environment variables:
#   POPSIGNER_MTLS_URL=https://rpc.popsigner.com:8546 \
#   POPSIGNER_CA_CERT=/path/to/popsigner-ca.crt \
#   POPSIGNER_CLIENT_CERT=/path/to/client.crt \
#   POPSIGNER_CLIENT_KEY=/path/to/client.key \
#   POPSIGNER_SIGNER_ADDRESS=0x... \
#   ./test-nitro.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Parse arguments or use environment variables
MTLS_URL="${1:-${POPSIGNER_MTLS_URL}}"
CA_CERT="${2:-${POPSIGNER_CA_CERT}}"
CLIENT_CERT="${3:-${POPSIGNER_CLIENT_CERT}}"
CLIENT_KEY="${4:-${POPSIGNER_CLIENT_KEY}}"
SIGNER_ADDRESS="${5:-${POPSIGNER_SIGNER_ADDRESS}}"
CHAIN_ID="${6:-${POPSIGNER_CHAIN_ID:-42161}}"

# Convert chain ID to hex if decimal
if [[ "$CHAIN_ID" =~ ^[0-9]+$ ]]; then
    CHAIN_ID_HEX=$(printf "0x%x" "$CHAIN_ID")
else
    CHAIN_ID_HEX="$CHAIN_ID"
fi

echo "=========================================="
echo -e "${CYAN}POPSigner Nitro mTLS Integration Test${NC}"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  mTLS URL:       $MTLS_URL"
echo "  CA Cert:        $CA_CERT"
echo "  Client Cert:    $CLIENT_CERT"
echo "  Client Key:     $CLIENT_KEY"
echo "  Signer Address: $SIGNER_ADDRESS"
echo "  Chain ID:       $CHAIN_ID ($CHAIN_ID_HEX)"
echo ""

# Validate inputs
if [ -z "$MTLS_URL" ]; then
    echo -e "${RED}Error: mTLS URL not provided${NC}"
    echo "Usage: $0 <mtls_url> <ca_cert> <client_cert> <client_key> <signer_address> [chain_id]"
    exit 1
fi

if [ -z "$CA_CERT" ] || [ ! -f "$CA_CERT" ]; then
    echo -e "${RED}Error: CA certificate not found: $CA_CERT${NC}"
    exit 1
fi

if [ -z "$CLIENT_CERT" ] || [ ! -f "$CLIENT_CERT" ]; then
    echo -e "${RED}Error: Client certificate not found: $CLIENT_CERT${NC}"
    exit 1
fi

if [ -z "$CLIENT_KEY" ] || [ ! -f "$CLIENT_KEY" ]; then
    echo -e "${RED}Error: Client key not found: $CLIENT_KEY${NC}"
    exit 1
fi

if [ -z "$SIGNER_ADDRESS" ]; then
    echo -e "${RED}Error: Signer address not provided${NC}"
    exit 1
fi

# Display certificate info
echo "----------------------------------------"
echo "Certificate Information"
echo "----------------------------------------"
echo -e "${CYAN}CA Certificate:${NC}"
openssl x509 -in "$CA_CERT" -noout -subject -issuer 2>/dev/null | head -2
echo ""
echo -e "${CYAN}Client Certificate:${NC}"
openssl x509 -in "$CLIENT_CERT" -noout -subject -dates 2>/dev/null | head -3
echo ""

# Test 1: Health check with mTLS
echo "----------------------------------------"
echo "Test 1: Health Check (mTLS)"
echo "----------------------------------------"
HEALTH=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$MTLS_URL/health" 2>&1)
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo "  Response: $HEALTH"
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo "  Response: $HEALTH"
    echo ""
    echo "  Possible issues:"
    echo "  - mTLS port not enabled (check POPSIGNER_MTLS_ENABLED=true)"
    echo "  - Wrong port (mTLS typically on 8546)"
    echo "  - Certificate not trusted by server"
    exit 1
fi
echo ""

# Test 2: eth_accounts
echo "----------------------------------------"
echo "Test 2: eth_accounts (mTLS)"
echo "----------------------------------------"
ACCOUNTS=$(curl -s -X POST "$MTLS_URL" \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}')

if echo "$ACCOUNTS" | grep -q '"error"'; then
    echo -e "${RED}✗ eth_accounts failed${NC}"
    echo "  Response: $ACCOUNTS"
    exit 1
else
    echo -e "${GREEN}✓ eth_accounts succeeded${NC}"
    echo "  Response: $ACCOUNTS"
fi
echo ""

# Test 3: eth_signTransaction (legacy)
echo "----------------------------------------"
echo "Test 3: eth_signTransaction (Legacy)"
echo "----------------------------------------"
SIGN_TX=$(curl -s -X POST "$MTLS_URL" \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"jsonrpc\": \"2.0\",
        \"method\": \"eth_signTransaction\",
        \"params\": [{
            \"from\": \"$SIGNER_ADDRESS\",
            \"to\": \"0x0000000000000000000000000000000000000000\",
            \"gas\": \"0x5208\",
            \"gasPrice\": \"0x3b9aca00\",
            \"value\": \"0x0\",
            \"nonce\": \"0x0\",
            \"data\": \"0x\",
            \"chainId\": \"$CHAIN_ID_HEX\"
        }],
        \"id\": 1
    }")

if echo "$SIGN_TX" | grep -q '"error"'; then
    echo -e "${RED}✗ eth_signTransaction failed${NC}"
    echo "  Response: $SIGN_TX"
else
    echo -e "${GREEN}✓ eth_signTransaction succeeded${NC}"
    SIGNED=$(echo "$SIGN_TX" | grep -o '"result":"[^"]*"' | head -1)
    echo "  Result: ${SIGNED:0:50}..."
fi
echo ""

# Test 4: eth_signTransaction (EIP-1559)
echo "----------------------------------------"
echo "Test 4: eth_signTransaction (EIP-1559)"
echo "----------------------------------------"
SIGN_EIP1559=$(curl -s -X POST "$MTLS_URL" \
    --cacert "$CA_CERT" \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"jsonrpc\": \"2.0\",
        \"method\": \"eth_signTransaction\",
        \"params\": [{
            \"from\": \"$SIGNER_ADDRESS\",
            \"to\": \"0x0000000000000000000000000000000000000000\",
            \"gas\": \"0x5208\",
            \"maxFeePerGas\": \"0x77359400\",
            \"maxPriorityFeePerGas\": \"0x3b9aca00\",
            \"value\": \"0x0\",
            \"nonce\": \"0x1\",
            \"data\": \"0x\",
            \"chainId\": \"$CHAIN_ID_HEX\"
        }],
        \"id\": 1
    }")

if echo "$SIGN_EIP1559" | grep -q '"error"'; then
    echo -e "${YELLOW}⚠ EIP-1559 signing failed (may not be supported)${NC}"
    echo "  Response: $SIGN_EIP1559"
else
    echo -e "${GREEN}✓ EIP-1559 signing succeeded${NC}"
    SIGNED=$(echo "$SIGN_EIP1559" | grep -o '"result":"[^"]*"' | head -1)
    echo "  Result: ${SIGNED:0:50}..."
fi
echo ""

# Test 5: Verify mTLS is required (should fail without client cert)
echo "----------------------------------------"
echo "Test 5: mTLS Required (should fail)"
echo "----------------------------------------"
NO_CERT=$(curl -s --cacert "$CA_CERT" -X POST "$MTLS_URL" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}' 2>&1)

if echo "$NO_CERT" | grep -q '"result"'; then
    echo -e "${RED}✗ Request without client cert succeeded (should have failed)${NC}"
else
    echo -e "${GREEN}✓ Request without client cert correctly rejected${NC}"
    echo "  (mTLS enforcement is working)"
fi
echo ""

echo "=========================================="
echo "All tests completed!"
echo "=========================================="
echo ""
echo -e "${CYAN}Arbitrum Nitro Configuration Example:${NC}"
echo ""
echo "  # Batch Poster"
echo "  ./nitro \\"
echo "    --node.batch-poster.enable=true \\"
echo "    --node.batch-poster.data-poster.external-signer.url=$MTLS_URL \\"
echo "    --node.batch-poster.data-poster.external-signer.address=$SIGNER_ADDRESS \\"
echo "    --node.batch-poster.data-poster.external-signer.method=eth_signTransaction \\"
echo "    --node.batch-poster.data-poster.external-signer.root-ca=$CA_CERT \\"
echo "    --node.batch-poster.data-poster.external-signer.client-cert=$CLIENT_CERT \\"
echo "    --node.batch-poster.data-poster.external-signer.client-private-key=$CLIENT_KEY"
echo ""
echo "  # Staker"
echo "  ./nitro \\"
echo "    --node.staker.enable=true \\"
echo "    --node.staker.data-poster.external-signer.url=$MTLS_URL \\"
echo "    --node.staker.data-poster.external-signer.address=$SIGNER_ADDRESS \\"
echo "    --node.staker.data-poster.external-signer.method=eth_signTransaction \\"
echo "    --node.staker.data-poster.external-signer.root-ca=$CA_CERT \\"
echo "    --node.staker.data-poster.external-signer.client-cert=$CLIENT_CERT \\"
echo "    --node.staker.data-poster.external-signer.client-private-key=$CLIENT_KEY"
echo ""

