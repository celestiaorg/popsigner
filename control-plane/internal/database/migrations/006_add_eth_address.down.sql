-- Remove Ethereum address column and indexes

DROP INDEX IF EXISTS idx_keys_org_eth_address_unique;
DROP INDEX IF EXISTS idx_keys_eth_address;
ALTER TABLE keys DROP COLUMN IF EXISTS eth_address;

