#!/bin/bash
# Apply bootstrap migrations manually to PostgreSQL
# These migrations are in internal/bootstrap/migrations/ and not run automatically

set -e

echo "🗄️  Applying Bootstrap Migrations to PostgreSQL..."
echo "=================================================="

# Wait for postgres to be ready
until docker exec popsigner-postgres pg_isready -U popsigner -d popsigner > /dev/null 2>&1; do
    echo "Waiting for PostgreSQL to be ready..."
    sleep 2
done

echo "✓ PostgreSQL is ready"
echo ""

# Apply each migration in order
MIGRATIONS_DIR="../internal/bootstrap/migrations"

for migration in $(ls $MIGRATIONS_DIR/*.up.sql | sort); do
    filename=$(basename "$migration")
    echo "Applying $filename..."

    # Copy migration to container and execute
    docker cp "$migration" popsigner-postgres:/tmp/migration.sql
    docker exec popsigner-postgres psql -U popsigner -d popsigner -f /tmp/migration.sql

    if [ $? -eq 0 ]; then
        echo "  ✓ $filename applied successfully"
    else
        echo "  ✗ Failed to apply $filename"
        exit 1
    fi
    echo ""
done

echo "=================================================="
echo "✅ All bootstrap migrations applied successfully!"
echo ""
echo "You can now start the popsigner service:"
echo "  docker compose -f docker-compose.dev.yml up -d popsigner"
