# Service Environment Variables

## CRITICAL REQUIREMENT: JWT Configuration
⚠️ **All services MUST use the same JWT_SECRET and JWT_ISSUER values for authentication to work across the platform.**
- This applies to ALL environments (development, staging, production)
- Token validation will fail if services have different JWT secrets or issuers
- Recommended standard issuer: `randlotteryltd`

## Admin Management Service
**Port**: 50057

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50057 |
| SERVER_MODE | ConfigMap | No | development |
| DATABASE_URL | Secret | Yes | postgresql://admin_mgmt:password@host:5432/admin_management?sslmode=disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | localhost |
| REDIS_PORT | ConfigMap | Yes | 6384 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MIN_IDLE_CONNS | ConfigMap | No | 5 |
| JWT_SECRET | Secret | Yes | (same secret for all services) |
| SECURITY_JWT_ISSUER | ConfigMap | Yes | randlotteryltd |
| SECURITY_BCRYPT_COST | ConfigMap | No | 10 |
| SECURITY_PASSWORD_MIN_LENGTH | ConfigMap | No | 8 |
| SECURITY_ACCESS_TOKEN_EXPIRY | ConfigMap | No | 3600s |
| SECURITY_REFRESH_TOKEN_EXPIRY | ConfigMap | No | 168h |
| SECURITY_SESSION_EXPIRY | ConfigMap | No | 168h |
| SECURITY_LOCKOUT_DURATION | ConfigMap | No | 30m |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SERVICE_NAME | ConfigMap | No | admin-management |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 9092 |
| ENVIRONMENT | ConfigMap | No | development |
| SERVICE_NAME | ConfigMap | No | admin-management |
| SERVICE_VERSION | ConfigMap | No | 1.0.0 |

## Agent Auth Service
**Port**: 50052

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50052 |
| SERVER_MODE | ConfigMap | No | development |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://agent:password@host:5432/agent_auth?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-agent-auth.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | agent_auth |
| DATABASE_USER | ConfigMap | Conditional | agent |
| DATABASE_PASSWORD | Secret | Conditional | AgentAuth2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | redis-agent-auth.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MIN_IDLE_CONNS | ConfigMap | No | 5 |
| JWT_SECRET | Secret | Yes | (same secret for all services) |
| SECURITY_JWT_ISSUER | ConfigMap | Yes | randlotteryltd |
| SECURITY_ACCESS_TOKEN_EXPIRY | ConfigMap | No | 900s |
| SECURITY_REFRESH_TOKEN_EXPIRY | ConfigMap | No | 168h |
| SECURITY_BCRYPT_COST | ConfigMap | No | 10 |
| SECURITY_PASSWORD_MIN_LENGTH | ConfigMap | No | 8 |
| SECURITY_MAX_FAILED_LOGINS | ConfigMap | No | 5 |
| SECURITY_LOCKOUT_DURATION | ConfigMap | No | 30m |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 9092 |
| ENVIRONMENT | ConfigMap | No | development |

## Agent Management Service
**Port**: 50058

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_HOST | ConfigMap | No | 0.0.0.0 |
| SERVER_PORT | ConfigMap | Yes | 50058 |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://agent_mgmt:password@host:5432/agent_management?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-agent-management.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | agent_management |
| DATABASE_USER | ConfigMap | Conditional | agent_mgmt |
| DATABASE_PASSWORD | Secret | Conditional | AgentMgmt2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | redis-agent-management.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| JWT_SECRET | Secret | Yes | (same secret for all services) |
| SERVICES_AGENT_AUTH_HOST | ConfigMap | No | service-agent-auth-dev |
| SERVICES_AGENT_AUTH_PORT | ConfigMap | No | 50052 |
| SERVICES_WALLET_HOST | ConfigMap | No | service-wallet-dev |
| SERVICES_WALLET_PORT | ConfigMap | No | 50059 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 8082 |
| ENVIRONMENT | ConfigMap | No | development |

