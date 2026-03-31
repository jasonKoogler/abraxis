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

# Application parameters
APP_NAME := myapp
BUILD_DIR := build
MAIN_PATH := ./cmd

# Define project root directory
ROOT_DIR := $(shell pwd)

# ------------------------------------------------------------
# Build
# ------------------------------------------------------------
.PHONY: all
all: test build ## Run tests and build the application

.PHONY: build
build: ## Build the application
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

# ------------------------------------------------------------
# Tests
# ------------------------------------------------------------
.PHONY: test
test: ## Run all tests
	go test -race ./... -count=1

.PHONY: test-verbose
test-verbose: ## Run all tests in verbose mode
	go test -v -race ./... -count=1

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out ./... -count=1
	go tool cover -html=coverage.out -o coverage.html
	rm coverage.out

.PHONY: test-with-race
test-with-race: ## Run tests with race detection
	$(GOTEST) -race -v ./...

# ------------------------------------------------------------
# Protocol Buffers
# ------------------------------------------------------------
.PHONY: proto
proto: ensure-proto-deps ## Generate Go code from proto files
	@$(ROOT_DIR)/scripts/protogen.sh
	@echo "Proto code generation complete!"

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
	@echo "Go plugins installed. Note: You still need to install the protoc compiler manually."
	@echo "   Visit: https://grpc.io/docs/protoc-installation/ for installation instructions."

# ------------------------------------------------------------
# Swagger
# ------------------------------------------------------------
.PHONY: swagger
swagger: ## Generate swagger docs via swag init
	GOWORK=off swag init -g cmd/main.go --parseDependency --parseInternal --useStructName

# ------------------------------------------------------------
# Clean
# ------------------------------------------------------------
.PHONY: clean
clean: ## Clean up generated files and build directory
	@find $(ROOT_DIR)/internal/ports/proto -name "*.pb.go" -type f -delete 2>/dev/null || true
	@rm -rf $(ROOT_DIR)/docs/docs.go $(ROOT_DIR)/docs/swagger.json $(ROOT_DIR)/docs/swagger.yaml
	@echo "Cleaned generated files"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Cleaned build directory"

.PHONY: clean-third-party
clean-third-party: ## Remove third-party dependencies (like googleapis)
	@rm -rf $(ROOT_DIR)/third_party
	@echo "Removed third-party dependencies"

.PHONY: remove-persistant-volumes
remove-persistant-volumes: ## Remove persistant volumes
	@sudo rm -rf ./redis-data ./pg-data

# ------------------------------------------------------------
# Help
# ------------------------------------------------------------
.PHONY: help
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m%-30s\033[0m %s\n", "Target", "Description"} /^[a-zA-Z_-]+:.*?##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
