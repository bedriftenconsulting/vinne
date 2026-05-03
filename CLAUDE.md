# CLAUDE.md — Vinne / WinBig Africa

Complete project reference. Every section is written so an AI assistant can understand the system, make correct changes, and deploy without any manual testing or exploration.

---

## What This Project Is

**WinBig Africa** is a raffle/lottery platform for Ghana. Players buy **Draw Entries** (GHS 20 each) via USSD (`*899*92#`) or the web, enter draws to win a prize (current: iPhone 17 Pro), and receive SMS confirmation. Administrators manage games, draws, players, agents, and financials via an internal portal.

> **Terminology:** The items players buy are called **Draw Entries** (never "tickets"). Serial format: `WB-ENT-XXXXXXXX`. This terminology is used everywhere — UI labels, SMS messages, API responses, and code comments.

| Domain | URL |
|---|---|
| Public website | `winbig.bedriften.xyz` |
| Admin portal | `admin.winbig.bedriften.xyz` |
| API | `api.winbig.bedriften.xyz` |
| Git remote | `https://github.com/bedriftenconsulting/vinne.git` |

---

## Servers

| Server IP | Role | SSH key |
|---|---|---|
| `34.121.254.209` | API + admin portal + USSD app | `C:\Users\Suraj\.ssh\google_compute_engine` |
| `34.42.87.251` | Public website only | `C:\Users\Suraj\.ssh\google_compute_engine` |

SSH: `ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@<IP>`

Repo path on both servers: `/home/Suraj/vinne/` (capital S in Suraj)

---

## Monorepo Structure

```
vinne/
├── vinne-microservices/          # Go 1.25 gRPC backend — 12 services
│   ├── services/api-gateway/     # HTTP/REST gateway, port 4000
│   ├── services/service-draw/
│   ├── services/service-ticket/
│   ├── services/service-game/
│   ├── services/service-notification/
│   ├── services/service-payment/
│   ├── services/service-player/
│   ├── services/service-wallet/
│   ├── services/service-admin-management/
│   ├── services/service-agent-auth/
│   ├── services/service-agent-management/
│   ├── services/service-terminal/
│   ├── proto/                    # .proto definitions + generated .pb.go files
│   ├── shared/                   # Shared Go utilities
│   ├── docker-compose.yml        # All infrastructure + service containers
│   └── go.work                   # Go workspace (go 1.25.0 / toolchain go1.26.0)
├── vinne-admin-web/              # React 19 admin dashboard (TypeScript, Vite 7)
├── vinne-website/                # React 18 public player site (TypeScript, Vite 5)
└── vinne-ussd-app/               # Python 3 Flask USSD + payment backend
    └── app.py                    # Single-file app — entire USSD/payment/SMS logic
```

---

## Active Game (as of 2026-05-03)

| Field | Value |
|---|---|
| Game name | iPhone 17 Pro |
| Game code | `IPHONE17` |
| Game ID | `6d02ec42-d611-44d6-97e7-8dbcd69fd300` |
| Game schedule ID | `8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34` |
| **Draw to use** | **Draw #2** — `59ad83f5-499c-4113-9292-1152b93f92c0` |
| Draw date | 2026-05-03 20:00 UTC |
| Entry price | GHS 20 (2000 pesewas) |
| USSD code | `*899*92#` |

### CRITICAL — Two draws exist, only Draw #2 is correct

| Draw | ID | Schedule ID | Entries | Use? |
|---|---|---|---|---|
| Draw #1 | `ed4f45e3-...` | `87e827c2-...` | 0 | ❌ Wrong |
| **Draw #2** | `59ad83f5-499c-4113-9292-1152b93f92c0` | `8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34` | 45+ | ✅ Correct |

The USSD app uses schedule `8aaa6e8d-...` so all USSD purchases land on Draw #2.

---

## Critical Business Rules

### Entry eligibility — ONLY paid entries count

An entry must have `payment_status = 'completed'` to be eligible. Enforced at every layer:
1. Admin UI Entries tab — `GetDrawTickets` filters `payment_status=completed`
2. Draw statistics — `GetDrawStatistics` counts only `payment_status=completed`
3. Stage 1 (preparation) — entry pool built with `payment_status=completed` filter
4. Stage 3 (winner selection) — double filter: `status=issued AND payment_status=completed`

### Entry serial number format

All entries use prefix `WB-ENT-` + 8 characters: e.g. `WB-ENT-A1B2C3D4`.

Controlled by `BUSINESS_SERIAL_PREFIX` env var in `docker-compose.yml` for `service-ticket`:
```yaml
service-ticket:
  environment:
    BUSINESS_SERIAL_PREFIX: WB-ENT
```
All historic `TKT-*` serials were renamed to `WB-ENT-*` via a one-time DB UPDATE. Never change this prefix without also updating existing records.

### Entry count mismatch — draw record vs ticket service

`total_tickets_sold` on the draw record is **stale** — USSD purchases bypass the Go draw service. **Fix in place:** `ListDraws` in `draw_handler.go` concurrently queries the ticket service for real `payment_status=completed` counts and overwrites the stale field before returning.

### Nil UUID handling

Go protobuf default for empty UUID: `"00000000-0000-0000-0000-000000000000"` (not `""`). Both `GetDrawTickets` and `GetDrawStatistics` define `const nilUUID` and treat it as empty when deciding whether to filter by `game_schedule_id` or fall back to `draw_id`.

---

## Unified SMS Format

Both USSD (Flask) and admin bulk/quick upload (Go api-gateway) send the **same SMS**:

```
CarPark payment confirmed!
WinBig Entry:
WB-ENT-XXXXXXXX

Draw: 03 May 2026
Good luck!
```

For multiple entries, label becomes "Entries:" and all serials listed one per line.

- **Sender ID:** `CARPARK` (mNotify)
- **USSD:** `send_winbig_sms()` in `app.py`
- **Bulk/Quick upload:** SMS assembly in `BulkUploadTickets` handler, `draw_handler.go`
- **Resend SMS:** `ResendSMS` handler, `draw_handler.go` — groups by phone, one SMS per person

---

## Component Details

### 1. `vinne-ussd-app/app.py` — Flask USSD App (Python)

- **Runtime:** Python 3, Flask, `127.0.0.1:5001`
- **Systemd service:** `ussd-app.service`
- **USSD code:** `*899*92#` (Hubtel USSD gateway)
- **Payment:** Hubtel MoMo STK push → webhook at `/payment/webhook`
- **SMS:** mNotify API — sender ID `CARPARK`, key in `mnotify_api_key_winbig_sms_key.txt`
- **USSD screen limit:** Max **130 characters**. MTN lenient; Telecel/AirtelTigo strict.

**Key constants:**

