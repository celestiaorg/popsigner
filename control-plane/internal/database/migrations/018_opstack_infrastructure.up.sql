-- OP Stack Infrastructure table for reusing deployed contracts per L1 chain.
-- Similar to Nitro infrastructure, this tracks OPCM and blueprint contracts
-- that can be reused across multiple L2 chain deployments.

CREATE TABLE IF NOT EXISTS opstack_infrastructure (
    l1_chain_id BIGINT PRIMARY KEY,
    
    -- OP Contracts Manager (OPCM) - the main entry point for deployments
    opcm_proxy_address VARCHAR(42) NOT NULL,
    opcm_impl_address VARCHAR(42) NOT NULL,
    
    -- Superchain contracts (shared across all chains on this L1)
    superchain_config_proxy_address VARCHAR(42) NOT NULL,
    protocol_versions_proxy_address VARCHAR(42) NOT NULL,
    
    -- Implementation addresses (blueprints + implementations)
    opcm_container_impl_address VARCHAR(42),
    opcm_deployer_impl_address VARCHAR(42),
    delayed_weth_impl_address VARCHAR(42),
    optimism_portal_impl_address VARCHAR(42),
    system_config_impl_address VARCHAR(42),
    l1_cross_domain_messenger_impl_address VARCHAR(42),
    l1_standard_bridge_impl_address VARCHAR(42),
    dispute_game_factory_impl_address VARCHAR(42),
    
    -- Metadata
    version VARCHAR(32) NOT NULL,
    create2_salt VARCHAR(66) NOT NULL,  -- 0x + 64 hex chars
    deployed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deployed_by UUID REFERENCES users(id),
    deployment_tx_hash VARCHAR(66),
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for version lookups (to check if infrastructure needs upgrade)
CREATE INDEX IF NOT EXISTS idx_opstack_infrastructure_version 
    ON opstack_infrastructure(version);

-- Trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_opstack_infrastructure_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER opstack_infrastructure_updated_at
    BEFORE UPDATE ON opstack_infrastructure
    FOR EACH ROW
    EXECUTE FUNCTION update_opstack_infrastructure_updated_at();

COMMENT ON TABLE opstack_infrastructure IS 'Tracks deployed OP Stack infrastructure per L1 chain for reuse';
COMMENT ON COLUMN opstack_infrastructure.l1_chain_id IS 'L1 chain ID where infrastructure is deployed';
COMMENT ON COLUMN opstack_infrastructure.opcm_proxy_address IS 'OP Contracts Manager proxy - main entry point for chain deployments';
COMMENT ON COLUMN opstack_infrastructure.version IS 'Artifact version (e.g., op-contracts/v2.0.0-beta.2)';
COMMENT ON COLUMN opstack_infrastructure.create2_salt IS 'CREATE2 salt used for deterministic deployment';
