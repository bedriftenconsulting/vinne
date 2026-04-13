# Protocol Buffers (Proto) Management

This directory contains all centralized Protocol Buffer definitions for the RANDCO microservices platform.

## Directory Structure

```
proto/
├── admin/management/v1/     # Admin management service definitions
├── agent/
│   ├── auth/v1/            # Agent authentication service
│   └── management/v1/      # Agent management service
├── draw/v1/                # Draw service definitions
├── game/v1/                # Game service definitions
├── payment/v1/             # Payment service definitions
├── terminal/v1/            # Terminal service definitions
└── wallet/v1/              # Wallet service definitions
```

## Usage

### Generating Proto Files

To regenerate all protobuf code after modifying `.proto` files:

```bash
make proto
```

### Checking Proto Files

To verify that generated code is up to date:

```bash
make proto-check
```

### Cleaning Generated Files

To remove all generated `.pb.go` files:

```bash
make proto-clean
```

## Automation

### Pre-commit Hook
The pre-commit hook automatically checks:
- Proto file syntax validation
- Package naming conventions
- Ensures generated files are staged when `.proto` files change

### GitHub Actions
On pull requests and pushes, the CI pipeline:
- Validates proto syntax
- Generates code and checks for uncommitted changes
- Detects breaking changes (PR only)
- Lints proto files using buf

### Workflow

1. **Modify `.proto` files** in the appropriate directory
2. **Run `make proto`** to generate Go code
3. **Stage both** `.proto` and generated `.pb.go` files
4. **Commit** - pre-commit hook validates everything
5. **Push** - GitHub Actions performs additional validation

## Best Practices

1. **Always use `proto3` syntax**
2. **Use versioned packages** (e.g., `package game.v1;`)
3. **Include `go_package` option** in all proto files
4. **Generate and commit** `.pb.go` files alongside `.proto` changes
5. **Run `make proto-check`** before committing to ensure consistency

## Adding New Services

1. Create directory structure: `proto/{service}/v1/`
2. Create `.proto` file with proper package and options
3. Run `make proto` to generate code
4. Update service imports to use `github.com/randco/randco-microservices/proto/{service}/v1`