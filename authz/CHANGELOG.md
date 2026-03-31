# Changelog

All notable changes to the Authz library will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2025-03-14

### Added

- Improved logging system with multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
- Logger interface for better integration with application logging
- Timestamp and log level formatting for all log messages

### Changed

### Fixed

## [0.2.1] - 2025-03-14

### Added

### Changed

### Fixed

## [0.2.0] - 2025-03-14

### Details

- manual release

## [0.1.0] - 2025-03-14

### Added

- Initial library structure
- Agent implementation for policy evaluation
- Support for local and external OPA policy evaluation
- Memory and Redis caching implementations
- HTTP middleware for securing endpoints
- Role provider interface with Redis implementation
- Webhook for dynamic policy updates
- Context transformers for input enrichment
- Comprehensive documentation and examples
- Makefile for development, testing, and release management
- Improved test reliability with miniredis

### Fixed

- Fixed functional options pattern in Redis cache and role provider tests
- Improved test reliability by replacing Docker containers with miniredis
- Fixed expiration tests to use deterministic time control

## [0.1.0-alpha] - 2025-03-14

### Added

- Initial library structure
- Agent implementation for policy evaluation
- Support for local and external OPA policy evaluation
- Memory and Redis caching implementations
- HTTP middleware for securing endpoints
- Role provider interface with Redis implementation
- Webhook for dynamic policy updates
- Context transformers for input enrichment
- Comprehensive documentation and examples
