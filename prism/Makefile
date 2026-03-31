-include .env
export

SHELL := /bin/bash
.DEFAULT_GOAL := help
MAKE := /usr/bin/make

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get

# Application parameters
APP_NAME := myapp
BUILD_DIR := build
MAIN_PATH := ./cmd

# Define project root directory and shell
ROOT_DIR := $(shell pwd)
SHELL := /bin/bash

# Available features (must match directory names in internal/features)
FEATURES := auth user tenant gateway audit apikey discovery ratelimit schema

# ------------------------------------------------------------
# Tests
# ------------------------------------------------------------
.PHONY: test-discovery
test-discovery: ## Run discovery tests (excluding integration tests)
	$(MAKE) -C internal/adapters/discovery/tests test

.PHONY: test-discovery-all
test-discovery-all: ## Run all discovery tests including integration tests
	$(MAKE) -C internal/adapters/discovery/tests test-all

.PHONY: test-discovery-integration
test-discovery-integration: ## Run discovery integration tests
	$(MAKE) -C internal/adapters/discovery/tests test-integration

.PHONY: test-discovery-consul
test-discovery-consul: ## Run discovery Consul tests
	$(MAKE) -C internal/adapters/discovery/tests test-consul

.PHONY: test-discovery-etcd
test-discovery-etcd: ## Run discovery etcd tests
	$(MAKE) -C internal/adapters/discovery/tests test-etcd

.PHONY: test-discovery-local
test-discovery-local: ## Run discovery local tests
	$(MAKE) -C internal/adapters/discovery/tests test-local

.PHONY: test-with-race
test-with-race: ## Run tests with race detection
	$(GOTEST) -race -v ./...

# ------------------------------------------------------------
# Swagger
# ------------------------------------------------------------
.PHONY: swagger
swagger: ## Generate swagger docs from swaggo annotations
	swag init -g cmd/main.go --parseDependency --parseInternal --useStructName

.PHONY: sdk
sdk: swagger ## Generate swagger docs and TypeScript SDK
	@echo "TypeScript SDK generation can be added here (openapi-ts)"


# ------------------------------------------------------------
# Protocol Buffers
# ------------------------------------------------------------
.PHONY: proto
proto: ensure-proto-deps ## Generate Go code from proto files
	@$(ROOT_DIR)/scripts/protogen.sh
	@echo "✨ Proto code generation complete!"

.PHONY: ensure-proto-deps
ensure-proto-deps: ## Ensure protobuf dependencies are installed
	@echo "Checking protobuf dependencies..."
	@if ! command -v protoc >/dev/null 2>&1; then \
		echo "Warning: protoc is not installed."; \
		echo "Please install manually from: https://grpc.io/docs/protoc-installation/"; \
		exit 1; \
	fi
	@if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi
	@if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi
	@if ! command -v git >/dev/null 2>&1; then \
		echo "Warning: git is not installed."; \
		echo "Please install git using your package manager."; \
		exit 1; \
	fi

.PHONY: install-proto-tools
install-proto-tools: ## Install protobuf code generation tools
	@echo "Installing Go protobuf plugins..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Go plugins installed. Note: You still need to install the protoc compiler manually."
	@echo "   Visit: https://grpc.io/docs/protoc-installation/ for installation instructions."

# ------------------------------------------------------------
# Clean
# ------------------------------------------------------------
.PHONY: clean
clean: ## Clean up generated files and build directory
	@rm -rf $(ROOT_DIR)/docs/
	@find $(ROOT_DIR)/internal/ports/proto -name "*.pb.go" -type f -delete 2>/dev/null || true
	@echo "Cleaned generated files"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Cleaned build directory"

.PHONY: clean-third-party
clean-third-party: ## Remove third-party dependencies (like googleapis)
	@rm -rf $(ROOT_DIR)/third_party
	@echo "🧹 Removed third-party dependencies"

# ------------------------------------------------------------
# Build
# ------------------------------------------------------------
.PHONY: all
all: test build ## Run tests and build the application

.PHONY: build
build: ## Build the application
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

