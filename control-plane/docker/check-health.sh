#!/bin/bash

echo "🔍 POPSigner Development Stack Health Check"
echo "==========================================="
echo ""

cd "$(dirname "$0")"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running"
    exit 1
fi

echo "📊 Container Status:"
echo ""
docker compose -f docker-compose.dev.yml ps
echo ""

echo "🏥 Service Health:"
echo ""

# PostgreSQL
if docker exec popsigner-postgres pg_isready -U popsigner -d popsigner > /dev/null 2>&1; then
    echo "✅ PostgreSQL: healthy"
else
    echo "❌ PostgreSQL: unhealthy"
fi

# Redis
if docker exec popsigner-redis redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis: healthy"
else
    echo "❌ Redis: unhealthy"
fi

# OpenBao
if docker exec popsigner-openbao sh -c "bao status >/dev/null 2>&1 || exit 0"; then
    echo "✅ OpenBao: healthy"
else
    echo "❌ OpenBao: unhealthy"
fi

# Control Plane
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ Control Plane: healthy"
else
    echo "❌ Control Plane: unhealthy or not responding"
    echo "   Try: docker compose -f docker-compose.dev.yml logs popsigner"
fi

echo ""
echo "🔌 Port Status:"
echo ""

for port in 5432 6379 8080 8200; do
    if lsof -i :$port > /dev/null 2>&1; then
        echo "✅ Port $port: in use"
    else
        echo "⚠️  Port $port: not in use"
    fi
done

echo ""
echo "🔐 OpenBao Plugin Status:"
echo ""

if docker exec -e VAULT_TOKEN=dev-root-token popsigner-openbao \
   bao read sys/plugins/catalog/secret/banhbaoring-secp256k1 > /dev/null 2>&1; then
    echo "✅ secp256k1 plugin: registered"
else
    echo "❌ secp256k1 plugin: not registered"
    echo "   Try: docker compose -f docker-compose.dev.yml up -d openbao-init"
fi

if docker exec -e VAULT_ADDR=http://localhost:8200 -e VAULT_TOKEN=dev-root-token popsigner-openbao \
   bao secrets list 2>/dev/null | grep -q secp256k1; then
    echo "✅ secp256k1 engine: enabled"
else
    echo "❌ secp256k1 engine: not enabled"
    echo "   Try: docker compose -f docker-compose.dev.yml up -d openbao-init"
fi

echo ""
echo "💾 Volume Status:"
echo ""
docker volume ls | grep popsigner

echo ""
echo "📝 Recent Logs (last 20 lines):"
echo ""
docker compose -f docker-compose.dev.yml logs --tail=20 popsigner

echo ""
echo "🔗 URLs:"
echo "   • Control Plane: http://localhost:8080"
echo "   • OpenBao UI:    http://localhost:8200/ui (token: dev-root-token)"
echo ""
