package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/jasonKoogler/abraxis/prism/internal/ports"
	pb "github.com/jasonKoogler/abraxis/prism/internal/ports/proto"
)

// SchemaManager handles schema synchronization and dynamic service registration
type SchemaManager struct {
	registryClient    pb.SchemaRegistryClient
	registryConn      *grpc.ClientConn
	discoveryClient   ports.ServiceDiscoverer
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

// ServiceInfo holds information about a registered service
type ServiceInfo struct {
	Name          string
	Version       string
	SchemaVersion string
	Endpoints     []string
	Instances     []*ports.ServiceInstance
}

// DynamicClient represents a dynamically created gRPC client
type DynamicClient struct {
	ServiceDesc         protoreflect.ServiceDescriptor
	Conn                *grpc.ClientConn
	Methods             map[string]protoreflect.MethodDescriptor
	MessageTypes        map[string]protoreflect.MessageDescriptor
	IsReflectionEnabled bool // Indicates whether the service supports reflection
}

// EndpointInfo holds REST endpoint mapping information
type EndpointInfo struct {
	Method      string
	Path        string
	ServiceName string
	MethodName  string
	PathParams  []string
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager(
	registryAddr string,
	discoveryClient ports.ServiceDiscoverer,
	protoDir, genDir string,
	router chi.Router,
) (*SchemaManager, error) {
	// Connect to schema registry
	conn, err := grpc.Dial(registryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to schema registry: %w", err)
	}

	registryClient := pb.NewSchemaRegistryClient(conn)

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

	grpcGatewayMux := runtime.NewServeMux(
		jsonOption,
		runtime.WithIncomingHeaderMatcher(runtime.DefaultHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(runtime.DefaultHeaderMatcher),
	)

	// Ensure directories exist
	for _, dir := range []string{protoDir, genDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &SchemaManager{
		registryClient:   registryClient,
		registryConn:     conn,
		discoveryClient:  discoveryClient,
		protoDir:         protoDir,
		genDir:           genDir,
		router:           router,
		grpcGatewayMux:   grpcGatewayMux,
		services:         make(map[string]*ServiceInfo),
		fileDescriptors:  make(map[string]protoreflect.FileDescriptor),
		dynamicClients:   make(map[string]*DynamicClient),
		endpoints:        make(map[string]EndpointInfo),
		serviceWatchChan: make(chan *ports.ServiceInstance, 100),
	}, nil
}

// Start begins schema and service monitoring
func (sm *SchemaManager) Start(ctx context.Context) error {
	// Initial schema sync
	if err := sm.SyncSchemas(ctx); err != nil {
		return fmt.Errorf("initial schema sync failed: %w", err)
	}

	// Setup API routes
	sm.setupRouter()

	// Start schema watch
	schemaCtx, cancel := context.WithCancel(ctx)
	sm.schemaWatchCancel = cancel
	go sm.watchSchemas(schemaCtx)

	// Start service watch
	go sm.watchServices(ctx)

	return nil
}

// Stop stops all watches and closes connections
func (sm *SchemaManager) Stop() {
	if sm.schemaWatchCancel != nil {
		sm.schemaWatchCancel()
	}

	if sm.registryConn != nil {
		sm.registryConn.Close()
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close all dynamic client connections
	for _, client := range sm.dynamicClients {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}
}

// setupRouter configures the HTTP router
func (sm *SchemaManager) setupRouter() {
	// Mount the gRPC-Gateway mux
	sm.router.Mount("/api/v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle REST API requests
		endpoint, params, err := sm.findEndpoint(r.Method, r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Extract body if present
		var body []byte
		if r.Body != nil {
			body, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
				return
			}
		}

		// Process the request
		responseData, err := sm.processRequest(r.Context(), endpoint, params, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set content type and write response
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseData)
	}))

	// Add service listing endpoint
	sm.router.Get("/services", func(w http.ResponseWriter, r *http.Request) {
		services := sm.listServices()
		// Direct marshaling to JSON
		response, err := json.Marshal(services)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	// Add schema listing endpoint
	sm.router.Get("/schemas", func(w http.ResponseWriter, r *http.Request) {
		schemas := sm.listSchemas()
		// Direct marshaling to JSON
		response, err := json.Marshal(schemas)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	// Add endpoint listing
	sm.router.Get("/endpoints", func(w http.ResponseWriter, r *http.Request) {
		endpoints := sm.listEndpoints()
		// Direct marshaling to JSON
		response, err := json.Marshal(endpoints)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	// Add service introspection endpoint using reflection
	sm.router.Get("/services/{serviceName}/introspect", func(w http.ResponseWriter, r *http.Request) {
		serviceName := chi.URLParam(r, "serviceName")
		if serviceName == "" {
			http.Error(w, "Service name is required", http.StatusBadRequest)
			return
		}

		// Use reflection to introspect the service
		err := sm.UpdateServiceFromReflection(r.Context(), serviceName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to introspect service: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","message":"Service introspected successfully"}`))
	})
}

// findEndpoint matches a REST path to a registered endpoint
func (sm *SchemaManager) findEndpoint(method, path string) (EndpointInfo, map[string]string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Normalize the path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove trailing slash if present
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	// First try exact match
	key := method + " " + path
	if endpoint, ok := sm.endpoints[key]; ok {
		return endpoint, make(map[string]string), nil
	}

	// Try pattern matching
	for epKey, endpoint := range sm.endpoints {
		if strings.HasPrefix(epKey, method+" ") {
			pattern := strings.TrimPrefix(epKey, method+" ")
			params, match := matchPathPattern(pattern, path)
			if match {
				return endpoint, params, nil
			}
		}
	}

	return EndpointInfo{}, nil, fmt.Errorf("endpoint not found: %s %s", method, path)
}

// matchPathPattern matches a path against a pattern with parameters
func matchPathPattern(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Extract parameter name
			paramName := part[1 : len(part)-1]
			params[paramName] = pathParts[i]
		} else if part != pathParts[i] {
			return nil, false
		}
	}

	return params, true
}

// processRequest processes a REST API request
func (sm *SchemaManager) processRequest(ctx context.Context, endpoint EndpointInfo, params map[string]string, body []byte) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Find the service client
	var client *DynamicClient
	for fqn, c := range sm.dynamicClients {
		if extractServiceName(fqn) == endpoint.ServiceName {
			client = c
			break
		}
	}

	if client == nil {
		return nil, fmt.Errorf("service %s not found", endpoint.ServiceName)
	}

	// Find the method
	method, ok := client.Methods[endpoint.MethodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in service %s", endpoint.MethodName, endpoint.ServiceName)
	}

	// Create dynamic message for the request
	inputMsg := dynamicpb.NewMessage(method.Input())

	// Merge path parameters and body into request
	if len(params) > 0 {
		// Convert params to JSON
		paramDataJSON, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal path parameters: %w", err)
		}

		if err := protojson.Unmarshal(paramDataJSON, inputMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal path parameters: %w", err)
		}
	}

	// Add body content if present
	if len(body) > 0 {
		if err := protojson.Unmarshal(body, inputMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal request body: %w", err)
		}
	}

	// Forward any headers to the gRPC service
	md := metadata.New(nil)
	// Add authorization header if present
	// md.Set("authorization", authHeader)

	// Create context with metadata
	ctxWithMetadata := metadata.NewOutgoingContext(ctx, md)

	// Invoke the method
	outputMsg := dynamicpb.NewMessage(method.Output())
	err := client.Conn.Invoke(ctxWithMetadata, string(method.FullName()), inputMsg, outputMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke method: %w", err)
	}

	// Marshal the response to JSON
	responseData, err := protojson.Marshal(outputMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return responseData, nil
}

// SyncSchemas fetches all schemas from the registry and processes them
func (sm *SchemaManager) SyncSchemas(ctx context.Context) error {
	log.Println("Syncing schemas from registry...")

	// List all services
	servicesResp, err := sm.discoveryClient.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// For each service, get its schema bundle
	for _, service := range servicesResp {
		// Get latest schema bundle for the service
		bundles, err := sm.registryClient.ListBundles(ctx, &pb.ListBundlesRequest{
			ServiceName: service.Name,
			Page:        1,
			PageSize:    1,
		})
		if err != nil {
			log.Printf("Failed to get bundles for service %s: %v", service.Name, err)
			continue
		}

		if len(bundles.Bundles) == 0 {
			log.Printf("No schema bundles found for service %s", service.Name)
			continue
		}

		bundle := bundles.Bundles[0]
		log.Printf("Processing schema bundle for %s v%s", bundle.ServiceName, bundle.Version)

		// Fetch and save all schemas in the bundle
		for _, schemaID := range bundle.SchemaIds {
			// In a real implementation, you would query by ID
			// This is simplified for clarity
			schemas, err := sm.registryClient.ListSchemas(ctx, &pb.ListSchemasRequest{
				ServiceName: bundle.ServiceName,
				PageSize:    100,
			})
			if err != nil {
				log.Printf("Failed to list schemas: %v", err)
				continue
			}

			for _, schema := range schemas.Schemas {
				if schema.Id == schemaID {
					// Save the schema to disk
					protoPath := filepath.Join(sm.protoDir, schema.ServiceName, schema.Name)
					if err := os.MkdirAll(filepath.Dir(protoPath), 0755); err != nil {
						log.Printf("Failed to create schema directory: %v", err)
						continue
					}

					if err := os.WriteFile(protoPath, schema.Content, 0644); err != nil {
						log.Printf("Failed to write schema file: %v", err)
						continue
					}

					log.Printf("Saved schema: %s/%s v%s", schema.ServiceName, schema.Name, schema.Version)
					break
				}
			}
		}
	}

	// Parse proto files and build file descriptors
	if err := sm.parseProtoFiles(ctx); err != nil {
		return fmt.Errorf("failed to parse proto files: %w", err)
	}

	// Extract REST endpoint mappings
	if err := sm.extractEndpoints(); err != nil {
		return fmt.Errorf("failed to extract endpoints: %w", err)
	}

	// Create dynamic clients for each service
	if err := sm.createDynamicClients(ctx); err != nil {
		return fmt.Errorf("failed to create dynamic clients: %w", err)
	}

	return nil
}

// parseProtoFiles parses all proto files in the proto directory
func (sm *SchemaManager) parseProtoFiles(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear existing descriptors
	sm.fileDescriptors = make(map[string]protoreflect.FileDescriptor)

	// Find all proto files
	var protoFiles []string
	err := filepath.Walk(sm.protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			relPath, err := filepath.Rel(sm.protoDir, path)
			if err != nil {
				return err
			}
			protoFiles = append(protoFiles, relPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk proto directory: %w", err)
	}

	if len(protoFiles) == 0 {
		log.Println("No proto files found")
		return nil
	}

	// Create a temporary directory for descriptor output
	tempDir, err := os.MkdirTemp("", "proto-desc-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a descriptor set file path
	descSetPath := filepath.Join(tempDir, "descriptors.pb")

	// Build protoc command to generate descriptors
	args := []string{
		fmt.Sprintf("--descriptor_set_out=%s", descSetPath),
		"--include_imports",
		"--include_source_info",
		fmt.Sprintf("-I=%s", sm.protoDir),
	}

	// Add all proto files to the command
	for _, protoFile := range protoFiles {
		args = append(args, protoFile)
	}

	// Execute protoc command
	cmd := exec.Command("protoc", args...)
	cmd.Dir = sm.protoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run protoc: %w, stderr: %s", err, stderr.String())
	}

	// Read the descriptor set file
	descSetData, err := os.ReadFile(descSetPath)
	if err != nil {
		return fmt.Errorf("failed to read descriptor set file: %w", err)
	}

	// Parse the descriptor set
	fileDescSet := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(descSetData, fileDescSet); err != nil {
		return fmt.Errorf("failed to unmarshal descriptor set: %w", err)
	}

	// Create a new Files registry
	files, err := protodesc.NewFiles(fileDescSet)
	if err != nil {
		return fmt.Errorf("failed to create file registry: %w", err)
	}

	// Store and register all files
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		filePath := fd.Path()
		sm.fileDescriptors[filePath] = fd
		log.Printf("Parsed proto file: %s", filePath)

		// Register with global proto registry
		if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
			log.Printf("Failed to register file descriptor for %s: %v", filePath, err)
		}
		return true
	})

	log.Printf("Parsed %d proto files", len(sm.fileDescriptors))
	return nil
}

// extractEndpoints extracts REST endpoint mappings from proto file options
func (sm *SchemaManager) extractEndpoints() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear existing endpoints
	sm.endpoints = make(map[string]EndpointInfo)

	// Process all file descriptors
	for _, fd := range sm.fileDescriptors {
		// Process all services
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			serviceName := extractServiceName(string(service.FullName()))

			// Process all methods
			for j := 0; j < service.Methods().Len(); j++ {
				method := service.Methods().Get(j)
				// Check for HTTP annotations
				// Note: This would normally use proper option extension access
				// This is a simplified implementation for demonstration

				// Assume we have a way to extract HTTP rules
				// In practice, you'd use something like:
				// rule := proto.GetExtension(methodOptions, annotations.E_Http).(*annotations.HttpRule)

				// For demonstration, we'll create some example mappings
				// In a real implementation, you'd parse these from the proto options

				httpMethod := "GET"
				path := fmt.Sprintf("/api/v1/%s/%s", strings.ToLower(serviceName), strings.ToLower(string(method.Name())))

				// Extract path parameters
				var pathParams []string
				parts := strings.Split(path, "/")
				for _, part := range parts {
					if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
						paramName := part[1 : len(part)-1]
						pathParams = append(pathParams, paramName)
					}
				}

				endpoint := EndpointInfo{
					Method:      httpMethod,
					Path:        path,
					ServiceName: serviceName,
					MethodName:  string(method.Name()),
					PathParams:  pathParams,
				}

				key := httpMethod + " " + path
				sm.endpoints[key] = endpoint

				log.Printf("Registered endpoint: %s %s -> %s.%s", httpMethod, path, serviceName, string(method.Name()))
			}
		}
	}

	return nil
}

// createDynamicClients creates dynamic gRPC clients for all services
func (sm *SchemaManager) createDynamicClients(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear existing clients
	for _, client := range sm.dynamicClients {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}
	sm.dynamicClients = make(map[string]*DynamicClient)

	// Find all service descriptors in the parsed files
	for _, fd := range sm.fileDescriptors {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			serviceName := string(service.FullName())
			log.Printf("Creating dynamic client for service: %s", serviceName)

			// Create a map of methods
			methods := make(map[string]protoreflect.MethodDescriptor)
			messageTypes := make(map[string]protoreflect.MessageDescriptor)

			for j := 0; j < service.Methods().Len(); j++ {
				method := service.Methods().Get(j)
				methods[string(method.Name())] = method

				// Track input and output message types
				inputType := method.Input()
				outputType := method.Output()

				messageTypes[string(inputType.FullName())] = inputType
				messageTypes[string(outputType.FullName())] = outputType

				log.Printf("  Method: %s, Input: %s, Output: %s",
					string(method.Name()),
					string(inputType.FullName()),
					string(outputType.FullName()))
			}

			sm.dynamicClients[serviceName] = &DynamicClient{
				ServiceDesc:  service,
				Methods:      methods,
				MessageTypes: messageTypes,
			}
		}
	}

	// Connect to active service instances
	instances, err := sm.discoveryClient.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list service instances: %w", err)
	}

	for _, instance := range instances {
		if err := sm.connectToServiceInstance(instance); err != nil {
			log.Printf("Failed to connect to %s at %s:%d: %v",
				instance.ServiceName, instance.Address, instance.Port, err)
		}
	}

	return nil
}

