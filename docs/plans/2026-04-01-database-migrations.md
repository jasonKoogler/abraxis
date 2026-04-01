# Database Migrations Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split shared monolith migrations into service-appropriate sets — Aegis keeps auth/RBAC/ABAC tables, Prism gets only gateway tables.

**Architecture:** Trim Aegis's 00012 to remove `api_routes`. Replace Prism's entire migration directory with 3 clean, renumbered files containing only the tables Prism needs. Verify with Docker Compose.

**Tech Stack:** PostgreSQL 16, golang-migrate

**Spec:** `docs/specs/2026-04-01-database-migrations-design.md`

---

### Task 1: Trim Aegis migration — remove api_routes

**Files:**
- Modify: `/home/jason/jdk/abraxis/aegis/deploy/migrations/00012_create_api_gateway_tables.up.sql`
- Modify: `/home/jason/jdk/abraxis/aegis/deploy/migrations/00012_create_api_gateway_tables.down.sql`

- [ ] **Step 1: Remove api_routes from the up migration**

In `/home/jason/jdk/abraxis/aegis/deploy/migrations/00012_create_api_gateway_tables.up.sql`, delete everything from `-- API Routes table for dynamic routing configuration` to the end of the file (the `CREATE TABLE api_routes` block, its indexes, comments, and trigger). Keep the `api_keys` and `audit_logs` sections intact. Also keep all the commented-out blocks (oauth_clients, etc.) as-is.

- [ ] **Step 2: Remove api_routes from the down migration**

In `/home/jason/jdk/abraxis/aegis/deploy/migrations/00012_create_api_gateway_tables.down.sql`, delete the line:
```sql
DROP TABLE IF EXISTS api_routes;
```

The file should become:
```sql
-- Down Migration - Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
```

- [ ] **Step 3: Verify Aegis migrations run cleanly**

```bash
cd /home/jason/jdk/abraxis
docker compose up postgres -d
sleep 5
docker compose up migrate-aegis
docker compose logs migrate-aegis
```

Expected: migrate-aegis exits with code 0, no errors.

- [ ] **Step 4: Verify api_routes does NOT exist in aegis_db**

```bash
docker compose exec postgres psql -U postgres -d aegis_db -c "\dt"
```

Expected: lists users, tenants, user_tenant_memberships, api_keys, audit_logs, roles, permissions, role_permissions, user_roles, resource_types, policies, policy_conditions, user_attributes, resource_attributes. NO api_routes.

- [ ] **Step 5: Stop postgres and commit**

```bash
docker compose down -v
cd /home/jason/jdk/abraxis
git add aegis/deploy/migrations/
git commit -m "fix: remove api_routes from aegis migrations (gateway-only table)"
```

---

### Task 2: Replace Prism migrations with clean gateway-only set

**Files:**
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00000_create_trigger_function_set_timestamp.up.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00000_create_trigger_function_set_timestamp.down.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00009_create_users_table.up.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00009_create_users_table.down.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00011_create_tenants_table.up.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00011_create_tenants_table.down.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00012_create_api_gateway_tables.up.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00012_create_api_gateway_tables.down.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00013_create_roles_table.up.sql`
- Delete: `/home/jason/jdk/abraxis/prism/deploy/migrations/00013_create_roles_table.down.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00001_create_trigger_function.up.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00001_create_trigger_function.down.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00002_create_tenants_table.up.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00002_create_tenants_table.down.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00003_create_gateway_tables.up.sql`
- Create: `/home/jason/jdk/abraxis/prism/deploy/migrations/00003_create_gateway_tables.down.sql`

- [ ] **Step 1: Delete all old Prism migration files**

```bash
rm /home/jason/jdk/abraxis/prism/deploy/migrations/*.sql
```

- [ ] **Step 2: Create 00001_create_trigger_function.up.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00001_create_trigger_function.up.sql`:

```sql
-- Create trigger function for updated_at
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
```

- [ ] **Step 3: Create 00001_create_trigger_function.down.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00001_create_trigger_function.down.sql`:

```sql
DROP FUNCTION IF EXISTS trigger_set_updated_at();
```

- [ ] **Step 4: Create 00002_create_tenants_table.up.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00002_create_tenants_table.up.sql`:

```sql
-- Tenants table (synced from Aegis via gRPC, no FK to users)
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    plan_type VARCHAR(50),
    max_users INTEGER,
    owner_id UUID, -- References Aegis users, no FK constraint (separate database)
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

COMMENT ON TABLE tenants IS 'Tenant data synced from Aegis. No user FK — separate database.';

CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();
```

- [ ] **Step 5: Create 00002_create_tenants_table.down.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00002_create_tenants_table.down.sql`:

```sql
DROP TABLE IF EXISTS tenants;
```

- [ ] **Step 6: Create 00003_create_gateway_tables.up.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00003_create_gateway_tables.up.sql`:

```sql
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
```

- [ ] **Step 7: Create 00003_create_gateway_tables.down.sql**

Create `/home/jason/jdk/abraxis/prism/deploy/migrations/00003_create_gateway_tables.down.sql`:

```sql
DROP TABLE IF EXISTS api_routes;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
```

- [ ] **Step 8: Verify Prism migrations run cleanly**

```bash
cd /home/jason/jdk/abraxis
docker compose up postgres -d
sleep 5
docker compose up migrate-prism
docker compose logs migrate-prism
```

Expected: migrate-prism exits with code 0.

- [ ] **Step 9: Verify Prism tables**

```bash
docker compose exec postgres psql -U postgres -d prism_db -c "\dt"
```

Expected: tenants, api_keys, audit_logs, api_routes. NO users, roles, permissions, or ABAC tables.

- [ ] **Step 10: Verify both databases together**

```bash
docker compose down -v
docker compose up postgres -d
sleep 5
docker compose up migrate-aegis migrate-prism
docker compose logs migrate-aegis migrate-prism
```

Expected: both exit 0.

```bash
echo "=== AEGIS ===" && docker compose exec postgres psql -U postgres -d aegis_db -c "\dt" && echo "=== PRISM ===" && docker compose exec postgres psql -U postgres -d prism_db -c "\dt"
```

Aegis: 14 tables (users, tenants, user_tenant_memberships, api_keys, audit_logs, roles, permissions, role_permissions, user_roles, + 5 ABAC tables).
Prism: 4 tables (tenants, api_keys, audit_logs, api_routes).

- [ ] **Step 11: Clean up and commit**

```bash
docker compose down -v
cd /home/jason/jdk/abraxis
git add prism/deploy/migrations/
git commit -m "feat: replace prism migrations with clean gateway-only set"
```

- [ ] **Step 12: Push**

```bash
git push origin main
```
