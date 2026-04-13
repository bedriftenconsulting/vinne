# Service Notification - Kafka Event Consumption & Notification Logic Analysis

## Executive Summary

The `service-notification` is a gRPC-based microservice responsible for consuming Kafka events (game draw execution and sales cutoff notifications), managing notification queues, rendering email templates, and sending notifications via external providers (Mailgun for email, Hubtel for SMS).

**Key Architecture:**
- Two Kafka event consumers: General notifications and game-specific events
- Redis-based queue management with worker pool pattern
- Template-driven notification system with support for email/SMS/push
- Idempotency tracking to prevent duplicate notifications
- Provider abstraction for email/SMS implementations

---

## 1. Kafka Event Consumers

### 1.1 General Notification Consumer

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/kafka/consumer.go`

**Purpose:** Consumes direct notification requests from the `notification.requests` Kafka topic.

**Key Components:**

```go
type KafkaConsumer struct {
    config           ConsumerConfig
    eventBus         events.EventBus
    queue            queue.QueueManager
    idempotencyStore idempotency.IdempotencyStore
    logger           logger.Logger
    tracer           trace.Tracer
}
```

**Configuration:**
- **Consumer Group:** `notification-service`
- **Topic:** `notification.requests`
- **Session Timeout:** 30 seconds
- **Heartbeat Interval:** 10 seconds
- **Max Retries:** 3
- **Retry Delay:** 5 seconds

**Flow (Lines 73-217):**
1. Subscribe to notification.requests topic (Line 83)
2. Parse incoming notification requests (Line 148)
3. Validate required fields:
   - `idempotency_key` (Line 232)
   - `channel` (Line 235)
   - `recipient` (Line 238)
   - `template_id` (Line 241)
4. Perform idempotency check (Line 166)
5. Create queue item (Line 179)
6. Enqueue for processing (Line 190)

**Idempotency Management (Lines 248-277):**
- Checks if notification already processed
- Handles three statuses:
  - `StatusCompleted` - Skip notification (duplicate)
  - `StatusPending` - Already being processed
  - Otherwise - Proceed with processing

---

### 1.2 Game Event Consumer

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/kafka/game_events_consumer.go`

**Purpose:** Consumes game-specific events (draw execution and sales cutoff) and generates admin notifications.

**Key Components:**

```go
type GameEventConsumer struct {
    config           ConsumerConfig
    eventBus         events.EventBus
    queue            queue.QueueManager
    logger           logger.Logger
    tracer           trace.Tracer
    adminClient      clients.AdminClient
    idempotencyStore idempotency.IdempotencyStore
    fallbackRecipients []string  // Used if admin client fails
}
```

**Configuration:**
- **Consumer Group:** `notification-service-game-events`
- **Topic:** `game.events`
- **Supported Event Types:**
  - `DrawExecuted` (Line 207)
  - `SalesCutoffReached` (Line 209)

**Recipient Resolution (Lines 108-158):**

The consumer dynamically fetches admin email addresses from the Admin Management Service:

1. Attempts to fetch active admin emails via gRPC (Line 114)
2. Falls back to configured recipients if admin client unavailable (Line 125)
3. Returns empty list handling with fallback (Line 136)
4. Traces usage of fallback (Line 121-124)

**Configuration:**
- **Admin Service Address:** Environment variables `ADMIN_MANAGEMENT_HOST` and `ADMIN_MANAGEMENT_PORT`
- **Fallback Recipients:** `NOTIFICATION_GAME_END_RECIPIENTS`
- **Default Fallback:** `["paulakabah@gmail.com", "jeffrey@bedriften.xyz"]` (Lines 207-208 in main.go)

---

## 2. Event Handlers

### 2.1 Draw Executed Event Handler

**Location:** `game_events_consumer.go`, Lines 244-348

**Triggered By:** `events.DrawExecuted` event type

**Data Expected from Event:**

```go
type GameDrawExecutedEvent struct {
    BaseEvent
    ScheduleID        string    // Game schedule identifier
    GameID            string    // Game identifier
    GameName          string    // Display name of game
    GameCode          string    // Short code for game
    DrawID            string    // Unique draw identifier
    ScheduledDrawTime time.Time // When draw was scheduled
    ActualDrawTime    time.Time // When draw actually executed
    ExecutedBy        string    // "scheduler-service"
}
```

