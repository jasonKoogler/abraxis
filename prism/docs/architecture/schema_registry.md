# Schema Registry Integration

This document explains the schema registry system and how it integrates with the service proxy and service discovery components to enable dynamic service contract validation and integration.

## Architecture Overview

The schema registry system consists of several key components:

1. **Schema Registry Service**: Central repository for storing and managing API schemas
2. **Schema Manager**: Client-side component that synchronizes schemas and enables dynamic service integration
3. **Dynamic Clients**: Generated clients based on schema definitions
4. **Protocol Buffers**: Schema definition format used for service contracts
5. **gRPC-Gateway**: Translates RESTful HTTP requests to gRPC calls

## Schema Registry Service

The Schema Registry Service serves as the central repository for API schemas and provides the following capabilities:

- Storage and versioning of schemas
- Schema validation and compatibility checking
- Service registration and discovery
- Real-time schema updates via streaming API

It exposes a gRPC API with the following key methods:

```proto
service SchemaRegistry {
  // Schema management
  rpc CreateSchema(CreateSchemaRequest) returns (SchemaResponse);
  rpc GetSchema(GetSchemaRequest) returns (SchemaResponse);
  rpc UpdateSchema(UpdateSchemaRequest) returns (SchemaResponse);
  rpc DeleteSchema(DeleteSchemaRequest) returns (DeleteSchemaResponse);
  rpc ListSchemas(ListSchemasRequest) returns (ListSchemasResponse);

  // Bundle management
  rpc CreateBundle(CreateBundleRequest) returns (BundleResponse);
  rpc GetBundle(GetBundleRequest) returns (BundleResponse);
  rpc ListBundles(ListBundlesRequest) returns (ListBundlesResponse);

  // Service management
  rpc RegisterService(RegisterServiceRequest) returns (RegisterServiceResponse);
  rpc DeregisterService(DeregisterServiceRequest) returns (DeregisterServiceResponse);
  rpc ServiceHeartbeat(ServiceHeartbeatRequest) returns (ServiceHeartbeatResponse);
  rpc ListServices(ListServicesRequest) returns (ListServicesResponse);

  // Schema operations
  rpc CheckCompatibility(CheckCompatibilityRequest) returns (CheckCompatibilityResponse);
  rpc CompileProto(CompileProtoRequest) returns (CompileProtoResponse);

  // Streaming APIs
  rpc WatchSchemas(WatchSchemasRequest) returns (stream SchemaEvent);
  rpc WatchServices(WatchServicesRequest) returns (stream ServiceEvent);
}
```

## Schema Manager

The Schema Manager is responsible for interacting with the Schema Registry Service and providing dynamic service integration. It is defined in `internal/service/service_schema_registry.go` and has these main responsibilities:

1. **Schema Synchronization**: Fetches schemas from the registry and keeps them up-to-date
2. **Dynamic Client Generation**: Creates gRPC clients based on schema definitions
3. **Request Routing**: Routes HTTP requests to appropriate services based on schema definitions
4. **Service Discovery Integration**: Works with the service discovery system to find service instances

The Schema Manager maintains several important data structures:

```go
type SchemaManager struct {
    registryClient    pb.SchemaRegistryClient
    registryConn      *grpc.ClientConn
    discoveryClient   ports.ServiceDiscovery
    protoDir          string
    genDir            string
    router            chi.Router
    grpcGatewayMux    *runtime.ServeMux
    services          map[string]*ServiceInfo
    fileDescriptors   map[string]protoreflect.FileDescriptor
    dynamicClients    map[string]*DynamicClient
    endpoints         map[string]EndpointInfo
    schemaWatchCancel context.CancelFunc
    serviceWatchChan  chan *ports.ServiceInstance
    mu                sync.RWMutex
}
```

### Schema Synchronization

The Schema Manager synchronizes schemas with the registry through the `SyncSchemas` method:

