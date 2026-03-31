package discovery

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/protocompile"
)

// ProtoService handles protocol buffer processing
type ProtoService struct {
	workDir     string
	includeArgs []string
	mu          sync.Mutex
}

// CompileResult represents the result of a protobuf compilation
type CompileResult struct {
	Language      string
	OutputFiles   map[string][]byte
	GeneratedDeps []string
	Errors        []string
}

// NewProtoService creates a new protocol buffer service
func NewProtoService(opts ...Option) *ProtoService {
	service := &ProtoService{
		workDir:     os.TempDir(),
		includeArgs: []string{},
	}

	// Apply options
	for _, opt := range opts {
		opt(service)
	}

	return service
}

// Option is a function type for configuring the proto service
type Option func(*ProtoService)

// WithWorkDir sets the working directory for proto compilation
func WithWorkDir(dir string) Option {
	return func(ps *ProtoService) {
		ps.workDir = dir
	}
}

// WithIncludePaths adds additional include paths for imports
func WithIncludePaths(paths ...string) Option {
	return func(ps *ProtoService) {
		for _, path := range paths {
			ps.includeArgs = append(ps.includeArgs, "-I"+path)
		}
	}
}

// Parse parses a proto file and returns its descriptor
func (ps *ProtoService) Parse(ctx context.Context, name string, content []byte) (protoreflect.FileDescriptor, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp(ps.workDir, "proto-parse-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write the proto file to the temp directory
	tempFile := filepath.Join(tempDir, name)
	if err := os.MkdirAll(filepath.Dir(tempFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write proto file: %w", err)
	}

	// Use protocompile to parse the file
	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			ImportPaths: append([]string{tempDir}, ps.getImportPaths()...),
		},
	}

	// Parse the file
	files, err := compiler.Compile(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no file descriptors found")
	}

	// Return the file descriptor
	return files[0], nil
}

// Helper to extract import paths from includeArgs
func (ps *ProtoService) getImportPaths() []string {
	paths := make([]string, 0, len(ps.includeArgs))
	for _, arg := range ps.includeArgs {
		if strings.HasPrefix(arg, "-I") {
			paths = append(paths, arg[2:])
		}
	}
	return paths
}

// ValidateCompatibility checks if a new proto definition is compatible with an old one
func (ps *ProtoService) ValidateCompatibility(ctx context.Context, name string, oldContent, newContent []byte) (bool, []string, error) {
	// Parse old and new proto files
	oldFD, err := ps.Parse(ctx, name, oldContent)
	if err != nil {
		return false, []string{fmt.Sprintf("Failed to parse old proto: %v", err)}, err
	}

	newFD, err := ps.Parse(ctx, name, newContent)
	if err != nil {
		return false, []string{fmt.Sprintf("Failed to parse new proto: %v", err)}, err
	}

	// Check compatibility
	issues := checkBackwardCompatibility(oldFD, newFD)
	return len(issues) == 0, issues, nil
}

// Compile compiles a set of proto files into target languages
func (ps *ProtoService) Compile(ctx context.Context, files map[string][]byte, languages []string, options map[string]string) (map[string]*CompileResult, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp(ps.workDir, "proto-compile-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create output directory
	outDir := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write proto files to temp directory
	protoFiles := []string{}
	for name, content := range files {
		filePath := filepath.Join(tempDir, name)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", name, err)
		}

		// Write file
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write proto file %s: %w", name, err)
		}

		protoFiles = append(protoFiles, name)
	}

	// Results
	results := make(map[string]*CompileResult)

	// Compile for each language
	for _, lang := range languages {
		langOutDir := filepath.Join(outDir, lang)
		if err := os.MkdirAll(langOutDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory for %s: %w", lang, err)
		}

		result, err := ps.compileForLanguage(ctx, tempDir, langOutDir, protoFiles, lang, options)
		if err != nil {
			return nil, fmt.Errorf("failed to compile for %s: %w", lang, err)
		}

		results[lang] = result
	}

	return results, nil
}