// connectToServiceInstance establishes a connection to a service instance
func (sm *SchemaManager) connectToServiceInstance(instance *ports.ServiceInstance) error {
	// Find the service definition
	var serviceFQN string
	for fqn, client := range sm.dynamicClients {
		if extractServiceName(fqn) == instance.ServiceName {
			serviceFQN = fqn

			// Create a connection to the service instance
			addr := fmt.Sprintf("%s:%d", instance.Address, instance.Port)
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to connect to service: %w", err)
			}

			client.Conn = conn

			// Check if reflection is enabled by attempting to create a reflection client
			refClient := grpc_reflection_v1.NewServerReflectionClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			stream, err := refClient.ServerReflectionInfo(ctx)
			if err != nil {
				log.Printf("Reflection not available for %s: %v", instance.ServiceName, err)
				client.IsReflectionEnabled = false
			} else {
				// Try to list services to confirm reflection is working
				if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
					MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
				}); err != nil {
					log.Printf("Reflection request failed for %s: %v", instance.ServiceName, err)
					client.IsReflectionEnabled = false
				} else {
					// Successfully sent request, close the stream
					stream.CloseSend()
					client.IsReflectionEnabled = true
					log.Printf("Reflection enabled for %s", instance.ServiceName)
				}
			}

			log.Printf("Connected to %s at %s", instance.ServiceName, addr)
			break
		}
	}

	if serviceFQN == "" {
		return fmt.Errorf("no service definition found for %s", instance.ServiceName)
	}

	return nil
}

