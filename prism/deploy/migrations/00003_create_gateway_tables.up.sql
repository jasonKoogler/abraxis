-- API Keys for service-to-service authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL CHECK (key_prefix LIKE 'ak_%'),
    key_hash TEXT NOT NULL,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID, -- References Aegis users, no FK constraint (separate database)
    scopes TEXT[],
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_ip_address INET,
    last_used_ip_address INET,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT api_keys_key_prefix_unique UNIQUE (key_prefix)
);

CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE UNIQUE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX idx_api_keys_active_expiry ON api_keys(is_active, expires_at) WHERE is_active = true;

COMMENT ON TABLE api_keys IS 'API keys for service-to-service authentication with prefix lookup and hash verification';
COMMENT ON COLUMN api_keys.key_prefix IS 'Display prefix (format: ak_XXXXXXXX)';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA-256 hash of the complete API key';

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- Audit logs for gateway events
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    actor_type VARCHAR(20) NOT NULL,
    actor_id UUID,
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    ip_address INET,
    user_agent TEXT,
    event_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_type, actor_id);
CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

COMMENT ON TABLE audit_logs IS 'Gateway request and security audit logs';

-- API Routes for dynamic gateway routing
CREATE TABLE IF NOT EXISTS api_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    path_pattern VARCHAR(255) NOT NULL,
    http_method VARCHAR(20) NOT NULL,
    backend_service VARCHAR(255) NOT NULL,
    backend_path VARCHAR(255),
    requires_authentication BOOLEAN NOT NULL DEFAULT TRUE,
    required_scopes TEXT[],
    rate_limit_per_minute INTEGER,
    cache_ttl_seconds INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT api_routes_path_method_tenant_unique UNIQUE (path_pattern, http_method, tenant_id)
);

CREATE INDEX idx_api_routes_path_method ON api_routes(path_pattern, http_method);
CREATE INDEX idx_api_routes_tenant_id ON api_routes(tenant_id);
CREATE INDEX idx_api_routes_is_active ON api_routes(is_active);
CREATE INDEX idx_api_routes_backend_service ON api_routes(backend_service);

COMMENT ON TABLE api_routes IS 'Dynamic routing configuration for the API gateway';

CREATE TRIGGER update_api_routes_updated_at
    BEFORE UPDATE ON api_routes
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();
