# CLAUDE.md — Vinne / WinBig Africa

This file gives Claude Code full context on the project so every session starts informed.

---

## What This Project Is

**WinBig Africa** is a lottery and competition platform for the African market. Players buy entries via USSD (`*899*92#`) or the web, enter draws to win prizes (current: iPhone 17 Pro Max), and receive SMS confirmation. Administrators manage games, draws, players, agents, and financials via an internal portal.

**Production domain:** `winbig.bedriften.xyz`  
**Admin portal:** `admin.winbig.bedriften.xyz`  
**API:** `api.winbig.bedriften.xyz`  
**Git remote:** `https://github.com/bedriftenconsulting/vinne.git`  
**Production server:** `suraj@34.121.254.209`  
**SSH key:** `/Users/jeffery/Documents/Documents/Bedriften/Tech/saf/winbig_key`

---

## Monorepo Structure

```
vinne/
├── vinne-microservices/     # Go 1.25 gRPC backend — 12 services
├── vinne-admin-web/         # React 19 admin dashboard (TypeScript, Vite 7)
├── vinne-website/           # React 18 public player site (TypeScript, Vite)
├── vinne-ussd-app/          # Python 3 Flask USSD + payment backend (port 5001)
│   └── app.py               # Single-file app — the entire USSD/payment logic
└── docs/                    # Implementation notes and feature docs
```

---

## Component Details

### 1. `vinne-ussd-app/` — Flask USSD App (Python)

- **Runtime:** Python 3, Flask, port `127.0.0.1:5001`
- **Systemd service:** `ussd-app.service`
- **Entry point:** `vinne-ussd-app/app.py` — single file, all logic here
- **Current game:** CarPark Ed. 7 — WinBig (iPhone 17 Pro Max), draw `2026-05-03`
- **USSD code:** `*899*92#` (Hubtel USSD gateway)
- **Payment:** Hubtel MoMo STK push → webhook at `/payment/webhook`
- **SMS:** mNotify API — sender ID `CARPARK`, key in `mnotify_api_key_winbig_sms_key.txt`
- **USSD provider limit:** Max **130 characters** per menu screen (all networks). MTN is lenient; Telecel/AirtelTigo enforce strictly.

**Key constants in app.py:**
| Constant | Value | Notes |
|---|---|---|
| `WINBIG_UNIT_PRICE` | 2000 (pesewas) | GHS 20 per draw entry |
| `MAX_ENTRIES_PER_TXN` | 10 | Server-side cap |
| `TICKET_EXPIRY_MINUTES` | 30 | Pending tickets expire after 30 min |
| `USSD_RATE_LIMIT` | 30 req/60s | Per-MSISDN rate limit |
| `HUBTEL_WEBHOOK_IPS` | 3 known IPs | Logs unknown IPs but still processes |

**Background workers (daemon threads, auto-start on service start):**
- `_sms_retry_worker` — every 120s, retries `sms_sent=FALSE` completed tickets
- `_payment_reconciliation_worker` — every 180s, polls Hubtel for stuck `pending` payments
- `_ticket_expiry_worker` — every 300s, expires pending tickets older than 30 min

**DB connections (all via Docker on localhost):**
| DB | Port | User | Password |
|---|---|---|---|
| ticket_service | 5442 | ticket | `#kettic@333!` |
| player_service | 5444 | player | `#yerpla@333!` |
| payment_service | 5440 | payment | `#mentpay@333!` |
| wallet_service | 5438 | wallet | `wallet123` ← not yet rotated |

**Deploy after changes:**
```bash
# From local
git push origin main
# On server
ssh -i /Users/jeffery/Documents/Documents/Bedriften/Tech/saf/winbig_key suraj@34.121.254.209
cd /home/Suraj/vinne && git pull origin main && sudo systemctl restart ussd-app.service
```

**Check logs:**
```bash
sudo journalctl -u ussd-app.service -f
sudo journalctl -u ussd-app.service -n 200 --no-pager
```

---

### 2. `vinne-microservices/` — Go Microservices

- **Language:** Go 1.25, gRPC + Protocol Buffers
- **Module:** `github.com/randco/randco-microservices`
- **12 services:** api-gateway, admin-management, agent-auth, agent-management, draw, game, notification, payment, player, terminal, ticket, wallet
- **API Gateway:** HTTP/REST on port `4000`, proxied by nginx at `api.winbig.bedriften.xyz /` 
- **Infrastructure:** PostgreSQL (one per service, ports 5434–5442), Redis (6379–6389), Kafka (9092), MinIO (9000), Jaeger (16686)

**Start local dev:**
```bash
cd vinne-microservices
docker compose up -d          # starts all DBs, Redis, Kafka, MinIO, Jaeger
./scripts/start-services.sh   # starts all 12 Go services
```

---

### 3. `vinne-admin-web/` — Admin Portal (React 19)

- **Stack:** React 19, TypeScript, Vite 7, TanStack Router, TanStack Query, Zustand, Tailwind CSS, Shadcn/ui
- **Served from:** `/var/www/admin/` on the server
- **nginx vhost:** `admin.winbig.bedriften.xyz`
- **API base:** `https://api.winbig.bedriften.xyz/api/v1`

