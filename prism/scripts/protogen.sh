#!/bin/bash

set -eu

# Constants - using absolute paths from project root
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT_DIR}/api/grpc/proto"
OUTPUT_DIR="${ROOT_DIR}/internal/ports/proto"
THIRD_PARTY_DIR="${ROOT_DIR}/third_party"
GOOGLEAPIS_DIR="${THIRD_PARTY_DIR}/googleapis"

# Function to check if required tools are installed
check_dependencies() {
    local deps=("protoc" "protoc-gen-go" "protoc-gen-go-grpc" "git")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            echo "Error: Required dependency '$dep' is not installed."
            echo "Please install the missing dependencies:"
            echo "  - protoc: https://grpc.io/docs/protoc-installation/"
            echo "  - Go plugins: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
            echo "                go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
            echo "  - git: Use your package manager to install git"
            exit 1
        fi
    done
}

# Function to ensure googleapis are available
ensure_googleapis() {
    if [ ! -d "$GOOGLEAPIS_DIR" ]; then
        echo "Google APIs not found, downloading to ${GOOGLEAPIS_DIR}..."
        mkdir -p "$THIRD_PARTY_DIR"
        git clone --depth 1 https://github.com/googleapis/googleapis.git "$GOOGLEAPIS_DIR"
        echo "✅ Google APIs downloaded."
    else
        echo "Google APIs found at ${GOOGLEAPIS_DIR}"
    fi
}

# Function to clean output directory
clean_output_dir() {
    echo "Cleaning output directory..."
    if [ -d "$OUTPUT_DIR" ]; then
        rm -rf "${OUTPUT_DIR:?}/"*
    else
        mkdir -p "$OUTPUT_DIR"
    fi
}

# Function to find all proto files
find_proto_files() {
    find "$PROTO_DIR" -name "*.proto" -type f
}

# Function to generate code for a single proto file
generate_proto() {
    local proto_file="$1"
    local rel_path=$(realpath --relative-to="$PROTO_DIR" "$proto_file")
    local proto_dir=$(dirname "$rel_path")
    
    echo "Generating code for $rel_path..."
    
    # Create output subdirectory if needed
    if [ "$proto_dir" != "." ]; then
        mkdir -p "${OUTPUT_DIR}/${proto_dir}"
    fi
    
    # Run protoc with Go plugins
    # Using explicit include paths for both our proto files and Google APIs
    protoc \
        -I"$PROTO_DIR" \
        -I"$GOOGLEAPIS_DIR" \
        -I/usr/local/include \
        --go_out="${OUTPUT_DIR}" \
        --go_opt=paths=source_relative \
        --go-grpc_out="${OUTPUT_DIR}" \
        --go-grpc_opt=paths=source_relative \
        "$proto_file"
}

# Function to generate Go code from all proto files
generate_all_protos() {
    echo "Generating Go code from proto files..."
    local proto_files=$(find_proto_files)
    
    if [ -z "$proto_files" ]; then
        echo "No proto files found in $PROTO_DIR"
        exit 1
    fi
    
    for proto_file in $proto_files; do
        generate_proto "$proto_file"
    done
}

# Main function
main() {
    echo "Starting protobuf code generation..."
    
    # Check if dependencies are installed
    check_dependencies
    
    # Ensure we have googleapis for imports
    ensure_googleapis
    
    # Clean output directory
    clean_output_dir
    
    # Generate code
    generate_all_protos
    
    echo "✨ Protobuf code generation complete!"
    echo "Generated Go files are in: $OUTPUT_DIR"
}

# Run the main function
main "$@" 