// extractServiceName extracts the service name from a fully qualified name
func extractServiceName(fqn string) string {
	// This is a simplified implementation - in practice, you'd parse the package and service name
	// FQN format is typically: package.ServiceName
	parts := strings.Split(fqn, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fqn
}

// watchSchemas monitors for schema changes
func (sm *SchemaManager) watchSchemas(ctx context.Context) {
	log.Println("Starting schema watch...")

	// Watch all services
	stream, err := sm.registryClient.WatchSchemas(ctx, &pb.WatchSchemasRequest{})
	if err != nil {
		log.Printf("Failed to start schema watch: %v", err)
		return
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			log.Printf("Schema watch error: %v", err)
			// Try to reconnect after a delay
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				stream, err = sm.registryClient.WatchSchemas(ctx, &pb.WatchSchemasRequest{})
				if err != nil {
					log.Printf("Failed to restart schema watch: %v", err)
					continue
				}
			}
			continue
		}

		// Process the schema event
		schema := event.Schema
		log.Printf("Schema event: %s %s/%s v%s",
			event.EventType, schema.ServiceName, schema.Name, schema.Version)

		// Update the schema
		protoPath := filepath.Join(sm.protoDir, schema.ServiceName, schema.Name)

		switch event.EventType {
		case pb.SchemaEvent_CREATED, pb.SchemaEvent_UPDATED:
			// Save the schema
			if err := os.MkdirAll(filepath.Dir(protoPath), 0755); err != nil {
				log.Printf("Failed to create schema directory: %v", err)
				continue
			}

			if err := os.WriteFile(protoPath, schema.Content, 0644); err != nil {
				log.Printf("Failed to write schema file: %v", err)
				continue
			}

		case pb.SchemaEvent_DELETED:
			// Remove the schema
			if err := os.Remove(protoPath); err != nil {
				log.Printf("Failed to delete schema file: %v", err)
			}
		}

		// Refresh proto files
		if err := sm.parseProtoFiles(ctx); err != nil {
			log.Printf("Failed to refresh proto files: %v", err)
			continue
		}

		// Extract endpoints
		if err := sm.extractEndpoints(); err != nil {
			log.Printf("Failed to extract endpoints: %v", err)
			continue
		}

		// Update dynamic clients
		if err := sm.createDynamicClients(ctx); err != nil {
			log.Printf("Failed to refresh dynamic clients: %v", err)
		}
	}
}

