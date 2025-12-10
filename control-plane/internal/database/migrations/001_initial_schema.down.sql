-- Drop triggers
DROP TRIGGER IF EXISTS update_webhooks_updated_at ON webhooks;
DROP TRIGGER IF EXISTS update_usage_metrics_updated_at ON usage_metrics;
DROP TRIGGER IF EXISTS update_keys_updated_at ON keys;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS usage_metrics;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS keys;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS namespaces;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

-- Drop extensions
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS "uuid-ossp";