## API Gateway Service
**Port**: 4000

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 4000 |
| SERVER_MODE | ConfigMap | No | development |
| SERVER_READ_TIMEOUT | ConfigMap | No | 15s |
| SERVER_WRITE_TIMEOUT | ConfigMap | No | 15s |
| SERVER_IDLE_TIMEOUT | ConfigMap | No | 60s |
| REDIS_HOST | ConfigMap | Yes | redis-session.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| JWT_SECRET | Secret | Yes | (same secret for all services) |
| SECURITY_JWT_ISSUER | ConfigMap | Yes | randlotteryltd |
| SECURITY_ALLOWED_ORIGINS | ConfigMap | No | http://localhost:3000,http://localhost:5173,https://admin.dev.randlottery.com |
| SECURITY_ALLOWED_HEADERS | ConfigMap | No | Accept,Authorization,Content-Type,X-Request-ID |
| SECURITY_ALLOWED_METHODS | ConfigMap | No | GET,POST,PUT,DELETE,OPTIONS |
| SECURITY_ALLOW_CREDENTIALS | ConfigMap | No | true |
| CACHE_ENABLED | ConfigMap | No | true |
| CACHE_TTL | ConfigMap | No | 300s |
| CACHE_MAX_SIZE | ConfigMap | No | 1048576 |
| RATE_LIMIT_ENABLED | ConfigMap | No | true |
| RATE_LIMIT_REQUESTS_PER_MIN | ConfigMap | No | 100 |
| RATE_LIMIT_BURST_SIZE | ConfigMap | No | 10 |
| SERVICES_ADMIN_MANAGEMENT_HOST | ConfigMap | No | service-admin-management-dev |
| SERVICES_ADMIN_MANAGEMENT_PORT | ConfigMap | No | 50057 |
| SERVICES_AGENT_AUTH_HOST | ConfigMap | No | service-agent-auth-dev |
| SERVICES_AGENT_AUTH_PORT | ConfigMap | No | 50052 |
| SERVICES_AGENT_MANAGEMENT_HOST | ConfigMap | No | service-agent-management-dev |
| SERVICES_AGENT_MANAGEMENT_PORT | ConfigMap | No | 50058 |
| SERVICES_WALLET_HOST | ConfigMap | No | service-wallet-dev |
| SERVICES_WALLET_PORT | ConfigMap | No | 50059 |
| SERVICES_GAME_HOST | ConfigMap | No | service-game-dev |
| SERVICES_GAME_PORT | ConfigMap | No | 50053 |
| SERVICES_DRAW_HOST | ConfigMap | No | service-draw-dev |
| SERVICES_DRAW_PORT | ConfigMap | No | 50060 |
| SERVICES_TICKET_HOST | ConfigMap | No | service-ticket-dev |
| SERVICES_TICKET_PORT | ConfigMap | No | 50062 |
| SERVICES_PAYMENT_HOST | ConfigMap | No | service-payment-dev |
| SERVICES_PAYMENT_PORT | ConfigMap | No | 50061 |
| SERVICES_TERMINAL_HOST | ConfigMap | No | service-terminal-dev |
| SERVICES_TERMINAL_PORT | ConfigMap | No | 50054 |
| SERVICES_NOTIFICATION_HOST | ConfigMap | No | service-notification-dev |
| SERVICES_NOTIFICATION_PORT | ConfigMap | No | 50063 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_SERVICE_NAME | ConfigMap | No | api-gateway |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 9090 |
| METRICS_PATH | ConfigMap | No | /metrics |
| ENVIRONMENT | ConfigMap | No | development |
| SERVICE_NAME | ConfigMap | No | api-gateway |
| SERVICE_VERSION | ConfigMap | No | 1.0.0 |

