# Docker Compose Design Spec

**Date:** 2026-04-01
**Scope:** Root-level docker-compose.yml for full dev experience with hot reload

## Goal

`docker compose up` from the monorepo root starts both services with all dependencies, auto-migrations, hot reload, and debug ports. Clone and run in under 5 minutes.

## Services

| Service | Image | Host Ports | Internal Ports | Purpose |
|---------|-------|------------|----------------|---------|
| `aegis` | Custom (air + Go) | `8080`, `9090`, `40000` | `8080`, `9090`, `40000` | Auth service HTTP, gRPC, delve |
| `prism` | Custom (air + Go) | `8081`, `40001` | `8080`, `40000` | API gateway HTTP (remapped), delve |
| `postgres` | `postgres:16-alpine` | `5432` | `5432` | Shared instance, two databases |
| `redis` | `redis:7-alpine` | `6379` | `6379` | Shared instance |
| `migrate-aegis` | `migrate/migrate` | none | none | Run-once: aegis migrations |
| `migrate-prism` | `migrate/migrate` | none | none | Run-once: prism migrations |

## Dependency Chain

```
postgres (healthy) → migrate-aegis → aegis
postgres (healthy) → migrate-prism → prism
redis (healthy) → aegis
redis (healthy) → prism
aegis (healthy) → prism
```

Prism depends on Aegis because it needs Aegis's gRPC and JWKS endpoints to start correctly.

## Postgres Init

A SQL init script mounted at `/docker-entrypoint-initdb.d/` creates both databases on first container start:

```sql
CREATE DATABASE aegis_db;
CREATE DATABASE prism_db;
```

The default `postgres` database is also available for ad-hoc queries.

## Dockerfiles

One dev Dockerfile per service at `docker/aegis.Dockerfile` and `docker/prism.Dockerfile`. Multi-purpose dev image:

- Base: `golang:1.24-alpine`
- Installs `air` (hot reload) and `dlv` (delve debugger)
- Copies `go.mod`/`go.sum`, runs `go mod download`
- Copies source code
- Entry: `air -c .air.toml`

Build context is the service directory (`aegis/` or `prism/`). Source is mounted as a volume for hot reload — the container rebuilds on file changes via air.

## Environment Variables

All env vars set directly in the compose file (no external `.env` file dependency). Key values:

**Shared:**
- `ENV=development`, `LOG_LEVEL=debug`
- `POSTGRES_USER=postgres`, `POSTGRES_PASSWORD=postgres`
- `REDIS_HOST=redis`, `REDIS_PORT=6379`, `REDIS_PASSWORD=redis`, `REDIS_USERNAME=default`
- `HTTP_SERVER_PORT=8080`
- `OAUTH_VERIFIER_STORAGE=redis`
- OAuth providers use placeholder values (users replace with their own)

**Aegis-specific:**
- `POSTGRES_HOST=postgres`, `POSTGRES_DB=aegis_db`
- `GRPC_PORT=9090`, `GRPC_ENABLED=true`

**Prism-specific:**
- `POSTGRES_HOST=postgres`, `POSTGRES_DB=prism_db`
- `AEGIS_GRPC_ADDRESS=aegis:9090`
- `AEGIS_JWKS_URL=http://aegis:8080/.well-known/jwks.json`
- `AEGIS_SYNC_ENABLED=true`

Note: Service hostnames (`aegis`, `redis`, `postgres`) resolve via Docker's internal DNS.

## Hot Reload

Both service containers mount their source directory as a volume:
```yaml
volumes:
  - ./aegis:/app
```

Air watches for `.go` file changes and rebuilds. The `.air.toml` files already exist in both services and are configured correctly.

## Health Checks

- **postgres:** `pg_isready`
- **redis:** `redis-cli ping`
- **aegis:** `wget -q --spider http://localhost:8080/health` (wget available in alpine, curl is not)
- **prism:** `wget -q --spider http://localhost:8080/health`

## Volumes

- `pg-data` — persistent Postgres data
- `redis-data` — persistent Redis data

## Files to Create

```
abraxis/
├── docker-compose.yml
└── docker/
    ├── aegis.Dockerfile
    ├── prism.Dockerfile
    └── init-db.sql
```

## What Stays Untouched

- Existing per-service Docker setups in `deploy/docker/` (kept for reference)
- `.air.toml` files (already correct)
- Application code
- Migration files in `deploy/migrations/`
