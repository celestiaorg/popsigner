# POPSigner Development Stack

This directory contains Docker Compose configuration for running the complete POPSigner stack locally.

## Quick Start

### 1. Prerequisites

- Docker Desktop or Docker Engine with Docker Compose
- At least 4GB RAM available for Docker
- Ports 5432, 6379, 8080, and 8200 available

### 2. Start the Stack

```bash
cd control-plane/docker
./start-dev.sh
```

Or manually:

```bash
docker compose -f docker-compose.dev.yml up -d
```

### 3. Access the Services

| Service | URL | Credentials |
|---------|-----|-------------|
| **Control Plane** | http://localhost:8080 | See **Dev Login** below |
| **PostgreSQL** | localhost:5432 | user: `popsigner`<br>pass: `popsigner`<br>db: `popsigner` |
| **Redis** | localhost:6379 | No auth |
| **OpenBao** | http://localhost:8200 | token: `dev-root-token` |

### 4. Dev Login (Bypass OAuth)

The `start-dev.sh` script automatically creates a dev user with a known session token. To login without OAuth:

**Method 1: Browser Console (Easiest)**

1. Open http://localhost:8080 OR http://popkins.localhost:8080 in your browser
2. Open Developer Tools (F12)
3. Paste this in the Console tab and press Enter:
   ```javascript
   document.cookie = "banhbao_session=dev-session-token-12345; domain=.localhost; path=/; max-age=31536000"
   ```
4. Navigate to your desired page:
   - Main Dashboard: http://localhost:8080/dashboard
   - POPKins: http://popkins.localhost:8080/deployments

**Important:** Note the `domain=.localhost` parameter with the leading dot - this shares the cookie across all localhost subdomains (localhost, popkins.localhost, etc.)

**Method 2: Manual Cookie**

Use your browser's cookie editor to add:
- **Name**: `banhbao_session`
- **Value**: `dev-session-token-12345`
- **Domain**: `.localhost` (WITH the leading dot - required for subdomain sharing)
- **Path**: `/`

**Method 3: cURL for API Testing**

```bash
# Main dashboard
curl -H "Cookie: banhbao_session=dev-session-token-12345" http://localhost:8080/dashboard

# POPKins
curl -H "Cookie: banhbao_session=dev-session-token-12345" http://popkins.localhost:8080/deployments
```

**Dev User Details:**
- Email: `dev@popsigner.local`
- Organization: `Dev Organization`
- Session expires in 1 year

### 5. Understanding Subdomain Routing

The development stack uses **Caddy** as a reverse proxy to handle subdomain routing:

- **http://localhost:8080/** → Main dashboard (requires `/popkins/` prefix for POPKins routes)
- **http://popkins.localhost:8080/** → POPKins interface (automatic path rewriting)

**Why Caddy?**
- Automatically rewrites paths: `/deployments` → `/popkins/deployments` when accessed via subdomain
- Shares cookies across subdomains correctly
- Matches production behavior exactly
- No code changes needed to templates

**Accessing POPKins:**
- Via subdomain (recommended): http://popkins.localhost:8080/deployments
- Via path: http://localhost:8080/popkins/deployments

Both work identically thanks to Caddy's path rewriting!

## Services

### 🌐 Control Plane (popsigner)

The main web application providing:
- Web dashboard
- REST API for key management
- JSON-RPC endpoint for Ethereum signing
- Deployment orchestration (OP Stack, Nitro, POPKins Bundle)

**Built from:** `control-plane/`

### 🗄️ PostgreSQL

Primary database storing:
- Users and organizations
- Keys and certificates
- Deployments and artifacts
- Audit logs

**Image:** `postgres:16-alpine`

### 🚀 Redis

Session storage and caching layer for:
- User sessions
- Rate limiting
- API usage tracking

**Image:** `redis:7-alpine`

### 🔐 OpenBao

