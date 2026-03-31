# Postgres Adapter

## Overview

The Postgres adapter provides a robust database access layer for the application, implementing the repository interfaces defined in the ports package. It enables the application to store and retrieve domain entities using PostgreSQL, with support for transactions, pagination, and efficient querying.

This adapter is essential for data persistence, implementing repositories for users, roles, permissions, tenants, API keys, audit logs, and more. It abstracts the database operations while ensuring proper data integrity, security, and performance.

## Key Components

The Postgres adapter is composed of multiple repository implementations:

### User Repository

```go
type UserRepository struct {
    db *db.PostgresPool
}
```

Implements the `ports.UserRepository` interface, handling user data operations:

- Create/update/delete user accounts
- Query users by ID, email, or provider
- Manage user roles and tenant relationships
- User authentication data access
- List users with pagination

### Role and Permission Repositories

```go
type RoleRepository struct {
    db *db.PostgresPool
}

type PermissionRepository struct {
    db *db.PostgresPool
}

type RolePermissionRepository struct {
    db *db.PostgresPool
}
```

These repositories handle role-based access control (RBAC):

- Create and manage roles
- Define and update permissions
- Associate permissions with roles
- Query role memberships

### Tenant Repository

```go
type TenantRepository struct {
    db *db.PostgresPool
}
```

Manages multi-tenancy aspects of the system:

- Create/update/delete tenants
- Manage tenant metadata
- Handle tenant access control
- Tenant-level configuration

### API Key Repository

```go
type APIKeyRepository struct {
    db *db.PostgresPool
}
```

Manages API keys for service access:

- Create/revoke API keys
- Track key usage and permissions
- API key validation
- Security tracking features

### Audit Log Repository

```go
type AuditLogRepository struct {
    db *db.PostgresPool
}
```

Provides comprehensive audit logging:

- Record user actions
- Track security events
- Support compliance requirements
- Query audit history

### API Route Repository

```go
type APIRouteRepository struct {
    db *db.PostgresPool
}
```

Manages API route configuration:

- Define service routes
- Configure endpoints
- Map requests to services
- Store route metadata

## Implementation Details

### PostgresPool

The core database connection is managed by the `PostgresPool` struct:

```go
type PostgresPool struct {
    *pgxpool.Pool
}
```

Features:

- Connection pooling for optimal resource usage
- Retry logic for connection resilience
- Configurable SSL modes
- Transaction support
- Context-aware operations

### SQL Implementation

Repository implementations use PostgreSQL-specific features:

- UUID primary keys with `gen_random_uuid()`
- JSON data type for complex data
- Full-text search capabilities
- Efficient indexing strategies
- Pagination with LIMIT/OFFSET

### Transaction Management

```go
// Transaction example
tx, err := repo.db.Begin(ctx)
if err != nil {
    return nil, fmt.Errorf("failed to begin transaction: %w", err)
}
defer func() {
    if p := recover(); p != nil {
        _ = tx.Rollback(ctx)
        panic(p)
    } else if err != nil {
        _ = tx.Rollback(ctx)
    } else {
        err = tx.Commit(ctx)
    }
}()
```

The adapter provides robust transaction handling:

- Automatic rollback on error
- Proper cleanup on panic
- Context-based timeout handling
- Error propagation

### Error Handling Strategy

Each repository method follows a consistent error handling pattern:

- Descriptive error messages
- Error wrapping with `fmt.Errorf`
- Context propagation
- SQL error translation to domain errors

## Database Schema

The repositories interact with a well-defined database schema:

### Users Table

- `id` - UUID primary key
- `email` - Unique user email
- `first_name`, `last_name` - User name fields
- `phone` - Contact information
- `status` - Account status (active, inactive, etc.)
- `last_login_date` - Security tracking
- `avatar_url` - Profile image
- `auth_provider` - Authentication source
- `password_hash` - Securely hashed password
- `created_at`, `updated_at` - Timestamps

### Many-to-Many Relationships

- `user_roles` - Links users to roles within tenants
- `role_permissions` - Associates roles with permissions
- `user_tenant_memberships` - Tracks user membership in tenants

## Configuration Options

The Postgres adapter is configured using the `PostgresConfig` struct:

```go
type PostgresConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    DB       string
    SSLMode  string
}
```

Configuration options include:

- **Host** - Database server hostname
- **Port** - Database server port
- **User** - Database username
- **Password** - Database password
- **DB** - Database name
- **SSLMode** - SSL connection mode (disable, require, verify-ca, verify-full)

