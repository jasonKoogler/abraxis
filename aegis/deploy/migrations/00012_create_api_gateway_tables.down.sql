-- Down Migration - Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