Secure key management (Vault fork) with:
- **secp256k1 plugin** - Blockchain key signing
- **KV v2 engine** - API key storage
- **PKI engine** - mTLS certificate management

**Image:** `quay.io/openbao/openbao:2.1.0`

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Docker Network                         │
│                   (popsigner-network)                     │
│                                                           │
│  ┌─────────────┐    ┌──────────────┐    ┌────────────┐ │
│  │  PostgreSQL │◄───┤  Control     │◄───┤   Redis    │ │
│  │   :5432     │    │   Plane      │    │   :6379    │ │
│  └─────────────┘    │   :8080      │    └────────────┘ │
│                     └──────┬───────┘                    │
│                            │                             │
│                     ┌──────▼───────┐                    │
│                     │   OpenBao    │                    │
│                     │    :8200     │                    │
│                     │ + secp256k1  │                    │
│                     └──────────────┘                    │
│                                                           │
└──────────────────────────────────────────────────────────┘
              │
              │ Port mappings
              ▼
    Host: localhost:8080 (Control Plane)
          localhost:5432 (PostgreSQL)
          localhost:6379 (Redis)
          localhost:8200 (OpenBao)
```

## Development Workflow

### View Logs

```bash
# All services
docker compose -f docker-compose.dev.yml logs -f

# Specific service
docker compose -f docker-compose.dev.yml logs -f popsigner
docker compose -f docker-compose.dev.yml logs -f openbao
```

### Service Status

```bash
docker compose -f docker-compose.dev.yml ps
```

### Restart a Service

```bash
docker compose -f docker-compose.dev.yml restart popsigner
```

### Rebuild After Code Changes

```bash
# Rebuild and restart
docker compose -f docker-compose.dev.yml up -d --build popsigner
```

### Database Access

```bash
# Connect to PostgreSQL
docker exec -it popsigner-postgres psql -U popsigner -d popsigner

# Common queries
\dt                           # List tables
\d deployments                # Describe deployments table
SELECT * FROM deployments;    # View all deployments
```

### OpenBao Access

```bash
# OpenBao CLI (from host)
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=dev-root-token

# Or use Docker
docker exec -it popsigner-openbao bao status

# List secrets engines
docker exec -e VAULT_TOKEN=dev-root-token popsigner-openbao \
  bao secrets list

# List keys in secp256k1 engine
docker exec -e VAULT_TOKEN=dev-root-token popsigner-openbao \
  bao list secp256k1/keys
```

### Redis Access

```bash
docker exec -it popsigner-redis redis-cli

# Common commands
KEYS *           # List all keys
GET <key>        # Get value
FLUSHALL         # Clear all data
```

## Testing POPKins Bundle Deployment

Once the stack is running, you can test the new POPKins Bundle feature:

1. **Navigate to the UI**: http://localhost:8080
2. **Create a deployment**:
   - Go to Deployments → New Deployment
   - Select "POPKins Devnet Bundle"
   - Enter Chain ID: `42069`
   - Enter Chain Name: `my-local-devnet`
   - Click "Create Deployment"
3. **Monitor progress**: The UI will show real-time progress
4. **Download bundle**: After completion (~10-15 min), download the tar.gz
5. **Test the bundle**:
   ```bash
   tar xzf my-local-devnet-popdeployer-bundle.tar.gz
   cd my-local-devnet-popdeployer-bundle
   docker compose up -d
   curl http://localhost:9545 -X POST -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
   ```

## Cleanup

### Stop Services (Keep Data)

```bash
docker compose -f docker-compose.dev.yml down
```

### Stop Services and Remove Volumes (Fresh Start)

```bash
docker compose -f docker-compose.dev.yml down -v
```

This will delete:
- All PostgreSQL data (users, keys, deployments)
- All Redis sessions
- All OpenBao data (plugin will be re-registered on next start)

## Database Migrations

The project has two migration systems:

1. **Main migrations** (`internal/database/migrations/`) - User management, keys, certs, etc.
2. **Bootstrap migrations** (`internal/bootstrap/migrations/`) - Deployment management (deployments, artifacts, transactions)

The bootstrap migrations are **NOT automatically run** by the application. The `start-dev.sh` script handles this by:
- Checking if the `deployments` table exists
- Applying all bootstrap migrations if needed
- Adding `pop-bundle` to the `deployment_stack` enum
- Fixing any dirty migration states

### Manual Migration Application

If you need to apply bootstrap migrations manually:

```bash
cd control-plane/docker
./apply-bootstrap-migrations.sh
```

## Troubleshooting

### Port Already in Use

If you get "port already in use" errors:

```bash
# Find what's using the port
lsof -i :8080  # or :5432, :6379, :8200

