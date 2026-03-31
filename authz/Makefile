.PHONY: all test test-verbose test-race test-cover lint fmt vet tidy clean release release-patch release-minor release-major next-dev-patch next-dev-minor next-dev-major docs examples help bench ci-check golangci-lint install-tools git-flow-init feature-start feature-finish hotfix-start hotfix-finish

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt

# Project parameters
PROJECT_NAME=authz
PKG=github.com/jasonKoogler/$(PROJECT_NAME)
VERSION=$(shell grep -E "Version = \"[0-9]+\.[0-9]+\.[0-9]+\"" version.go | grep -Eo "[0-9]+\.[0-9]+\.[0-9]+")
PRERELEASE=$(shell grep -E "VersionPrerelease = \"[^\"]*\"" version.go | grep -Eo "\"[^\"]*\"" | tr -d \")

# Default target
all: test lint

# Help target
help:
	@echo "Available targets:"
	@echo "  all           - Run tests and linting"
	@echo "  test          - Run tests"
	@echo "  test-verbose  - Run tests with verbose output"
	@echo "  test-race     - Run tests with race detection"
	@echo "  test-cover    - Run tests with coverage report"
	@echo "  bench         - Run benchmarks"
	@echo "  lint          - Run linting tools"
	@echo "  golangci-lint - Run golangci-lint"
	@echo "  fmt           - Run go fmt"
	@echo "  vet           - Run go vet"
	@echo "  tidy          - Run go mod tidy"
	@echo "  clean         - Clean build artifacts"
	@echo "  ci-check      - Run CI validation checks"
	@echo "  release       - Create a new release (requires version argument)"
	@echo "  release-patch - Create a new patch release"
	@echo "  release-minor - Create a new minor release"
	@echo "  release-major - Create a new major release"
	@echo "  next-dev-patch - Prepare for next patch development cycle"
	@echo "  next-dev-minor - Prepare for next minor development cycle"
	@echo "  next-dev-major - Prepare for next major development cycle"
	@echo "  docs          - Generate documentation"
	@echo "  examples      - Run examples"
	@echo "  install-tools - Install development tools"
	@echo "  git-flow-init - Initialize Git Flow branches"
	@echo "  feature-start - Start a new feature branch (requires name argument)"
	@echo "  feature-finish - Finish a feature branch (requires name argument)"
	@echo "  hotfix-start  - Start a hotfix branch (requires version argument)"
	@echo "  hotfix-finish - Finish a hotfix branch (requires version argument)"

# Test targets
test:
	$(GOTEST) ./...

test-verbose:
	$(GOTEST) -v ./...

test-race:
	$(GOTEST) -race ./...

test-cover:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Benchmark target
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Lint targets
lint: fmt vet golangci-lint
	@echo "Linting completed"

golangci-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-tools' to install it." && exit 1)
	golangci-lint run ./...

fmt:
	$(GOFMT) ./...

vet:
	$(GOVET) ./...

tidy:
	$(GOMOD) tidy

# CI validation
ci-check: tidy test-race test-cover lint
	@echo "CI validation completed"

# Clean target
clean:
	$(GOCLEAN)
	rm -f coverage.out coverage.html

# Git Flow initialization
git-flow-init:
	@echo "Initializing Git Flow branches..."
	@if git show-ref --verify --quiet refs/heads/master; then \
		echo "Master branch exists"; \
	else \
		echo "Creating master branch..."; \
		git checkout -b master; \
		git push -u origin master; \
	fi
	@if git show-ref --verify --quiet refs/heads/dev; then \
		echo "Dev branch exists"; \
	else \
		echo "Creating dev branch..."; \
		git checkout -b dev; \
		git push -u origin dev; \
	fi
	@echo "Git Flow branches initialized. Current branch is dev."
	@git checkout dev

# Feature branch management
feature-start:
	@if [ -z "$(name)" ]; then \
		echo "Error: feature name is required. Usage: make feature-start name=feature-name"; \
		exit 1; \
	fi
	@echo "Starting feature branch for '$(name)'..."
	@git checkout dev
	@git pull origin dev
	@git checkout -b feature/$(name)
	@echo "Feature branch 'feature/$(name)' created. You can now start working on your feature."

feature-finish:
	@if [ -z "$(name)" ]; then \
		echo "Error: feature name is required. Usage: make feature-finish name=feature-name"; \
		exit 1; \
	fi
	@echo "Finishing feature branch 'feature/$(name)'..."
	@git checkout feature/$(name)
	@git pull origin feature/$(name) || true
	@git checkout dev
	@git pull origin dev
	@git merge --no-ff feature/$(name) -m "Merge feature '$(name)' into dev"
	@git push origin dev
	@echo "Feature '$(name)' merged into dev. You can delete the feature branch with:"
	@echo "  git branch -d feature/$(name)"
	@echo "  git push origin --delete feature/$(name)"

# Hotfix branch management
hotfix-start:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required. Usage: make hotfix-start version=X.Y.Z"; \
		exit 1; \
	fi
	@echo "Starting hotfix branch for version $(version)..."
	@git checkout master
	@git pull origin master
	@git checkout -b hotfix/v$(version)
	@echo "Hotfix branch 'hotfix/v$(version)' created. You can now start working on your hotfix."

hotfix-finish:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required. Usage: make hotfix-finish version=X.Y.Z"; \
		exit 1; \
	fi
	@echo "Finishing hotfix branch 'hotfix/v$(version)'..."
	@git checkout hotfix/v$(version)
	@git pull origin hotfix/v$(version) || true
	@git checkout master
	@git pull origin master
	@git merge --no-ff hotfix/v$(version) -m "Merge hotfix v$(version) into master"
	@git tag -a "v$(version)" -m "Release v$(version) (hotfix)"
	@git push origin master
	@git push origin "v$(version)"
	@git checkout dev
	@git pull origin dev
	@git merge --no-ff hotfix/v$(version) -m "Merge hotfix v$(version) into dev"
	@git push origin dev
	@echo "Hotfix v$(version) merged into master and dev, and tagged. You can delete the hotfix branch with:"
	@echo "  git branch -d hotfix/v$(version)"
	@echo "  git push origin --delete hotfix/v$(version)"

# Release targets
release:
	@if [ -z "$(version)" ]; then \
		echo "Error: version argument is required. Usage: make release version=X.Y.Z"; \
		exit 1; \
	fi
	@echo "Creating release v$(version)"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to create a release."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/release.sh $(version)

release-patch:
	@echo "Creating patch release"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to create a release."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/version.sh bump patch
	@VERSION=$$(grep -E "Version = \"[0-9]+\.[0-9]+\.[0-9]+\"" version.go | grep -Eo "[0-9]+\.[0-9]+\.[0-9]+"); \
	./scripts/release.sh $$VERSION

release-minor:
	@echo "Creating minor release"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to create a release."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/version.sh bump minor
	@VERSION=$$(grep -E "Version = \"[0-9]+\.[0-9]+\.[0-9]+\"" version.go | grep -Eo "[0-9]+\.[0-9]+\.[0-9]+"); \
	./scripts/release.sh $$VERSION

release-major:
	@echo "Creating major release"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to create a release."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/version.sh bump major
	@VERSION=$$(grep -E "Version = \"[0-9]+\.[0-9]+\.[0-9]+\"" version.go | grep -Eo "[0-9]+\.[0-9]+\.[0-9]+"); \
	./scripts/release.sh $$VERSION

# Next development cycle targets
next-dev-patch:
	@echo "Preparing for next patch development cycle"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to prepare for the next development cycle."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/next-dev.sh patch

next-dev-minor:
	@echo "Preparing for next minor development cycle"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to prepare for the next development cycle."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/next-dev.sh minor

next-dev-major:
	@echo "Preparing for next major development cycle"
	@# Check if we're on dev branch
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "dev" ]; then \
		echo "Error: You must be on the dev branch to prepare for the next development cycle."; \
		echo "Current branch is $$current_branch. Please checkout dev first."; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if ! git diff-index --quiet HEAD --; then \
		echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."; \
		exit 1; \
	fi
	./scripts/next-dev.sh major

# Documentation target
docs:
	@echo "Generating documentation"
	@mkdir -p docs
	$(GOCMD) doc -all > docs/api.txt
	@echo "Documentation generated in docs/api.txt"

# Examples target
examples:
	@echo "Running examples"
	$(GOCMD) run ./examples/basic/...

# Version info
version:
	@if [ -z "$(PRERELEASE)" ]; then \
		echo "Current version: v$(VERSION)"; \
	else \
		echo "Current version: v$(VERSION)-$(PRERELEASE)"; \
	fi

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed successfully" 