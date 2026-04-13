#!/bin/bash

# Installation script for Git hooks
# This script sets up Git hooks for the Rand Lottery Microservices project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}    Rand Lottery Microservices Git Hooks Setup${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_DIR="$PROJECT_ROOT/.githooks"
GIT_HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Check if we're in the right directory
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo -e "${RED}✗ Error: Not in a Git repository${NC}"
    echo "  Please run this script from the randco-microservices directory"
    exit 1
fi

echo "📍 Project root: $PROJECT_ROOT"
echo ""

# Check if .githooks directory exists
if [ ! -d "$HOOKS_DIR" ]; then
    echo -e "${RED}✗ Error: .githooks directory not found${NC}"
    echo "  The .githooks directory should be part of the repository"
    exit 1
fi

# Function to install a hook
install_hook() {
    local hook_name=$1
    local source_file="$HOOKS_DIR/$hook_name"
    local target_file="$GIT_HOOKS_DIR/$hook_name"

    if [ -f "$source_file" ]; then
        # Backup existing hook if it exists and is not a symlink
        if [ -f "$target_file" ] && [ ! -L "$target_file" ]; then
            echo "📦 Backing up existing $hook_name to $hook_name.backup..."
            cp "$target_file" "$target_file.backup"
        fi

        # Create symlink in .git/hooks pointing to .githooks
        echo "🔗 Creating symlink for $hook_name..."
        ln -sf "../../.githooks/$hook_name" "$target_file"

        # Ensure the hook is executable
        chmod +x "$source_file"

        echo -e "${GREEN}✓ $hook_name installed successfully${NC}"
    else
        echo -e "${YELLOW}⚠ $hook_name not found in .githooks directory${NC}"
    fi
}

# Install hooks
echo "🔧 Installing Git hooks..."
echo ""

# List of hooks to install
HOOKS=("pre-commit" "commit-msg")

for hook in "${HOOKS[@]}"; do
    install_hook "$hook"
done

echo ""

# Check for required tools
echo "🔍 Checking for required tools..."
echo ""

check_tool() {
    local tool=$1
    local install_cmd=$2
    
    if command -v "$tool" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ $tool is installed${NC}"
    else
        echo -e "${YELLOW}⚠ $tool is not installed${NC}"
        echo "  Install with: $install_cmd"
    fi
}

check_tool "go" "https://go.dev/doc/install"
check_tool "gofmt" "Included with Go installation"
check_tool "golangci-lint" "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
check_tool "staticcheck" "go install honnef.co/go/tools/cmd/staticcheck@latest"
check_tool "protoc" "https://grpc.io/docs/protoc-installation/"
check_tool "protoc-gen-go" "go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
check_tool "protoc-gen-go-grpc" "go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"

echo ""

# Configure Git to use hooks from .githooks directory
echo "⚙️  Configuring Git to use .githooks directory..."
git config core.hooksPath .githooks
echo -e "${GREEN}✓ Git configured to use .githooks directory${NC}"

echo ""

# Create or update README for hooks
cat > "$HOOKS_DIR/README.md" << 'EOF'
# Git Hooks for Rand Lottery Microservices

This directory contains Git hooks that enforce code quality standards for the Rand Lottery Microservices project.

## Hooks Included

### pre-commit
Comprehensive pre-commit checks to ensure code quality:

**Go Code Quality:**
- Go formatting check (`gofmt`)
- Go vet analysis for potential issues
- Linting with golangci-lint (checks for bugs, style, performance issues)
- Static analysis with staticcheck (advanced bug detection)

**Migration Validation:**
- Database migration file format validation (Goose single-file format)
- Ensures migrations contain both Up and Down directives

**Architecture Enforcement:**
- Repository/Service interface pattern validation
- Constructor pattern checks (NewXxxRepository/NewXxxService)
- Interface size limits (max 8-10 methods)

**Security Checks:**
- Sensitive data detection (passwords, API keys, tokens)
- Prevents committing secrets and credentials

**Configuration Standards:**
- Viper configuration usage validation
- Prevents direct os.Getenv usage in config files

**Code Organization:**
- Error handling pattern validation (error wrapping)
- Test file placement validation (tests alongside implementation)

**Proto/gRPC Validation:**
- Proto file syntax validation (proto3)
- Package declaration checks
- go_package option verification
- Ensures generated .pb.go files are staged with proto changes

