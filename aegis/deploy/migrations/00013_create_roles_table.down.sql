-- Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS resource_attributes;
DROP TABLE IF EXISTS user_attributes;
DROP TABLE IF EXISTS policy_conditions;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS resource_types;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