## Draw Service
**Port**: 50060

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50060 |
| SERVER_READ_TIMEOUT | ConfigMap | No | 30s |
| SERVER_WRITE_TIMEOUT | ConfigMap | No | 30s |
| SERVER_IDLE_TIMEOUT | ConfigMap | No | 120s |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://draw:password@host:5432/draw_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-draw.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | draw_service |
| DATABASE_USER | ConfigMap | Conditional | draw |
| DATABASE_PASSWORD | Secret | Conditional | Draw2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| REDIS_HOST | ConfigMap | Yes | redis-draw.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| RNG_SEED | Secret | No | dev-seed-2025-change-in-production |
| VALIDATION_NLA_API_ENDPOINT | ConfigMap | No | https://api.nla.gov.gh/v1 |
| VALIDATION_MIN_DOCUMENTS | ConfigMap | No | 2 |
| VALIDATION_VALIDATION_TIMEOUT | ConfigMap | No | 30s |
| VALIDATION_REQUIRE_CERTIFICATE | ConfigMap | No | true |
| VALIDATION_REQUIRE_WITNESS | ConfigMap | No | true |
| SERVICES_TICKET_HOST | ConfigMap | No | service-ticket-dev |
| SERVICES_TICKET_PORT | ConfigMap | No | 50062 |
| SERVICES_WALLET_HOST | ConfigMap | No | service-wallet-dev |
| SERVICES_WALLET_PORT | ConfigMap | No | 50059 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| KAFKA_CONSUMER_GROUP | ConfigMap | No | draw-service-group |
| KAFKA_TOPIC_PREFIX | ConfigMap | No | randco |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_SERVICE_NAME | ConfigMap | No | draw-service |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| ENVIRONMENT | ConfigMap | No | development |

## Game Service
**Port**: 50053

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50053 |
| SERVER_READ_TIMEOUT | ConfigMap | No | 30s |
| SERVER_WRITE_TIMEOUT | ConfigMap | No | 30s |
| SERVER_IDLE_TIMEOUT | ConfigMap | No | 120s |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://game:password@host:5432/game_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-game.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | game_service |
| DATABASE_USER | ConfigMap | Conditional | game |
| DATABASE_PASSWORD | Secret | Conditional | Game2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 10 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 1800s |
| DATABASE_CONN_MAX_IDLE_TIME | ConfigMap | No | 600s |
| REDIS_HOST | ConfigMap | Yes | redis-game.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MIN_IDLE_CONNS | ConfigMap | No | 5 |
| SERVICES_DRAW_HOST | ConfigMap | No | service-draw-dev |
| SERVICES_DRAW_PORT | ConfigMap | No | 50060 |
| SERVICES_DRAW_TIMEOUT | ConfigMap | No | 30s |
| SERVICES_DRAW_MAX_RETRIES | ConfigMap | No | 3 |
| SERVICES_DRAW_RETRY_BACKOFF | ConfigMap | No | 1s |
| SERVICES_TICKET_HOST | ConfigMap | No | service-ticket-dev |
| SERVICES_TICKET_PORT | ConfigMap | No | 50062 |
| SERVICES_TICKET_TIMEOUT | ConfigMap | No | 30s |
| SERVICES_TICKET_MAX_RETRIES | ConfigMap | No | 3 |
| SERVICES_TICKET_RETRY_BACKOFF | ConfigMap | No | 1s |
| SERVICES_NOTIFICATION_HOST | ConfigMap | No | service-notification-dev |
| SERVICES_NOTIFICATION_PORT | ConfigMap | No | 50063 |
| SERVICES_NOTIFICATION_TIMEOUT | ConfigMap | No | 30s |
| SERVICES_NOTIFICATION_MAX_RETRIES | ConfigMap | No | 3 |
| SERVICES_NOTIFICATION_RETRY_BACKOFF | ConfigMap | No | 1s |
| SERVICES_ADMIN_HOST | ConfigMap | No | service-admin-management-dev |
| SERVICES_ADMIN_PORT | ConfigMap | No | 50057 |
| SERVICES_ADMIN_TIMEOUT | ConfigMap | No | 30s |
| SERVICES_ADMIN_MAX_RETRIES | ConfigMap | No | 3 |
| SERVICES_ADMIN_RETRY_BACKOFF | ConfigMap | No | 1s |
| SCHEDULER_ENABLED | ConfigMap | No | true |
| SCHEDULER_INTERVAL | ConfigMap | No | 60s |
| SCHEDULER_WINDOW_MINUTES | ConfigMap | No | 5 |
| SCHEDULER_TIMEZONE | ConfigMap | No | Africa/Accra |
| NOTIFICATION_FALLBACK_EMAILS | ConfigMap | No | admin@randlottery.com,ops@randlottery.com |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| KAFKA_CONSUMER_GROUP | ConfigMap | No | game-service-group |
| KAFKA_TOPIC_PREFIX | ConfigMap | No | randco |
| KAFKA_TOPICS_GAME_EVENTS | ConfigMap | No | game.events |
| KAFKA_TOPICS_APPROVAL_EVENTS | ConfigMap | No | game.approval.events |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_SERVICE_NAME | ConfigMap | No | service-game |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| ENVIRONMENT | ConfigMap | No | development |

