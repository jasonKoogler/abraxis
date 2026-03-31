-- Create tenants table with reference to users
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    plan_type VARCHAR(50),
    max_users INTEGER,
    owner_id UUID REFERENCES users(id),
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

COMMENT ON TABLE tenants IS 'Stores tenant/organization information for multi-tenant support';
COMMENT ON COLUMN tenants.owner_id IS 'The user who owns/created this tenant';

-- Create trigger for updating updated_at on tenants
CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- Create user_tenant_memberships table for many-to-many relationship
CREATE TABLE IF NOT EXISTS user_tenant_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    is_tenant_admin BOOLEAN NOT NULL DEFAULT FALSE,
    is_default_tenant BOOLEAN NOT NULL DEFAULT FALSE,
    invitation_email VARCHAR(255),
    invitation_token TEXT,
    invitation_expires_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT user_tenant_unique UNIQUE (user_id, tenant_id),
    CONSTRAINT membership_status_check 
        CHECK (status IN ('active', 'inactive', 'invited', 'suspended', 'rejected'))
);

CREATE INDEX idx_memberships_user_id ON user_tenant_memberships(user_id);
CREATE INDEX idx_memberships_tenant_id ON user_tenant_memberships(tenant_id);
CREATE INDEX idx_memberships_status ON user_tenant_memberships(status);
CREATE INDEX idx_memberships_is_tenant_admin ON user_tenant_memberships(is_tenant_admin);
CREATE INDEX idx_memberships_is_default_tenant ON user_tenant_memberships(is_default_tenant);
CREATE INDEX idx_memberships_invitation_email ON user_tenant_memberships(invitation_email);
CREATE INDEX idx_memberships_invitation_token ON user_tenant_memberships(invitation_token);

COMMENT ON TABLE user_tenant_memberships IS 'Maps users to tenants they belong to';
COMMENT ON COLUMN user_tenant_memberships.is_tenant_admin IS 'Whether the user is an admin for this tenant';
COMMENT ON COLUMN user_tenant_memberships.is_default_tenant IS 'Whether this is the user''s default/primary tenant';
COMMENT ON COLUMN user_tenant_memberships.invitation_email IS 'Email address the invitation was sent to (for users not yet registered)';
COMMENT ON COLUMN user_tenant_memberships.invitation_token IS 'Token for accepting an invitation';
COMMENT ON COLUMN user_tenant_memberships.invitation_expires_at IS 'When the invitation expires';

-- Create trigger for updating updated_at
CREATE TRIGGER update_memberships_updated_at
    BEFORE UPDATE ON user_tenant_memberships
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- Ensure each user has at most one default tenant
CREATE OR REPLACE FUNCTION ensure_single_default_tenant()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_default_tenant = TRUE THEN
        UPDATE user_tenant_memberships
        SET is_default_tenant = FALSE
        WHERE user_id = NEW.user_id
        AND id != NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_single_default_tenant
    AFTER INSERT OR UPDATE ON user_tenant_memberships
    FOR EACH ROW
    WHEN (NEW.is_default_tenant = TRUE)
    EXECUTE FUNCTION ensure_single_default_tenant(); 