// compileForLanguage compiles proto files for a specific language
func (ps *ProtoService) compileForLanguage(
	ctx context.Context,
	baseDir,
	outDir string,
	protoFiles []string,
	language string,
	options map[string]string,
) (*CompileResult, error) {
	result := &CompileResult{
		Language:    language,
		OutputFiles: make(map[string][]byte),
		Errors:      []string{},
	}

	// Build protoc command
	args := []string{
		"-I" + baseDir,
	}

	// Add standard include paths
	args = append(args, ps.includeArgs...)

	switch language {
	case "go":
		// Go options
		goOut := "--go_out=paths=source_relative:" + outDir
		goGrpcOut := "--go-grpc_out=paths=source_relative:" + outDir

		if modulePrefix, ok := options["go_module_prefix"]; ok && modulePrefix != "" {
			goOut = fmt.Sprintf("--go_out=module=%s,paths=source_relative:%s", modulePrefix, outDir)
			goGrpcOut = fmt.Sprintf("--go-grpc_out=module=%s,paths=source_relative:%s", modulePrefix, outDir)
		}

		args = append(args, goOut, goGrpcOut)

		// Add gRPC gateway if requested
		if _, ok := options["gateway"]; ok {
			gatewayOut := "--grpc-gateway_out=paths=source_relative:" + outDir
			openAPIOut := "--openapiv2_out=" + outDir

			args = append(args, gatewayOut, openAPIOut)
		}

	case "python":
		// Python options
		pythonOut := "--python_out=" + outDir
		grpcPythonOut := "--grpc_python_out=" + outDir

		args = append(args, pythonOut, grpcPythonOut)

	case "java":
		// Java options
		javaOut := "--java_out=" + outDir
		grpcJavaOut := "--grpc-java_out=" + outDir

		if javaPackage, ok := options["java_package"]; ok && javaPackage != "" {
			javaOut = fmt.Sprintf("--java_out=%s:%s", javaPackage, outDir)
		}

		args = append(args, javaOut, grpcJavaOut)

	case "typescript":
		// TypeScript options
		tsOut := "--ts_out=" + outDir

		if tsOpts, ok := options["ts_options"]; ok && tsOpts != "" {
			tsOut = fmt.Sprintf("--ts_out=%s:%s", tsOpts, outDir)
		}

		args = append(args, tsOut)

	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Add proto files
	args = append(args, protoFiles...)

	// Run protoc
	cmd := exec.CommandContext(ctx, "protoc", args...)
	cmd.Dir = baseDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		result.Errors = append(result.Errors, stderr.String())
		return result, fmt.Errorf("protoc failed: %w\n%s", err, stderr.String())
	}

	// Collect generated files
	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(outDir, path)
			if err != nil {
				return err
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			result.OutputFiles[relPath] = content
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to collect generated files: %w", err)
	}

	return result, nil
}

// checkBackwardCompatibility checks if the new proto definition is backward compatible with the old one
func checkBackwardCompatibility(oldFD, newFD protoreflect.FileDescriptor) []string {
	var issues []string

	// Check package
	if oldFD.Package() != newFD.Package() {
		issues = append(issues, fmt.Sprintf("Package changed from '%s' to '%s'", oldFD.Package(), newFD.Package()))
	}

	// Check messages
	oldMessages := getMessageMap(oldFD)
	newMessages := getMessageMap(newFD)

	for name, oldMsg := range oldMessages {
		newMsg, exists := newMessages[name]

		if !exists {
			issues = append(issues, fmt.Sprintf("Message '%s' was removed", name))
			continue
		}

		// Check fields
		oldFields := oldMsg.Fields()
		for i := 0; i < oldFields.Len(); i++ {
			oldField := oldFields.Get(i)
			newField := newMsg.Fields().ByNumber(oldField.Number())

			if newField == nil {
				issues = append(issues, fmt.Sprintf("Field %d (%s) was removed from message '%s'",
					oldField.Number(), oldField.Name(), name))
				continue
			}

			// Check field type
			if oldField.Kind() != newField.Kind() {
				issues = append(issues, fmt.Sprintf("Field %d (%s) in message '%s' changed type from %s to %s",
					oldField.Number(), oldField.Name(), name,
					oldField.Kind().String(), newField.Kind().String()))
			}
		}
	}

	// Check services
	oldServices := getServiceMap(oldFD)
	newServices := getServiceMap(newFD)

	for name, oldSvc := range oldServices {
		newSvc, exists := newServices[name]

		if !exists {
			issues = append(issues, fmt.Sprintf("Service '%s' was removed", name))
			continue
		}

		// Check methods
		oldMethods := oldSvc.Methods()
		for i := 0; i < oldMethods.Len(); i++ {
			oldMethod := oldMethods.Get(i)
			newMethod := findMethodByName(newSvc.Methods(), oldMethod.Name())

			if newMethod == nil {
				issues = append(issues, fmt.Sprintf("Method '%s' was removed from service '%s'",
					oldMethod.Name(), name))
				continue
			}

			// Check method input/output types
			if oldMethod.Input().FullName() != newMethod.Input().FullName() {
				issues = append(issues, fmt.Sprintf("Method '%s' in service '%s' changed input type from %s to %s",
					oldMethod.Name(), name,
					oldMethod.Input().FullName(),
					newMethod.Input().FullName()))
			}

			if oldMethod.Output().FullName() != newMethod.Output().FullName() {
				issues = append(issues, fmt.Sprintf("Method '%s' in service '%s' changed output type from %s to %s",
					oldMethod.Name(), name,
					oldMethod.Output().FullName(),
					newMethod.Output().FullName()))
			}
		}
	}

	return issues
}