**Source File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/shared/events/game_events.go` (Lines 159-198)

**Processing Steps (Lines 244-349):**

1. **Parse Event** (Line 254)
   - Unmarshal JSON payload into `GameDrawExecutedEvent`
   
2. **Idempotency Check** (Lines 276-297)
   - Generate event hash using SHA256 (Line 277)
   - Store event-level idempotency to prevent duplicates
   - Skip if already processed

3. **Get Recipients** (Line 300)
   - Call `getNotificationRecipients()` for dynamic email list
   - Admin emails + fallback mechanism

4. **Send Notifications** (Lines 307-325)
   - Loop through each recipient
   - Call `sendGameEndNotification()` for each
   - Continue on error (partial failure tolerance)

5. **Mark Completion** (Lines 328-341)
   - Record in idempotency store with metadata
   - Tracks recipients_processed, notifications_sent

**Notification Template Population (Lines 361-391):**

```go
Variables: map[string]any{
    "CompanyName":       "RAND Lottery",
    "GameName":          drawEvent.GameName,
    "GameCode":          drawEvent.GameCode,
    "ScheduledDrawTime": "3:04 PM, Jan 2, 2006",  // Formatted time
    "ActualDrawTime":    "3:04 PM, Jan 2, 2006",  // Formatted time
    "DrawID":            drawEvent.DrawID,
    "ScheduleID":        drawEvent.ScheduleID,
    "NotificationTime":  "3:04 PM, Jan 2, 2006",  // Current time
    "CurrentYear":       "2006",                  // Current year
    "CompanyAddress":    "Accra, Ghana",
}
```

**Template:** `game_end` (TemplateID)

---

### 2.2 Sales Cutoff Reached Event Handler

**Location:** `game_events_consumer.go`, Lines 429-532

**Triggered By:** `events.SalesCutoffReached` event type

**Data Expected from Event:**

```go
type GameSalesCutoffReachedEvent struct {
    BaseEvent
    ScheduleID       string    // Game schedule identifier
    GameID           string    // Game identifier
    GameName         string    // Display name of game
    GameCode         string    // Short code for game
    ScheduledEndTime time.Time // When sales were scheduled to close
    ActualCutoffTime time.Time // When sales actually closed
    NextDrawTime     time.Time // When next draw is scheduled
    ExecutedBy       string    // "scheduler-service"
}
```

**Source File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/shared/events/game_events.go` (Lines 200-238)

**Processing Steps (Lines 429-532):**

1. **Parse Event** (Line 439)
   - Unmarshal JSON payload into `GameSalesCutoffReachedEvent`

2. **Idempotency Check** (Lines 458-480)
   - Generate event hash using SHA256 (Line 460)
   - Store event-level idempotency to prevent duplicates
   - Skip if already processed

3. **Get Recipients** (Line 483)
   - Call `getNotificationRecipients()` for dynamic email list

4. **Send Notifications** (Lines 490-508)
   - Loop through each recipient
   - Call `sendSalesCutoffNotification()` for each
   - Continue on error (partial failure tolerance)

5. **Mark Completion** (Lines 511-524)
   - Record in idempotency store with metadata

**Notification Template Population (Lines 543-575):**

```go
Variables: map[string]any{
    "CompanyName":      "RAND Lottery",
    "GameName":         salesCutoffEvent.GameName,
    "GameCode":         salesCutoffEvent.GameCode,
    "ScheduledEndTime": "3:04 PM, Jan 2, 2006",     // Formatted time
    "ActualCutoffTime": "3:04 PM, Jan 2, 2006",     // Formatted time
    "NextDrawTime":     "3:04 PM, Jan 2, 2006",     // Formatted time
    "ScheduleID":       salesCutoffEvent.ScheduleID,
    "NotificationTime": "3:04 PM, Jan 2, 2006",     // Current time
    "CurrentYear":      "2006",                     // Current year
    "CompanyAddress":   "Accra, Ghana",
}
```

**Template:** `sales_cutoff` (TemplateID)

---

## 3. Notification Queue System

### 3.1 Queue Item Structure

**File:** Queue item payload wrapping (Lines 393-414 in game_events_consumer.go)

```go
type QueueItem struct {
    ID        string              // Unique queue item ID
    Type      string              // "email", "sms", "push"
    Channel   string              // "email", "sms", "push"
    Priority  int                 // 1=low, 2=normal, 3=high, 4=critical
    Payload   map[string]any      // Contains "notification_request"
    RetryCount int
    MaxRetries int                // Default: 3
    CreatedAt time.Time
    Tags      map[string]string   // For tracking/filtering
}
```

