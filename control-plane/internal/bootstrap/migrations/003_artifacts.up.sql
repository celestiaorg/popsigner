-- Bootstrap deployment artifacts schema
-- Migration: 003_artifacts.up.sql

-- Table for storing deployment artifacts (genesis, configs, etc.)
CREATE TABLE deployment_artifacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    artifact_type   TEXT NOT NULL,
    content         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Each deployment can have only one artifact of each type
    UNIQUE(deployment_id, artifact_type)
);

-- Index for fetching all artifacts for a deployment
CREATE INDEX idx_artifacts_deployment ON deployment_artifacts(deployment_id);