```go
func (sm *SchemaManager) SyncSchemas(ctx context.Context) error {
    log.Println("Syncing schemas from registry...")

    // Fetch schemas from registry
    schemas, err := sm.registryClient.ListSchemas(ctx, &pb.ListSchemasRequest{})
    if err != nil {
        return fmt.Errorf("failed to list schemas: %w", err)
    }

    // Process each schema
    for _, schema := range schemas.Schemas {
        // Write proto files
        // Parse and compile schemas
        // Extract endpoint information
        // Create dynamic clients
    }

    return nil
}
```

### Dynamic Clients

The Schema Manager creates dynamic gRPC clients based on schema definitions:

```go
type DynamicClient struct {
    ServiceDesc         protoreflect.ServiceDescriptor
    Conn                *grpc.ClientConn
    Methods             map[string]protoreflect.MethodDescriptor
    MessageTypes        map[string]protoreflect.MessageDescriptor
    IsReflectionEnabled bool
}
```

These clients are created dynamically based on protocol buffer descriptors, enabling communication with services even without compile-time knowledge of their interfaces.

### Request Routing

The Schema Manager configures routing based on schema definitions:

```go
func (sm *SchemaManager) setupRouter() {
    // Mount health endpoint
    sm.router.Get("/schema-registry/health", func(w http.ResponseWriter, r *http.Request) {
        // Return health status
    })

    // Mount dynamic service endpoints
    sm.router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
        // Find matching endpoint
        endpoint, params, err := sm.findEndpoint(r.Method, r.URL.Path)
        if err != nil {
            // Handle routing error
            return
        }

        // Process the request using dynamic clients
        response, err := sm.processRequest(r.Context(), endpoint, params, requestBody)
        if err != nil {
            // Handle processing error
            return
        }

        // Return response
        w.Header().Set("Content-Type", "application/json")
        w.Write(response)
    })
}
```

## Integration with Service Discovery

The Schema Manager integrates with the service discovery system to locate service instances:

```go
func (sm *SchemaManager) watchServices(ctx context.Context) {
    // Create watch channel for service updates
    watchCh, err := sm.discoveryClient.WatchServices(ctx)
    if err != nil {
        log.Printf("Failed to watch services: %v", err)
        return
    }

    // Process service updates
    for {
        select {
        case <-ctx.Done():
            return
        case instance, ok := <-watchCh:
            if !ok {
                return
            }

            // Process service instance update
            sm.mu.Lock()
            if instance.Status == "active" {
                // Add or update service instance
                sm.connectToServiceInstance(instance)
            } else {
                // Remove service instance
                sm.removeServiceInstance(instance)
            }
            sm.mu.Unlock()
        }
    }
}
```

This integration enables:

- Automatic discovery of service instances
- Dynamic client creation for newly discovered services
- Health monitoring of services
- Graceful handling of service unavailability

## Protocol Buffers and gRPC

The schema registry uses Protocol Buffers (protobuf) as the schema definition format and gRPC as the communication protocol:

1. **Protocol Buffers**: Provide a language-neutral, platform-neutral extensible mechanism for serializing structured data
2. **gRPC**: High-performance RPC framework built on HTTP/2

The system uses protobuf's reflection capabilities to dynamically work with schemas:

```go
func (sm *SchemaManager) introspectService(ctx context.Context, serviceName string) error {
    // Connect to the service
    conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return fmt.Errorf("failed to connect to service %s: %w", serviceName, err)
    }

    // Create reflection client
    reflectionClient := grpc_reflection_v1.NewServerReflectionClient(conn)

    // Use reflection to discover service methods and message types
    stream, err := reflectionClient.ServerReflectionInfo(ctx)
    if err != nil {
        return fmt.Errorf("failed to create reflection stream: %w", err)
    }

    // Extract service descriptor information
    // Build dynamic client based on reflection data

    return nil
}
```

## gRPC-Gateway Integration

The system uses gRPC-Gateway to translate RESTful HTTP requests to gRPC calls:

