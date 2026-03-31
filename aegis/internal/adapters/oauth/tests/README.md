# OAuth Tests

This directory contains unit tests for the OAuth implementation in the `internal/adapters/oauth` package.

## Test Files

- `mocks_test.go`: Contains mock implementations of interfaces used throughout the tests
- `providers_test.go`: Tests for the OAuth provider configuration and functionality
- `manager_test.go`: Tests for the OAuth manager functionality (generating auth URLs, exchanging codes, refreshing tokens)
- `userinfo_test.go`: Tests for the user information retrieval functionality
- `verifier_test.go`: Tests for the verifier storage implementations (Memory and Redis)

## Running Tests

To run all tests in this package:

```bash
go test -v github.com/jasonKoogler/gauth/internal/adapters/oauth/tests
```

To run a specific test file:

```bash
go test -v github.com/jasonKoogler/gauth/internal/adapters/oauth/tests -run TestProviders
```

## Test Coverage

To generate test coverage reports:

```bash
go test -coverprofile=coverage.out github.com/jasonKoogler/gauth/internal/adapters/oauth/tests
go tool cover -html=coverage.out
```

## Dependencies

The tests have the following dependencies:

- `github.com/stretchr/testify/assert`: For test assertions
- `github.com/stretchr/testify/mock`: For mocking interfaces

Make sure these dependencies are installed using:

```bash
go get github.com/stretchr/testify
```

## Notes

1. The tests use a combination of real implementations, mocks, and test servers to verify functionality.
2. Some tests for network-related functionality like token exchange might be disabled by default.
3. Redis-related tests will require a Redis server to be running, or they will be skipped.

## TODO

- Add integration tests that test with actual OAuth providers (using test accounts)
- Add more edge case tests for error handling
- Improve test coverage for the Facebook and Twitter provider implementations
