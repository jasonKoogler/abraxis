-- Down Migration - Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS user_tenant_memberships;
DROP TABLE IF EXISTS tenants; 