// watchServices monitors for service instance changes
func (sm *SchemaManager) watchServices(ctx context.Context) {
	log.Println("Starting service watch...")

	// Watch for service changes
	watchCh, err := sm.discoveryClient.WatchServices(ctx)
	if err != nil {
		log.Printf("Failed to start service watch: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return

		case instance, ok := <-watchCh:
			if !ok {
				log.Println("Service watch channel closed")
				return
			}

			log.Printf("Service event: %s %s v%s at %s:%d",
				instance.Status, instance.ServiceName, instance.Version,
				instance.Address, instance.Port)

			// Handle instance updates
			switch instance.Status {
			case "REGISTERED", "ACTIVE":
				if err := sm.connectToServiceInstance(instance); err != nil {
					log.Printf("Failed to connect to service instance: %v", err)
				}

			case "DEREGISTERED", "INACTIVE":
				sm.mu.Lock()
				for fqn, client := range sm.dynamicClients {
					if extractServiceName(fqn) == instance.ServiceName && client.Conn != nil {
						client.Conn.Close()
						client.Conn = nil
						log.Printf("Closed connection to %s", instance.ServiceName)
					}
				}
				sm.mu.Unlock()

				// Refresh connections to active instances
				if err := sm.createDynamicClients(ctx); err != nil {
					log.Printf("Failed to refresh dynamic clients: %v", err)
				}
			}
		}
	}
}

