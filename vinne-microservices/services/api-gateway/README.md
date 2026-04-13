# API Gateway

The API Gateway is the single entry point for all client applications to interact with the RANDCO microservices architecture. It handles REST to gRPC translation, authentication, rate limiting, circuit breaking, and request routing.

## Features

- **REST to gRPC Translation**: Converts REST API calls to gRPC calls for internal microservices
- **Authentication & Authorization**: JWT-based authentication with role-based access control
- **Rate Limiting**: Token bucket and sliding window algorithms with Redis support
- **Circuit Breaker**: Protects against cascading failures
- **CORS Support**: Configurable cross-origin resource sharing
- **Request Logging**: Comprehensive request/response logging
- **Health Checks**: Service health monitoring
- **Graceful Shutdown**: Proper connection cleanup on shutdown

## Architecture

The API Gateway uses only Go standard library for HTTP handling (no external router dependencies) and implements:

- Custom router with middleware support
- Parameter extraction from URLs
- Route grouping
- JSON request/response handling

## Quick Start

### Prerequisites

- Go 1.21+
- Redis (optional, for distributed rate limiting)
- Running microservices (e.g., service-admin-auth)

### Installation

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Update the `.env` file with your configuration

3. Install dependencies:
```bash
make deps
```

### Running

#### Local Development
```bash
make run
```

#### Production Build
```bash
make build
./bin/api-gateway
```

#### Docker
```bash
make docker-build
make docker-run
```

## Configuration

The API Gateway is configured via environment variables. See `.env.example` for all available options.

Key configurations:
- `PORT`: Server port (default: 8080)
- `JWT_ACCESS_SECRET`: Secret for JWT access tokens
- `REDIS_HOST`: Redis host for distributed features
- `*_SERVICE_ADDR`: Addresses of microservices

## API Endpoints

### Public Endpoints

#### Authentication
- `POST /api/v1/auth/login` - Admin login
- `POST /api/v1/auth/refresh` - Refresh access token
- `GET /health` - Health check

### Protected Endpoints (Require JWT)

#### Admin Profile
- `POST /api/v1/admin/logout` - Logout
- `GET /api/v1/admin/profile` - Get profile
- `PUT /api/v1/admin/profile` - Update profile
- `POST /api/v1/admin/change-password` - Change password

## Middleware

### Global Middleware
1. **Logging**: Logs all requests with timing information
2. **CORS**: Handles cross-origin requests
3. **Rate Limiting**: Prevents abuse
4. **Circuit Breaker**: Prevents cascading failures

### Route-Specific Middleware
1. **Authentication**: Validates JWT tokens
2. **Authorization**: Checks user roles and permissions

## Rate Limiting

Two algorithms are implemented:

### Token Bucket
- Fixed capacity with refill rate
- Good for allowing bursts

### Sliding Window
- Counts requests in a time window
- More accurate rate limiting

## Circuit Breaker

Protects services from overload:
- **Closed**: Normal operation
- **Open**: Requests fail fast
- **Half-Open**: Limited requests to test recovery

## Development

### Testing
```bash
make test
```

### Linting
```bash
make lint
```

### Code Formatting
```bash
make fmt
```

## Monitoring

The gateway exposes metrics and health endpoints:
- `/health` - Basic health check
- `/metrics` - Prometheus metrics (when implemented)

## Error Handling

All errors are properly handled and returned as JSON:
```json
{
  "error": "Error message"
}
```

HTTP status codes are used appropriately:
- 200: Success
- 400: Bad Request
- 401: Unauthorized
- 403: Forbidden
- 404: Not Found
- 429: Too Many Requests
- 500: Internal Server Error
- 503: Service Unavailable

## Security

- JWT tokens for authentication
- Rate limiting to prevent abuse
- CORS configuration for web clients
- Request ID tracking for audit trails
- Graceful degradation with circuit breakers

## Performance

- No external router dependencies (uses Go standard library)
- Efficient middleware chain execution
- Connection pooling for gRPC clients
- Redis for distributed caching and rate limiting
- Configurable timeouts

## Troubleshooting

### Gateway won't start
- Check if port is already in use
- Verify Redis connection (if configured)
- Check microservice addresses

### Authentication failures
- Verify JWT secrets match across services
- Check token expiration times
- Ensure service-admin-auth is running

### Rate limiting issues
- Check Redis connection for distributed limiting
- Verify rate limit configuration
- Monitor rate limit metrics

### Service unavailable errors
- Check if microservices are running
- Verify service addresses in configuration
- Check circuit breaker status

## License

Copyright © 2024 RANDCO. All rights reserved.