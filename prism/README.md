# Schema Registry

A centralized Schema Registry for Golang microservices architecture with gRPC and protocol buffers.

## Overview

The Schema Registry serves as a central source of truth for all service API definitions. It enables:

- Dynamic service discovery
- Schema versioning
- Compatibility checking
- Zero-downtime service updates
- Dynamic client generation

## Features

- Store and retrieve Protocol Buffer schemas
- Register and discover service instances
- Check schema compatibility between versions
- Watch for schema and service changes
- Compile proto files to various languages

## Service Discovery

The platform includes a robust service discovery system that supports multiple backends:

- **Local**: In-memory implementation for development and testing
- **Consul**: Integration with HashiCorp Consul
- **etcd**: Integration with etcd distributed key-value store
- **Kubernetes**: Native service discovery in Kubernetes clusters

### Discovery Adapters

The service discovery system is designed with a clean ports and adapters architecture:

```
├── internal/
│   ├── adapters/
│   │   ├── discovery/
│   │   │   ├── consul.go      # Consul implementation
│   │   │   ├── etcd.go        # etcd implementation
│   │   │   ├── factory.go     # Factory for creating discovery instances
│   │   │   ├── kube.go        # Kubernetes implementation
│   │   │   └── local.go       # Local in-memory implementation
│   │   └── ...
│   ├── ports/
│   │   └── discovery.go       # Port interface definition
```

### Testing Service Discovery

The service discovery implementations include a comprehensive test suite:

```bash
# Run only local tests (no Docker needed)
make test-discovery-local

# Run integration tests
make test-discovery-integration

# Run Consul tests
make test-discovery-consul

# Run etcd tests
make test-discovery-etcd

# Run all discovery tests
make test-discovery-all
```

For more detailed test information, see the [discovery tests README](internal/adapters/discovery/tests/README.md).

## Getting Started

### Prerequisites

- Go 1.24 or higher
- PostgreSQL database
- Protocol Buffers compiler (protoc)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/jasonKoogler/gauth.git
cd gauth/schema-registry
```

2. Install dependencies:

```bash
go mod download
```

3. Build the service:

```bash
go build -o schema-registry ./cmd/server
```

4. Run the service:

```bash
./schema-registry
```

## Usage

### Registering a Service

Services should register themselves with the Schema Registry during startup:

```go
func registerServiceSchema(registry pb.SchemaRegistryClient, serviceID, serviceName, version string) error {
    // Read proto files
    protoDir := "./proto"
    schemas := make(map[string][]byte)

    err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() && filepath.Ext(path) == ".proto" {
            content, err := ioutil.ReadFile(path)
            if err != nil {
                return err
            }

            relPath, err := filepath.Rel(protoDir, path)
            if err != nil {
                return err
            }

            schemas[relPath] = content
        }

        return nil
    })
    if err != nil {
        return err
    }

    // Register each schema file
    for name, content := range schemas {
        _, err := registry.CreateSchema(context.Background(), &pb.CreateSchemaRequest{
            ServiceName: serviceName,
            Name:        name,
            Version:     version,
            SchemaType:  pb.SchemaType_PROTOBUF,
            Content:     content,
        })
        if err != nil {
            return err
        }
    }

    // Register service instance
    _, err = registry.RegisterService(context.Background(), &pb.RegisterServiceRequest{
        ServiceId:     serviceID,
        ServiceName:   serviceName,
        Version:       version,
        SchemaVersion: version,
        Address:       getOutboundIP(),
        Port:          50051, // Your service port
    })

    return err
}
```

### API Gateway Integration

The API Gateway should connect to the Schema Registry to discover services and create dynamic clients:

```go
func NewSchemaManager(registryAddr string) (*SchemaManager, error) {
    conn, err := grpc.Dial(registryAddr, grpc.WithInsecure())
    if err != nil {
        return nil, err
    }

    client := pb.NewSchemaRegistryClient(conn)

    manager := &SchemaManager{
        registryClient: client,
        services:       make(map[string]*ServiceInfo),
        clients:        make(map[string]interface{}),
    }

    // Start watching for service changes
    go manager.watchServices()

    return manager, nil
}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
