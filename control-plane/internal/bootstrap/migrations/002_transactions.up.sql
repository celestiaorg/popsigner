-- Bootstrap deployment transactions schema
-- Migration: 002_transactions.up.sql

-- Table for tracking on-chain transactions per deployment
CREATE TABLE deployment_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    stage           TEXT NOT NULL,
    tx_hash         TEXT NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Prevent duplicate transaction records for the same deployment
    UNIQUE(deployment_id, tx_hash)
);

-- Index for fetching all transactions for a deployment
CREATE INDEX idx_txns_deployment ON deployment_transactions(deployment_id);

