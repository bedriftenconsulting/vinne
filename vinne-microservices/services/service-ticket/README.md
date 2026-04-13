# Ticket Service

Lottery ticket management service for Rand Lottery platform.

## Features

- **Ticket Issuance** - Issue tickets with multiple bet lines (straight, perm)
- **Multi-Channel Support** - POS, web, mobile app, USSD, Telegram, WhatsApp
- **Ticket Validation** - Verify ticket authenticity via QR, barcode, serial number
- **Ticket Management** - Cancel, void, reprint tickets
- **Winning Validation** - Check if tickets won and process payouts
- **Security Features** - SHA-256 hash, QR codes, barcodes, verification codes
- **Sales Reporting** - Issuer sales reports and customer history

## Key Design Principles

### Single Ticket with Multiple Bet Lines
- **No batch purchases** - Each ticket is independent
- Tickets can have multiple bet lines with cost breakdown
- Each bet line has its own bet type (straight or perm)

### Multi-Channel Issuance
Tickets can be issued from:
- **POS** - Retailer/agent terminals
- **Web** - Player web portal
- **Mobile App** - Player mobile application
- **USSD** - USSD sessions
- **Telegram** - Telegram bot
- **WhatsApp** - WhatsApp bot

### Bet Types
1. **Straight** - Direct number selection
2. **Perm (Permutation)** - Uses banker + opposed numbers

### Ticket Lifecycle
1. **issued** - Ticket created and paid for
2. **validated** - Ticket scanned and verified
3. **won** - Ticket matched winning numbers
4. **paid** - Winnings paid out
5. **cancelled** - Ticket cancelled before draw
6. **expired** - Draw passed, not claimed
7. **void** - Invalidated by admin

## Setup

### Database Setup

1. Create database:
```bash
createdb ticket_service
```

2. Run migrations:
```bash
goose -dir migrations postgres "postgresql://ticket:ticket123@localhost:5438/ticket_service?sslmode=disable" up
```

### Configuration

Create `config.env` file:
```env
# Server Configuration
SERVER_PORT=50059
SERVER_HOST=localhost

# Database Configuration
DATABASE_URL=postgresql://ticket:ticket123@localhost:5438/ticket_service?sslmode=disable

# Redis Configuration
REDIS_URL=redis://localhost:6385/0

# Service Discovery
GAME_SERVICE_URL=localhost:50054
DRAW_SERVICE_URL=localhost:50055
PAYMENT_SERVICE_URL=localhost:50056
WALLET_SERVICE_URL=localhost:50053

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=ticket-service

# OpenTelemetry
OTEL_EXPORTER_JAEGER_ENDPOINT=http://localhost:14268/api/traces
SERVICE_NAME=ticket-service

# Serial Number Generation
SERIAL_PREFIX=TKT
```

### Run Service

```bash
go run cmd/server/main.go
```

## API (gRPC)

See `proto/ticket/v1/ticket.proto` for full API definition.

### Core RPCs

#### Ticket Operations
- `IssueTicket` - Issue a new ticket with bet lines
- `ValidateTicket` - Validate ticket authenticity
- `CancelTicket` - Cancel ticket before draw
- `VoidTicket` - Admin void operation
- `ReprintTicket` - Request ticket reprint

#### Queries
- `GetTicket` - Get ticket by ID
- `GetTicketBySerial` - Get ticket by serial number
- `ListTickets` - List tickets with filters
- `GetValidationHistory` - Get ticket validation history

#### Winning and Payment
- `CheckWinning` - Check if ticket is winning
- `PayWinnings` - Process winning payout

#### Reporting
- `GetIssuerSales` - Get sales report for issuer
- `GetCustomerHistory` - Get customer ticket history

## Database Schema

### Main Tables

#### tickets
- Game and draw information (game_code, game_schedule_id, draw_number)
- Number selections (selected_numbers, banker_numbers, opposed_numbers)
- Bet lines (JSONB array) with pricing breakdown
- Issuer tracking (type, ID, details)
- Customer information (optional)
- Payment details
- Security features (hash, QR code, barcode)
- Lifecycle states and timestamps
- All monetary values in pesewas (BIGINT)

