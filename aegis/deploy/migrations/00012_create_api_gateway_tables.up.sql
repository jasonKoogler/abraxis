-- API Keys for service-to-service authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(8) NOT NULL,
    key_hash TEXT NOT NULL,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    scopes TEXT[], -- Array of permission scopes this key has access to
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
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);

COMMENT ON TABLE api_keys IS 'API keys for service-to-service authentication';
COMMENT ON COLUMN api_keys.key_prefix IS 'First few characters of the API key for lookup';
COMMENT ON COLUMN api_keys.key_hash IS 'Hashed value of the full API key';
COMMENT ON COLUMN api_keys.scopes IS 'Permission scopes this API key has access to';

-- Create trigger for updating updated_at
CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- -- OAuth clients for third-party application integration
-- CREATE TABLE IF NOT EXISTS oauth_clients (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     client_id VARCHAR(100) NOT NULL,
--     client_secret_hash TEXT NOT NULL,
--     name VARCHAR(255) NOT NULL,
--     description TEXT,
--     tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
--     redirect_uris TEXT[] NOT NULL,
--     allowed_grant_types TEXT[] NOT NULL,
--     allowed_scopes TEXT[] NOT NULL,
--     pkce_required BOOLEAN NOT NULL DEFAULT TRUE,
--     is_confidential BOOLEAN NOT NULL DEFAULT TRUE,
--     is_first_party BOOLEAN NOT NULL DEFAULT FALSE,
--     logo_url TEXT,
--     website_url TEXT,
--     privacy_policy_url TEXT,
--     terms_of_service_url TEXT,
--     is_active BOOLEAN NOT NULL DEFAULT TRUE,
--     created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
--     CONSTRAINT oauth_clients_client_id_unique UNIQUE (client_id)
-- );

-- CREATE INDEX idx_oauth_clients_tenant_id ON oauth_clients(tenant_id);
-- CREATE INDEX idx_oauth_clients_is_active ON oauth_clients(is_active);

-- COMMENT ON TABLE oauth_clients IS 'OAuth clients for third-party application integration';
-- COMMENT ON COLUMN oauth_clients.client_id IS 'Public identifier for the OAuth client';
-- COMMENT ON COLUMN oauth_clients.client_secret_hash IS 'Hashed client secret';
-- COMMENT ON COLUMN oauth_clients.redirect_uris IS 'Allowed redirect URIs for this client';
-- COMMENT ON COLUMN oauth_clients.allowed_grant_types IS 'OAuth grant types this client can use';
-- COMMENT ON COLUMN oauth_clients.allowed_scopes IS 'OAuth scopes this client can request';
-- COMMENT ON COLUMN oauth_clients.pkce_required IS 'Whether PKCE is required for this client';
-- COMMENT ON COLUMN oauth_clients.is_confidential IS 'Whether this is a confidential client that can securely store secrets';
-- COMMENT ON COLUMN oauth_clients.is_first_party IS 'Whether this is a first-party application owned by the same organization';

-- -- Create trigger for updating updated_at
-- CREATE TRIGGER update_oauth_clients_updated_at
--     BEFORE UPDATE ON oauth_clients
--     FOR EACH ROW
--     EXECUTE FUNCTION trigger_set_updated_at();

-- -- OAuth access tokens
-- CREATE TABLE IF NOT EXISTS oauth_access_tokens (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     token_hash TEXT NOT NULL,
--     client_id UUID NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
--     user_id UUID REFERENCES users(id) ON DELETE CASCADE,
--     scopes TEXT[] NOT NULL,
--     expires_at TIMESTAMPTZ NOT NULL,
--     issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     refresh_token_hash TEXT,
--     refresh_token_expires_at TIMESTAMPTZ,
--     is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
--     created_ip_address INET,
--     last_used_ip_address INET,
--     last_used_at TIMESTAMPTZ,
--     created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
--     CONSTRAINT oauth_access_tokens_token_hash_unique UNIQUE (token_hash)
-- );

-- CREATE INDEX idx_oauth_access_tokens_client_id ON oauth_access_tokens(client_id);
-- CREATE INDEX idx_oauth_access_tokens_user_id ON oauth_access_tokens(user_id);
-- CREATE INDEX idx_oauth_access_tokens_refresh_token_hash ON oauth_access_tokens(refresh_token_hash);
-- CREATE INDEX idx_oauth_access_tokens_expires_at ON oauth_access_tokens(expires_at);
-- CREATE INDEX idx_oauth_access_tokens_is_revoked ON oauth_access_tokens(is_revoked);

-- COMMENT ON TABLE oauth_access_tokens IS 'OAuth access tokens for API access';
-- COMMENT ON COLUMN oauth_access_tokens.token_hash IS 'Hashed value of the access token';
-- COMMENT ON COLUMN oauth_access_tokens.refresh_token_hash IS 'Hashed value of the refresh token';
-- COMMENT ON COLUMN oauth_access_tokens.scopes IS 'Scopes granted to this token';