**Payload Wrapping (Line 400):**

```go
Payload: map[string]any{
    "notification_request": notificationRequest,
}
```

---

### 3.2 Queue Worker

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/queue/worker.go`

**Configuration (Lines 254-268 in main.go):**

```go
workerConfig := queue.WorkerConfig{
    WorkerID:       "notification-worker-1",
    PollInterval:   5 * time.Second,
    MaxConcurrency: 10,
    RetryBackoff:   30 * time.Second,
    MaxBackoff:     5 * time.Minute,
    BatchSize:      1,
}
```

**Processing Flow (Lines 178-226):**

1. **Dequeue Item** (Line 185)
   - Pull from Redis queue

2. **Claim Item** (Line 191)
   - Mark as claimed by worker to prevent duplicate processing

3. **Parse Queue Item** (Line 229-257)
   - Handles multiple payload formats:
     - Wrapped `notification_request` (from Kafka)
     - Direct payload format (from templates)
     - Map-based conversion

4. **Process by Channel** (Lines 424-433)
   - Route to appropriate processor:
     - `SendEmail()` for email
     - `SendSMS()` for SMS
     - `SendPush()` for push (not yet implemented)

5. **Retry Logic** (Lines 436-525)
   - On failure: Calculate exponential backoff delay
   - Re-enqueue with incremented retry_count
   - After max retries: Send to Dead Letter Queue (DLQ)

---

## 4. Template System

### 4.1 Template Configuration

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/templates/templates.go`

**Template Registry (Lines 30-44):**

```go
var EmailTemplates = map[TemplateName]TemplateObj{
    TemplateNameWelcome:       {Path: "welcome/welcome_email.html", Subject: "Welcome to RAND Lottery!"},
    TemplateNamePasswordReset: {Path: "password_reset/password_reset.html", Subject: "Password Reset Request"},
    TemplateNameGameEnd:       {Path: "game_end/game_end_email.html", Subject: "Game Draw Completed"},
    TemplateNameSalesCutoff:   {Path: "sales_cutoff/sales_cutoff_email.html", Subject: "Game Sales Closed"},
}

var SMSTemplates = map[TemplateName]TemplateObj{
    TemplateNameWelcome:      {Path: "welcome/welcome_sms.txt"},
    TemplateNameVerification: {Path: "password_reset/verification_sms.txt"},
}

var PushTemplates = map[TemplateName]TemplateObj{
    TemplateNameWelcome: {Path: "welcome/welcome_push.json"},
}
```

**Template Base Path:** `./internal/templates/public`

### 4.2 Email Templates for Game Events

**Game End Email Template**

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/templates/public/game_end/game_end_email.html`

**Key Variables:**
- `{{.CompanyName}}` - "RAND Lottery"
- `{{.GameName}}` - Game name from event
- `{{.GameCode}}` - Game code from event
- `{{.ScheduledDrawTime}}` - Formatted scheduled time
- `{{.ActualDrawTime}}` - Formatted actual time
- `{{.DrawID}}` - Draw identifier
- `{{.ScheduleID}}` - Schedule identifier
- `{{.NotificationTime}}` - When notification generated
- `{{.CurrentYear}}` - Current year for footer
- `{{.CompanyAddress}}` - "Accra, Ghana"

**Design Elements:**
- Green header with gradient (RGB: 40,167,69 → 32,201,151)
- Status badge: "Draw Executed"
- Info box with draw details
- Next steps for admins
- Footer with company info

**Sales Cutoff Email Template**

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/templates/public/sales_cutoff/sales_cutoff_email.html`

**Key Variables:**
- `{{.CompanyName}}` - "RAND Lottery"
- `{{.GameName}}` - Game name from event
- `{{.GameCode}}` - Game code from event
- `{{.ScheduledEndTime}}` - Formatted scheduled end time
- `{{.ActualCutoffTime}}` - Formatted actual cutoff time
- `{{.NextDrawTime}}` - Formatted next draw time
- `{{.ScheduleID}}` - Schedule identifier
- `{{.NotificationTime}}` - When notification generated
- `{{.CurrentYear}}` - Current year for footer
- `{{.CompanyAddress}}` - "Accra, Ghana"

