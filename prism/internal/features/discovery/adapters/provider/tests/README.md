# Discovery Adapter Tests

This directory contains tests for the discovery service adapters, including integration tests with real service discovery servers running in Docker containers.

## Quick Start

To run the local tests only (no Docker needed):

```bash
make test
```

To run all tests (requires Docker):

```bash
make test-all
```

## Test Organization

The tests are organized as follows:

- `main_test.go` - Main test runner that sets up the test environment
- `local_test.go` - Tests for the in-memory local discovery provider
- `consul_test.go` - Tests for the Consul discovery adapter
- `etcd_test.go` - Tests for the etcd discovery adapter
- `kube_test.go` - Tests for the Kubernetes discovery adapter
- `integration_test.go` - End-to-end integration tests
- `dockertest_helper.go` - Helper functions for Docker integration tests
- `Makefile` - Simplifies test execution with various targets
- `run-tests.sh` - Legacy script to run the tests with various options

## Running Tests with Make

The following Make targets are available:

```bash
# Run only local tests (default)
make test

# Run only local discovery tests (same as above)
make test-local

# Run integration tests
make test-integration

# Run Consul tests
make test-consul

# Run etcd tests 
make test-etcd

# Run Kubernetes tests
make test-kube

# Run all tests
make test-all

# Run tests with race detection
make test-with-race

# Run tests with coverage report
make test-coverage
```

## Skipping Integration Tests

Integration tests require Docker to be installed and running. If Docker is not available, the integration tests will be automatically skipped.

To explicitly skip integration tests:

```bash
# Using the legacy script
./run-tests.sh --skip-integration

# Or set the environment variable
SKIP_INTEGRATION_TESTS=true go test ./...
```

## Integration Test Setup

The integration tests use `dockertest` to create and manage Docker containers for each service discovery provider:

- **Consul**: A containerized Consul server running on a random port.
- **etcd**: A containerized etcd server running on a random port.
- **Kubernetes**: Uses the K8s client-go fake client for testing, as running a full Kubernetes cluster in a container is complex.

The Docker containers are automatically created before the tests and cleaned up after the tests complete.

## Kubernetes Testing

For Kubernetes, we use two approaches:

1. Integration tests with a real K3s container (when Docker is available)
2. Unit tests using Kubernetes client-go's fake client (for testing without Docker)

## Troubleshooting

### Docker Issues

If you're running into issues with Docker-based tests:

1. Ensure Docker is installed and running: `docker ps`
2. Check for permission issues (you might need to run with sudo)
3. Try increasing Docker timeouts if container startup is slow

### Test Failures

For test failures:

1. Run specific failing tests with verbose output: `go test -v -run TestSpecificTest`
2. Check for timing issues (tests might be sensitive to system load)
3. Inspect test logs for detailed error messages

### Common Issues

1. **"no such host"**: Docker daemon is not running
2. **"context deadline exceeded"**: Container startup took too long
3. **"port already in use"**: Another process is using the required port

## Writing New Tests

When writing new tests:

1. Create a new test file or add tests to an existing file.
2. Use the `DockerResources` type for accessing Docker containers.
3. For integration tests, check for Docker availability:

```go
if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
    t.Skip("Skipping integration test")
}
```

4. Use the helper functions in `dockertest_helper.go` to set up containers.
5. Make your tests robust by adding proper cleanup and using retries where appropriate.

## Contributing

When adding new tests, please follow these guidelines:

1. Test both happy path and error cases
2. Include tests for edge cases
3. Use mocks for external dependencies
4. Keep tests isolated and independent
5. Clean up resources in tests that create them 