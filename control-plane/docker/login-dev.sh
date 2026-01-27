#!/bin/bash
# Helper script to test dev login
# This opens the browser and automatically sets the dev session cookie

set -e

URL="http://popkins.localhost:8080/deployments"

echo "🔐 Setting up dev session..."
echo ""
echo "Opening browser to: $URL"
echo ""
echo "The browser will open with instructions to set the session cookie."
echo "Copy and paste this into the browser console (F12):"
echo ""
echo "  document.cookie = \"banhbao_session=dev-session-token-12345; domain=.localhost; path=/; max-age=31536000\""
echo ""
echo "Then refresh the page to see POPKins deployments."
echo ""
echo "Available URLs:"
echo "  • Main Dashboard: http://localhost:8080/dashboard"
echo "  • POPKins: http://popkins.localhost:8080/deployments"
echo ""

# Try to open the browser
if command -v xdg-open > /dev/null; then
    xdg-open "$URL"
elif command -v open > /dev/null; then
    open "$URL"
elif command -v start > /dev/null; then
    start "$URL"
else
    echo "ℹ️  Could not auto-open browser. Please navigate to:"
    echo "   $URL"
fi

echo ""
echo "✅ Ready to test! Set the cookie and refresh the page."