## Usage Examples

### Initializing the Database Connection

```go
// Create a logger
logger := log.NewLogger()

// Define database configuration
dbConfig := &config.PostgresConfig{
    Host:     "localhost",
    Port:     "5432",
    User:     "postgres",
    Password: "password",
    DB:       "gauth",
    SSLMode:  "disable",
}

// Create database pool
ctx := context.Background()
pool, err := db.NewPostgresPool(ctx, dbConfig, logger)
if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
}

// Use the pool to create repositories
userRepo := postgres.NewUserRepository(pool)
roleRepo := postgres.NewRoleRepository(pool)
tenantRepo := postgres.NewTenantRepository(pool)
```

### Creating a User

```go
// Create a new user
user := &domain.User{
    ID:        uuid.New(),
    Email:     "user@example.com",
    FirstName: "John",
    LastName:  "Doe",
    Status:    domain.UserStatusActive,
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
}

// Insert the user into the database
ctx := context.Background()
createdUser, err := userRepo.Create(ctx, user)
if err != nil {
    log.Errorf("Failed to create user: %v", err)
    return
}

fmt.Printf("Created user with ID: %s\n", createdUser.ID)
```

### Querying with Pagination

```go
// List users with pagination
page := 0
pageSize := 10
ctx := context.Background()

users, err := userRepo.ListAll(ctx, page, pageSize)
if err != nil {
    log.Errorf("Failed to list users: %v", err)
    return
}

fmt.Printf("Found %d users\n", len(users))
for _, user := range users {
    fmt.Printf("User: %s %s (%s)\n", user.FirstName, user.LastName, user.Email)
}
```

### Using Transactions

```go
// Example of a function that uses a transaction
func CreateUserWithRoles(ctx context.Context, userRepo *postgres.UserRepository, user *domain.User,
                         tenantID uuid.UUID, roles []string) (*domain.User, error) {

    // Begin a transaction
    tx, err := userRepo.db.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Use defer for cleanup
    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback(ctx)
            panic(p)
        } else if err != nil {
            _ = tx.Rollback(ctx)
        } else {
            err = tx.Commit(ctx)
        }
    }()

    // Create the user
    createdUser, err := userRepo.Create(ctx, user)
    if err != nil {
        return nil, err
    }

    // Add roles
    for _, role := range roles {
        err = userRepo.AddRoleToTenant(ctx, createdUser.ID, tenantID, role)
        if err != nil {
            return nil, err
        }
    }

    return createdUser, nil
}
```

## Performance Considerations

For optimal performance with the Postgres adapter:

1. **Index Usage**

   - Create indexes for frequently queried columns
   - Use composite indexes for complex queries
   - Monitor index usage with PostgreSQL tools

2. **Connection Pool Sizing**

   - Configure connection pool size based on expected load
   - Monitor pool saturation
   - Consider separate pools for read/write operations

3. **Query Optimization**

   - Use prepared statements for repeated queries
   - Minimize data transfer by selecting only required columns
   - Use pagination for large result sets

4. **Transaction Scope**
   - Keep transactions as short as possible
   - Avoid mixing reads and writes in the same transaction when possible
   - Set appropriate isolation levels

## Integration with the Application

The Postgres adapter is typically initialized in the application's main setup:

```go
func SetupRepositories(ctx context.Context, cfg *config.Config, logger *log.Logger) (*App, error) {
    // Create database connection
    pgPool, err := db.NewPostgresPool(ctx, &cfg.Postgres, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to create database connection: %w", err)
    }

    // Create repositories
    userRepo := postgres.NewUserRepository(pgPool)
    roleRepo := postgres.NewRoleRepository(pgPool)
    permissionRepo := postgres.NewPermissionRepository(pgPool)
    tenantRepo := postgres.NewTenantRepository(pgPool)
    apiKeyRepo := postgres.NewAPIKeyRepository(pgPool)
    auditLogRepo := postgres.NewAuditLogRepository(pgPool)

    // Create an application instance with repositories
    app := &App{
        UserRepository:       userRepo,
        RoleRepository:       roleRepo,
        PermissionRepository: permissionRepo,
        TenantRepository:     tenantRepo,
        APIKeyRepository:     apiKeyRepo,
        AuditLogRepository:   auditLogRepo,
    }

    return app, nil
}
```

## Migrations and Schema Management

While not part of the adapter itself, the application typically manages database schema through migrations. Consider using tools like:

- golang-migrate
- Atlas
- Flyway

Migration files should be version-controlled and applied as part of deployment.
