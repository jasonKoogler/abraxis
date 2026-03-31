# Makefile for discovery adapter tests
.PHONY: test test-local test-integration test-consul test-etcd test-kube test-all test-with-race

# Base test command
GO_TEST = go test -v
TEST_DIR = ./

# Run only the local tests (no external dependencies)
test-local:
	$(GO_TEST) $(TEST_DIR) -run TestLocalDiscovery

# Run integration tests
test-integration:
	$(GO_TEST) $(TEST_DIR) -run TestDiscoveryIntegration

# Run Consul tests
test-consul:
	$(GO_TEST) $(TEST_DIR) -run TestConsul

# Run etcd tests
test-etcd:
	$(GO_TEST) $(TEST_DIR) -run TestEtcd

# Run Kubernetes tests
test-kube:
	$(GO_TEST) $(TEST_DIR) -run TestKubernetes

# Run all tests
test-all:
	$(GO_TEST) $(TEST_DIR)

# Run with race detection
test-with-race:
	go test -race -v $(TEST_DIR)

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out $(TEST_DIR)
	go tool cover -html=coverage.out

# Default target
test: test-local

# Help
help:
	@echo "Available targets:"
	@echo "  test          - Run local tests (default)"
	@echo "  test-local    - Run only local discovery tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-consul   - Run Consul tests"
	@echo "  test-etcd     - Run etcd tests" 
	@echo "  test-kube     - Run Kubernetes tests"
	@echo "  test-all      - Run all tests"
	@echo "  test-with-race - Run tests with race detection"
	@echo "  test-coverage - Run tests with coverage report" 