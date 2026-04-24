#!/usr/bin/env python3
"""
Backfill transactions — CarPark Ed. 7
Reads from ticket_service + player_service, writes to payment_service + wallet_service.

For each distinct payment_ref in tickets:
  → 1 row in payment_service.transactions
For each distinct player_id in player_service:
  → 1 row in wallet_service.player_wallets
"""

import json
import subprocess
import psycopg2
import psycopg2.extras
from datetime import datetime, timezone

TICKET_DB = dict(host="localhost", port=5442, dbname="ticket_service", user="ticket",     password="ticket123")
PLAYER_DB = dict(host="localhost", port=5444, dbname="player_service", user="player",     password="player123")

PAYMENT_CONTAINER = "2f90f486aa08_vinne-microservices_service-payment-db_1"
WALLET_CONTAINER  = "f7b5b203c5bd_vinne-microservices_service-wallet-db_1"
PAYMENT_CREDS     = ("payment", "payment_service")   # user, dbname
WALLET_CREDS      = ("wallet",  "wallet_service")


def log(msg):
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def psql_exec(container, user, dbname, sql):
    """Run a SQL statement inside a Docker container via psql."""
    result = subprocess.run(
        ["sudo", "docker", "exec", container,
         "psql", "-U", user, "-d", dbname, "-c", sql],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip())
    return result.stdout.strip()


def player_phone_fmt(raw):
    """Normalise to +233XXXXXXXXX."""
    p = raw.strip()
    if p.startswith("+"):
        return p
    if p.startswith("233"):
        return "+" + p
    if p.startswith("0"):
        return "+233" + p[1:]
    return "+233" + p


def status_map(payment_status):
    return {"completed": "SUCCESS", "failed": "FAILED", "pending": "PENDING"}.get(payment_status, "PENDING")


# ---------------------------------------------------------------------------
# Step 1: gather data from ticket_service and player_service
# ---------------------------------------------------------------------------

log("Connecting to ticket_service and player_service …")
tconn = psycopg2.connect(**TICKET_DB, connect_timeout=5)
pconn = psycopg2.connect(**PLAYER_DB, connect_timeout=5)
tcur  = tconn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
pcur  = pconn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

# One row per payment_ref — use MAX(total_amount) to avoid picking 0 from DRAW_ENTRYs
tcur.execute("""
    SELECT
        payment_ref,
        MAX(customer_phone)  AS customer_phone,
        MAX(payment_status)  AS payment_status,
        MAX(total_amount)    AS total_amount,
        MIN(created_at)      AS purchased_at,
        MAX(paid_at)         AS paid_at
    FROM tickets
    WHERE payment_ref IS NOT NULL
      AND customer_phone IS NOT NULL
    GROUP BY payment_ref
    HAVING MAX(total_amount) > 0
    ORDER BY MIN(created_at)
""")
purchases = tcur.fetchall()
log(f"Found {len(purchases)} payment_ref(s) in ticket_service")

# Build phone → player_id map
pcur.execute("SELECT id, phone_number FROM players")
phone_to_player = {r["phone_number"]: str(r["id"]) for r in pcur.fetchall()}
log(f"Found {len(phone_to_player)} player(s) in player_service")

# All players → need wallets
pcur.execute("SELECT id FROM players")
all_player_ids = [str(r["id"]) for r in pcur.fetchall()]

tcur.close(); tconn.close()
pcur.close(); pconn.close()

# ---------------------------------------------------------------------------
# Step 2: insert into payment_service.transactions
# ---------------------------------------------------------------------------

log("Backfilling payment_service.transactions …")
tx_ok = tx_skip = tx_err = 0

for row in purchases:
    ref    = row["payment_ref"]
    phone  = player_phone_fmt(row["customer_phone"])
    status = status_map(row["payment_status"])
    amount = row["total_amount"]
    purchased_at = row["purchased_at"].isoformat() if row["purchased_at"] else "NOW()"
    paid_at      = f"'{row['paid_at'].isoformat()}'" if row["paid_at"] else "NULL"

    player_id = phone_to_player.get(phone)
    if not player_id:
        log(f"  SKIP ref={ref[:8]} — no player for {phone}")
        tx_skip += 1
        continue

    narration = f"CarPark ticket purchase from {phone} via Hubtel MoMo"
    metadata  = json.dumps({"source": "ussd", "user_role": "player", "game": "IPHONE17"}).replace("'", "''")

    sql = f"""
        INSERT INTO transactions (
            reference, type, status, amount, currency, narration,
            provider_name, source_type, source_identifier, source_name,
            destination_type, destination_identifier, destination_name,
            user_id, metadata,
            requested_at, completed_at, created_at, updated_at
        ) VALUES (
            '{ref}', 'DEPOSIT', '{status}', {amount}, 'GHS', '{narration}',
            'HUBTEL', 'mobile_money', '{phone}', '{phone}',
            'stake_wallet', '{player_id}', 'CarPark Player',
            '{player_id}', '{metadata}'::jsonb,
            '{purchased_at}', {paid_at}, '{purchased_at}', NOW()
        )
        ON CONFLICT (reference) DO NOTHING;
    """.strip()

    try:
        out = psql_exec(PAYMENT_CONTAINER, *PAYMENT_CREDS, sql)
        if "INSERT 0 0" in out:
            tx_skip += 1
        else:
            tx_ok += 1
            log(f"  OK  ref={ref[:8]} status={status} amount={amount} player={player_id[:8]}")
    except RuntimeError as e:
        log(f"  ERR ref={ref[:8]}: {e}")
        tx_err += 1

log(f"Transactions: inserted={tx_ok} skipped={tx_skip} errors={tx_err}")

# ---------------------------------------------------------------------------
# Step 3: insert player_wallets for each player
# ---------------------------------------------------------------------------

log("Backfilling wallet_service.player_wallets …")
wl_ok = wl_skip = wl_err = 0

for player_id in all_player_ids:
    sql = f"""
        INSERT INTO player_wallets (player_id, balance, pending_balance, available_balance, currency, status)
        VALUES ('{player_id}', 0, 0, 0, 'GHS', 'ACTIVE')
        ON CONFLICT (player_id) DO NOTHING;
    """.strip()
    try:
        out = psql_exec(WALLET_CONTAINER, *WALLET_CREDS, sql)
        if "INSERT 0 0" in out:
            wl_skip += 1
        else:
            wl_ok += 1
            log(f"  OK  wallet for player={player_id[:8]}")
    except RuntimeError as e:
        log(f"  ERR player={player_id[:8]}: {e}")
        wl_err += 1

log(f"Wallets: inserted={wl_ok} skipped={wl_skip} errors={wl_err}")

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
log("=" * 55)
log(f"Done. transactions={tx_ok} wallets={wl_ok} errors={tx_err + wl_err}")
