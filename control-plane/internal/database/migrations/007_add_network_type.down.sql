DROP INDEX IF EXISTS idx_keys_network_type;
ALTER TABLE keys DROP COLUMN IF EXISTS network_type;
DROP TYPE IF EXISTS network_type;

