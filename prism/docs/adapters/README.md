# Adapters Documentation

## Overview

Adapters in our architecture serve as implementations of the ports (interfaces) defined in the `internal/ports` package. They provide concrete implementations of functionality required by the application, while allowing for flexibility and interchangeability.

The adapter pattern enables our application to interface with external systems, libraries, and frameworks without tight coupling, making the codebase more maintainable, testable, and flexible.

## Adapter Design Philosophy

Each adapter:

1. Implements one or more interfaces defined in the `ports` package
2. Encapsulates external dependencies
3. Provides configuration options through functional options pattern
4. Handles conversion between domain models and external representations
5. Contains its own error handling and logging

## Available Adapters

| Adapter                                | Purpose                                                                   | Port Interface                |
| -------------------------------------- | ------------------------------------------------------------------------- | ----------------------------- |
| [HTTP](./http.md)                      | Provides HTTP server, client, middleware, and service proxy functionality | `HTTPServer`, `HTTPClient`    |
| [Circuit Breaker](./circuitbreaker.md) | Implements circuit breaker pattern for fault tolerance                    | `CircuitBreaker`              |
| [Discovery](./discovery.md)            | Service discovery functionality (Consul, Kubernetes, etcd, local)         | `ServiceDiscovery`            |
| [Postgres](./postgres.md)              | Database access and repository implementations                            | Various repository interfaces |
| [Storage](./storage.md)                | File-based storage and configuration loading                              | Various repository interfaces |
| [AuthZ](./authz.md)                    | Authorization functionality using OPA                                     | `AuthorizationAdapter`        |
| [OAuth](./oauth.md)                    | OAuth authentication providers                                            | OAuth-related interfaces      |
| [Crypto](./crypto.md)                  | Cryptographic functions (hashing, encryption, etc.)                       | `CryptoService`               |
| [Rate Limiter](./ratelimiter.md)       | Rate limiting for API endpoints                                           | `RateLimiter`                 |
| [Session](./session.md)                | Session management                                                        | `SessionStore`                |

## Adapter Configuration

Most adapters use the functional options pattern for configuration, which allows for:

- Default configurations that work out of the box
- Selective overriding of specific options
- Clear, readable code when instantiating adapters
- Easy addition of new options without breaking existing code

Example:

```go
// Creating an adapter with default options
adapter, err := http.NewServiceProxy(
    http.WithProxyLogger(logger),
    http.WithCircuitBreaker(circuitBreaker),
)
```

## Integration with App

The application uses functional options for initializing adapters:

```go
app, err := app.NewApp(
    app.WithConfig(cfg),
    app.WithDefaultServiceDiscovery(ctx),
    app.WithDefaultCircuitBreaker(),
    app.WithDefaultServiceProxy(),
)
```

See each adapter's documentation for detailed information about its functionality, configuration options, and usage examples.
