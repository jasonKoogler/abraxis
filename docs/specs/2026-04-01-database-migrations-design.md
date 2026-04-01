# Database Migrations Cleanup Design Spec

**Date:** 2026-04-01
**Scope:** Split shared monolith migrations into service-appropriate sets for separate databases

## Motivation

Both services inherited identical migration files from the pre-split monolith. Each service's database (`aegis_db`, `prism_db`) gets all 14 tables — including ones the service never queries. Now that the services have separate databases, each should only create the tables it owns.

## Current State

Both `aegis/deploy/migrations/` and `prism/deploy/migrations/` contain the same 5 migration files creating 14 tables. The only difference is Prism's 00012 has enhanced indexes/constraints on `api_keys` and `audit_logs`.

## Target State

### Aegis Migrations

Aegis owns auth, users, tenants, RBAC, and ABAC infrastructure. Remove `api_routes` (gateway-only).

| Migration | Tables | Change |
|-----------|--------|--------|
| `00000_create_trigger_function_set_timestamp` | trigger function | No change |
| `00009_create_users_table` | users | No change |
| `00011_create_tenants_table` | tenants, user_tenant_memberships | No change |
| `00012_create_api_gateway_tables` | api_keys, audit_logs | **Remove `api_routes` table** |
| `00013_create_roles_table` | roles, permissions, role_permissions, user_roles, resource_types, policies, policy_conditions, user_attributes, resource_attributes | No change (ABAC tables kept for future) |

### Prism Migrations

Prism owns gateway routing, its own audit logs, and API key validation. Gets a clean, renumbered set with only the tables its repositories query.

| Migration | Tables | Source |
|-----------|--------|--------|
| `00001_create_trigger_function` | trigger function | Copied from shared 00000 |
| `00002_create_tenants_table` | tenants only (no user_tenant_memberships, no FK to users — `owner_id` becomes plain UUID) | Trimmed from shared 00011 |
| `00003_create_gateway_tables` | api_keys, audit_logs, api_routes | From Prism's enhanced 00012 |

**Prism does NOT get:** users, user_tenant_memberships, roles, permissions, role_permissions, user_roles, or any ABAC tables. These live in Aegis. Prism accesses auth/role data via gRPC, not database.

## Changes Required

### Aegis

**Modify** `00012_create_api_gateway_tables.up.sql` — remove the `CREATE TABLE api_routes` block and its indexes.

**Modify** `00012_create_api_gateway_tables.down.sql` — remove the `DROP TABLE IF EXISTS api_routes` line.

### Prism

**Replace** the entire `prism/deploy/migrations/` directory with 3 new migration files:

1. `00001_create_trigger_function.up.sql` / `.down.sql` — same trigger function
2. `00002_create_tenants_table.up.sql` / `.down.sql` — tenants table only, with `updated_at` trigger. No `user_tenant_memberships`.
3. `00003_create_gateway_tables.up.sql` / `.down.sql` — api_keys (with Prism's enhanced constraints), audit_logs (with Prism's additional indexes), api_routes

## Verification

- `docker compose up` — both migrate services exit successfully
- Aegis DB has: users, tenants, user_tenant_memberships, api_keys, audit_logs, roles, permissions, role_permissions, user_roles, + 5 ABAC tables. No api_routes.
- Prism DB has: tenants, api_keys, audit_logs, api_routes. No users, roles, permissions, or ABAC tables.

## What Stays Untouched

- All application code (repositories, services, handlers)
- Docker Compose configuration
- Config files
