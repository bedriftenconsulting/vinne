#!/bin/bash

# Script to generate all proto files for the microservices
# This script ensures all proto files are generated consistently

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔧 Generating proto files for all services..."

# Get the root directory
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}❌ protoc is not installed. Please install Protocol Buffers compiler.${NC}"
    exit 1
fi

# Ensure Go bin is in PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Check if Go plugins are installed
if ! command -v protoc-gen-go &> /dev/null || ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}⚠️  Installing Go protoc plugins...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Function to generate proto for a specific path
generate_proto() {
    local proto_file="$1"
    local proto_dir=$(dirname "$proto_file")

    echo "  Generating: $proto_file"

    protoc \
        --go_out=. \
        --go_opt=paths=source_relative \
        --go-grpc_out=. \
        --go-grpc_opt=paths=source_relative \
        --proto_path=. \
        "$proto_file"

    if [ $? -eq 0 ]; then
        echo -e "    ${GREEN}✓${NC} Generated successfully"
    else
        echo -e "    ${RED}✗${NC} Failed to generate"
        return 1
    fi
}

# Generate protos in the central location
echo ""
echo "📦 Generating central proto files..."
echo ""

# Find all proto files in the central proto directory
find proto -name "*.proto" -type f | while read -r proto_file; do
    generate_proto "$proto_file"
done

echo ""
echo -e "${GREEN}✅ Proto generation complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Update service imports to use the generated code"
echo "  2. Remove old proto directories from services"
echo "  3. Test that all services build correctly"