-- Fix the eth_address unique constraint to exclude both NULL and empty string values
-- Also excludes soft-deleted rows

-- Drop the existing unique index
DROP INDEX IF EXISTS idx_keys_org_eth_address_unique;

-- Create new unique index that excludes NULL, empty strings, and soft-deleted rows
CREATE UNIQUE INDEX idx_keys_org_eth_address_unique ON keys (org_id, eth_address)
WHERE eth_address IS NOT NULL AND eth_address != '' AND deleted_at IS NULL;

-- Also update the regular eth_address index to exclude empty strings
DROP INDEX IF EXISTS idx_keys_eth_address;
CREATE INDEX idx_keys_eth_address ON keys (eth_address)
WHERE eth_address IS NOT NULL AND eth_address != '' AND deleted_at IS NULL;