#### ticket_payments
- Winner/claimant information
- Bank details for large payouts
- Payment workflow tracking
- Approval process

#### ticket_cancellations
- Cancellation tracking
- Refund processing
- Approval workflow

#### ticket_voids
- Admin-only void operations
- Fraud/error tracking

#### ticket_reprints
- Reprint request tracking
- Terminal and printer information

#### ticket_validations
- Track every validation/scan attempt
- Validation method and result
- Location and context

## Business Rules

### Ticket Issuance
1. Validate game exists and is active
2. Check draw cutoff time hasn't passed
3. Validate bet lines against game rules
4. Calculate pricing based on bet type
5. Generate security features (hash, QR, barcode)
6. Debit payment from wallet/process payment
7. Emit `ticket.issued` event

### Ticket Validation
1. Verify security hash
2. Check ticket status
3. Check expiration
4. Record validation attempt
5. Emit `ticket.validated` event

### Ticket Cancellation
1. Only allowed before draw cutoff time
2. Calculate refund amount
3. Process refund to original payment method
4. Update ticket status
5. Emit `ticket.cancelled` event

### Winning Check
1. After draw results published
2. Match numbers against winning numbers
3. Determine prize tier
4. Calculate winning amount
5. Update ticket status
6. Emit `ticket.won` event

### Payout Processing
1. Verify ticket is winning
2. Verify not already paid
3. Collect claimant information
4. For large amounts, require ID verification
5. Process payment
6. Update ticket status
7. Emit `ticket.paid` event

## Integration Points

### Service Dependencies
1. **Game Service** - Validate game rules, bet types
2. **Draw Service** - Get scheduled games, check cutoff times
3. **Payment Service** - Process ticket purchases and payouts
4. **Wallet Service** - Debit/credit player wallets

### Events Published (Kafka)
1. `ticket.issued` - New ticket created
2. `ticket.validated` - Ticket scanned/verified
3. `ticket.cancelled` - Ticket cancelled
4. `ticket.won` - Winning ticket identified
5. `ticket.paid` - Winnings paid out

## Security Features

Each ticket includes:
1. **QR Code** - For quick scanning
2. **Barcode** - Alternative scanning method
3. **Security Hash** - SHA-256 of ticket data + secret
4. **Serial Number** - Unique sequential number (TKT-XXXXX)
5. **Verification Code** - Short code for phone validation

## Money Handling

**IMPORTANT**: All monetary values are stored in **pesewas** (100 pesewas = 1 GHS)

Database fields using pesewas:
- `unit_price` - Price per line
- `total_amount` - Total ticket cost
- `winning_amount` - Prize amount
- `refund_amount` - Refund amount
- `prize_amount` - Payment amount

## TODO - Implementation Pending

The following need to be implemented:

1. **Models** (`internal/models/`) - Domain models with validation
2. **Repositories** (`internal/repositories/`) - Data access layer with caching
3. **Services** (`internal/services/`) - Business logic layer
   - `ticket_service.go` - Core ticket operations
   - `security_service.go` - Security feature generation
4. **gRPC Handlers** (`internal/handlers/`) - gRPC request handlers
5. **Configuration** (`internal/config/`) - Viper-based config
6. **Events** (`internal/events/`) - Kafka event producers/consumers
7. **Cache** (`internal/cache/`) - Redis caching layer
8. **Middleware** (`internal/middleware/`) - Auth, tracing, etc.
9. **Main** (`cmd/server/main.go`) - Service entry point
10. **Tests** - Integration tests with Testcontainers

## Architecture

Follows the standard microservice pattern:

```
Request → gRPC Handler → Service Layer → Repository Layer → Database
                              ↓
                         Event Publisher → Kafka
                              ↓
                         Cache Layer → Redis
```

## Development

### Generate Proto Code

```bash
cd proto/ticket/v1
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       ticket.proto
```

### Run Migrations

```bash
# Up
goose -dir migrations postgres $DATABASE_URL up

# Down
goose -dir migrations postgres $DATABASE_URL down

# Create new migration
goose -dir migrations create migration_name sql
```

### Code Quality

```bash
go fmt ./...
go vet ./...
golangci-lint run
```

### Testing

```bash
go test ./...
```
