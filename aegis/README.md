# Aegis

Authentication and user management service for the [Abraxis](../) platform.

Aegis handles user registration, password and OAuth authentication, JWT token issuance (Ed25519/EdDSA), session management, multi-tenant RBAC, and API key management. It exposes a REST API for clients and a gRPC API for internal service communication with Prism.

## Features

- **JWT Authentication** -- Ed25519 (EdDSA) asymmetric signing with automatic key rotation and JWKS endpoint
- **OAuth 2.0** -- Google, Facebook provider support with pluggable provider architecture
- **User Management** -- Registration, profile updates, password hashing (Argon2)
- **Multi-Tenant RBAC** -- Roles and permissions scoped to tenants, with user-tenant memberships
- **API Key Management** -- Create, validate, and revoke service-to-service API keys
- **Session Management** -- Redis-backed sessions with configurable expiration and rotation
- **Audit Logging** -- Security event logging for authentication and authorization actions
- **gRPC Server** -- Auth data sync, policy sync, and permission checks for Prism
- **JWKS Endpoint** -- `/.well-known/jwks.json` for public key distribution

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | Public | Login with email and password |
| POST | `/auth/register` | Public | Register a new user |
| POST | `/auth/refresh` | Public | Refresh token pair |
| POST | `/auth/logout` | Protected | Invalidate session |
| GET | `/auth/{provider}` | Public | Initiate OAuth login |
| GET | `/auth/{provider}/callback` | Public | OAuth provider callback |
| GET | `/users` | Protected | List users (paginated) |
| GET | `/users/{userID}` | Protected | Get user by ID |
| POST | `/users/{userID}` | Protected | Update user profile |

Additional endpoints:
- `GET /swagger/*` -- Swagger UI
- `GET /health` -- Health check
- `GET /ready` -- Readiness check
- `GET /.well-known/jwks.json` -- JWKS public keys
- `GET /metrics` -- Prometheus metrics
- gRPC on `:9090` -- `AegisAuth` service (SyncAuthData, SyncPolicies, CheckPermission, ValidateAPIKey)

## Project Structure

```
aegis/
├── cmd/                        # Application entry point
├── api/grpc/aegispb/           # Proto definition and generated gRPC code
├── internal/
│   ├── adapters/
│   │   ├── http/               # HTTP handlers with Swagger annotations
│   │   ├── postgres/           # Repository implementations
│   │   ├── authz/              # OPA authorization adapter
│   │   ├── oauth/              # OAuth provider implementations
│   │   ├── session/            # Session store (Redis/memory)
│   │   ├── ratelimiter/        # Rate limiting (Redis/memory)
│   │   └── crypto/             # Encryption utilities
│   ├── app/                    # Application wiring and server lifecycle
│   ├── common/                 # Shared utilities (api, db, log, redis, validator)
│   ├── config/                 # Environment-based configuration
│   ├── domain/                 # Domain models (User, JWT, KeyManager, JWKS)
│   ├── grpc/                   # gRPC server implementation
│   ├── ports/                  # Interface definitions (repositories, services)
│   └── service/                # Business logic (AuthManager, UserService, etc.)
├── deploy/migrations/          # PostgreSQL migrations
├── policies/                   # OPA policy files
├── docs/                       # Generated Swagger docs
└── Makefile
```

## Configuration

Aegis is configured via environment variables. See [`.env.example`](.env.example) for all options.

Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_SERVER_PORT` | `8080` | HTTP server port |
| `GRPC_PORT` | `9090` | gRPC server port |
| `GRPC_ENABLED` | `true` | Enable gRPC server |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_DB` | -- | Database name |
| `REDIS_HOST` | `localhost` | Redis host |
| `SESSION_MANAGER` | -- | `redis` or `memory` |
| `JWT_ISSUER` | -- | JWT issuer name |
| `ACCESS_TOKEN_EXPIRATION` | `15m` | Access token lifetime |
| `REFRESH_TOKEN_EXPIRATION` | `24h` | Refresh token lifetime |
| `GOOGLE_KEY` | -- | Google OAuth client ID |
| `OAUTH_VERIFIER_STORAGE` | -- | `redis` or `memory` |

## Database

Aegis uses PostgreSQL with the following tables:

- `users` -- User accounts
- `tenants` -- Organizations/tenants
- `user_tenant_memberships` -- User-tenant relationships
- `api_keys` -- Service-to-service API keys
- `audit_logs` -- Security event logs
- `roles`, `permissions`, `role_permissions`, `user_roles` -- RBAC
- `resource_types`, `policies`, `policy_conditions`, `user_attributes`, `resource_attributes` -- ABAC (future)

Migrations are in `deploy/migrations/` and run automatically via Docker Compose.

## Development

```bash
# Build
make build

# Run tests
make test

# Regenerate Swagger docs
make swagger

# Regenerate protobuf
make proto
```
