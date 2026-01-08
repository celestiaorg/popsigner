-- Rollback OP Stack infrastructure table

DROP TRIGGER IF EXISTS opstack_infrastructure_updated_at ON opstack_infrastructure;
DROP FUNCTION IF EXISTS update_opstack_infrastructure_updated_at();
DROP INDEX IF EXISTS idx_opstack_infrastructure_version;
DROP TABLE IF EXISTS opstack_infrastructure;
