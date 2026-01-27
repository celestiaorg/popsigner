#!/bin/bash
# Test script to verify subdomain routing works correctly

set -e

COOKIE="banhbao_session=dev-session-token-12345"

echo "🧪 Testing POPSigner Routing"
echo "=============================="
echo ""

echo "1️⃣  Testing main dashboard (localhost:8080)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Cookie: $COOKIE" http://localhost:8080/dashboard)
if [ "$HTTP_CODE" = "200" ]; then
    echo "   ✅ Main dashboard accessible (HTTP $HTTP_CODE)"
else
    echo "   ❌ Main dashboard failed (HTTP $HTTP_CODE)"
    exit 1
fi

echo ""
echo "2️⃣  Testing POPKins via subdomain (popkins.localhost:8080)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Cookie: $COOKIE" http://popkins.localhost:8080/deployments)
if [ "$HTTP_CODE" = "200" ]; then
    echo "   ✅ POPKins subdomain accessible (HTTP $HTTP_CODE)"
else
    echo "   ❌ POPKins subdomain failed (HTTP $HTTP_CODE)"
    exit 1
fi

echo ""
echo "3️⃣  Testing POPKins via path (localhost:8080/popkins)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Cookie: $COOKIE" http://localhost:8080/popkins/deployments)
if [ "$HTTP_CODE" = "200" ]; then
    echo "   ✅ POPKins path accessible (HTTP $HTTP_CODE)"
else
    echo "   ❌ POPKins path failed (HTTP $HTTP_CODE)"
    exit 1
fi

echo ""
echo "4️⃣  Testing path rewriting (subdomain /new → /popkins/new)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Cookie: $COOKIE" http://popkins.localhost:8080/deployments/new)
if [ "$HTTP_CODE" = "200" ]; then
    echo "   ✅ Path rewriting works (HTTP $HTTP_CODE)"
else
    echo "   ❌ Path rewriting failed (HTTP $HTTP_CODE)"
    exit 1
fi

echo ""
echo "✅ All routing tests passed!"
echo ""
echo "You can now access:"
echo "  • Main Dashboard: http://localhost:8080/dashboard"
echo "  • POPKins: http://popkins.localhost:8080/deployments"