## Notification Service
**Port**: 50063

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50063 |
| SERVER_MODE | ConfigMap | No | development |
| SERVICE_NAME | ConfigMap | No | service-notification |
| DATABASE_URL | Secret | Yes | postgresql://notification:password@host:5437/notification?sslmode=disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_URL | Secret | Yes | redis://host:6389 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MIN_IDLE_CONNS | ConfigMap | No | 5 |
| REDIS_MAX_CONN_AGE | ConfigMap | No | 1800 |
| REDIS_CONNECT_TIMEOUT | ConfigMap | No | 5000 |
| REDIS_READ_TIMEOUT | ConfigMap | No | 3000 |
| REDIS_WRITE_TIMEOUT | ConfigMap | No | 3000 |
| REDIS_RETRY_COUNT | ConfigMap | No | 3 |
| REDIS_RETRY_DELAY | ConfigMap | No | 1000 |
| EMAIL_DEFAULT_PROVIDER | ConfigMap | No | mailgun |
| EMAIL_MAILGUN_ENABLED | ConfigMap | No | true |
| EMAIL_MAILGUN_API_KEY | Secret | Conditional | (required if using mailgun) |
| EMAIL_MAILGUN_DOMAIN | ConfigMap | Conditional | mg.randlottery.com |
| EMAIL_MAILGUN_BASE_URL | ConfigMap | No | https://api.mailgun.net |
| SMS_DEFAULT_PROVIDER | ConfigMap | No | hubtel |
| SMS_HUBTEL_ENABLED | ConfigMap | No | true |
| SMS_HUBTEL_CLIENT_ID | Secret | Conditional | (required if using hubtel) |
| SMS_HUBTEL_CLIENT_SECRET | Secret | Conditional | (required if using hubtel) |
| SMS_HUBTEL_BASE_URL | ConfigMap | No | https://sms.hubtel.com |
| SMS_HUBTEL_SENDER_ID | ConfigMap | Conditional | RandLottery |
| NOTIFICATION_GAME_END_RECIPIENTS | ConfigMap | No | admin@randlottery.com,ops@randlottery.com |
| SERVICES_ADMIN_MANAGEMENT_HOST | ConfigMap | No | service-admin-management-dev |
| SERVICES_ADMIN_MANAGEMENT_PORT | ConfigMap | No | 50057 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| KAFKA_TOPIC_AUDIT_LOGS | ConfigMap | No | audit.logs |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_SERVICE_NAME | ConfigMap | No | service-notification |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| LOGGING_LOG_FILE | ConfigMap | No | logs/service-notification.log |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 9092 |
| ENVIRONMENT | ConfigMap | No | development |