// Helper function to find a method by name
func findMethodByName(methods protoreflect.MethodDescriptors, name protoreflect.Name) protoreflect.MethodDescriptor {
	for i := 0; i < methods.Len(); i++ {
		method := methods.Get(i)
		if method.Name() == name {
			return method
		}
	}
	return nil
}

// Helper functions to create maps of messages and services
func getMessageMap(fd protoreflect.FileDescriptor) map[string]protoreflect.MessageDescriptor {
	result := make(map[string]protoreflect.MessageDescriptor)
	messages := fd.Messages()

	for i := 0; i < messages.Len(); i++ {
		msg := messages.Get(i)
		result[string(msg.FullName())] = msg

		// Add nested messages
		addNestedMessages(msg, result)
	}

	return result
}

func addNestedMessages(msg protoreflect.MessageDescriptor, result map[string]protoreflect.MessageDescriptor) {
	nestedMsgs := msg.Messages()
	for i := 0; i < nestedMsgs.Len(); i++ {
		nested := nestedMsgs.Get(i)
		result[string(nested.FullName())] = nested

		// Recursively add nested messages
		addNestedMessages(nested, result)
	}
}

func getServiceMap(fd protoreflect.FileDescriptor) map[string]protoreflect.ServiceDescriptor {
	result := make(map[string]protoreflect.ServiceDescriptor)
	services := fd.Services()

	for i := 0; i < services.Len(); i++ {
		svc := services.Get(i)
		result[string(svc.FullName())] = svc
	}

	return result
}

// GenerateOpenAPI generates OpenAPI specification from proto files
func (ps *ProtoService) GenerateOpenAPI(ctx context.Context, files map[string][]byte) ([]byte, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp(ps.workDir, "proto-openapi-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create output directory
	outDir := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write proto files to temp directory
	protoFiles := []string{}
	for name, content := range files {
		filePath := filepath.Join(tempDir, name)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", name, err)
		}

		// Write file
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write proto file %s: %w", name, err)
		}

		protoFiles = append(protoFiles, name)
	}

	// Build protoc command
	args := []string{
		"-I" + tempDir,
	}

	// Add standard include paths
	args = append(args, ps.includeArgs...)

	// Add OpenAPI output
	openAPIOut := "--openapiv2_out=" + outDir
	args = append(args, openAPIOut)

	// Add proto files
	args = append(args, protoFiles...)

	// Run protoc
	cmd := exec.CommandContext(ctx, "protoc", args...)
	cmd.Dir = tempDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("protoc failed: %w\n%s", err, stderr.String())
	}

	// Find the generated OpenAPI spec
	var openAPISpec []byte
	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".swagger.json") {
			openAPISpec, err = os.ReadFile(path)
			if err != nil {
				return err
			}

			// Found what we need, no need to continue walking
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find generated OpenAPI spec: %w", err)
	}

	if openAPISpec == nil {
		return nil, fmt.Errorf("no OpenAPI spec was generated")
	}

	return openAPISpec, nil
}