# Stop the conflicting service or change the port in docker-compose.dev.yml
```

### Services Won't Start

```bash
# Check Docker resources
docker system df

# Clean up unused resources
docker system prune -a

# Check logs for specific service
docker compose -f docker-compose.dev.yml logs popsigner
```

### OpenBao Plugin Not Working

```bash
# Check plugin registration
docker exec -e VAULT_TOKEN=dev-root-token popsigner-openbao \
  bao read sys/plugins/catalog/secret/banhbaoring-secp256k1

# Re-run initialization
docker compose -f docker-compose.dev.yml up -d openbao-init
```

### Database Migration Errors

```bash
# Check if migrations ran
docker compose -f docker-compose.dev.yml logs popsigner | grep migration

# Manually connect and check schema_migrations table
docker exec -it popsigner-postgres psql -U popsigner -d popsigner
SELECT * FROM schema_migrations;
```

### Fresh Start

If everything is broken, nuke it and start over:

```bash
# Stop and remove everything
docker compose -f docker-compose.dev.yml down -v

# Remove dangling images
docker image prune -a

# Start fresh
./start-dev.sh
```

## Production Deployment

**⚠️ WARNING:** This docker-compose.dev.yml is for **DEVELOPMENT ONLY**.

For production, use Kubernetes with the operator:
- See `operator/` directory for Kubernetes deployment
- See `operator/charts/popsigner-operator/` for Helm charts

Production requirements:
- TLS/HTTPS (Let's Encrypt or custom certs)
- OAuth configuration (GitHub/Google)
- Proper OpenBao token rotation
- Database backups
- Resource limits and scaling
- Monitoring and alerting

## Environment Variables

All configuration uses the `BANHBAO_` prefix (see `internal/config/config.go` for historical context).

### Required Variables

None! The stack works out of the box with defaults.

### Optional Variables (OAuth)

To enable GitHub/Google login:

```yaml
# Add to popsigner service environment in docker-compose.dev.yml
BANHBAO_AUTH_OAUTH_GITHUB_ID: your-github-oauth-client-id
BANHBAO_AUTH_OAUTH_GITHUB_SECRET: your-github-oauth-client-secret
BANHBAO_AUTH_OAUTH_GOOGLE_ID: your-google-oauth-client-id
BANHBAO_AUTH_OAUTH_GOOGLE_SECRET: your-google-oauth-client-secret
```

Get OAuth credentials:
- GitHub: https://github.com/settings/developers
- Google: https://console.cloud.google.com/apis/credentials

## Files in This Directory

| File | Purpose |
|------|---------|
| `docker-compose.dev.yml` | Main development stack configuration |
| `docker-compose.yml` | Legacy minimal stack (postgres + redis only) |
| `Dockerfile` | Control plane server image |
| `Dockerfile.rpc-gateway` | JSON-RPC gateway image |
| `start-dev.sh` | Helper script to start the stack |
| `README.md` | This file |

## Support

For issues or questions:
- Check logs: `docker compose -f docker-compose.dev.yml logs -f`
- GitHub: https://github.com/Bidon15/popsigner/issues
- Integration Summary: See `INTEGRATION_SUMMARY.md` for POPKins Bundle details