**Build and deploy:**
```bash
cd vinne-admin-web
npm run build           # outputs to dist/
# On server:
sudo cp -r dist/. /var/www/admin/
```

**Key pages:** Dashboard, Games, Draws, Players, Agents, Tickets, Transactions, USSD Sessions, Wallets, Wins, Audit Logs, Admin Users/Roles/Permissions

---

### 4. `vinne-website/` — Public Player Site (React 18)

- **Stack:** React 18, TypeScript, Vite, TanStack Router/Query, Tailwind CSS, Shadcn/ui
- **Served from:** `/var/www/winbig/` on the server
- **nginx vhost:** `winbig.bedriften.xyz`

**Build and deploy:**
```bash
cd vinne-website
npm run build           # outputs to dist/
# On server:
sudo cp -r dist/. /var/www/winbig/
```

---

## nginx Architecture (Production Server)

Three vhosts under `/etc/nginx/sites-enabled/`:

| File | Domain | Backend |
|---|---|---|
| `api.winbig.bedriften.xyz` | `api.winbig.bedriften.xyz` | Flask :5001, Go gateway :4000, MinIO :9000 |
| `admin.winbig.bedriften.xyz` | `admin.winbig.bedriften.xyz` | Static `/var/www/admin/` |
| `winbig.bedriften.xyz` | `winbig.bedriften.xyz` | Static `/var/www/winbig/` |

**Security headers** are in snippets:
- `/etc/nginx/snippets/security-headers.conf` — API + public site
- `/etc/nginx/snippets/security-headers-admin.conf` — Admin (includes CSP + `X-Frame-Options: DENY`)

**Rate limiting zones:**
- `ussd_callback` — 60r/m
- `api_general` — 120r/m  
- `admin_api` — 60r/m
- `admin_general` — 60r/m
- `website` — 30r/m

**Key nginx route note:** `/api/v1/ussd/` rewrites to `/ussd/` before proxying to Flask. `/payment/webhook` proxies directly to Flask at `:5001/payment/webhook`.

---

## Payment Flow (USSD)

```
User dials *899*92#
  → Hubtel USSD gateway → POST /api/v1/ussd/callback (nginx)
  → Flask /ussd/callback
  → User confirms purchase → create_winbig_tickets() → tickets inserted (status=pending)
  → _fire_momo_async() → Hubtel STK push to user's phone
  → User approves MoMo
  → Hubtel POSTs to https://api.winbig.bedriften.xyz/payment/webhook
  → Flask payment_webhook() → update tickets to completed
  → send_winbig_sms() in background thread → mNotify SMS to user
  → push_to_payment_service() in background thread → insert transaction record
```

**Fallback:** `_payment_reconciliation_worker` polls Hubtel every 3 min for any payments stuck in `pending` > 5 min.

---

## Manually Recovering a Stuck Payment

```bash
# 1. Find stuck tickets
docker exec -it $(docker ps | grep ticket | awk '{print $1}') psql -U ticket -d ticket_service -c \
"SELECT payment_ref, customer_phone, payment_status, created_at FROM tickets WHERE payment_status='pending' AND created_at > NOW() - INTERVAL '2 hours' ORDER BY created_at DESC;"

# 2. Mark completed (only after verifying user was actually charged)
docker exec -it $(docker ps | grep ticket | awk '{print $1}') psql -U ticket -d ticket_service -c \
"UPDATE tickets SET payment_status='completed', paid_at=NOW(), updated_at=NOW() WHERE payment_ref='<REF>';"

# 3. SMS retry worker picks it up within 120s — watch with:
sudo journalctl -u ussd-app.service -f
```

---

## Security Notes

- **UFW:** enabled — only ports 22, 80, 443 inbound
- **Flask:** bound to `127.0.0.1` only (not `0.0.0.0`)
- **Hubtel webhook IPs:** `18.202.122.131`, `34.240.73.225`, `54.194.245.127` — unknown IPs are logged but not blocked (to prevent silent failures if Hubtel rotates IPs)
- **mNotify key:** hardcoded in `app.py` line ~71 — should be moved to env var
- **wallet DB password:** `wallet123` — not yet rotated
- **Admin JWT:** stored in localStorage (XSS risk) — migration to httpOnly cookies is a pending improvement

---

## Active Game Config (as of 2026-04-28)

| Field | Value |
|---|---|
| Game code | `IPHONE17` |
| Game name | iPhone 17 Pro Max |
| Game schedule ID | `8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34` |
| Draw number | 1 |
| Draw date | 2026-05-03 |
| Entry price | GHS 20 (2000 pesewas) |
| USSD code | `*899*92#` |

To update the active game, change the constants at the top of `vinne-ussd-app/app.py` and update `vinne-ussd-app/config.json`.

---

## Common Tasks

**Restart USSD app on server:**
```bash
sudo systemctl restart ussd-app.service
```

**Reload nginx after config change:**
```bash
sudo nginx -t && sudo systemctl reload nginx
```

**Check all service health:**
```bash
sudo systemctl status ussd-app.service
docker ps --format "table {{.Names}}\t{{.Status}}"
```

**SSH to server:**
```bash
ssh -i /Users/jeffery/Documents/Documents/Bedriften/Tech/saf/winbig_key suraj@34.121.254.209
```