-- -- Create trigger for updating updated_at
-- CREATE TRIGGER update_oauth_access_tokens_updated_at
--     BEFORE UPDATE ON oauth_access_tokens
--     FOR EACH ROW
--     EXECUTE FUNCTION trigger_set_updated_at();

-- -- Rate limiting table
-- CREATE TABLE IF NOT EXISTS rate_limits (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     resource_type VARCHAR(50) NOT NULL, -- 'ip', 'user', 'tenant', 'api_key'
--     resource_id VARCHAR(255) NOT NULL, -- IP address, user ID, tenant ID, or API key ID
--     endpoint VARCHAR(255) NOT NULL, -- API endpoint or '*' for all endpoints
--     requests_count INTEGER NOT NULL DEFAULT 0,
--     last_request_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     reset_at TIMESTAMPTZ NOT NULL,
--     created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
--     CONSTRAINT rate_limits_resource_endpoint_unique UNIQUE (resource_type, resource_id, endpoint)
-- );

-- CREATE INDEX idx_rate_limits_resource ON rate_limits(resource_type, resource_id);
-- CREATE INDEX idx_rate_limits_reset_at ON rate_limits(reset_at);

-- COMMENT ON TABLE rate_limits IS 'Tracks rate limiting for API endpoints';
-- COMMENT ON COLUMN rate_limits.resource_type IS 'Type of resource being rate limited (ip, user, tenant, api_key)';
-- COMMENT ON COLUMN rate_limits.resource_id IS 'Identifier for the resource being rate limited';
-- COMMENT ON COLUMN rate_limits.endpoint IS 'API endpoint being rate limited, or * for all endpoints';
-- COMMENT ON COLUMN rate_limits.reset_at IS 'When the rate limit window resets';

-- -- Create trigger for updating updated_at
-- CREATE TRIGGER update_rate_limits_updated_at
--     BEFORE UPDATE ON rate_limits
--     FOR EACH ROW
--     EXECUTE FUNCTION trigger_set_updated_at();

-- Audit logs for security events
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    actor_type VARCHAR(20) NOT NULL, -- 'user', 'system', 'api_key', 'oauth_client'
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
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

COMMENT ON TABLE audit_logs IS 'Security audit logs for authentication and authorization events';
COMMENT ON COLUMN audit_logs.event_type IS 'Type of security event (login, logout, permission_change, etc)';
COMMENT ON COLUMN audit_logs.actor_type IS 'Type of actor that performed the action';
COMMENT ON COLUMN audit_logs.actor_id IS 'ID of the actor that performed the action';
COMMENT ON COLUMN audit_logs.resource_type IS 'Type of resource that was affected';
COMMENT ON COLUMN audit_logs.resource_id IS 'ID of the resource that was affected';
COMMENT ON COLUMN audit_logs.event_data IS 'Additional data about the event in JSON format';

-- -- Sessions table for tracking user sessions
-- CREATE TABLE IF NOT EXISTS sessions (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
--     token_hash TEXT NOT NULL,
--     refresh_token_hash TEXT,
--     expires_at TIMESTAMPTZ NOT NULL,
--     ip_address INET,
--     user_agent TEXT,
--     device_info JSONB,
--     last_active_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
--     created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
--     CONSTRAINT sessions_token_hash_unique UNIQUE (token_hash)
-- );

-- CREATE INDEX idx_sessions_user_id ON sessions(user_id);
-- CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
-- CREATE INDEX idx_sessions_is_revoked ON sessions(is_revoked);
-- CREATE INDEX idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);

-- COMMENT ON TABLE sessions IS 'User sessions for web and mobile applications';
-- COMMENT ON COLUMN sessions.token_hash IS 'Hashed session token';
-- COMMENT ON COLUMN sessions.refresh_token_hash IS 'Hashed refresh token';
-- COMMENT ON COLUMN sessions.device_info IS 'Information about the device used for this session';

-- -- Create trigger for updating updated_at
-- CREATE TRIGGER update_sessions_updated_at
--     BEFORE UPDATE ON sessions
--     FOR EACH ROW
--     EXECUTE FUNCTION trigger_set_updated_at();

-- API Routes table for dynamic routing configuration
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

COMMENT ON TABLE api_routes IS 'Configuration for API gateway routes';
COMMENT ON COLUMN api_routes.path_pattern IS 'URL path pattern for this route';
COMMENT ON COLUMN api_routes.http_method IS 'HTTP method for this route';
COMMENT ON COLUMN api_routes.backend_service IS 'Backend service to route requests to';
COMMENT ON COLUMN api_routes.backend_path IS 'Path to use when forwarding to the backend service';
COMMENT ON COLUMN api_routes.requires_authentication IS 'Whether authentication is required for this route';
COMMENT ON COLUMN api_routes.required_scopes IS 'OAuth scopes required to access this route';
COMMENT ON COLUMN api_routes.rate_limit_per_minute IS 'Rate limit for this specific route';

-- Create trigger for updating updated_at
CREATE TRIGGER update_api_routes_updated_at
    BEFORE UPDATE ON api_routes
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at(); 