## Payment Service
**Port**: 50061

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50061 |
| ENVIRONMENT | ConfigMap | No | development |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://payment:password@host:5432/payment_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-payment.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | payment_service |
| DATABASE_USER | ConfigMap | Conditional | payment |
| DATABASE_PASSWORD | Secret | Conditional | Payment2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | redis-payment.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| PAYMENT_DEFAULT_CURRENCY | ConfigMap | No | GHS |
| PAYMENT_MAX_AMOUNT | ConfigMap | No | 100000000 |
| PAYMENT_MIN_AMOUNT | ConfigMap | No | 50 |
| PAYMENT_TIMEOUT_SECONDS | ConfigMap | No | 300s |
| PAYMENT_RETRY_COUNT | ConfigMap | No | 3 |
| PAYMENT_RETRY_DELAY_SECONDS | ConfigMap | No | 30s |
| PAYMENT_TEST_MODE | ConfigMap | No | true |
| PROVIDERS_MTN_ENABLED | ConfigMap | No | true |
| PROVIDERS_MTN_BASE_URL | ConfigMap | No | https://sandbox.momodeveloper.mtn.com |
| PROVIDERS_MTN_ENVIRONMENT | ConfigMap | No | sandbox |
| PROVIDERS_MTN_API_KEY | Secret | Conditional | (required for MTN) |
| PROVIDERS_MTN_API_SECRET | Secret | Conditional | (required for MTN) |
| PROVIDERS_TELECEL_ENABLED | ConfigMap | No | true |
| PROVIDERS_TELECEL_BASE_URL | ConfigMap | No | https://api.telecelghana.com |
| PROVIDERS_TELECEL_ENVIRONMENT | ConfigMap | No | sandbox |
| PROVIDERS_TELECEL_API_KEY | Secret | Conditional | (required for Telecel) |
| PROVIDERS_TELECEL_CLIENT_ID | Secret | Conditional | (required for Telecel) |
| PROVIDERS_TELECEL_CLIENT_SECRET | Secret | Conditional | (required for Telecel) |
| PROVIDERS_AIRTELTIGO_ENABLED | ConfigMap | No | true |
| PROVIDERS_AIRTELTIGO_BASE_URL | ConfigMap | No | https://api.airteltigo.com.gh |
| PROVIDERS_AIRTELTIGO_ENVIRONMENT | ConfigMap | No | sandbox |
| PROVIDERS_BANKS_ENABLED | ConfigMap | No | true |
| PROVIDERS_BANKS_MANUAL_VERIFICATION | ConfigMap | No | true |
| SERVICES_WALLET_HOST | ConfigMap | No | service-wallet-dev |
| SERVICES_WALLET_PORT | ConfigMap | No | 50059 |
| KAFKA_ENABLED | ConfigMap | No | true |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| KAFKA_TOPIC | ConfigMap | No | payment-events |
| KAFKA_CONSUMER_GROUP | ConfigMap | No | payment-service-group |
| KAFKA_TOPIC_PREFIX | ConfigMap | No | randco |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SERVICE_NAME | ConfigMap | No | payment-service |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| TRACING_SAMPLE_RATE | ConfigMap | No | 0.1 |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |

## Terminal Service
**Port**: 50054

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50054 |
| SERVER_HOST | ConfigMap | No | 0.0.0.0 |
| SERVER_MODE | ConfigMap | No | development |
| SERVER_READ_TIMEOUT | ConfigMap | No | 30s |
| SERVER_WRITE_TIMEOUT | ConfigMap | No | 30s |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://terminal:password@host:5432/terminal_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-terminal.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | terminal_service |
| DATABASE_USER | ConfigMap | Conditional | terminal |
| DATABASE_PASSWORD | Secret | Conditional | Terminal2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | redis-terminal.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MAX_RETRIES | ConfigMap | No | 3 |
| REDIS_CACHE_TTL | ConfigMap | No | 300s |
| TERMINAL_DEFAULT_TRANSACTION_LIMIT | ConfigMap | No | 1000 |
| TERMINAL_DEFAULT_DAILY_LIMIT | ConfigMap | No | 10000 |
| TERMINAL_DEFAULT_SYNC_INTERVAL | ConfigMap | No | 5 |
| TERMINAL_HEARTBEAT_INTERVAL | ConfigMap | No | 60 |
| TERMINAL_HEALTH_CHECK_INTERVAL | ConfigMap | No | 300 |
| SERVICES_AGENT_AUTH_HOST | ConfigMap | No | service-agent-auth-dev |
| SERVICES_AGENT_AUTH_PORT | ConfigMap | No | 50052 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_SERVICE_NAME | ConfigMap | No | terminal-service |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 8088 |
| METRICS_PATH | ConfigMap | No | /metrics |
| ENVIRONMENT | ConfigMap | No | development |
| SERVICE_NAME | ConfigMap | No | terminal |
| SERVICE_VERSION | ConfigMap | No | 1.0.0 |

