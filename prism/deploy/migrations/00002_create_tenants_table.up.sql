-- Tenants table (synced from Aegis via gRPC, no FK to users)
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    plan_type VARCHAR(50),
    max_users INTEGER,
    owner_id UUID, -- References Aegis users, no FK constraint (separate database)
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT tenants_name_unique UNIQUE (name),
    CONSTRAINT tenants_domain_unique UNIQUE (domain),
    CONSTRAINT tenants_status_check
        CHECK (status IN ('active', 'inactive', 'suspended', 'deleted'))
);

CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_domain ON tenants(domain);
CREATE INDEX idx_tenants_owner_id ON tenants(owner_id);

COMMENT ON TABLE tenants IS 'Tenant data synced from Aegis. No user FK — separate database.';

CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();
