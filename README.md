# Abraxis

Auth service + API gateway platform built with Go.

Abraxis provides two interlocking microservices: **Aegis** handles authentication, user management, and authorization; **Prism** acts as an API gateway with service routing, rate limiting, and audit logging. Both share a common authorization library (**Authz**) powered by Open Policy Agent.

## Services

| Service | Description | Ports |
|---------|-------------|-------|
| [Aegis](aegis/) | Authentication, JWT (Ed25519/EdDSA), OAuth, RBAC, user management | `:8080` HTTP, `:9090` gRPC |
| [Prism](prism/) | API gateway, service routing, rate limiting, audit logging, API keys | `:8080` HTTP |

## Shared Libraries

| Library | Description |
|---------|-------------|
| [Authz](authz/) | OPA-based authorization with multi-tier caching, RBAC, and HTTP middleware |

## Architecture

```
                    +-----------+
  Clients --------> |   Prism   | --------> Backend Services
                    | (Gateway) |
                    +-----+-----+
                          |
                    gRPC / JWKS
                          |
                    +-----+-----+
                    |   Aegis   |
                    |  (Auth)   |
                    +-----------+
```

- **Aegis** issues JWT tokens signed with Ed25519 (EdDSA). It owns users, tenants, roles, permissions, sessions, and API keys.
- **Prism** validates tokens via Aegis's JWKS endpoint and proxies requests to backend services. It owns gateway routing, audit logging, and its own API key validation.
- Both services communicate over **gRPC** for auth data sync, policy sync, and permission checks.
- Both use **OPA** (via the Authz library) for fine-grained authorization decisions.
- Each service has its own **PostgreSQL database** and shares a **Redis** instance.

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for local development)

### Run with Docker Compose

```bash
git clone git@github.com:jasonKoogler/abraxis.git
cd abraxis
docker compose up --build
```

This starts:

| Service | Host Port | Purpose |
|---------|-----------|---------|
| Postgres | `:5432` | `aegis_db` + `prism_db` auto-created |
| Redis | `:6379` | Shared cache |
| Aegis | `:8080` | HTTP API |
| Aegis | `:9090` | gRPC |
| Aegis | `:40000` | Delve debugger |
| Prism | `:8081` | HTTP API |
| Prism | `:40001` | Delve debugger |

Migrations run automatically. Hot reload via [air](https://github.com/air-verse/air) rebuilds on file changes.

### Build locally

```bash
make build-all    # Build both services
make test-all     # Run all tests
make swagger-all  # Regenerate Swagger docs
make proto        # Regenerate protobuf code
make clean        # Clean build artifacts
```

## API Documentation

Swagger UI is available when services are running:

- **Aegis:** http://localhost:8080/swagger/
- **Prism:** http://localhost:8081/swagger/

## Project Structure

```
abraxis/
├── aegis/              # Auth service (Go module)
│   ├── cmd/            # Entry point
│   ├── internal/       # App logic, adapters, domain, services
│   ├── api/grpc/       # Proto definitions and generated code
│   ├── deploy/         # Migrations
│   └── docs/           # Generated Swagger
├── prism/              # API gateway (Go module)
│   ├── cmd/            # Entry point
│   ├── internal/       # Features (audit, apikey, gateway, auth, discovery)
│   ├── deploy/         # Migrations
│   └── docs/           # Generated Swagger
├── authz/              # Shared OPA authorization library (Go module)
├── docker/             # Dev Dockerfiles and DB init
├── docker-compose.yml  # Full dev stack
├── go.work             # Go workspace linking all modules
└── Makefile            # Root orchestration
```

## Tech Stack

| Category | Technology |
|----------|-----------|
| Language | Go 1.24 |
| HTTP Router | chi/v5 |
| Auth | Ed25519 JWT (EdDSA), OAuth 2.0 |
| Authorization | Open Policy Agent via Authz |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| RPC | gRPC + Protocol Buffers |
| API Docs | Swagger (swaggo) |
| Service Discovery | etcd, Consul |
| Dev Tools | air (hot reload), delve (debugger) |