```go
// Initialize gRPC-Gateway mux
jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
    MarshalOptions: protojson.MarshalOptions{
        UseProtoNames:   true,
        EmitUnpopulated: true,
    },
    UnmarshalOptions: protojson.UnmarshalOptions{
        DiscardUnknown: true,
    },
})

gwmux := runtime.NewServeMux(
    jsonOption,
    runtime.WithOutgoingHeaderMatcher(customOutgoingHeaderMatcher),
    runtime.WithMetadata(addMetadataFromRequest),
)
```

This enables:

- RESTful HTTP endpoints for gRPC services
- JSON/HTTP compatibility for gRPC services
- Automatic request/response translation

## Schema Versioning and Compatibility

The schema registry supports versioning and compatibility checking:

```go
func (sm *SchemaManager) CheckCompatibility(ctx context.Context, req *pb.CheckCompatibilityRequest) (*pb.CheckCompatibilityResponse, error) {
    // Get existing schema
    existingSchema, err := sm.GetSchema(ctx, &pb.GetSchemaRequest{
        Name:    req.Name,
        Version: req.ExistingVersion,
    })
    if err != nil {
        return nil, err
    }

    // Check compatibility between existing and proposed schema
    compatible, issues := checkSchemaCompatibility(existingSchema.Schema, req.ProposedSchema)

    return &pb.CheckCompatibilityResponse{
        Compatible: compatible,
        Issues:     issues,
    }, nil
}
```

This enables:

- Safe schema evolution
- Backward and forward compatibility checks
- Breaking change detection

## Sequence Diagram

```
┌──────────┐      ┌──────────────┐      ┌─────────────┐      ┌───────────┐
│ Service A│      │ Schema       │      │ Schema      │      │ Service B │
│          │      │ Registry     │      │ Manager     │      │           │
└────┬─────┘      └──────┬───────┘      └──────┬──────┘      └─────┬─────┘
     │                   │                     │                   │
     │ Register Schema   │                     │                   │
     │──────────────────►│                     │                   │
     │                   │                     │                   │
     │                   │                     │  Sync Schemas     │
     │                   │◄────────────────────│                   │
     │                   │                     │                   │
     │                   │  Schema Definitions │                   │
     │                   │────────────────────►│                   │
     │                   │                     │                   │
     │                   │                     │  Create Dynamic   │
     │                   │                     │  Clients          │
     │                   │                     │───────────────────│
     │                   │                     │                   │
     │ Register Service  │                     │                   │
     │──────────────────►│                     │                   │
     │                   │                     │                   │
     │                   │  Service Updated    │                   │
     │                   │────────────────────►│                   │
     │                   │                     │                   │
     │ HTTP Request      │                     │                   │
     │─────────────────────────────────────────►                   │
     │                   │                     │                   │
     │                   │                     │  gRPC Request     │
     │                   │                     │──────────────────►│
     │                   │                     │                   │
     │                   │                     │  gRPC Response    │
     │                   │                     │◄──────────────────│
     │                   │                     │                   │
     │ HTTP Response     │                     │                   │
     │◄─────────────────────────────────────────                   │
     │                   │                     │                   │
     │                   │                     │                   │
└────┴─────┘      └──────┴───────┘      └──────┴──────┘      └─────┴─────┘
```

## Best Practices

1. **Schema-First Development**: Define service contracts using protobuf before implementing
2. **Versioning**: Always version schemas to enable safe evolution
3. **Compatibility**: Check schema compatibility before deploying changes
4. **Documentation**: Include comprehensive documentation in schemas
5. **Error Handling**: Define standardized error responses in schemas
6. **Reflection**: Enable gRPC reflection in services for dynamic discovery

## Integration with Service Proxy

The Schema Registry and Service Proxy work together to enable dynamic routing and contract validation:

1. The Schema Registry maintains the service contracts (schemas)
2. The Schema Manager synchronizes these schemas
3. The Service Proxy routes requests to appropriate service instances
4. The Schema Manager validates requests against the schemas
5. The Service Discovery system locates service instances

This integration ensures that:

- Service contracts are well-defined and validated
- Clients can discover and use services dynamically
- Services can evolve while maintaining compatibility
- The gateway can route requests based on schema definitions
