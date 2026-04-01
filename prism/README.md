# Prism

API gateway service for the [Abraxis](../) platform.

Prism acts as the entry point for all client requests. It handles service routing, rate limiting, API key validation, audit logging, and service discovery. Prism validates JWT tokens issued by Aegis via its JWKS endpoint and communicates over gRPC for auth data and policy synchronization.

## Features

- **Service Proxy** -- Dynamic request routing to backend services with configurable routes
- **JWT Validation** -- Ed25519 (EdDSA) token validation via Aegis JWKS endpoint
- **API Key Management** -- Create, validate, and revoke API keys for service-to-service auth
- **Audit Logging** -- Request logging with filtering, aggregation, and CSV export
- **Rate Limiting** -- Per-endpoint rate limiting (Redis or memory-backed)
- **Service Discovery** -- etcd and Consul adapters for dynamic service registration
- **Circuit Breaker** -- Fault tolerance for backend service calls
- **Aegis Integration** -- gRPC client for auth data sync, policy sync, and permission checks

## API Endpoints

### Gateway Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/services` | List registered services |
| POST | `/api/services` | Register a service |
| GET | `/api/services/{name}` | Get service config |
| PUT | `/api/services/{name}` | Update service |
| DELETE | `/api/services/{name}` | Remove service |
| GET | `/api/routes` | List all routes (filter by `?type=public\|protected`) |
| GET | `/api/routes/{service}` | List routes for a service |
| POST | `/api/routes/{service}` | Add route to a service |

### API Keys

| Method | Path | Description |
|--------|------|-------------|
| GET | `/apikey` | List API keys (filter by `tenant_id`, `user_id`) |
| POST | `/apikey` | Create a new API key |
| POST | `/apikey/validate` | Validate a raw API key |
| GET | `/apikey/{apikeyID}` | Get API key metadata |
| PUT | `/apikey/{apikeyID}/metadata` | Update API key |
| DELETE | `/apikey/{apikeyID}` | Revoke API key |

### Audit Logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/audit` | List audit logs (filter by tenant, user, event type, resource, date range) |
| GET | `/audit/{auditID}` | Get single audit entry |
| GET | `/audit/aggregate/{groupBy}` | Aggregate by `event_type`, `actor_type`, or `tenant` |
| GET | `/audit/export` | Export audit logs as CSV |

Additional endpoints:
- `GET /swagger/*` -- Swagger UI
- `GET /health` -- Health check
- `GET /ready` -- Readiness check (gates on JWKS loaded + Aegis sync)
- `GET /metrics` -- Prometheus metrics
- `/*` -- Proxy to backend services (when service proxy is configured)

## Project Structure

```
prism/
├── cmd/                            # Application entry point
├── internal/
│   ├── app/                        # Application wiring, server, route registration
│   ├── common/                     # Shared utilities (api, db, log, redis, validator, id)
│   ├── config/                     # Environment-based configuration
│   ├── domain/                     # Domain models (AuditLog, APIKey, APIRoute, Tenant)
│   │   └── prefixid/              # Prefixed ID system (usr_, tnt_, aud_, apk_)
│   ├── features/
│   │   ├── apikey/                # API key feature (handler, server, dto, postgres adapter)
│   │   ├── audit/                 # Audit log feature (handler, server, dto, postgres adapter)
│   │   ├── auth/                  # JWT validation middleware, OPA authz adapter
│   │   ├── discovery/             # Service discovery (etcd, Consul, local adapters)
│   │   ├── gateway/               # Service proxy, routing, circuit breaker, Aegis gRPC client
│   │   ├── ratelimit/             # Rate limiting
│   │   ├── schema/                # Schema registry (planned)
│   │   └── tenant/                # Tenant repository (synced from Aegis)
│   ├── ports/                      # Interface definitions
│   └── middleware/                  # Combined auth + authz middleware
├── deploy/migrations/              # PostgreSQL migrations
├── policies/                       # OPA policy files
├── docs/                           # Generated Swagger docs
└── Makefile
```

## Configuration

Prism is configured via environment variables. See [`.env.example`](.env.example) for all options.

Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_SERVER_PORT` | `8080` | HTTP server port |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_DB` | -- | Database name |
| `REDIS_HOST` | `localhost` | Redis host |
| `AEGIS_GRPC_ADDRESS` | `localhost:9090` | Aegis gRPC endpoint |
| `AEGIS_JWKS_URL` | `http://localhost:8080/.well-known/jwks.json` | Aegis JWKS endpoint |
| `AEGIS_SYNC_ENABLED` | `true` | Enable gRPC sync with Aegis |
| `AEGIS_CACHE_TTL` | `60s` | Cache TTL for synced auth data |
| `AEGIS_JWKS_REFRESH_INTERVAL` | `5m` | How often to refresh JWKS keys |
| `SESSION_MANAGER` | `redis` | `redis` or `memory` |

## Database

Prism uses its own PostgreSQL database with gateway-specific tables:

- `tenants` -- Tenant data (synced from Aegis, no FK to users)
- `api_keys` -- API keys with enhanced prefix validation (`ak_` format)
- `audit_logs` -- Gateway request and security audit logs
- `api_routes` -- Dynamic routing configuration

Migrations are in `deploy/migrations/` and run automatically via Docker Compose.

## Development

```bash
# Build
make build

# Run tests
make test

# Regenerate Swagger docs
make swagger
```

## How Prism Connects to Aegis

1. **JWKS** -- On startup, Prism fetches Aegis's public keys from `/.well-known/jwks.json` and refreshes them periodically. These keys validate JWT tokens in incoming requests.

2. **gRPC Sync** -- When `AEGIS_SYNC_ENABLED=true`, Prism connects to Aegis's gRPC server to sync auth data (roles, permissions) and subscribe to policy updates. Synced data is cached in Redis with configurable TTL.

3. **Readiness** -- Prism's `/ready` endpoint gates on both JWKS keys being loaded and Aegis sync being established. This prevents serving traffic before auth validation is possible.