| Constant | Value | Notes |
|---|---|---|
| `WINBIG_UNIT_PRICE` | `2000` | GHS 20 in pesewas |
| `MAX_ENTRIES_PER_TXN` | `10` | Max entries per session |
| `TICKET_EXPIRY_MINUTES` | `30` | Pending entries expire |
| `USSD_RATE_LIMIT` | `30 req/60s` | Per-MSISDN |
| `WINBIG_GAME_CODE` | `"IPHONE17"` | Must match game service |
| `WINBIG_GAME_SCHEDULE_ID` | `"8aaa6e8d-..."` | Must match active schedule |
| `DRAW_DATE_LABEL` | `"03 May 2026"` | Used in SMS template |
| `DRAW_NUMBER` | `1` | Stored on ticket record |
| `HUBTEL_CALLBACK_URL` | `https://api.winbig.bedriften.xyz/payment/webhook` | |
| `MAX_TXN_AMOUNT_PESEWA` | `100_000` | GHS 1,000 hard cap |

**Background workers:**

| Worker | Interval | Purpose |
|---|---|---|
| `_sms_retry_worker` | 120s | Retries `sms_sent=FALSE` completed entries |
| `_payment_reconciliation_worker` | 180s | Polls Hubtel for pending payments > 5 min |
| `_ticket_expiry_worker` | 300s | Expires pending entries older than 30 min |

**Key functions:**

- **`create_winbig_tickets(session_id, msisdn, qty)`** — Creates N entries with `payment_status=pending`. Idempotent via session_id. Inserts into `ticket_service` DB with: `serial_number` (WB-ENT-*), `game_code`, `game_schedule_id`, `draw_number`, `bet_lines` (empty JSONB `[]`), `issuer_type='USSD'`, `payment_method='mobile_money'`, `payment_status='pending'`, `status='issued'`

- **`send_winbig_sms(reference)`** — Sends SMS for completed payment. Chunks 5 entries per SMS (mNotify rate limit). Sets `sms_sent=TRUE` on success.

- **`payment_webhook()`** — Receives Hubtel MoMo callbacks. `ResponseCode "0000"` = success. On success: updates `payment_status=completed`, spawns async threads for SMS and payment service sync. IP-checks against `HUBTEL_WEBHOOK_IPS` (logs warning if unlisted, still processes).

- **`_fire_momo_async(msisdn, amount_pesewas, reference)`** — Fires Hubtel MoMo STK push in background thread with 3-second delay (lets USSD session close first).

- **`push_to_payment_service(reference)`** — Syncs one completed payment to `payment_service` DB.

- **`normalise_phone(phone)`** — Normalizes any Ghana format to `233XXXXXXXXX`.

**CRITICAL — USSD bypasses Go microservices:**
Flask inserts directly into `ticket_service` PostgreSQL. This means:
- `total_tickets_sold` on draw record is never updated by USSD purchases
- `bet_lines` for USSD entries is stored as empty JSON array `[]`
- SMS handled by Flask, not notification microservice

**DB connections used by Flask:**

| DB | Port | User | Password |
|---|---|---|---|
| ticket_service | 5442 | ticket | `#kettic@333!` → URL-encoded `%23kettic%40333%21` |
| player_service | 5444 | player | `#yerpla@333!` |
| payment_service | 5440 | payment | `#mentpay@333!` |
| wallet_service | 5438 | wallet | `wallet123` ← not yet rotated |