**Design Elements:**
- Orange header with gradient (RGB: 255,107,53 → 247,147,30)
- Status badge: "Sales Cutoff Reached"
- Warning alert box about no more ticket sales
- Info box with timing details
- Next steps for admins

---

## 5. Email Recipient Handling

### 5.1 Dynamic Admin Email Fetching

**Admin Client Interface**

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/clients/admin_client.go`

**Methods (Lines 15-19):**

```go
type AdminClient interface {
    GetActiveAdminEmails(ctx context.Context) ([]string, error)
    Close() error
}
```

**Implementation - GetActiveAdminEmails (Lines 64-86):**

1. **Call Admin Service** (Lines 68-71)
   - gRPC call to Admin Management Service
   - ListAdminUsersRequest with:
     - Page: 1
     - PageSize: 1000 (assume max 1000 admins)
     - IsActive: true (only active users)

2. **Extract Emails** (Lines 78-83)
   - Filter for users with:
     - Non-empty email address
     - IsActive flag set to true
   - Build email list slice

3. **Error Handling** (Lines 74-75)
   - Returns error if gRPC call fails
   - Consumer falls back to static recipients

**Connection Details**

**Service Address:** Environment variables
- `CLIENTS_ADMIN_MANAGEMENT_HOST` (default: localhost)
- `CLIENTS_ADMIN_MANAGEMENT_PORT` (default: 50057)

**Connection Timeout:** 5 seconds (Line 40)

**Credentials:** Insecure (no TLS in local dev)

### 5.2 Fallback Recipient Configuration

**Configuration Location:**
- Environment variable: `NOTIFICATION_GAME_END_RECIPIENTS`
- Format: Comma-separated list of emails

**Default Fallback (main.go, Lines 207-208):**

```go
gameEventRecipients := []string{"paulakabah@gmail.com", "jeffrey@bedriften.xyz"}
```

**Fallback Scenarios:**
1. Admin client initialization fails (Line 188-193)
2. No active admin emails returned from service (Line 128-136)
3. Admin client nil (Line 150-157)

**Tracing:** Fallback usage tracked with OpenTelemetry spans (Line 121-124)

---

## 6. Notification Sending Implementation

### 6.1 Send Notification Service

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/services/service-notification/internal/services/send_notification_service.go`

**Email Sending Flow (Lines 57-155):**

1. **Create Notification** (Line 68)
   - Process notification request (template rendering, idempotency check)

2. **Check Duplicates** (Lines 76-80)
   - Return cached notification if duplicate

3. **Validate Recipients** (Lines 85-89)
   - At least one recipient required

4. **Build Email Request** (Lines 91-112)
   ```go
   emailReq := &providers.EmailRequest{
       To:          notification.Recipient[0].Address,
       Subject:     notification.Subject,
       HTMLContent: notification.Content,
       Priority:    providers.PriorityNormal,
       CC:          []string{...},  // Optional
       BCC:         []string{...},  // Optional
   }
   ```

5. **Send via Provider** (Line 114)
   - Call provider manager: `SendEmail(ctx, emailReq)`

6. **Handle Response** (Lines 131-144)
   - Mark notification as sent with provider response
   - Update idempotency record
   - Record metrics

**SMS Sending Flow (Lines 193-268):**

Similar to email:
1. Process notification
2. Validate recipients
3. Build SMS request (to, content, priority)
4. Send via provider manager
5. Update status and idempotency

**Bulk Email Sending (Lines 157-191):**

1. Process all notifications asynchronously in goroutine
2. Use concurrency control (max 50 concurrent)
3. Send each email directly (not through queue)
4. Track success/failure counts

---

### 6.2 Email/SMS Provider Management

**Provider Manager Interface:**

```go
providerManager.SendEmail(ctx, emailReq) → *providers.EmailResponse
providerManager.SendSMS(ctx, smsReq) → *providers.SMSResponse
```

**Configured Providers (from .env):**

1. **Email:** Mailgun
   - Environment variables: `PROVIDERS_EMAIL_MAILGUN_*`

2. **SMS:** Hubtel
   - Environment variables: `PROVIDERS_SMS_HUBTEL_*`

---

## 7. Idempotency Management

### 7.1 Idempotency Store

**Two-Level Idempotency:**

**1. Event-Level Idempotency (game_events_consumer.go)**

