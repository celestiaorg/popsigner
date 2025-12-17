#!/bin/bash
# test-opstack.sh - Quick test script for OP Stack integration
#
# This script simulates how OP Stack components interact with the RPC Gateway.
# It uses curl to send JSON-RPC requests with API key authentication.
#
# Usage:
#   ./test-opstack.sh <rpc_url> <api_key> <signer_address> [chain_id]
#
# Example:
#   ./test-opstack.sh https://popsigner.example.com:8545 pop_abc123... 0x742d35Cc... 11155111
#
# For local testing:
#   ./test-opstack.sh http://localhost:8545 pop_... 0x... 1

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
RPC_URL="${1:-${POPSIGNER_RPC_URL:-http://localhost:8545}}"
API_KEY="${2:-${POPSIGNER_API_KEY}}"
SIGNER_ADDRESS="${3:-${POPSIGNER_SIGNER_ADDRESS}}"
CHAIN_ID="${4:-${POPSIGNER_CHAIN_ID:-11155111}}"

# Convert chain ID to hex if decimal
if [[ "$CHAIN_ID" =~ ^[0-9]+$ ]]; then
    CHAIN_ID_HEX=$(printf "0x%x" "$CHAIN_ID")
else
    CHAIN_ID_HEX="$CHAIN_ID"
fi

echo "=========================================="
echo "POPSigner OP Stack Integration Test"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  RPC URL:        $RPC_URL"
echo "  Signer Address: $SIGNER_ADDRESS"
echo "  Chain ID:       $CHAIN_ID ($CHAIN_ID_HEX)"
echo "  API Key:        ${API_KEY:0:10}..."
echo ""

# Validate inputs
if [ -z "$API_KEY" ]; then
    echo -e "${RED}Error: API key not provided${NC}"
    echo "Usage: $0 <rpc_url> <api_key> <signer_address> [chain_id]"
    exit 1
fi

if [ -z "$SIGNER_ADDRESS" ]; then
    echo -e "${RED}Error: Signer address not provided${NC}"
    exit 1
fi

# Test 1: Health check
echo "----------------------------------------"
echo "Test 1: Health Check"
echo "----------------------------------------"
HEALTH=$(curl -s "$RPC_URL/health")
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo "  Response: $HEALTH"
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo "  Response: $HEALTH"
    exit 1
fi
echo ""

# Test 2: eth_accounts
echo "----------------------------------------"
echo "Test 2: eth_accounts"
echo "----------------------------------------"
ACCOUNTS=$(curl -s -X POST "$RPC_URL/rpc" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
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
SIGN_TX=$(curl -s -X POST "$RPC_URL/rpc" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
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
    # Extract just the first and last parts of the signed tx
    SIGNED=$(echo "$SIGN_TX" | grep -o '"result":"[^"]*"' | head -1)
    echo "  Result: ${SIGNED:0:50}..."
fi
echo ""

# Test 4: eth_signTransaction (EIP-1559)
echo "----------------------------------------"
echo "Test 4: eth_signTransaction (EIP-1559)"
echo "----------------------------------------"
SIGN_EIP1559=$(curl -s -X POST "$RPC_URL/rpc" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
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

# Test 5: Authentication with Bearer token
echo "----------------------------------------"
echo "Test 5: Authorization Bearer Token"
echo "----------------------------------------"
BEARER=$(curl -s -X POST "$RPC_URL/rpc" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}')

if echo "$BEARER" | grep -q '"error"'; then
    echo -e "${RED}✗ Bearer token auth failed${NC}"
else
    echo -e "${GREEN}✓ Bearer token auth succeeded${NC}"
fi
echo ""

# Test 6: Verify auth is required
echo "----------------------------------------"
echo "Test 6: Auth Required (should fail)"
echo "----------------------------------------"
NO_AUTH=$(curl -s -X POST "$RPC_URL/rpc" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}')

if echo "$NO_AUTH" | grep -q '"result"'; then
    echo -e "${RED}✗ Request without auth succeeded (should have failed)${NC}"
else
    echo -e "${GREEN}✓ Request without auth correctly rejected${NC}"
fi
echo ""

echo "=========================================="
echo "All tests completed!"
echo "=========================================="
echo ""
echo "OP Stack Configuration Example:"
echo ""
echo "  op-batcher \\"
echo "    --signer.endpoint=$RPC_URL/rpc \\"
echo "    --signer.address=$SIGNER_ADDRESS \\"
echo '    --signer.header="X-API-Key='$API_KEY'"'
echo ""

