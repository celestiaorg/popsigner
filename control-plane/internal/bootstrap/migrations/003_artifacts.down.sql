-- Revert bootstrap deployment artifacts schema
-- Migration: 003_artifacts.down.sql

-- Drop index first
DROP INDEX IF EXISTS idx_artifacts_deployment;

-- Drop table
DROP TABLE IF EXISTS deployment_artifacts;