## Ticket Service
**Port**: 50062

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50062 |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://ticket:password@host:5432/ticket_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-ticket.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | ticket_service |
| DATABASE_USER | ConfigMap | Conditional | ticket |
| DATABASE_PASSWORD | Secret | Conditional | Ticket2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 10 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 5m |
| REDIS_HOST | ConfigMap | Yes | redis-ticket.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6379 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| BUSINESS_SERIAL_PREFIX | ConfigMap | No | TKT |
| SERVICES_GAME_HOST | ConfigMap | No | service-game-dev |
| SERVICES_GAME_PORT | ConfigMap | No | 50053 |
| SERVICES_DRAW_HOST | ConfigMap | No | service-draw-dev |
| SERVICES_DRAW_PORT | ConfigMap | No | 50060 |
| SERVICES_PAYMENT_HOST | ConfigMap | No | service-payment-dev |
| SERVICES_PAYMENT_PORT | ConfigMap | No | 50061 |
| SERVICES_WALLET_HOST | ConfigMap | No | service-wallet-dev |
| SERVICES_WALLET_PORT | ConfigMap | No | 50059 |
| SERVICES_AGENT_MANAGEMENT_HOST | ConfigMap | No | service-agent-management-dev |
| SERVICES_AGENT_MANAGEMENT_PORT | ConfigMap | No | 50058 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| KAFKA_TOPICS_TICKET_EVENTS | ConfigMap | No | ticket.events |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_SERVICE_NAME | ConfigMap | No | service-ticket |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| ENVIRONMENT | ConfigMap | No | development |

## Wallet Service
**Port**: 50059

| Variable | Type | Required | Example/Default |
|----------|------|----------|-----------------|
| SERVER_PORT | ConfigMap | Yes | 50059 |
| SERVER_HOST | ConfigMap | No | 0.0.0.0 |
| SERVER_MODE | ConfigMap | No | development |
| SERVER_READ_TIMEOUT | ConfigMap | No | 30s |
| SERVER_WRITE_TIMEOUT | ConfigMap | No | 30s |
| DATABASE_URL | Secret | Yes (or components below) | postgresql://wallet:password@host:5432/wallet_service?sslmode=disable |
| DATABASE_HOST | ConfigMap | Conditional | postgres-wallet.external-services.svc.cluster.local |
| DATABASE_PORT | ConfigMap | Conditional | 5432 |
| DATABASE_NAME | ConfigMap | Conditional | wallet_service |
| DATABASE_USER | ConfigMap | Conditional | wallet |
| DATABASE_PASSWORD | Secret | Conditional | Wallet2025Secure |
| DATABASE_SSL_MODE | ConfigMap | No | disable |
| DATABASE_MAX_OPEN_CONNS | ConfigMap | No | 25 |
| DATABASE_MAX_IDLE_CONNS | ConfigMap | No | 5 |
| DATABASE_CONN_MAX_LIFETIME | ConfigMap | No | 300s |
| REDIS_HOST | ConfigMap | Yes | redis-wallet.external-services.svc.cluster.local |
| REDIS_PORT | ConfigMap | Yes | 6385 |
| REDIS_PASSWORD | Secret | No | (empty) |
| REDIS_DB | ConfigMap | Yes | 0 |
| REDIS_POOL_SIZE | ConfigMap | No | 10 |
| REDIS_MAX_RETRIES | ConfigMap | No | 3 |
| REDIS_CACHE_TTL | ConfigMap | No | 300s |
| WALLET_DEFAULT_COMMISSION_RATE | ConfigMap | No | 0.30 |
| WALLET_MAX_TRANSFER_AMOUNT | ConfigMap | No | 100000.00 |
| WALLET_MIN_TRANSFER_AMOUNT | ConfigMap | No | 1.00 |
| WALLET_MAX_CREDIT_AMOUNT | ConfigMap | No | 50000.00 |
| WALLET_MIN_CREDIT_AMOUNT | ConfigMap | No | 1.00 |
| WALLET_TRANSACTION_TIMEOUT | ConfigMap | No | 30s |
| WALLET_LOCK_TIMEOUT | ConfigMap | No | 10s |
| SERVICES_AGENT_MANAGEMENT_HOST | ConfigMap | No | service-agent-management-dev.microservices-dev.svc.cluster.local |
| SERVICES_AGENT_MANAGEMENT_PORT | ConfigMap | No | 50058 |
| SERVICES_PAYMENT_HOST | ConfigMap | No | service-payment-dev |
| SERVICES_PAYMENT_PORT | ConfigMap | No | 50061 |
| KAFKA_BROKERS | ConfigMap | Yes | kafka.external-services.svc.cluster.local:9092 |
| TRACING_ENABLED | ConfigMap | No | true |
| TRACING_JAEGER_ENDPOINT | ConfigMap | No | http://jaeger.observability.svc.cluster.local:4318 |
| TRACING_SAMPLE_RATE | ConfigMap | No | 1.0 |
| TRACING_SERVICE_NAME | ConfigMap | No | wallet-service |
| TRACING_SERVICE_VERSION | ConfigMap | No | 1.0.0 |
| TRACING_ENVIRONMENT | ConfigMap | No | development |
| LOGGING_LEVEL | ConfigMap | No | info |
| LOGGING_FORMAT | ConfigMap | No | json |
| METRICS_ENABLED | ConfigMap | No | true |
| METRICS_PORT | ConfigMap | No | 9093 |
| METRICS_PATH | ConfigMap | No | /metrics |
| ENVIRONMENT | ConfigMap | No | development |
| SERVICE_NAME | ConfigMap | No | wallet |
| SERVICE_VERSION | ConfigMap | No | 1.0.0 |

