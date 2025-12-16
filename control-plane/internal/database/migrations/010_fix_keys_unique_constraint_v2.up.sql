-- Fix unique constraint on keys table - second attempt
-- The constraint name varies between environments

-- Drop ALL possible constraint variations
DO $$
BEGIN
    -- Try dropping as constraint
    ALTER TABLE keys DROP CONSTRAINT IF EXISTS keys_org_id_namespace_id_name_key;
    ALTER TABLE keys DROP CONSTRAINT IF EXISTS keys_namespace_id_name_key;
    ALTER TABLE keys DROP CONSTRAINT IF EXISTS keys_org_id_name_key;
EXCEPTION WHEN OTHERS THEN
    -- Ignore errors
END $$;

-- Drop ALL possible index variations
DROP INDEX IF EXISTS keys_org_id_namespace_id_name_key;
DROP INDEX IF EXISTS keys_namespace_id_name_key;
DROP INDEX IF EXISTS keys_org_id_name_key;
DROP INDEX IF EXISTS idx_keys_org_namespace_name_unique;

-- Create the partial unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_keys_org_namespace_name_active 
ON keys (org_id, namespace_id, name) 
WHERE deleted_at IS NULL;