### commit-msg
Validates and formats commit messages:
- Minimum length requirements (10 characters)
- Conventional commit format suggestion (feat, fix, docs, etc.)
- Removes Claude Code signatures per project guidelines
- Checks for GitHub issue references (#123)
- Provides helpful formatting guidance

## Installation

Run the installation script from the project root:
```bash
./scripts/install-hooks.sh
```

## Manual Installation

If you prefer to install manually:

1. Create symlinks from .git/hooks to .githooks:
```bash
ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit
ln -sf ../../.githooks/commit-msg .git/hooks/commit-msg
```

2. Configure Git to use .githooks:
```bash
git config core.hooksPath .githooks
```

3. Ensure hooks are executable:
```bash
chmod +x .githooks/*
```

## Bypassing Hooks

In emergency situations only, you can bypass hooks with:
```bash
git commit --no-verify -m "Emergency fix: description"
```

**⚠️ Warning:** Use this sparingly and only when absolutely necessary. The hooks exist to maintain code quality and prevent issues.

## Requirements

### Required Tools
- **Go 1.23+** - Core language
- **gofmt** - Included with Go installation

### Recommended Tools
- **golangci-lint** - Comprehensive linter
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```
- **staticcheck** - Advanced static analysis
  ```bash
  go install honnef.co/go/tools/cmd/staticcheck@latest
  ```

### Proto Tools (for proto file changes)
- **protoc** - Protocol buffer compiler
- **protoc-gen-go** - Go code generator
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  ```
- **protoc-gen-go-grpc** - gRPC code generator
  ```bash
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

## Troubleshooting

### Hooks Not Running
1. Check hook configuration:
   ```bash
   git config --get core.hooksPath
   ```
2. Verify hooks are executable:
   ```bash
   ls -la .githooks/
   ```
3. Ensure you're in the project root when committing

### Formatting Errors
- Run `go fmt ./...` from the project root
- Or format specific files: `gofmt -w path/to/file.go`

### Linting Failures
- Run `golangci-lint run` to see detailed errors
- Fix issues or add exceptions to `.golangci.yml` if justified

### Proto Generation Issues
- Run `make proto` to regenerate proto files
- Ensure both `.proto` and `.pb.go` files are staged together

## Standards Enforced

These hooks enforce the standards defined in:
- `/docs/project/PRD.md` - Architecture and coding standards
- `/CLAUDE.md` - Project-specific instructions
- `/proto/README.md` - Proto file management guidelines

## Hook Details

### Pre-commit Workflow
1. Checks for modified Go and Proto files
2. Runs formatting checks (fails fast on format issues)
3. Executes go vet for basic correctness
4. Runs golangci-lint for comprehensive linting
5. Performs staticcheck for advanced analysis
6. Validates migration file format
7. Checks architectural patterns
8. Scans for sensitive data
9. Validates configuration patterns
10. Checks proto file requirements
11. Ensures proto generation is complete

### Commit-msg Workflow
1. Validates message length
2. Suggests conventional commit format
3. Removes any Claude Code signatures
4. Checks for issue references
5. Provides improvement suggestions

## Contributing

When modifying hooks:
1. Edit files in `.githooks/` directory
2. Test thoroughly with sample commits
3. Update this README if behavior changes
4. Commit the changes to the repository
EOF

echo -e "${GREEN}✓ README.md created in .githooks directory${NC}"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ Git hooks installation completed successfully!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "📌 Next steps:"
echo "  1. Install recommended tools if not already installed:"
echo ""
echo "     Code Quality Tools:"
echo "     • golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
echo "     • staticcheck: go install honnef.co/go/tools/cmd/staticcheck@latest"
echo ""
echo "     Proto Generation Tools:"
echo "     • protoc: See https://grpc.io/docs/protoc-installation/"
echo "     • protoc-gen-go: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
echo "     • protoc-gen-go-grpc: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
echo ""
echo "  2. Test the hooks by making a commit:"
echo "     git add ."
echo "     git commit -m \"test: verify git hooks are working\""
echo ""
echo "  3. For proto file changes, use:"
echo "     make proto          # Generate proto files"
echo "     make proto-check    # Verify proto files are up to date"
echo ""
echo "  4. Check .githooks/README.md for detailed information"
echo ""