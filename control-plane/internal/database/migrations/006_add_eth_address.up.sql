-- Add Ethereum address column to keys table
-- This stores the EIP-55 checksummed Ethereum address (42 chars with 0x prefix)

ALTER TABLE keys
ADD COLUMN eth_address VARCHAR(42);

-- Create index for efficient lookups by Ethereum address
-- Used by JSON-RPC gateway to find keys by 'from' address
CREATE INDEX idx_keys_eth_address ON keys (eth_address)
WHERE eth_address IS NOT NULL AND deleted_at IS NULL;

-- Create unique constraint to prevent duplicate Ethereum addresses within an org
-- Different orgs can have the same address (multi-tenant)
CREATE UNIQUE INDEX idx_keys_org_eth_address_unique ON keys (org_id, eth_address)
WHERE eth_address IS NOT NULL AND deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN keys.eth_address IS 'EIP-55 checksummed Ethereum address derived from public key (0x prefixed, 42 chars)';

