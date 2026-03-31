# Storage Adapter

## Overview

The Storage adapter provides file-based persistence mechanisms for configuration and data storage in the application. It enables the system to persist and retrieve data from the local filesystem in a thread-safe manner, handling serialization, deserialization, and file management operations.

Currently, the primary implementation is the `ServiceRepository`, which manages service configurations used by the HTTP service proxy. This adapter allows dynamic service configurations to persist across application restarts.

## Key Components

### ServiceRepository

The `ServiceRepository` provides JSON-based file storage for service configurations:

- Thread-safe access to the configuration file
- Automatic directory creation if needed
- JSON serialization/deserialization
- Data transfer object (DTO) mapping

```go
// ServiceRepository provides persistence for service configurations
type ServiceRepository struct {
    filePath string
    mu       sync.RWMutex
}
```

## Implementation Details

### File Storage

The adapter uses standard Go file operations with proper locking mechanisms:

- Read-write mutex to prevent concurrent file access
- JSON encoding/decoding for data serialization
- File path management for configuration storage

### Data Transfer Objects

The adapter uses DTOs to handle the translation between domain objects and their serialized form:

```go
// serviceDTO is a data transfer object for service configuration
type serviceDTO struct {
    Name            string   `json:"name"`
    URL             string   `json:"url"`
    HealthCheckPath string   `json:"health_check_path"`
    RequiresAuth    bool     `json:"requires_auth"`
    Timeout         string   `json:"timeout"`
    RetryCount      int      `json:"retry_count"`
    AllowedRoles    []string `json:"allowed_roles"`
    AllowedMethods  []string `json:"allowed_methods"`
}
```

This pattern:

- Isolates serialization concerns from domain logic
- Handles type conversions (e.g., `time.Duration` to string)
- Provides a stable storage format even if domain models change

## Methods and Operations

### NewServiceRepository

Creates a new service repository with a specified configuration directory:

```go
func NewServiceRepository(configDir string) (*ServiceRepository, error)
```

- Creates the configuration directory if it doesn't exist
- Sets up the file path for service storage
- Returns the initialized repository

### LoadServices

Loads service configurations from the JSON file:

```go
func (sr *ServiceRepository) LoadServices() ([]config.ServiceConfig, error)
```

- Acquires a read lock to ensure thread safety
- Checks if the configuration file exists
- Reads and parses the JSON data
- Converts the DTOs to domain objects
- Handles type conversions and default values

### SaveServices

Saves service configurations to the JSON file:

```go
func (sr *ServiceRepository) SaveServices(services []config.ServiceConfig) error
```

- Acquires a write lock to ensure thread safety
- Converts domain objects to DTOs
- Marshals the data to JSON with indentation for readability
- Writes the data to the file

## Usage Examples

### Creating a Service Repository

```go
// Create a repository with a specified config directory
repo, err := storage.NewServiceRepository("./config/services")
if err != nil {
    log.Fatalf("Failed to create service repository: %v", err)
}
```

### Loading Service Configurations

```go
// Load service configurations from storage
services, err := repo.LoadServices()
if err != nil {
    log.Errorf("Failed to load services: %v", err)
    return err
}

// Use the loaded services
for _, svc := range services {
    log.Infof("Loaded service: %s (%s)", svc.Name, svc.URL)
}
```

### Saving Service Configurations

```go
// Create or update service configurations
services := []config.ServiceConfig{
    {
        Name:            "user-service",
        URL:             "http://user-service:8080",
        HealthCheckPath: "/health",
        RequiresAuth:    true,
        Timeout:         5 * time.Second,
        RetryCount:      3,
        AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE"},
        AllowedRoles:    []string{"admin", "user"},
    },
    {
        Name:            "auth-service",
        URL:             "http://auth-service:8081",
        HealthCheckPath: "/health",
        RequiresAuth:    false,
        Timeout:         3 * time.Second,
        RetryCount:      2,
        AllowedMethods:  []string{"POST"},
    },
}

// Save the configurations
if err := repo.SaveServices(services); err != nil {
    log.Errorf("Failed to save services: %v", err)
    return err
}
```

## Integration with Service Registry

The `ServiceRepository` is primarily used by the HTTP adapter's `ServiceRegistry` to provide persistence for service configurations:

```go
// Create a service repository
repository, err := storage.NewServiceRepository("./config/services")
if err != nil {
    return nil, fmt.Errorf("failed to create service repository: %w", err)
}

// Create a service registry with the repository
registry := http.NewServiceRegistry(repository)

// Load services from the repository
if err := registry.LoadFromRepository(); err != nil {
    // Handle error or fallback to configuration
}
```

When services are registered or updated in the registry, the changes are automatically persisted to the storage:

```go
// RegisterService adds a new service to the registry
func (sr *ServiceRegistry) RegisterService(svcConfig config.ServiceConfig) error {
    // ... service registration logic ...

    // Persist the updated services
    if sr.repository != nil {
        go sr.persistServices()
    }

    return nil
}
```

## Performance Considerations

The file-based storage adapter:

- Uses a background goroutine for persistence to avoid blocking operations
- Employs mutex locking for thread safety with minimal contention
- Is optimized for infrequent writes and occasional reads
- Provides good performance for configuration data with small to medium volume

## Future Extensions

The Storage adapter could be expanded with additional repositories:

1. **PolicyRepository** - For storing authorization policies
2. **CacheRepository** - For persistent caching
3. **ConfigRepository** - For general application configuration
4. **TemplateRepository** - For UI templates and email templates

## Security Considerations

When using the Storage adapter:

1. **File Permissions** - Ensure proper file permissions are set (0644 for files, 0755 for directories)
2. **Sensitive Data** - Avoid storing sensitive data like credentials in plain text
3. **Path Validation** - Validate file paths to prevent directory traversal attacks
4. **Error Handling** - Properly handle and log errors without exposing implementation details
