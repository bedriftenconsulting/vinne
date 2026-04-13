# Environment Variables Naming Standard

This document defines the standardized environment variable naming conventions that **MUST** be used across all microservices in the Randco platform. Following these standards ensures consistency, maintainability, and proper configuration management across Kubernetes deployments.

## Table of Contents
- [General Rules](#general-rules)
- [Database Configuration](#database-configuration)
- [Redis Configuration](#redis-configuration)
- [Security Configuration](#security-configuration)
- [Server Configuration](#server-configuration)
- [Observability Configuration](#observability-configuration)
- [Messaging Configuration](#messaging-configuration)
- [Service Discovery](#service-discovery)
- [Business Logic Configuration](#business-logic-configuration)

## General Rules

1. **NO ALIASES**: Each environment variable has exactly ONE canonical name. No aliases or duplicate names are allowed
2. **Use UPPERCASE with underscores**: All environment variables must be in `UPPER_SNAKE_CASE`
3. **Use consistent prefixes**: Group related variables with common prefixes (e.g., `DATABASE_*`, `REDIS_*`)
4. **Be explicit**: Avoid abbreviations that could be ambiguous
5. **Use standard names**: Follow the exact names defined in this document
6. **Place sensitive data in Secrets**: Passwords, API keys, and tokens must only be in Kubernetes Secrets, never in ConfigMaps

## Database Configuration

### ConfigMap Variables (Non-sensitive)
```yaml
DATABASE_HOST              # PostgreSQL host address
DATABASE_PORT              # PostgreSQL port (default: 5432)
DATABASE_NAME              # Database name
DATABASE_USER              # Database username
DATABASE_SSL_MODE          # SSL mode (disable/require/verify-ca/verify-full)
DATABASE_MAX_OPEN_CONNS    # Maximum number of open connections
DATABASE_MAX_IDLE_CONNS    # Maximum number of idle connections
DATABASE_CONN_MAX_LIFETIME # Maximum connection lifetime in seconds
```

### Secret Variables (Sensitive)
```yaml
DATABASE_PASSWORD          # Database password
DATABASE_URL              # Full connection string with credentials
```

**Note**: Never use `DB_*` prefix. Always use full `DATABASE_*` prefix.

## Redis Configuration

### ConfigMap Variables (Non-sensitive)
```yaml
REDIS_HOST                # Redis host address
REDIS_PORT                # Redis port (default: 6379)
REDIS_DB                  # Redis database number (0-15)
REDIS_POOL_SIZE           # Connection pool size
REDIS_MIN_IDLE_CONNS      # Minimum idle connections
REDIS_MAX_RETRIES         # Maximum retry attempts
REDIS_CACHE_TTL           # Cache TTL in seconds
```

### Secret Variables (Sensitive)
```yaml
REDIS_PASSWORD            # Redis password
REDIS_URL                 # Full Redis URL with password
```

### Shared Session Store (API Gateway and other services)
```yaml
SHARED_REDIS_HOST         # Shared Redis host for sessions
SHARED_REDIS_PORT         # Shared Redis port
SHARED_REDIS_PASSWORD     # Shared Redis password (Secret)
```

## Security Configuration

### ConfigMap Variables (Non-sensitive)
```yaml
SECURITY_BCRYPT_COST           # Bcrypt hashing cost (10-12 recommended)
SECURITY_PASSWORD_MIN_LENGTH   # Minimum password length
SECURITY_JWT_ISSUER           # JWT token issuer
SECURITY_SESSION_EXPIRY        # Session expiry in hours
SECURITY_ACCESS_TOKEN_EXPIRY   # Access token expiry in seconds
SECURITY_REFRESH_TOKEN_EXPIRY  # Refresh token expiry in hours
SECURITY_MFA_ISSUER           # MFA issuer name
SECURITY_MAX_FAILED_LOGINS    # Max failed login attempts
SECURITY_LOCKOUT_DURATION     # Account lockout duration in minutes
SECURITY_REQUIRE_SPECIAL_CHAR # Require special character in password (true/false)
SECURITY_REQUIRE_UPPERCASE    # Require uppercase in password (true/false)
SECURITY_REQUIRE_NUMBER       # Require number in password (true/false)
```

### Secret Variables (Sensitive)
```yaml
JWT_SECRET                    # JWT signing secret (the only accepted name)
```

## Server Configuration

### ConfigMap Variables
```yaml
SERVER_PORT               # gRPC/HTTP server port
SERVER_MODE               # Server mode (development/production)
SERVER_READ_TIMEOUT       # Read timeout in seconds
SERVER_WRITE_TIMEOUT      # Write timeout in seconds
SERVER_IDLE_TIMEOUT       # Idle timeout in seconds
ENVIRONMENT               # Deployment environment (development/staging/production)
SERVICE_NAME              # Service identifier name
SERVICE_VERSION           # Service version
```

## Observability Configuration

### Logging
```yaml
LOGGING_LEVEL             # Log level (debug/info/warn/error)
LOGGING_FORMAT            # Log format (json/text)
```

### Metrics
```yaml
METRICS_ENABLED           # Enable metrics collection (true/false)
METRICS_PORT              # Metrics endpoint port
METRICS_PATH              # Metrics endpoint path (default: /metrics)
```

### Tracing
```yaml
TRACING_ENABLED           # Enable distributed tracing (true/false)
TRACING_JAEGER_ENDPOINT   # Jaeger collector endpoint
TRACING_SAMPLE_RATE       # Trace sampling rate (0.0-1.0)
TRACING_SERVICE_NAME      # Service name for traces
TRACING_SERVICE_VERSION   # Service version for traces
TRACING_ENVIRONMENT       # Environment tag for traces
```

## Messaging Configuration

### Kafka
```yaml
KAFKA_ENABLED             # Enable Kafka event publishing/consuming (true/false)
KAFKA_BROKERS             # Comma-separated list of Kafka brokers
KAFKA_TOPIC               # Primary Kafka topic for the service
KAFKA_CONSUMER_GROUP      # Consumer group ID
KAFKA_TOPIC_PREFIX        # Topic prefix for the service
KAFKA_TOPICS_*            # Specific topic configurations
```

Example topic variables:
```yaml
KAFKA_TOPICS_AGENT_EVENTS
KAFKA_TOPICS_DEVICE_EVENTS
KAFKA_TOPICS_PAYMENT_EVENTS
KAFKA_TOPICS_AUDIT_LOGS
```

**Note**:
- `KAFKA_ENABLED` controls whether Kafka is used. When false, services fall back to no-op publishers
- `KAFKA_TOPIC` is the primary topic for services that publish/consume from a single topic
- `KAFKA_TOPICS_*` is for services that need multiple topic configurations

## Service Discovery

For inter-service communication, use the following pattern:
```yaml
SERVICES_<SERVICE_NAME>_HOST   # Service host
SERVICES_<SERVICE_NAME>_PORT   # Service port
```

Examples:
```yaml
SERVICES_ADMIN_MANAGEMENT_HOST
SERVICES_ADMIN_MANAGEMENT_PORT
SERVICES_AGENT_AUTH_HOST
SERVICES_AGENT_AUTH_PORT
SERVICES_PAYMENT_HOST
SERVICES_PAYMENT_PORT
```

## Business Logic Configuration

Service-specific business logic configurations should follow the pattern:
```yaml
BUSINESS_<FEATURE>_<SETTING>
```

Examples:
```yaml
# Agent Management Service
BUSINESS_DEFAULT_AGENT_COMMISSION_PERCENTAGE
BUSINESS_MAX_RETAILERS_PER_AGENT
BUSINESS_KYC_DOCUMENT_EXPIRY_DAYS

# Payment Service
PAYMENT_DEFAULT_CURRENCY
PAYMENT_MIN_AMOUNT
PAYMENT_MAX_AMOUNT
PAYMENT_TIMEOUT_SECONDS
PAYMENT_RETRY_COUNT
PAYMENT_RETRY_DELAY_SECONDS
PAYMENT_TEST_MODE
PAYMENT_WEBHOOK_SECRET         # (Secret)

# Draw Service
RNG_PROVIDER                   # Random number generator provider
RNG_SEED                       # (Secret)
VALIDATION_MIN_DOCUMENTS
VALIDATION_NLA_API_ENDPOINT
VALIDATION_REQUIRE_CERTIFICATE
VALIDATION_REQUIRE_WITNESS
VALIDATION_TIMEOUT_SECONDS
```

## Payment Provider Configuration

### ConfigMap Variables
```yaml
PROVIDERS_<PROVIDER>_ENABLED
PROVIDERS_<PROVIDER>_BASE_URL
PROVIDERS_<PROVIDER>_ENVIRONMENT
PROVIDERS_<PROVIDER>_CALLBACK_URL
```

### Secret Variables
```yaml
PROVIDERS_<PROVIDER>_CLIENT_ID
PROVIDERS_<PROVIDER>_CLIENT_SECRET
PROVIDERS_<PROVIDER>_API_KEY
PROVIDERS_<PROVIDER>_API_SECRET
PROVIDERS_<PROVIDER>_SUBSCRIPTION_KEY
```

Examples:
```yaml
# MTN Mobile Money
PROVIDERS_MTN_ENABLED
PROVIDERS_MTN_BASE_URL
PROVIDERS_MTN_API_KEY          # (Secret)
PROVIDERS_MTN_API_SECRET       # (Secret)

# Telecel
PROVIDERS_TELECEL_ENABLED
PROVIDERS_TELECEL_CLIENT_ID    # (Secret)
PROVIDERS_TELECEL_CLIENT_SECRET # (Secret)
```

## Implementation Checklist

When implementing or updating a service, ensure:

- [ ] All database variables use `DATABASE_*` prefix (not `DB_*`)
- [ ] All Redis variables use `REDIS_*` prefix
- [ ] All security variables use `SECURITY_*` prefix
- [ ] JWT secret is provided as `JWT_SECRET` only (no aliases)
- [ ] Logging variables use `LOGGING_*` prefix (not `LOG_*`)
- [ ] Metrics variables use `METRICS_*` prefix
- [ ] Tracing variables use `TRACING_*` prefix (not `JAEGER_*`)
- [ ] Service discovery uses `SERVICES_<NAME>_*` pattern
- [ ] All sensitive data is in Secrets, not ConfigMaps
- [ ] Variable names match exactly as defined in this document
- [ ] **NO ALIASES** - each variable has exactly one name

## Migration Notes

For services currently using non-standard variable names:
1. Update the service's `config.go` to use Viper's `BindEnv` to map old names to new standards
2. Update Helm ConfigMaps and Secrets to use standard names
3. Update local `.env` files to use standard names
4. Test thoroughly in development environment before deploying

## Examples of Incorrect vs Correct

❌ **Incorrect:**
```yaml
DB_URL                    # Should be DATABASE_URL
DB_PASSWORD               # Should be DATABASE_PASSWORD
db_password               # Should be DATABASE_PASSWORD (uppercase)
LOG_LEVEL                 # Should be LOGGING_LEVEL
REDIS_ADDR               # Should be REDIS_HOST and REDIS_PORT separately
JWT_ACCESS_TOKEN_DURATION # Should be SECURITY_ACCESS_TOKEN_EXPIRY
SECURITY_JWT_SECRET       # Should be JWT_SECRET (no SECURITY_ prefix for JWT)
JAEGER_ENDPOINT          # Should be TRACING_JAEGER_ENDPOINT
SERVER_ENVIRONMENT        # Should be ENVIRONMENT
```

✅ **Correct:**
```yaml
DATABASE_URL
DATABASE_PASSWORD
LOGGING_LEVEL
REDIS_HOST
REDIS_PORT
SECURITY_ACCESS_TOKEN_EXPIRY
JWT_SECRET                # The only accepted name for JWT secret
TRACING_JAEGER_ENDPOINT   # The only accepted name for Jaeger endpoint
ENVIRONMENT               # The only accepted name for environment
```

## Version History

- **v1.1.0** (2025-01-26): Removed all aliases - each variable has exactly one canonical name
- **v1.0.0** (2025-01-26): Initial standard definition