# .PHONY: test
# test: ## Run all tests
# 	$(GOTEST) -v ./...


.PHONY: prod
prod: ## Run production environment
	@$(MAKE) -C docker/prod

# ------------------------------------------------------------
# Auth Docker Compose
# ------------------------------------------------------------
.PHONY: run-auth-dev
run-auth-dev: ## Run Auth development environment with docker-compose
	@$(MAKE) -C docker/auth/dev up

.PHONY: stop-auth-dev
stop-auth-dev: ## Stop Auth development environment
	@$(MAKE) -C docker/auth/dev down

.PHONY: run-auth-prod
run-auth-prod: ## Run Auth production environment with docker-compose
	@$(MAKE) -C docker/auth/prod up

.PHONY: stop-auth-prod
stop-auth-prod: ## Stop Auth production environment
	@$(MAKE) -C docker/auth/prod down

# ------------------------------------------------------------
# Organization Docker Compose
# ------------------------------------------------------------
.PHONY: run-org-dev
run-org-dev: ## Run Organization development environment with docker-compose
	# @$(MAKE) -C docker/org/dev setup-volumes
	# @$(MAKE) -C docker/org/dev up
	cd docker/org/dev && $(MAKE) setup-volumes
	cd docker/org/dev && $(MAKE) up

.PHONY: run-org-prod
run-org-prod: ## Run Organization production environment with docker-compose
	@$(MAKE) -C docker/org/prod up

.PHONY: stop-org-dev
stop-org-dev: ## Stop Organization development environment
	# @$(MAKE) -C docker/org/dev fix-permissions
	@$(MAKE) -C docker/org/dev down

.PHONY: stop-org-prod
stop-org-prod: ## Stop Organization production environment
	@$(MAKE) -C docker/org/prod down

# ------------------------------------------------------------
# Help
# ------------------------------------------------------------
.PHONY: help
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m%-30s\033[0m %s\n", "Target", "Description"} /^[a-zA-Z_-]+:.*?##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: help-auth-dev
help-auth-dev: ## Display Auth development environment help screen
	@$(MAKE) -C docker/auth/dev help

.PHONY: help-auth-prod
help-auth-prod: ## Display Auth production environment help screen
	@$(MAKE) -C docker/auth/prod help

.PHONY: help-org-dev
help-org-dev: ## Display Organization development environment help screen
	@$(MAKE) -C docker/org/dev help

.PHONY: help-org-prod
help-org-prod: ## Display Organization production environment help screen
	@$(MAKE) -C docker/org/prod help


.PHONY: remove-persistant-volumes
remove-persistant-volumes: ## Remove persistant volumes
	@sudo rm -rf ./redis-data ./pg-data


# .PHONY: setup-volumes
# setup-volumes: ## Setup persistant volumes
# 	@sudo chmod 777 ./redis-data ./pg-data

.PHONY: docs
docs: swagger ## Generate docs (alias for swagger)


# ------------------------------------------------------------
# Test
# ------------------------------------------------------------
.PHONY: test
test: ## Run all tests
	go test -race ./... -count=1

.PHONY: test-verbose
test-verbose: ## Run all tests in verbose mode
	go test -v -race ./... -count=1

# Run tests with coverage report
test-coverage:
	go test -race -coverprofile=coverage.out ./... -count=1
	go tool cover -html=coverage.out -o coverage.html
	rm coverage.out

# Clean test cache
test-clean:
	go clean -testcache

.PHONY: test-org
test-org: ## Run Organization tests
	@go test -v ./internal/test/servicetest -run ^TestOrganizationService$ -count=1

.PHONY: test-resource
test-resource: ## Run Resource tests
	@go test -v ./internal/test/servicetest -run ^TestResourceService$


.PHONY: test-employee
test-employee: ## Run Employee tests
	@go test -v ./internal/test/servicetest -run ^TestEmployeeService$

.PHONY: test-membership
test-membership: ## Run Membership tests
	@go test -v ./internal/test/servicetest -run ^TestMembershipService$

.PHONY: test-db
test-db: ## Run all database tests
	@go test -v ./internal/test/servicetest -count=1
