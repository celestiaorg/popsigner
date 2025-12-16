-- Revert to previous unique index (without empty string check)
DROP INDEX IF EXISTS idx_keys_org_eth_address_unique;
DROP INDEX IF EXISTS idx_keys_eth_address;

-- Recreate original indexes
CREATE UNIQUE INDEX idx_keys_org_eth_address_unique ON keys (org_id, eth_address)
WHERE eth_address IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_keys_eth_address ON keys (eth_address)
WHERE eth_address IS NOT NULL AND deleted_at IS NULL;