// GetHealth returns the health status of the schema manager
func (sm *SchemaManager) GetHealth() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Count connected services
	connectedServices := 0
	for _, client := range sm.dynamicClients {
		if client.Conn != nil {
			connectedServices++
		}
	}

	return map[string]interface{}{
		"status": "UP",
		"details": map[string]interface{}{
			"schema_registry_connected": sm.registryConn != nil,
			"proto_files_count":         len(sm.fileDescriptors),
			"services_count":            len(sm.dynamicClients),
			"connected_services":        connectedServices,
			"endpoints_count":           len(sm.endpoints),
		},
	}
}

// Helper methods for API endpoints

// listServices returns a list of all registered services
func (sm *SchemaManager) listServices() *pb.ListServicesResponse {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	services := &pb.ListServicesResponse{
		Services: []*pb.ServiceResponse{},
	}

	for fqn, client := range sm.dynamicClients {
		if client.Conn != nil {
			serviceName := extractServiceName(fqn)

			// In a real implementation, you'd get this information from your service registry
			service := &pb.ServiceResponse{
				ServiceName: serviceName,
				Version:     "v1", // Placeholder
			}

			services.Services = append(services.Services, service)
		}
	}

	return services
}

// listEndpoints returns information about all registered API endpoints
func (sm *SchemaManager) listEndpoints() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]interface{})
	endpoints := make([]map[string]interface{}, 0, len(sm.endpoints))

	for _, endpoint := range sm.endpoints {
		endpointInfo := map[string]interface{}{
			"method":       endpoint.Method,
			"path":         endpoint.Path,
			"service_name": endpoint.ServiceName,
			"method_name":  endpoint.MethodName,
			"path_params":  endpoint.PathParams,
		}

		endpoints = append(endpoints, endpointInfo)
	}

	result["endpoints"] = endpoints
	result["count"] = len(endpoints)

	return result
}

