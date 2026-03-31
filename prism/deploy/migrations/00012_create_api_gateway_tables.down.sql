-- Down Migration - Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS api_routes;
-- DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS audit_logs;
-- DROP TABLE IF EXISTS rate_limits;
-- DROP TABLE IF EXISTS oauth_access_tokens;
-- DROP TABLE IF EXISTS oauth_clients;
DROP TABLE IF EXISTS api_keys; 