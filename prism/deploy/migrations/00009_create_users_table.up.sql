-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    phone VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_login_date TIMESTAMPTZ,
    avatar_url TEXT,
    auth_provider VARCHAR(50) DEFAULT 'password',
    provider_access_token TEXT,
    provider_refresh_token TEXT,
    provider_token_expiry TIMESTAMPTZ,
    provider_user_id VARCHAR(255),
    password_hash TEXT,
    last_password_reset_date TIMESTAMPTZ,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_type VARCHAR(20),
    mfa_secret TEXT,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    account_locked_until TIMESTAMPTZ,
    preferences JSONB DEFAULT '{}'::JSONB,
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT status_check 
        CHECK (status IN ('active', 'inactive', 'locked', 'deleted')),
    CONSTRAINT mfa_type_check
        CHECK (mfa_type IN ('app', 'sms', 'email', NULL)),
    CONSTRAINT auth_provider_check
        CHECK (auth_provider IN ('password', 'google', 'github', 'microsoft', 'apple', 'facebook', 'twitter', 'saml', 'oidc'))
);

-- Create indexes for common queries
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_auth_provider ON users(auth_provider);
CREATE INDEX idx_users_provider_user_id ON users(provider_user_id);

-- Add comments for documentation
COMMENT ON TABLE users IS 'Stores user account information';
COMMENT ON COLUMN users.id IS 'Unique identifier for the user';
COMMENT ON COLUMN users.email IS 'User''s email address (unique)';
COMMENT ON COLUMN users.status IS 'Account status: active, inactive, locked, or deleted';
COMMENT ON COLUMN users.auth_provider IS 'Authentication provider (password, google, etc)';
COMMENT ON COLUMN users.password_hash IS 'Hashed password for users using password authentication';
COMMENT ON COLUMN users.mfa_enabled IS 'Whether multi-factor authentication is enabled';
COMMENT ON COLUMN users.mfa_type IS 'Type of MFA: app (authenticator), sms, or email';
COMMENT ON COLUMN users.provider_refresh_token IS 'Refresh token for OAuth providers';
COMMENT ON COLUMN users.provider_token_expiry IS 'Expiration time for OAuth access tokens';
COMMENT ON COLUMN users.preferences IS 'User preferences stored as JSON (UI settings, notifications, etc)';
COMMENT ON COLUMN users.metadata IS 'Additional user metadata stored as JSON';

-- Create trigger for updating updated_at
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- Create function to reset failed login attempts when password is changed
CREATE OR REPLACE FUNCTION trigger_set_last_password_reset_date()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.password_hash IS DISTINCT FROM NEW.password_hash THEN
        NEW.last_password_reset_date := NOW();
        NEW.failed_login_attempts := 0;
        NEW.account_locked_until := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_last_password_reset_date
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_last_password_reset_date();
