-- Bootstrap deployments schema
-- Migration: 001_deployments.up.sql

-- Create custom types for deployment tracking
CREATE TYPE deployment_stack AS ENUM ('opstack', 'nitro');
CREATE TYPE deployment_status AS ENUM ('pending', 'running', 'paused', 'completed', 'failed');

-- Core deployments table for tracking chain deployments
CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id        BIGINT NOT NULL UNIQUE,
    stack           deployment_stack NOT NULL,
    status          deployment_status NOT NULL DEFAULT 'pending',
    current_stage   TEXT,
    config          JSONB NOT NULL,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for filtering by status (common for finding active deployments)
CREATE INDEX idx_deployments_status ON deployments(status);

-- Index for looking up by chain_id (common for resumption checks)
CREATE INDEX idx_deployments_chain_id ON deployments(chain_id);