- **For Draw Events (Lines 276-297):**
  - Key: `event-processed-draw-{EventID}-{ScheduleID}`
  - Hash: SHA256 of entire GameDrawExecutedEvent
  - Tracks: Prevents duplicate draw notifications

- **For Cutoff Events (Lines 458-480):**
  - Key: `event-processed-cutoff-{EventID}-{ScheduleID}`
  - Hash: SHA256 of entire GameSalesCutoffReachedEvent
  - Tracks: Prevents duplicate cutoff notifications

**2. Notification Request-Level Idempotency (consumer.go)**

- **Key:** User-provided `idempotency_key` from request
- **Hash:** Generated from channel, recipient, template, event_id
- **Statuses:**
  - `StatusCompleted` - Skip (already sent)
  - `StatusPending` - Skip (being processed)
  - Otherwise - Process

### 7.2 Idempotency Record Storage

**Storage:** Redis
**TTL:** Configured in idempotency store
**Fields:**
- Key: Idempotency key
- Hash: Content hash (for duplicate detection)
- Status: Completed/Pending/Failed
- Result/Error: Response data or error message

---

## 8. Service Initialization

**File:** `/Users/paulakabah/Projects/nexa/randco/randco-microservices/cmd/server/main.go`

**Startup Sequence (Lines 44-320):**

1. **Load Configuration** (Line 50)
   - From `.env` or environment variables

2. **Initialize Logger** (Lines 55-65)

3. **Initialize Tracing** (Lines 67-84)
   - OpenTelemetry with Jaeger endpoint: `http://localhost:4318`

4. **Connect to Database** (Lines 86-90)
   - PostgreSQL connection pool

5. **Initialize Repositories** (Line 93)
   - Notification repository

6. **Initialize Providers** (Lines 96-103)
   - Email providers (Mailgun)
   - SMS providers (Hubtel)

7. **Connect to Redis** (Lines 105-135)
   - Cache manager
   - Queue manager
   - Idempotency store

8. **Initialize Kafka Event Bus** (Lines 150-163)
   - Consumer group: `notification-service-eventbus`

9. **Start Kafka Consumers** (Lines 165-224)

   **A. General Notification Consumer**
   - Subscribes to `notification.requests` topic
   - Consumer group: `notification-service`

   **B. Game Event Consumer**
   - Subscribes to `game.events` topic
   - Consumer group: `notification-service-game-events`
   - Initializes admin client for dynamic recipient fetching
   - Sets fallback recipients from config

10. **Initialize gRPC Services** (Lines 244-305)
    - Notification service
    - Health check service
    - Reflection for grpcurl

11. **Start Queue Worker** (Lines 254-282)
    - Worker ID: "notification-worker-1"
    - Poll interval: 5 seconds
    - Max concurrency: 10

---

## 9. Configuration Environment Variables

### Critical Configuration

```bash
# Server
SERVER_PORT=50063

# Database (PostgreSQL)
DATABASE_URL=postgresql://notification:notification123@localhost:5443/notification

# Redis
REDIS_URL=redis://localhost:6390/0

# Kafka
KAFKA_BROKERS=localhost:9092

# Admin Service Connection
CLIENTS_ADMIN_MANAGEMENT_HOST=localhost
CLIENTS_ADMIN_MANAGEMENT_PORT=50057

# Game Event Notification Recipients (Fallback)
NOTIFICATION_GAME_END_RECIPIENTS=paulakabah@gmail.com,jeffrey@bedriften.xyz

# Email Provider (Mailgun)
PROVIDERS_EMAIL_MAILGUN_API_KEY=<mailgun_api_key>
PROVIDERS_EMAIL_MAILGUN_DOMAIN=<mailgun_domain>
PROVIDERS_EMAIL_MAILGUN_FROM_EMAIL=<from_email>

# SMS Provider (Hubtel)
PROVIDERS_SMS_HUBTEL_API_KEY=<hubtel_api_key>
PROVIDERS_SMS_HUBTEL_CLIENT_ID=<hubtel_client_id>
```

---

## 10. Flow Diagrams

### Game Draw Executed Notification Flow

