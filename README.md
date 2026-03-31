# Abraxis

Auth service + API gateway platform.

## Services

| Service | Description | Port |
|---------|-------------|------|
| **Aegis** | Authentication, JWT (Ed25519/EdDSA), OAuth, RBAC, user management | :8080 (HTTP), :9090 (gRPC) |
| **Prism** | API gateway, service routing, rate limiting, audit logging, API keys | :8080 (HTTP) |

## Shared Libraries

| Library | Description |
|---------|-------------|
| **Authz** | OPA-based authorization with caching, RBAC, and middleware |

## Quick Start

    # Build both services
    make build-all

    # Run tests
    make test-all

    # Regenerate swagger docs
    make swagger-all

    # Regenerate protobuf
    make proto

## Architecture

Aegis handles authentication and issues JWT tokens signed with Ed25519. Prism validates tokens via JWKS and proxies requests to backend services. Both services communicate over gRPC for auth data sync, policy sync, and permission checks. Authz provides OPA policy evaluation used by both services.

## API Documentation

After building, swagger UI is available at:
- Aegis: `http://localhost:8080/swagger/`
- Prism: `http://localhost:8080/swagger/`
