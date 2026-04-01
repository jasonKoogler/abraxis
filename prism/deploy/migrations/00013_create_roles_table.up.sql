CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    is_system_role BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID REFERENCES tenants(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tenant_role_name_unique UNIQUE (tenant_id, name) 
);

COMMENT ON TABLE roles IS 'Defines roles for RBAC';
COMMENT ON COLUMN roles.is_system_role IS 'Whether this is a system-defined role that cannot be modified';
COMMENT ON COLUMN roles.tenant_id IS 'The tenant this role belongs to (NULL for system-wide roles)';

-- Permissions table for RBAC
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    action VARCHAR(50) NOT NULL, -- e.g., 'read', 'write', 'delete', 'admin'
    resource VARCHAR(50) NOT NULL, -- e.g., 'users', 'documents', 'settings'
    is_system_permission BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(action, resource)
);

COMMENT ON TABLE permissions IS 'Defines permissions for RBAC';
COMMENT ON COLUMN permissions.action IS 'The action this permission grants (read, write, delete, etc)';
COMMENT ON COLUMN permissions.resource IS 'The resource this permission applies to (users, documents, etc)';
COMMENT ON COLUMN permissions.is_system_permission IS 'Whether this is a system-defined permission that cannot be modified';

-- Role-Permission mapping for RBAC
CREATE TABLE IF NOT EXISTS role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(role_id, permission_id)
);

COMMENT ON TABLE role_permissions IS 'Maps roles to permissions for RBAC';

-- User-Role mapping with tenant context
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id, tenant_id)
);

COMMENT ON TABLE user_roles IS 'Maps users to roles within a tenant context';

-- Resource types for ABAC
CREATE TABLE IF NOT EXISTS resource_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE resource_types IS 'Defines resource types for ABAC';

-- Policies for ABAC
CREATE TABLE IF NOT EXISTS policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    resource_type_id UUID NOT NULL REFERENCES resource_types(id) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL, -- e.g., 'read', 'write', 'delete'
    effect VARCHAR(10) NOT NULL CHECK (effect IN ('allow', 'deny')),
    priority INTEGER NOT NULL DEFAULT 0, -- Higher priority policies are evaluated first
    tenant_id UUID REFERENCES tenants(id),
    is_system_policy BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE policies IS 'Defines access control policies for ABAC';
COMMENT ON COLUMN policies.effect IS 'Whether this policy allows or denies access';
COMMENT ON COLUMN policies.priority IS 'Higher priority policies are evaluated first';
COMMENT ON COLUMN policies.tenant_id IS 'The tenant this policy belongs to (NULL for system-wide policies)';
COMMENT ON COLUMN policies.is_system_policy IS 'Whether this is a system-defined policy that cannot be modified';

-- Policy conditions for ABAC
CREATE TABLE IF NOT EXISTS policy_conditions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    attribute VARCHAR(255) NOT NULL, -- e.g., 'user.department', 'resource.owner', 'context.time'
    operator VARCHAR(50) NOT NULL, -- e.g., 'equals', 'contains', 'greater_than'
    value TEXT NOT NULL, -- The value to compare against
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE policy_conditions IS 'Defines conditions for when policies apply';
COMMENT ON COLUMN policy_conditions.attribute IS 'The attribute to evaluate (user.department, resource.owner, etc)';
COMMENT ON COLUMN policy_conditions.operator IS 'The comparison operator (equals, contains, greater_than, etc)';
COMMENT ON COLUMN policy_conditions.value IS 'The value to compare against';

-- User attributes for ABAC
CREATE TABLE IF NOT EXISTS user_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, key)
);

COMMENT ON TABLE user_attributes IS 'Stores user attributes for ABAC decisions';

-- Resource attributes for ABAC
CREATE TABLE IF NOT EXISTS resource_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type_id UUID NOT NULL REFERENCES resource_types(id) ON DELETE CASCADE,
    resource_id UUID NOT NULL,
    key VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(resource_type_id, resource_id, key)
);

COMMENT ON TABLE resource_attributes IS 'Stores resource attributes for ABAC decisions';

-- Indexes for performance
CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX idx_user_roles_tenant_id ON user_roles(tenant_id);
CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON role_permissions(permission_id);
CREATE INDEX idx_policy_conditions_policy_id ON policy_conditions(policy_id);
CREATE INDEX idx_user_attributes_user_id ON user_attributes(user_id);
CREATE INDEX idx_resource_attributes_resource_type_id_resource_id ON resource_attributes(resource_type_id, resource_id);
CREATE INDEX idx_roles_tenant_id ON roles(tenant_id);
CREATE INDEX idx_roles_is_system_role ON roles(is_system_role);
CREATE INDEX idx_permissions_is_system_permission ON permissions(is_system_permission);
CREATE INDEX idx_policies_tenant_id ON policies(tenant_id);
CREATE INDEX idx_policies_is_system_policy ON policies(is_system_policy);