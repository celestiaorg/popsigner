-- The deployments table referenced by 016+ lives in internal/bootstrap/migrations, a tree
-- nothing runs: on a fresh database RunMigrations fails at 016 with "relation deployments
-- does not exist" and the server cannot boot. This folds the bootstrap-tree schema
-- (001_deployments + 004_deployment_org_id + 005_add_pop_bundle_stack) into the main tree,
-- guarded so it is a no-op on databases where that tree was already applied by hand.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'deployment_stack') THEN
        CREATE TYPE deployment_stack AS ENUM ('opstack', 'nitro', 'pop-bundle');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'deployment_status') THEN
        CREATE TYPE deployment_status AS ENUM ('pending', 'running', 'paused', 'completed', 'failed');
    END IF;
END $$;

ALTER TYPE deployment_stack ADD VALUE IF NOT EXISTS 'pop-bundle';

CREATE TABLE IF NOT EXISTS deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id        BIGINT NOT NULL UNIQUE,
    stack           deployment_stack NOT NULL,
    status          deployment_status NOT NULL DEFAULT 'pending',
    current_stage   TEXT,
    config          JSONB NOT NULL,
    error_message   TEXT,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A database that got the table from bootstrap 001-003 but not 004 lacks org_id.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'deployments' AND column_name = 'org_id') THEN
        ALTER TABLE deployments ADD COLUMN org_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
        ALTER TABLE deployments ALTER COLUMN org_id DROP DEFAULT;
        ALTER TABLE deployments ADD CONSTRAINT fk_deployments_org_id
            FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);
CREATE INDEX IF NOT EXISTS idx_deployments_chain_id ON deployments(chain_id);
CREATE INDEX IF NOT EXISTS idx_deployments_org_id ON deployments(org_id);
