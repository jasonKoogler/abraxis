#!/bin/bash

# Constants
TESTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SKIP_INTEGRATION=false
SELECTED_TESTS=()

# Show help
function show_help() {
    echo "Usage: $0 [options] [test names...]"
    echo ""
    echo "Options:"
    echo "  --help                  Show this help message"
    echo "  --skip-integration      Skip integration tests that require Docker"
    echo ""
    echo "Example: $0 --skip-integration consul_test etcd_test"
    echo "         This will run only the consul and etcd tests, skipping integration tests."
    echo ""
    echo "If no tests are selected, all tests will be run."
}

# Parse command line arguments
function parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help)
                show_help
                exit 0
                ;;
            --skip-integration)
                SKIP_INTEGRATION=true
                shift
                ;;
            *)
                # If not a flag, assume it's a test name
                SELECTED_TESTS+=("$1")
                shift
                ;;
        esac
    done
}

# Run the tests
function run_tests() {
    cd "$TESTS_DIR"
    
    # Prepare test command
    TEST_CMD="go test -v"
    
    # Add flags
    if [ "$SKIP_INTEGRATION" = true ]; then
        echo "Skipping integration tests..."
        export SKIP_INTEGRATION_TESTS=true
        TEST_CMD="$TEST_CMD -skip-integration"
    fi
    
    # Add selected tests if specified
    if [ ${#SELECTED_TESTS[@]} -gt 0 ]; then
        for test in "${SELECTED_TESTS[@]}"; do
            # If it doesn't end with _test.go, add it
            if [[ "$test" != *_test.go ]]; then
                test="${test}_test.go"
            fi
            TEST_CMD="$TEST_CMD $test"
        done
    fi
    
    # Run the tests
    echo "Running: $TEST_CMD"
    eval "$TEST_CMD"
}

# Main function
function main() {
    parse_args "$@"
    run_tests
}

main "$@" 