**Flask endpoints:**
- `POST /payment/webhook` — Hubtel MoMo callbacks
- `POST /ussd` — USSD menu handler (*899*92#)
- `GET /ussd/sessions` — Admin: list USSD sessions
- `GET /ussd/tickets` — Admin: list entries by session
- `POST /api/v1/web-payment/initiate` — Web payment flow
- `GET /api/v1/web-payment/status/{reference}` — Payment status check

**Deploy after changes to app.py:**
```bash
git push origin main
ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@34.121.254.209
cd /home/Suraj/vinne && git pull origin main && sudo systemctl restart ussd-app.service
sudo journalctl -u ussd-app.service -f
```

---

### 2. `vinne-microservices/` — Go Microservices

- **Language:** Go 1.25 (`go.work`: `go 1.25.0 / toolchain go1.26.0`)
- **Module:** `github.com/randco/randco-microservices`
- **API Gateway:** HTTP/REST port `4000`, proxied by nginx
- **Docker Compose version on server:** `1.29.2` — use `sudo docker-compose` (not `docker compose`)

**Infrastructure containers** (docker-compose.yml):
- Jaeger (tracing, port 16686)
- Prometheus + Grafana (metrics)
- Kafka 7.5.0 KRaft mode (port 9092)
- Redis-shared 7.4-alpine (port 6379)
- MinIO S3-compatible storage (port 9000)

**Microservice containers** (each has own PostgreSQL + Redis):

| Service | gRPC port | PostgreSQL port | Purpose |
|---|---|---|---|
| service-admin-management | 51057 | ~5434 | Admin user/role/permission mgmt |
| service-agent-auth | 51052 | ~5436 | Agent authentication |
| service-agent-management | 51056 | ~5437 | Agent profile/commission |
| service-wallet | 51059 | 5438 | Wallet transactions, balances |
| service-terminal | 51054 | ~5439 | POS terminal config/health |
| service-payment | 51061 | 5440 | Mobile money payment processing |
| service-game | 51053 | ~5441 | Game lifecycle management |
| service-draw | 51060 | ~5436 | Draw execution, winner selection |
| service-ticket | 51062 | 5442 | Entry issuance, management |
| service-notification | 51063 | ~5443 | Push notifications, SMS routing |
| service-player | 51064 | 5444 | Player profiles, authentication |

#### Build & deploy (api-gateway)

```bash
# On server:
cd /home/Suraj/vinne/vinne-microservices
sudo docker-compose build api-gateway
sudo docker stop vinne-microservices_api-gateway_1
sudo docker rm vinne-microservices_api-gateway_1
sudo docker-compose up -d api-gateway
sudo docker logs vinne-microservices_api-gateway_1 --tail 20
```

If `docker-compose up` fails with `KeyError: 'ContainerConfig'` (docker-compose 1.29.2 bug) — stop/rm the container first, then `up -d`.

**IMPORTANT — Docker build cache gotcha:** If you edit a Go file locally without pushing to git first, `git pull` on the server won't pick up the change, and Docker's layer cache will silently use the old code. To bypass: SCP the file directly to the server, then build with `--no-cache`:
```bash
scp -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no \
  vinne-microservices/services/api-gateway/internal/handlers/draw_handler.go \
  suraj@34.121.254.209:/home/Suraj/vinne/vinne-microservices/services/api-gateway/internal/handlers/
# Then on server:
sudo docker-compose build --no-cache api-gateway
```

#### Local build (compile check only):
```bash
cd vinne-microservices/services/api-gateway
GOWORK=off go build ./...
```

#### Docker volume gotcha — ticket DB password

`postgres-ticket-data` volume was initialized with `#kettic@333!`. Persists even if docker-compose changes. Must URL-encode in `DATABASE_URL`:
```yaml
DATABASE_URL: postgresql://ticket:%23kettic%40333%21@service-ticket-db:5432/ticket_service?sslmode=disable
```
`%23`=`#`, `%40`=`@`, `%21`=`!`. Never change without also dropping the volume.

#### Key source files in api-gateway

| File | Lines | Purpose |
|---|---|---|
| `cmd/server/main.go` | ~700 | All route registrations |
| `internal/handlers/draw_handler.go` | ~2047 | Full draw lifecycle |
| `internal/handlers/game_handler.go` | ~2063 | Game CRUD, scheduling, prizes |
| `services/service-game/internal/services/game_schedule_service.go` | — | Schedule generation; uses `HasScheduleForGameInRange` to skip per-game if a schedule already exists for the week — prevents duplicate schedules |
| `internal/handlers/player_handler.go` | ~1364 | Player auth, wallet, entries |
| `internal/handlers/ticket_handler.go` | ~1180 | Entry CRUD, analytics |
| `internal/handlers/wallet_handler.go` | ~1283 | Wallet ops, commissions |
| `internal/handlers/admin_auth.go` | ~542 | Admin login, MFA, profile |
| `internal/handlers/admin_management.go` | ~839 | User/role/permission CRUD |
| `internal/handlers/retailer_handler.go` | ~1394 | Retailer mgmt |
| `internal/handlers/agent_auth.go` | ~645 | Agent login, password reset |
| `internal/handlers/payment_handler.go` | ~739 | MoMo topup, verification |
| `internal/handlers/interfaces.go` | ~196 | Handler interface definitions |
| `internal/grpc/client.go` | — | gRPC client manager |

#### Available gRPC clients

```go
h.grpcManager.TicketServiceClient()       // ticketv1.TicketServiceClient
h.grpcManager.GameServiceClient()         // gamepb.GameServiceClient
h.grpcManager.NotificationServiceClient() // notificationv1.NotificationServiceClient
h.grpcManager.GetConnection("draw")       // *grpc.ClientConn
h.grpcManager.GetConnection("player")
h.grpcManager.GetConnection("wallet")
```

#### Key gRPC calls

```go
// List entries
ticketClient.ListTickets(ctx, &ticketv1.ListTicketsRequest{
    Filter:   &ticketv1.TicketFilter{GameScheduleId: "...", PaymentStatus: "completed"},
    Page: 1, PageSize: 100,
})

// Issue a single entry
ticketClient.IssueTicket(ctx, &ticketv1.IssueTicketRequest{
    GameCode: "IPHONE17", GameScheduleId: "8aaa6e8d-...", DrawNumber: 2,
    BetLines: []*ticketv1.BetLine{{LineNumber: 1, BetType: "RAFFLE", TotalAmount: 2000}},
    IssuerType: "ADMIN", IssuerId: "admin-bulk-upload",
    CustomerPhone: "233241234567", PaymentMethod: "external",
})

// Send SMS (unified format)
notifClient.SendSMS(ctx, &notificationv1.SendSMSRequest{
    To: "233241234567",
    Content: "CarPark payment confirmed!\nWinBig Entry:\nWB-ENT-XXXXXXXX\nDraw: 03 May 2026\nGood luck!",
    IdempotencyKey: "unique-key",
})

// Bulk SMS
notifClient.SendBulkSMS(ctx, &notificationv1.SendBulkSMSRequest{
    Requests: []*notificationv1.SendSMSRequest{...},
})

// Resolve game_code when draw.GameCode is empty
gameClient.GetScheduleById(ctx, &gamepb.GetScheduleByIdRequest{ScheduleId: draw.GameScheduleId})
```

#### ListDraws — fields included in response

`ListDraws` in `draw_handler.go` transforms each draw proto into a map. The following fields are included (some required fixes to add):
- `id`, `name`, `status`, `draw_date`, `game_code`, `game_schedule_id` — all present
- `total_tickets_sold` — **overwritten live** from ticket service (concurrent fetch per draw, `payment_status=completed`)
- Pagination: `per_page` param (NOT `page_size`) controls page size

`game_code` and `game_schedule_id` are required so the frontend can correctly filter schedules when computing entry counts.

#### GetDrawTickets filter pattern

```go
const nilUUID = "00000000-0000-0000-0000-000000000000"
filter := &ticketv1.TicketFilter{PaymentStatus: "completed"}
if draw.GameScheduleId != "" && draw.GameScheduleId != nilUUID {
    filter.GameScheduleId = draw.GameScheduleId
} else {
    filter.DrawId = drawID
}
```

#### All API routes

**Public (no auth):**
```
GET  /health, /health/detailed
POST /api/v1/admin/auth/login
POST /api/v1/admin/auth/refresh
POST /api/v1/admin/auth/register
POST /api/v1/players/register
POST /api/v1/players/verify-otp
POST /api/v1/players/resend-otp
POST /api/v1/players/login
POST /api/v1/players/refresh-token
POST /api/v1/players/ussd/register
POST /api/v1/players/ussd/login
POST /api/v1/players/forgot-password
POST /api/v1/players/reset-password
POST /api/v1/players/password-reset/*
POST /api/v1/otp/send, /otp/verify
GET  /api/v1/public/games
GET  /api/v1/public/draws/completed
GET  /api/v1/public/winners
GET  /api/v1/public/tickets/by-phone/{phone}
POST /api/v1/retailer/auth/pos-login
POST /api/v1/agent/auth/login
POST /api/v1/webhooks/orange, /webhooks/mtn, /webhooks/telecel
POST /api/v1/ussd/callback
POST /ussd/callback
```

**Protected admin (prefix `/api/v1/admin`, JWT required):**
```
POST   /auth/logout
GET    /profile                              → GetProfile
PUT    /profile                              → UpdateProfile
POST   /auth/change-password                 → ChangePassword
POST   /auth/mfa/enable, /mfa/verify, /mfa/disable

GET    /users                                → ListUsers
POST   /users                                → CreateUser
GET    /users/{id}                           → GetUser
PUT    /users/{id}                           → UpdateUser
DELETE /users/{id}                           → DeleteUser
POST   /users/{id}/activate
POST   /users/{id}/deactivate

GET    /roles                                → ListRoles
POST   /roles                                → CreateRole
PUT    /roles/{id}                           → UpdateRole
DELETE /roles/{id}                           → DeleteRole
GET    /permissions                          → ListPermissions
POST   /role-assignments                     → AssignRole
DELETE /role-assignments                     → RemoveRole
GET    /audit-logs                           → GetAuditLogs

GET    /games                                → ListGames
POST   /games                                → CreateGame
GET    /games/{id}
PUT    /games/{id}
DELETE /games/{id}
PUT    /games/{id}/prize-structure
POST   /games/{id}/logo
GET    /games/{id}/schedule
GET    /games/{id}/statistics
POST   /scheduling/weekly/generate          → GenerateWeeklySchedule (idempotent — skips per-game if schedule already exists for that week)
GET    /scheduling/weekly
DELETE /scheduling/weekly/clear

GET    /draws                                → ListDraws
POST   /draws                                → CreateDraw
GET    /draws/{id}                           → GetDraw
POST   /draws/{id}/prepare                   → PrepareDraw (Stage 1)
POST   /draws/{id}/execute                   → ExecuteDraw (Stage 2/3)
GET    /draws/{id}/statistics                → GetDrawStatistics
GET    /draws/{id}/tickets                   → GetDrawTickets (paid only)
POST   /draws/{id}/tickets/bulk-upload       → BulkUploadTickets
POST   /draws/{id}/tickets/resend-sms        → ResendSMS
POST   /draws/{id}/machine-numbers           → UpdateMachineNumbers
POST   /draws/{id}/commit-results            → CommitDrawResults
POST   /draws/{id}/process-payout            → ProcessPayout
POST   /draws/{id}/restart                   → RestartDraw
GET    /draws/{id}/results                   → GetDrawResults
POST   /draws/{id}/cancel
POST   /draws/{id}/save-progress

GET    /tickets                              → ListTickets
GET    /tickets/{id}
GET    /tickets/serial/{serial}
POST   /tickets/{id}/void
POST   /tickets/{id}/cancel

GET    /players/{id}
GET    /players/search
PUT    /players/{id}/suspend
PUT    /players/{id}/activate

GET    /agents                               → ListAgents (admin view)
GET    /retailers                            → ListRetailers (admin view)

GET    /analytics/daily-metrics
GET    /analytics/monthly-metrics
GET    /analytics/top-performing-agents

GET    /terminals                            → ListTerminals
POST   /terminals                            → CreateTerminal
PUT    /terminals/{id}/assign
PUT    /terminals/{id}/unassign

GET    /wallet/transactions
PUT    /wallet/transactions/{id}/reverse
POST   /wallet/agent/{id}/credit
POST   /wallet/retailer/{id}/credit
```

**ResendSMS request/response:**
```json
// POST /api/v1/admin/draws/{id}/tickets/resend-sms
// Request:
{ "tickets": [{ "serial_number": "WB-ENT-XXXXXXXX", "customer_phone": "233XXXXXXXXX", "customer_name": "John Doe" }] }
// Response:
{ "total": 3, "sms_sent": 3, "results": [{ "serial_number": "WB-ENT-...", "phone": "233...", "sms_sent": true }] }
```

#### `TicketFilter` proto fields (ticket.proto)

```protobuf
string game_schedule_id = 1;
string draw_id          = 2;
string status           = 3;   // "issued", "won", "cancelled"
string issuer_id        = 4;
string game_code        = 5;
int32  draw_number      = 6;
string player_id        = 7;
string ticket_id        = 8;
string serial_number    = 9;
string payment_status   = 10;  // "completed", "pending", "failed" — field 10 added in commit c458b82
```

#### Draw execution stages

1. **Preparation** — Lock sales, load `payment_status=completed` entries into pool, backfill `draw_id` on entries missing it
2. **Machine numbers** — Admin enters physical random numbers (optional for raffle)
3. **Winner selection** — Filter `status=issued AND payment_status=completed`, select via Google RNG or cryptographic RNG (up to 3 verification attempts)
4. **Commit results** — Mark winning entry `status=won`, record winning numbers
5. **Payouts** — Auto-process normal wins; manual approval for big wins

---

### 3. `vinne-admin-web/` — Admin Portal (React 19)

- **Stack:** React 19, TypeScript, Vite 7, TanStack Router (file-based), TanStack Query, Zustand, Tailwind CSS, Shadcn/ui
- **Served from:** `/var/www/admin/` on `34.121.254.209`
- **API base URL (prod):** `https://api.winbig.bedriften.xyz/api/v1`
- **API base URL (local dev):** Proxied via Vite → `http://34.121.254.209:4000` (configured in `vite.config.ts`)
- **Dev port:** 6176
- **Auth:** JWT stored in `localStorage` as `access_token` (15-min access token, 7-day refresh token)

**Build — ALWAYS use `npx vite build`, NOT `npm run build`:**
Pre-existing TS errors in `transactionsService.ts`, `vite.config.ts`, and `CreateGameWizard.tsx` cause `npm run build` (which runs `tsc`) to fail. Vite's esbuild skips type checking.

```bash
cd vinne-admin-web
npx vite build
```

**Deploy:**
```bash
scp -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no -r dist/. suraj@34.121.254.209:~/admin-dist/
ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@34.121.254.209 "sudo cp -r ~/admin-dist/. /var/www/admin/ && echo done"
```

**Environment variables (`.env.localdev`):**
```
VITE_API_URL=/api/v1
VITE_APP_URL=http://localhost:6176
VITE_ENVIRONMENT=local
VITE_ENABLE_DEBUG=true
VITE_LOG_LEVEL=debug
VITE_RANDOM_ORG_API_KEY=5a005036-3424-44c5-afb5-b7cc7a2e244b
```

#### Authentication & Auth Store

**`src/stores/auth.ts`** — Zustand store:
- State: `user`, `isAuthenticated`, `isLoading`, `isInitialized`
- Actions: `adminLogin()`, `adminLogout()`, `refreshToken()`, `validateAuth()`, `initializeAuth()`
- Persists to localStorage via Zustand middleware
- `initializeAuth()` called on app boot; validates existing token

**`src/lib/api.ts`** — Axios instance:
- Base URL from `VITE_API_URL` env var (defaults to `/api/v1`)
- Request interceptor: adds `Authorization: Bearer <access_token>`
- Response interceptor: on 401 → auto-refresh token → retry; on failure → redirect to `/login`
- Timeout: 30 seconds

**`src/utils/authCheck.ts`** — TanStack Router guard:
- `requireAuth()` — reads Zustand store, triggers `initializeAuth()` if not initialized, redirects to `/login` if not authenticated
- Used as `beforeLoad` on all protected routes

#### Role-based navigation

Sidebar hides/shows sections based on JWT `roles` claim. Logic in `AdminLayout.tsx`:

| Role | Operations | Commerce | Administration |
|---|---|---|---|
| `super_admin` / `admin` | ✅ | ✅ | ✅ |
| `commerce_manager` / `manager` | ❌ | ✅ | ❌ |

**IMPORTANT — logout flash fix:** `isCommerceOnly` defaults to `true` when `userRoles` is empty (token cleared during logout). This prevents a flash of the full nav during redirect. Never revert to `userRoles.length > 0 &&` logic.

```js
const isCommerceOnly = userRoles.length === 0 ||
  userRoles.every(r => r === 'commerce_manager' || r === 'manager')
```

#### All admin portal routes

| Path | Page file | Notes |
|---|---|---|
| `/dashboard` | `Dashboard.tsx` | Analytics overview, revenue metrics |
| `/games` | `Games.tsx` | Game list + create wizard; Resume/Delete actions on each row |
| `/game/:gameId` | `GameConfiguration.tsx` | Edit game, prize structure, schedule |
| `/draws` | `Draws.tsx` | Draw list with real entry counts |
| `/draw/:drawId` | `DrawDetails.tsx` | Full draw management (3 tabs) |
| `/wins` | `WinsModule.tsx` | Winner management |
| `/entries` | `QuickAddEntry.tsx` | Quick Add + Bulk Import (commerce-accessible) |
| `/players` | `Players.tsx` | Player search, list |
| `/players/:id` | `PlayerProfile.tsx` | Player detail, wallet, entries |
| `/transactions` | `Transactions.tsx` | Transaction list, analytics |
| `/ussd-sessions` | `UssdSessions.tsx` | USSD session tracking |
| `/admin/users` | `AdminUsers.tsx` | Admin user management |
| `/admin/roles` | `AdminRoles.tsx` | Role management |
| `/admin/permissions` | `AdminPermissions.tsx` | Permission list |
| `/admin/audit-logs` | `AuditLogs.tsx` | Audit trail |
| `/config/winner-selection` | `WinnerSelectionConfig.tsx` | RNG method config |
| `/settings` | `Settings.tsx` | Own profile + change password |

#### Key pages — detailed features

**`/draw/:drawId` — Draw detail (3 tabs):**

*Overview tab:*
- Stats cards: Total Entries, Total Winners, Total Winnings (all real-time from ticket service)
- Draw execution flow UI: stages 1–5 with progress indicators and action buttons
- Machine numbers entry dialog
- Restart draw button + dialog
- **Winner Exclusions card** — per-draw list of phone numbers excluded from winning (e.g. organizers). Stored in `localStorage` as `draw_exclusions_${drawId}`. Phones shown as red pills with remove button. Passed to `executeGoogleRNGSelection` / `executeCryptographicSelection` as `excludedPhones` — excluded phones are removed from the eligible pool before RNG runs, so they cannot be selected.

*Entries tab:*
- Shows only `payment_status=completed` entries
- Filter bar: serial number search + issuer type dropdown (USSD/Admin)
- **Send SMS button** — resends SMS to all currently-filtered entries via `POST /draws/{id}/tickets/resend-sms`; shows per-entry delivery status inline after send
- Table: Entry Number, Sale Date/Time, Draw Date/Time, Issuer, Game Type, Lines, Numbers, Payment Status, Amount, Status, Amount Won, Channel, Actions
- Click any row → Entry Detail dialog (player name/phone/email, payment ref/method/status, entry mechanics)

*Add Entries tab:*
- **Quick Add** mode: phone + name + qty stepper → instant create + SMS
- **Bulk Import** mode: paste `phone, name, quantity` lines → Preview → Create & Send SMS → results table
- Both hit `POST /api/v1/admin/draws/{id}/tickets/bulk-upload`

**`/entries` — Standalone Quick Entry:**
- Same Quick Add + Bulk Import tabs as draw detail (including confirmation steps)
- In `commerceNav` — always visible, including to `commerce_manager`
- Auto-selects first active draw; dropdown appears if multiple active draws
- Quick Add shows session history (all entries created this browser session)
- Both flows require a confirmation step before entries are created (see Admin Entry Upload Flow)
- Dropdown filters out cancelled/completed/failed draws; labels include draw number: `"${gameName} — Draw #N"`

**`/transactions` — TransactionsModule:**
- Fetches all entries (`payment_status=completed`) and groups them into "transactions" by `payment_ref`
- **Admin entries have no `payment_ref`** (null) — grouped via synthetic key `admin::${phone}::${minuteBucket}` where `minuteBucket = Math.floor(Date.getTime() / 60000)`. This prevents all admin entries collapsing into one transaction.
- `deriveType()` logic (determines transaction label):
  - Checks `game_type === 'DRAW_ENTRY'` OR `serial_number?.startsWith('WB-ENT-')` OR `serial_number?.startsWith('CP-ENT-')` → `"Extra WinBig (N)"`
  - Falls back to access pass count (1-Day Pass / 2-Day Pass) or generic "Purchase"
  - The `WB-ENT-` prefix check is required because admin-uploaded entries may not have `game_type` set
- `GatewayBadge` values: `MTN`, `Telecel`, `AirtelTigo`, **`Admin Upload`** (green badge for admin entries detected by `issuer_type === 'ADMIN'` or null `payment_ref`)
- Never infer MTN/Telecel from phone prefix for admin entries — always use `Admin Upload` label

**`/game/:gameId` → GameDetails Overview tab:**
- Shows 4 stat cards: **Entries Sold**, **Revenue**, **Entry Price**, **Top Prize**
- Entries Sold and Revenue are fetched live by calling `GET /admin/games/{id}/schedule` (returns array of schedules each with `tickets_sold` pre-computed from ticket service). Sum all `tickets_sold` across schedules.
- Do NOT go through draws to compute entry counts — the schedule endpoint already does it correctly.
- Revenue = `sum(tickets_sold) × base_price` (base_price in pesewas → divide by 100 for GHS)
- Shows "Unlimited" when `total_tickets` is 0 or null
- Prizes tab: first-place prize highlighted in gold
- Resets to Overview tab each time dialog opens (via `useEffect` on `isOpen`)

**`/games` — Game List (`GameList.tsx`):**
- Row actions (icon buttons): ⭐ Set Default | 👁 View Details | ✏️ Edit | 🎨 Branding | status-dependent action | 🗑 Delete
- Status-dependent action: `ACTIVE` → Pause (suspend), `SUSPENDED` → PlayCircle (resume), `DRAFT`/`APPROVED` → Play (activate)
- **Resume** calls `gameService.activateGame(id)` — same as activate, shown only for SUSPENDED games
- **Delete** calls `gameService.deleteGame(id)` — destructive confirmation dialog (red "Delete" button); permanent, removes all schedules
- `gameService.deleteGame(id)` → `DELETE /admin/games/{id}`

**`/admin/audit-logs` — Audit Logs:**
- Color-coded action badges: green=create, blue=update, red=delete, purple=login, gray=logout, orange=assign
- Filters: user search, action select, resource select, date range
- Relative timestamps + exact time
- Click row → detail dialog (IP, user agent, request data — password stripped)
- 25 per page, skeleton loading, empty state

**`/settings` — Settings:**
- Profile card: edit first name, last name, email → `PUT /admin/profile`. Save button only active when form is dirty. Form initialized via `useEffect` on profile load (NOT in render body — causes React error #301 infinite loop if done inline).
- Change Password card: current + new password with show/hide toggles, confirm field → `POST /admin/auth/change-password`
- 2FA card: read-only MFA status badge; contact super admin to enable

#### Key service files

**`src/services/draws.ts`** — `drawService`:
- `getDraws()`, `getDraw(id)`, `prepareDraw(id)`, `executeWinnerSelection(id)`, `commitDrawResults(id)`, `processPayout(id)`
- `getDrawTickets(id, filters)`, `getDrawStatistics(id)`, `getWinningTickets(id)`
- Types: `Ticket`, `BetLine`, `DrawStage`

**`src/services/admin.ts`** — `adminService`:
- Users CRUD + activate/deactivate
- Roles CRUD, permissions list, role assignments
- `getAuditLogs(page, pageSize, filters)` — filters: user_id, action, resource, start_date, end_date

**`src/services/players.ts`** — `playerService`:
- `searchPlayers()`, `getPlayer(id)`, `getPlayerWallet(id)`, `suspendPlayer(id)`, `activatePlayer(id)`
- Player status values: `'ACTIVE' | 'SUSPENDED' | 'BANNED'`

**`src/services/games.ts`** — `gameService`:
- Game CRUD, logo upload, prize structure, scheduling
- Game fields: `code`, `name`, `draw_frequency`, `draw_days`, `sales_cutoff_minutes`, `base_price`, `multi_draw_enabled`, `prize_details`

**`src/services/dashboard.ts`** — `dashboardService`:
- `getDailyMetrics()`, `getMonthlyMetrics()`, `getTopPerformingAgents()`
- Metrics: gross revenue, entries sold, payouts, win rates, commissions

**`src/services/winnerSelectionService.ts`** — winner selection:
- `executeGoogleRNGSelection(drawId, totalEntries, maxWinners, excludedPhones?)` — uses random.org API (key in `.env`)
- `executeCryptographicSelection(drawId, totalEntries, maxWinners, excludedPhones?)` — local CSPRNG
- When `excludedPhones` is non-empty: fetches full ticket list, builds eligible positions pool (excluding those phones), picks from [1, eligible_count] via RNG, maps back to original positions. Phone normalization via `_normalizePhone()`.
- Internal helpers: `_getRngNumbers(min, max, count)`, `_cryptoPickInRange(min, max, count)`, `_normalizePhone(phone)`

**`src/lib/bet-utils.ts`:**
- `isPermBet(betType)`, `isBankerBet(betType)` — null-safe (return `false` for undefined/null)
- `getBetLineNumbers()`, `calculateCombinations()`, `formatAmount()` (pesewas → GHS)
- USSD entries have `bet_lines: [{}]` — always handle undefined `bet_type`

---

### 4. `vinne-website/` — Public Player Site (React 18)

- **Stack:** React 18, TypeScript, Vite 5, react-router-dom v6, TanStack Query, Tailwind CSS, Shadcn/ui, framer-motion, canvas-confetti
- **Served from:** `/var/www/html/` on `34.42.87.251`
- **API base URL:** `/api/v1` (Vite proxy → `http://34.121.254.209:4000`)

**Build and deploy:**
```bash
cd vinne-website
npm run build
scp -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no -r dist/. suraj@34.42.87.251:~/winbig-dist/
ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@34.42.87.251 "sudo cp -r ~/winbig-dist/. /var/www/html/"
```

#### Routes

| Path | Component | Notes |
|---|---|---|
| `/` | `Index` | Landing page, hero, live competitions widget |
| `/competitions` | `CompetitionsPage` | Active competitions |
| `/competitions/:id` | `CompetitionDetail` | Single competition detail |
| `/results` | `ResultsPage` | Winners + "Open Live Draw Reveal" button |
| `/draw-reveal` | `DrawReveal` | Fullscreen projector winner reveal |
| `/faq` | `FAQPage` | |
| `/sign-in` | `SignInPage` + `OtpLogin.tsx` | OTP-based phone login (2-step) |
| `/sign-up` | `SignUpPage` | |
| `/my-tickets` | `MyTicketsPage` | Player's purchased entries |
| `/profile` | `ProfilePage` | |

**Draw cutoff enforcement (`CompetitionDetail.tsx`):**
- Schedule picker only selects `status === "SCHEDULED"` schedules — will NOT pick a COMPLETED/CANCELLED one
- `isClosed` = true when: no SCHEDULED schedule found, OR `schedule.status !== "SCHEDULED"`, OR `salesCutoffDate <= now`, OR countdown reaches 0
- `salesCutoffDate` derived from `schedule.scheduled_end` (the actual sales cutoff, typically 2h before draw)
- When closed: "LIVE" badge → "DRAW ENDED" (gray), buy button → "DRAW CLOSED" (disabled), message shown
- `CompetitionsPage` card: "CLOSES IN 00:00:00" → "DRAW ENDED" gray badge once `useCountdown.total === 0`

**OTP login flow:**
1. Enter Ghana phone → `POST /api/v1/players/verify-otp` (sends OTP via mNotify)
2. Enter 6-digit OTP → `POST /api/v1/players/login`
3. 60-second countdown before resend available
4. Phone normalized to `233XXXXXXXXX` format

#### `/draw-reveal` — Fullscreen Winner Reveal Page

Used at live events on projector/TV. Fetches latest winner automatically, or target a specific draw:
- `winbig.bedriften.xyz/draw-reveal` — latest winner
- `winbig.bedriften.xyz/draw-reveal?drawId=59ad83f5-...` — specific draw

Animation sequence:
1. **Ready** — draw info + "Reveal Winner" button
2. **Countdown** — 3…2…1
3. **Rolling** — serial chars spin and lock left-to-right (slot machine)
4. **Reveal** — serial zooms in with gold glow
5. **Celebrating** — "WE HAVE A WINNER!" + confetti + winner name/masked phone

---

## USSD Payment Flow (end to end)

```
User dials *899*92#
  → Hubtel USSD gateway → POST /api/v1/ussd/callback → Flask :5001
  → User selects quantity and confirms
  → create_winbig_tickets(session_id, msisdn, qty)
      INSERT into ticket_service DB (status=issued, payment_status=pending, serial=WB-ENT-*)
  → _fire_momo_async() — 3-sec delay so USSD session can close
      → Hubtel STK push to user's MoMo phone
  → User approves on phone
  → Hubtel POST to https://api.winbig.bedriften.xyz/payment/webhook → Flask :5001
  → payment_webhook()
      UPDATE tickets SET payment_status=completed, paid_at=NOW() WHERE payment_ref=...
      → send_winbig_sms() [background thread]
          mNotify SMS: "CarPark payment confirmed!\nWinBig Entry:\nWB-ENT-XXXXX\nDraw: 03 May 2026\nGood luck!"
      → push_to_payment_service() [background thread]
          INSERT into payment_service DB

Fallback: _payment_reconciliation_worker every 180s
  → polls Hubtel for payments pending > 5 min → marks completed if Hubtel confirms
```

**Nginx routing:**
- `/api/v1/ussd/` → rewritten to `/ussd/` → Flask `:5001`
- `/payment/webhook` → Flask `:5001/payment/webhook` (no rewrite)

---

## Admin Entry Upload Flow

**Quick Add (single):**
1. `/entries` page or draw detail → Add Entries tab → Quick Add
2. Enter phone, name (optional), qty (1–10 stepper)
3. Click **"Review & Create"** → inline confirmation card shows Phone, Name, Entries count, GHS total
4. Click **"Edit"** to go back, or **"Confirm & Create"** to submit → `POST /api/v1/admin/draws/{id}/tickets/bulk-upload`
5. Serial shown on screen, session history appended below form

**Bulk Import (multiple):**
1. Add Entries tab → Bulk Import
2. Paste: `phone, name, quantity` (one per line; name + qty optional)
3. Preview → table with totals
4. Click **"Create & Send SMS"** → confirmation **Dialog** shows Recipients count, total Entries, GHS total
5. Click **"Confirm & Create"** in dialog to submit → `POST /api/v1/admin/draws/{id}/tickets/bulk-upload`
6. Backend: `IssueTicket` gRPC per entry (max 8 concurrent), then `SendBulkSMS`
7. Results table: per-person serials + SMS tick/cross

**Resend SMS:**
1. Draw detail → Entries tab → optionally filter entries
2. "Send SMS" → `POST /api/v1/admin/draws/{id}/tickets/resend-sms`
3. Groups by phone, one SMS per person with all their entries
4. Per-entry status shown inline

---

## nginx Architecture (Production)

**Server 34.121.254.209:**

| Domain | Backend |
|---|---|
| `api.winbig.bedriften.xyz` | Flask :5001 + Go gateway :4000 + MinIO :9000 |
| `admin.winbig.bedriften.xyz` | Static `/var/www/admin/` (React SPA) |

**Server 34.42.87.251:**

| Domain | Backend |
|---|---|
| `winbig.bedriften.xyz` | Static `/var/www/html/` (React SPA) |

Security header snippets:
- `/etc/nginx/snippets/security-headers.conf` — API + public site
- `/etc/nginx/snippets/security-headers-admin.conf` — Admin (includes CSP + `X-Frame-Options: DENY`)

Rate limiting: `ussd_callback` 60r/m, `api_general` 120r/m, `admin_api` 60r/m, `website` 30r/m

**Reload:** `sudo nginx -t && sudo systemctl reload nginx`

---

## Database Layout

All DBs are Docker containers on `34.121.254.209`.

| Service | Container | Port | DB name | User | Notes |
|---|---|---|---|---|---|
| ticket | `service-ticket-db` | 5442 | `ticket_service` | `ticket` | Password `#kettic@333!` in volume |
| player | `service-player-db` | 5444 | `player_service` | `player` | |
| payment | `service-payment-db` | 5440 | `payment_service` | `payment` | |
| wallet | `service-wallet-db` | 5438 | `wallet_service` | `wallet` | Password `wallet123` — not rotated |
| draw | `service-draw-db` | ~5436 | `draw_service` | `draw` | |
| game | `service-game-db` | ~5434 | `game_service` | `game` | |

**Key ticket table columns:**
```sql
id
serial_number      -- WB-ENT-XXXXXXXX
game_code          -- 'IPHONE17'
game_schedule_id   -- '8aaa6e8d-...'
draw_id
draw_number
customer_phone     -- 233XXXXXXXXX
issuer_type        -- 'USSD' | 'ADMIN' | 'POS'
issuer_id          -- MSISDN (USSD) | 'admin-bulk-upload' (admin)
payment_status     -- 'pending' | 'completed' | 'failed'
payment_ref        -- Hubtel payment reference
payment_method     -- 'mobile_money' | 'external' | 'cash'
status             -- 'issued' | 'won' | 'cancelled' | 'expired'
sms_sent           -- BOOLEAN
bet_lines          -- JSONB ([] for USSD entries)
created_at, paid_at, updated_at
```

**Game service DB password:** `game123` (connect: `docker exec -it $(docker ps | grep service-game-db | awk '{print $1}') psql -U game -d game_service`)

**One-time DB fixes applied (2026-05-03):**
```sql
-- Renamed game to "iPhone 17 Pro" (was "iPhone 17 Pro Max")
-- In game_service DB:
UPDATE games SET name = 'iPhone 17 Pro' WHERE code = 'IPHONE17';
-- In ticket_service DB (updates ~256 rows):
UPDATE tickets SET game_name = 'iPhone 17 Pro' WHERE game_code = 'IPHONE17';

-- Restored deleted schedule UUID (was lost when GenerateWeeklySchedule blanket-deleted schedules).
-- New schedule 114b4d22-... was renamed to original 8aaa6e8d-... so all tickets/Draw#2 still match:
-- In game_service DB:
UPDATE game_schedules SET id = '8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34'
  WHERE id = '114b4d22-e4ab-4668-80a8-c4ff7609fc29';

-- Reactivate game after accidental suspension:
UPDATE games SET status = 'ACTIVE', updated_at = NOW() WHERE code = 'IPHONE17';

-- Cancel duplicate Draw #3 (created when schedule was regenerated):
-- In draw_service DB:
UPDATE draws SET status = 'cancelled' WHERE id = 'd4db69f0-...';
```

**Manually recovering a stuck payment:**
```bash
# 1. Find stuck entries
docker exec -it $(docker ps | grep service-ticket-db | awk '{print $1}') \
  psql -U ticket -d ticket_service -c \
  "SELECT payment_ref, customer_phone, payment_status, created_at FROM tickets
   WHERE payment_status='pending' AND created_at > NOW() - INTERVAL '2 hours'
   ORDER BY created_at DESC;"

# 2. Mark completed (only after confirming Hubtel charged the user)
docker exec -it $(docker ps | grep service-ticket-db | awk '{print $1}') \
  psql -U ticket -d ticket_service -c \
  "UPDATE tickets SET payment_status='completed', paid_at=NOW(), updated_at=NOW()
   WHERE payment_ref='<REF>';"

# 3. SMS retry worker picks up within 120s
sudo journalctl -u ussd-app.service -f
```

---

## Common Operational Tasks

```bash
# Check all running containers
sudo docker ps --format "table {{.Names}}\t{{.Status}}"

# USSD app logs
sudo journalctl -u ussd-app.service -f
sudo journalctl -u ussd-app.service -n 200 --no-pager

# Restart USSD app
sudo systemctl restart ussd-app.service

# Rebuild & restart api-gateway after Go changes
cd /home/Suraj/vinne && git pull origin main
cd vinne-microservices
sudo docker-compose build api-gateway
sudo docker stop vinne-microservices_api-gateway_1 && sudo docker rm vinne-microservices_api-gateway_1
sudo docker-compose up -d api-gateway
sudo docker logs vinne-microservices_api-gateway_1 --tail 20

# Deploy admin web
cd vinne-admin-web && npx vite build
scp -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no -r dist/. suraj@34.121.254.209:~/admin-dist/
ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@34.121.254.209 "sudo cp -r ~/admin-dist/. /var/www/admin/ && echo done"

# Deploy public website
cd vinne-website && npm run build
scp -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no -r dist/. suraj@34.42.87.251:~/winbig-dist/
ssh -i "C:\Users\Suraj\.ssh\google_compute_engine" -o StrictHostKeyChecking=no suraj@34.42.87.251 "sudo cp -r ~/winbig-dist/. /var/www/html/"
```

---

## Security Notes

- **UFW:** enabled on both servers — only ports 22, 80, 443 inbound
- **Flask:** bound to `127.0.0.1:5001` only (never `0.0.0.0`)
- **Hubtel webhook IPs:** `18.202.122.131`, `34.240.73.225`, `54.194.245.127` — unknown IPs logged but not blocked
- **mNotify key:** hardcoded in `app.py` line ~71 — should be moved to env var
- **wallet DB password:** `wallet123` — not yet rotated
- **Admin JWT:** stored in `localStorage` as `access_token` (XSS risk) — pending migration to httpOnly cookie
- **Random.org API key:** `5a005036-3424-44c5-afb5-b7cc7a2e244b` — in `.env.localdev` as `VITE_RANDOM_ORG_API_KEY`

---

## Known Issues / Pending Work

| Issue | Status | Notes |
|---|---|---|
| `total_tickets_sold` stale on draw record | ✅ Mitigated | `ListDraws` fetches real counts from ticket service live |
| TKT-* serial prefix | ✅ Fixed | All renamed to WB-ENT-*; `BUSINESS_SERIAL_PREFIX=WB-ENT` in docker-compose |
| Unified SMS format | ✅ Done | Both USSD and bulk upload use "CarPark payment confirmed!" format |
| Logout nav flash | ✅ Fixed | `isCommerceOnly` defaults true when roles empty (token cleared) |
| Settings infinite re-render | ✅ Fixed | Profile form init moved to `useEffect`, was causing React error #301 |
| TransactionsModule admin entries all in one group | ✅ Fixed | Synthetic key `admin::phone::minuteBucket` for null payment_ref entries |
| TransactionsModule admin entries labeled "Ticket Purchase" | ✅ Fixed | `deriveType()` now checks `serial_number?.startsWith('WB-ENT-')` as fallback |
| TransactionsModule gateway showed MTN/Telecel for admin entries | ✅ Fixed | Admin entries (null payment_ref or issuer_type=ADMIN) now show "Admin Upload" badge |
| GameDetails Entries Sold / Revenue showing 0 | ✅ Fixed | Now fetches `GET /admin/games/{id}/schedule` which has live `tickets_sold` per schedule |
| `game_code` / `game_schedule_id` missing from ListDraws response | ✅ Fixed | Added both fields to `transformedDraws` map in `draw_handler.go` |
| Game name inconsistency (Pro Max vs Pro) | ✅ Fixed | Renamed to "iPhone 17 Pro" in game_service DB + 256 ticket records updated via SQL |
| Quick Add / Bulk Import — no confirmation before creating entries | ✅ Fixed | Added inline review card (Quick Add) and Dialog (Bulk Import) confirmation step |
| QuickAddEntry dropdown showed cancelled/duplicate draws | ✅ Fixed | Explicitly filters out cancelled/completed/failed; labels include draw number |
| Website allowed ticket purchase after draw ended | ✅ Fixed | `CompetitionDetail` blocks after `scheduled_end`; shows "DRAW ENDED" badge |
| Website "CLOSES IN 00:00:00" after draw expired | ✅ Fixed | `CompetitionsPage` card shows "DRAW ENDED" once countdown hits 0 |
| Winner exclusions (organizers can't win) | ✅ Done | Per-draw phone exclusion list in DrawDetails overview; excluded from RNG pool |
| Schedule deduplication — duplicate created on regenerate | ✅ Fixed | `game_schedule_service.go` checks per-game before creating; skips if exists |
| Game list missing Resume/Delete actions | ✅ Fixed | Resume (PlayCircle) for suspended; Delete (Trash2) with destructive confirm |
| Schedule UUID `8aaa6e8d-...` lost after regenerate | ✅ Fixed | Renamed new `114b4d22-...` to original UUID via SQL UPDATE in game_service DB |
| Admin JWT in localStorage | ⚠️ Open | Should migrate to httpOnly cookie |
| `wallet123` password not rotated | ⚠️ Open | Low urgency, internal only |
| mNotify key hardcoded in app.py | ⚠️ Open | Move to env var |
| Draw #1 (iPhone 17 Pro) has 0 entries | ℹ️ Expected | Always use Draw #2 `59ad83f5-...` |
| USSD bet_lines empty `[{}]` | ℹ️ By design | Flask writes directly to DB, not via gRPC |

---

## Proto/gRPC Workflow

```bash
# Regenerate Go files (run from vinne-microservices root)
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/ticket/v1/ticket.proto
```

After regenerating, rebuild the affected service Docker image and redeploy.

---

## Key Integrations

| Integration | Purpose | Config location |
|---|---|---|
| **Hubtel** | MoMo STK push payments (MTN, Telecel, AirtelTigo) | `app.py` constants |
| **mNotify** | SMS gateway (OTPs + entry confirmations) | `app.py` + notification service |
| **Random.org** | True RNG for winner selection | `VITE_RANDOM_ORG_API_KEY` in `.env.localdev` |
| **MinIO** | S3-compatible storage for game logos/assets | `docker-compose.yml` |
| **Jaeger** | Distributed tracing | `docker-compose.yml`, port 16686 |
| **Kafka** | Event streaming between microservices | `docker-compose.yml`, port 9092 |
