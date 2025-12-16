-- Revert to the original unique constraint (will fail if there are duplicate names with soft deletes)
DROP INDEX IF EXISTS idx_keys_org_namespace_name_unique;

-- Restore original constraint (using the name from initial schema)
ALTER TABLE keys ADD CONSTRAINT keys_org_id_namespace_id_name_key UNIQUE (org_id, namespace_id, name);

