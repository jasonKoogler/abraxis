# Makefile Documentation

This document provides information on how to use the Makefile for common development tasks in the Authz library.

## Prerequisites

- Go 1.24 or later
- Make

## Available Commands

### Basic Commands

```bash
# Run tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with race detection
make test-race

# Run tests with coverage report
make test-cover

# Run benchmarks
make bench

# Run linting tools
make lint

# Run go fmt
make fmt

# Run go vet
make vet

# Run golangci-lint
make golangci-lint

# Run go mod tidy
make tidy

# Clean build artifacts
make clean

# Run CI validation checks
make ci-check

# Install development tools
make install-tools
```

### Release Commands

```bash
# Create a new release with a specific version
make release version=1.0.0

# Create a new patch release (automatically increments the patch version)
make release-patch

# Create a new minor release (automatically increments the minor version)
make release-minor

# Create a new major release (automatically increments the major version)
make release-major

# Prepare for next patch development cycle
make next-dev-patch

# Prepare for next minor development cycle
make next-dev-minor

# Prepare for next major development cycle
make next-dev-major
```

### Documentation and Examples

```bash
# Generate documentation
make docs

# Run examples
make examples

# Show current version
make version
```

## Release Process

The release process involves the following steps:

1. Ensure all tests pass: `make test`
2. Ensure linting passes: `make lint`
3. Create a release: `make release version=X.Y.Z`
4. Push changes and tags to the repository:
   ```bash
   git push origin master
   git push origin vX.Y.Z
   ```
5. Prepare for the next development cycle: `make next-dev-minor` (or patch/major as appropriate)

## Continuous Integration

For CI environments, use the `ci-check` target which runs a comprehensive set of checks:

```bash
make ci-check
```

This will:

- Ensure the go.mod file is tidy
- Run tests with race detection
- Generate a coverage report
- Run all linting tools

## Development Workflow

A typical development workflow might look like:

1. Make changes to the code
2. Run tests: `make test`
3. Run linting: `make lint`
4. Fix any issues
5. Commit changes
6. Before submitting a pull request, run: `make ci-check`

## Getting Help

To see all available commands:

```bash
make help
```
