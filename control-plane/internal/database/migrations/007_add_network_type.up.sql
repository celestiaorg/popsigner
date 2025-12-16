DO $$ BEGIN
    CREATE TYPE network_type AS ENUM ('celestia', 'evm', 'all');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

ALTER TABLE keys
ADD COLUMN IF NOT EXISTS network_type network_type NOT NULL DEFAULT 'all';

CREATE INDEX IF NOT EXISTS idx_keys_network_type ON keys (network_type)
WHERE deleted_at IS NULL;

COMMENT ON COLUMN keys.network_type IS 'Primary network: celestia, evm, or all';