// listSchemas returns a list of all available schemas
func (sm *SchemaManager) listSchemas() *pb.ListSchemasResponse {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	schemas := &pb.ListSchemasResponse{
		Schemas: []*pb.SchemaResponse{},
	}

	// Get all schema files
	err := filepath.Walk(sm.protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			relPath, err := filepath.Rel(sm.protoDir, path)
			if err != nil {
				return err
			}

			// Extract service name and schema name from path
			parts := strings.Split(relPath, string(os.PathSeparator))
			var serviceName, schemaName string

			if len(parts) > 1 {
				serviceName = parts[0]
				schemaName = parts[len(parts)-1]
			} else {
				schemaName = parts[0]
			}

			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Create schema response
			schema := &pb.SchemaResponse{
				ServiceName: serviceName,
				Name:        schemaName,
				Version:     "v1", // Placeholder
				SchemaType:  pb.SchemaType_PROTOBUF,
				Content:     content,
			}

			schemas.Schemas = append(schemas.Schemas, schema)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error listing schemas: %v", err)
	}

	return schemas
}

// introspectService uses reflection to discover service methods and types at runtime
func (sm *SchemaManager) introspectService(ctx context.Context, serviceName string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Find the service client
	var client *DynamicClient
	for fqn, c := range sm.dynamicClients {
		if extractServiceName(fqn) == serviceName {
			client = c
			break
		}
	}

	if client == nil || client.Conn == nil {
		return fmt.Errorf("service %s not found or not connected", serviceName)
	}

	// Check if reflection is enabled
	if !client.IsReflectionEnabled {
		return fmt.Errorf("reflection is not enabled for service %s", serviceName)
	}

	// Create a reflection client
	refClient := grpc_reflection_v1.NewServerReflectionClient(client.Conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get reflection info: %w", err)
	}

	// Request list of services
	if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return fmt.Errorf("failed to send list services request: %w", err)
	}

	// Receive list of services
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive list services response: %w", err)
	}

	// Process the list of services
	serviceList := resp.GetListServicesResponse()
	if serviceList == nil {
		return fmt.Errorf("unexpected response type")
	}

	log.Printf("Discovered %d services via reflection for %s:", len(serviceList.Service), serviceName)
	for _, service := range serviceList.Service {
		log.Printf("  - %s", service.Name)

		// Request file containing the service
		if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service.Name,
			},
		}); err != nil {
			log.Printf("Failed to request file for service %s: %v", service.Name, err)
			continue
		}

		// Receive file descriptor
		fdResp, err := stream.Recv()
		if err != nil {
			log.Printf("Failed to receive file descriptor for service %s: %v", service.Name, err)
			continue
		}

		fdProto := fdResp.GetFileDescriptorResponse()
		if fdProto == nil || len(fdProto.FileDescriptorProto) == 0 {
			log.Printf("No file descriptor received for service %s", service.Name)
			continue
		}

		// Process file descriptor
		for _, fdBytes := range fdProto.FileDescriptorProto {
			fd := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(fdBytes, fd); err != nil {
				log.Printf("Failed to unmarshal file descriptor: %v", err)
				continue
			}

			log.Printf("    File: %s", fd.GetName())

			// Process services in the file
			for _, svc := range fd.GetService() {
				log.Printf("    Service: %s", svc.GetName())

				// Process methods
				for _, method := range svc.GetMethod() {
					log.Printf("      Method: %s (Input: %s, Output: %s)",
						method.GetName(), method.GetInputType(), method.GetOutputType())
				}
			}
		}
	}

	// Close the stream
	stream.CloseSend()

	return nil
}

// UpdateServiceFromReflection updates service metadata using reflection
func (sm *SchemaManager) UpdateServiceFromReflection(ctx context.Context, serviceName string) error {
	// Use reflection to introspect the service
	if err := sm.introspectService(ctx, serviceName); err != nil {
		return fmt.Errorf("failed to introspect service: %w", err)
	}

	// In a real implementation, you would update the service metadata in your registry
	// and potentially regenerate endpoints based on the discovered methods

	return nil
}
