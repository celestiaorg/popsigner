-- Revert bootstrap deployments schema
-- Migration: 001_deployments.down.sql

-- Drop indexes first
DROP INDEX IF EXISTS idx_deployments_chain_id;
DROP INDEX IF EXISTS idx_deployments_status;

-- Drop table
DROP TABLE IF EXISTS deployments;

-- Drop custom types
DROP TYPE IF EXISTS deployment_status;
DROP TYPE IF EXISTS deployment_stack;

