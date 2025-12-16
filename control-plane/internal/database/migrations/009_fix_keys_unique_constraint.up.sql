-- Fix unique constraint on keys table to exclude soft-deleted records
-- This allows reusing key names after deletion

-- Drop the existing constraints (try both possible names)
ALTER TABLE keys DROP CONSTRAINT IF EXISTS keys_org_id_namespace_id_name_key;
ALTER TABLE keys DROP CONSTRAINT IF EXISTS keys_namespace_id_name_key;

-- Also drop if it was created as a unique index
DROP INDEX IF EXISTS keys_org_id_namespace_id_name_key;
DROP INDEX IF EXISTS keys_namespace_id_name_key;

-- Create a partial unique index that only applies to non-deleted keys
DROP INDEX IF EXISTS idx_keys_org_namespace_name_unique;
CREATE UNIQUE INDEX idx_keys_org_namespace_name_unique 
ON keys (org_id, namespace_id, name) 
WHERE deleted_at IS NULL;

COMMENT ON INDEX idx_keys_org_namespace_name_unique IS 'Ensures key names are unique within a namespace, but allows reusing names after soft delete';

