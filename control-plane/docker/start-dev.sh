#!/bin/bash
set -e

echo "🚀 Starting POPSigner Development Stack"
echo "========================================"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Error: Docker is not running"
    echo "   Please start Docker and try again"
    exit 1
fi

# Navigate to docker directory
cd "$(dirname "$0")"

echo "📦 Building images..."
docker compose -f docker-compose.dev.yml build

echo ""
echo "🔧 Starting services..."
docker compose -f docker-compose.dev.yml up -d

echo ""
echo "⏳ Waiting for PostgreSQL to be ready..."
until docker exec popsigner-postgres pg_isready -U popsigner -d popsigner > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo " ✓"

echo ""
echo "🗄️  Applying bootstrap migrations (deployments schema)..."
# Check if deployments table exists
if docker exec popsigner-postgres psql -U popsigner -d popsigner -tAc "SELECT to_regclass('public.deployments');" | grep -q "deployments"; then
    echo "   ✓ Bootstrap migrations already applied"
else
    echo "   Applying migrations from internal/bootstrap/migrations/..."
    for migration in $(ls ../internal/bootstrap/migrations/*.up.sql | sort); do
        filename=$(basename "$migration")
        docker cp "$migration" popsigner-postgres:/tmp/migration.sql
        if docker exec popsigner-postgres psql -U popsigner -d popsigner -f /tmp/migration.sql > /dev/null 2>&1; then
            echo "   ✓ $filename"
        else
            echo "   ✗ Failed to apply $filename"
        fi
    done

    # Add pop-bundle to deployment_stack enum if not present
    if ! docker exec popsigner-postgres psql -U popsigner -d popsigner -tAc "SELECT unnest(enum_range(NULL::deployment_stack));" | grep -q "pop-bundle"; then
        echo "   Adding 'pop-bundle' to deployment_stack enum..."
        docker exec popsigner-postgres psql -U popsigner -d popsigner -c "ALTER TYPE deployment_stack ADD VALUE 'pop-bundle';" > /dev/null 2>&1
        echo "   ✓ pop-bundle added"
    fi

    # Fix migration state if needed
    echo "   Checking migration state..."
    if docker exec popsigner-postgres psql -U popsigner -d popsigner -tAc "SELECT dirty FROM schema_migrations WHERE version = 16;" | grep -q "t"; then
        docker exec popsigner-postgres psql -U popsigner -d popsigner -c "UPDATE schema_migrations SET version = 12, dirty = false WHERE version = 16;" > /dev/null 2>&1
        echo "   ✓ Reset dirty migration state"
    fi
fi

echo ""
echo "👤 Creating dev user and session..."
# Check if dev user already exists
if docker exec popsigner-postgres psql -U popsigner -d popsigner -tAc "SELECT email FROM users WHERE email = 'dev@popsigner.local';" | grep -q "dev@popsigner.local"; then
    echo "   ✓ Dev user already exists"
else
    echo "   Creating dev user with known session token..."
    docker cp seed-dev-user.sql popsigner-postgres:/tmp/seed-dev-user.sql
    docker exec popsigner-postgres psql -U popsigner -d popsigner -f /tmp/seed-dev-user.sql 2>&1 | grep -E "✅|📝|🔧|Email|Session|cookie" || true
    echo "   ✓ Dev user created"
fi

echo ""
echo "⏳ Waiting for all services to be healthy..."
echo "   This may take 30-60 seconds..."

# Restart popsigner to apply migrations
docker compose -f docker-compose.dev.yml restart popsigner > /dev/null 2>&1

# Wait for all services to be healthy
for i in {1..60}; do
    if docker compose -f docker-compose.dev.yml ps | grep -q "unhealthy\|starting"; then
        echo -n "."
        sleep 2
    else
        break
    fi
done

echo ""
echo ""
echo "✅ POPSigner Development Stack is ready!"
echo ""
echo "📊 Service URLs:"
echo "   • Control Plane: http://localhost:8080"
echo "   • PostgreSQL:    localhost:5432 (user: popsigner, pass: popsigner, db: popsigner)"
echo "   • Redis:         localhost:6379"
echo "   • OpenBao:       http://localhost:8200 (token: dev-root-token)"
echo ""
echo "🔍 Useful commands:"
echo "   • View logs:        docker compose -f docker-compose.dev.yml logs -f"
echo "   • View API logs:    docker compose -f docker-compose.dev.yml logs -f popsigner"
echo "   • Stop stack:       docker compose -f docker-compose.dev.yml down"
echo "   • Reset all data:   docker compose -f docker-compose.dev.yml down -v"
echo "   • Service status:   docker compose -f docker-compose.dev.yml ps"
echo ""
echo "📝 Notes:"
echo "   • All data persists in Docker volumes until you run 'down -v'"
echo "   • OpenBao dev server uses root token: dev-root-token"
echo "   • secp256k1 plugin is automatically registered and enabled"
echo ""
echo "🔐 Dev Login (no OAuth required):"
echo "   To bypass login, set this cookie in your browser:"
echo "   Name:  banhbao_session"
echo "   Value: dev-session-token-12345"
echo ""
echo "   Paste this in browser console on http://localhost:8080 or http://popkins.localhost:8080:"
echo '   document.cookie = "banhbao_session=dev-session-token-12345; domain=.localhost; path=/; max-age=31536000"'
echo ""
echo "   Then navigate to:"
echo "   • Main Dashboard: http://localhost:8080/dashboard"
echo "   • POPKins: http://popkins.localhost:8080/deployments"
echo ""
echo "🎉 Happy coding!"
