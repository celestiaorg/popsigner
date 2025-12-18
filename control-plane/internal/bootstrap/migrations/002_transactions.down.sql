-- Revert bootstrap deployment transactions schema
-- Migration: 002_transactions.down.sql

-- Drop index first
DROP INDEX IF EXISTS idx_txns_deployment;

-- Drop table
DROP TABLE IF EXISTS deployment_transactions;