```
[Draw Service] 
    ↓ (publishes DrawExecuted event)
[Kafka: game.events topic]
    ↓ (GameEventConsumer subscribes)
[GameEventConsumer.handleGameEvent()]
    ↓
[Parse GameDrawExecutedEvent]
    ↓
[Idempotency Check: event-processed-draw-{id}]
    ├─ If duplicate → Skip (return)
    └─ If new → Continue
    ↓
[getNotificationRecipients()]
    ├─ Try: Admin Service gRPC call
    ├─ Fallback: Static config recipients
    └─ Return: List of admin emails
    ↓
[For each recipient]
    ↓
[sendGameEndNotification(recipient, drawEvent)]
    ├─ Create NotificationRequest
    ├─ Populate template variables
    ├─ Create QueueItem
    └─ Enqueue to Redis queue
    ↓
[Mark event as completed in idempotency store]
    ↓
[QueueWorker dequeues and processes]
    ├─ Parse notification_request
    ├─ Render template with variables
    ├─ Build email request
    ├─ Send via Mailgun provider
    └─ Update notification status (sent)
```

### Sales Cutoff Reached Notification Flow

```
[Draw Service] 
    ↓ (publishes SalesCutoffReached event)
[Kafka: game.events topic]
    ↓ (GameEventConsumer subscribes)
[GameEventConsumer.handleGameEvent()]
    ↓
[Parse GameSalesCutoffReachedEvent]
    ↓
[Idempotency Check: event-processed-cutoff-{id}]
    ├─ If duplicate → Skip (return)
    └─ If new → Continue
    ↓
[getNotificationRecipients()]
    ├─ Try: Admin Service gRPC call
    ├─ Fallback: Static config recipients
    └─ Return: List of admin emails
    ↓
[For each recipient]
    ↓
[sendSalesCutoffNotification(recipient, cutoffEvent)]
    ├─ Create NotificationRequest
    ├─ Populate template variables
    ├─ Create QueueItem
    └─ Enqueue to Redis queue
    ↓
[Mark event as completed in idempotency store]
    ↓
[QueueWorker dequeues and processes]
    ├─ Parse notification_request
    ├─ Render template with variables
    ├─ Build email request
    ├─ Send via Mailgun provider
    └─ Update notification status (sent)
```

---

## 11. Error Handling & Resilience

### Queue Processing Resilience

**Retry Strategy (worker.go, Lines 436-525):**

1. **Exponential Backoff:**
   - First retry: 30 seconds
   - Second retry: 60 seconds
   - Third retry: 120 seconds
   - Maximum: 5 minutes

2. **Dead Letter Queue (DLQ):**
   - After 3 max retries exceeded
   - Item moved to DLQ for manual review
   - Released from active queue

3. **Partial Failure Tolerance:**
   - Game events continue to next recipient if one fails
   - Tracks success/failure counts separately

### Kafka Consumer Resilience

**Connection Management:**
- Automatic reconnection on broker failure
- Consumer group provides load balancing
- Offset management prevents message loss

---

## 12. Summary of Key Files

| File Path | Purpose | Key Lines |
|-----------|---------|-----------|
| `internal/kafka/consumer.go` | General notification consumer | 73-217 |
| `internal/kafka/game_events_consumer.go` | Game event consumer with draw/cutoff handlers | 72-532 |
| `internal/clients/admin_client.go` | Admin service gRPC client for email fetching | 28-86 |
| `internal/queue/worker.go` | Queue item processor with retry logic | 67-525 |
| `internal/services/send_notification_service.go` | Email/SMS/Push sending logic | 57-823 |
| `internal/templates/templates.go` | Template registry and configuration | 30-109 |
| `internal/templates/public/game_end/game_end_email.html` | Game draw completed email | 1-186 |
| `internal/templates/public/sales_cutoff/sales_cutoff_email.html` | Sales cutoff reached email | 1-207 |
| `cmd/server/main.go` | Service initialization and startup | 44-320 |
| `internal/models/notification.go` | Notification data structures | 1-116 |
| `internal/models/models.go` | NotificationRequest structures | 1-39 |
| `/shared/events/game_events.go` | Event definitions | 159-238 |

---

## 13. Key Design Patterns

### 1. Observer Pattern
- Kafka consumers subscribe to topics
- Event handlers react to specific event types

### 2. Queue Pattern
- Redis-backed job queue
- Worker pool for concurrent processing

### 3. Repository Pattern
- Data access abstraction
- Notification repository handles DB operations

### 4. Strategy Pattern
- Provider abstraction for email/SMS/push
- Plugin-style provider management

### 5. Decorator Pattern
- Idempotency wrapper around event processing
- Template rendering as processing step