## Notes

### Important Time Format Requirements
- **Time values MUST include units**: `300s`, `30m`, `168h`, `5m`
- **Seconds**: Use `s` suffix (e.g., `300s`, `3600s`, `900s`)
- **Minutes**: Use `m` suffix (e.g., `5m`, `30m`)
- **Hours**: Use `h` suffix (e.g., `168h`, `24h`)

### Required vs Optional
- **Required**: Service will not start without this variable
- **Conditional**: Required if the alternative method is not provided (e.g., DATABASE_URL vs individual DATABASE_* components)
- **No**: Has a default value and is optional

### Database Configuration
Services accept either:
1. `DATABASE_URL` as a complete connection string, OR
2. Individual components: `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_NAME`, `DATABASE_USER`, `DATABASE_PASSWORD`

### Redis Configuration
**STANDARDIZED**: All services MUST use the same Redis configuration format with individual components:
- `REDIS_HOST`: Redis server hostname
- `REDIS_PORT`: Redis server port
- `REDIS_PASSWORD`: Redis password (empty string if no auth)
- `REDIS_DB`: Redis database number (0-15)
- Additional optional: `REDIS_POOL_SIZE`, `REDIS_MIN_IDLE_CONNS`, `REDIS_MAX_RETRIES`, `REDIS_CACHE_TTL`

### JWT Secret and Issuer
**CRITICAL**: All services MUST use the same JWT secret and issuer for proper token validation across the platform.
- **JWT_SECRET**: Must be identical across ALL services (including dev environment)
- **JWT_ISSUER / SECURITY_JWT_ISSUER**: Must be the same across ALL services for token validation to work
- Example for all services:
  - JWT_SECRET: `randlotteryltd-jwt-secret-h8f9x2w1v7q5k3m9n6b4c8d2e5f7g0j1`
  - JWT_ISSUER: `randlotteryltd`

### Metrics Ports
Each service uses different metrics ports to avoid conflicts:
- Admin Management: 9092
- Agent Auth: 9092
- Agent Management: 8082
- API Gateway: 9090
- Terminal: 8088
- Wallet: 9093

### Environment Detection
All services check for `ENVIRONMENT` or `ENV` variables to determine if running in local mode (which enables defaults).