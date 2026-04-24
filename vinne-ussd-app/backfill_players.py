#!/usr/bin/env python3
"""
Backfill script — CarPark Ed. 7
Runs once to:
  1. Register every existing USSD ticket buyer as a player in player_service
  2. Log one USSD session record per payment_ref in player_service.ussd_sessions
  3. Fix game_schedule_id on all existing tickets to the correct value
"""

import psycopg2
import psycopg2.extras
from datetime import datetime

TICKET_DB = {
    "host":     "localhost",
    "port":     5442,
    "dbname":   "ticket_service",
    "user":     "ticket",
    "password": "ticket123",
}

PLAYER_DB = {
    "host":     "localhost",
    "port":     5444,
    "dbname":   "player_service",
    "user":     "player",
    "password": "player123",
}

CORRECT_SCHEDULE_ID = "8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34"
WRONG_SCHEDULE_ID   = "aa5d0c8b-d7b8-4148-9955-e8453b88198f"


def log(msg):
    ts = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def player_phone(msisdn):
    """Normalise to +233XXXXXXXXX format."""
    phone = msisdn.strip()
    if phone.startswith("+"):
        return phone
    if phone.startswith("233"):
        return "+" + phone
    if phone.startswith("0"):
        return "+233" + phone[1:]
    return "+233" + phone


def run():
    ticket_conn  = psycopg2.connect(**TICKET_DB, connect_timeout=5)
    player_conn  = psycopg2.connect(**PLAYER_DB, connect_timeout=5)
    ticket_cur   = ticket_conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    player_cur   = player_conn.cursor()

    # ------------------------------------------------------------------
    # 1. Fix wrong game_schedule_id on existing tickets
    # ------------------------------------------------------------------
    ticket_cur.execute(
        "SELECT COUNT(*) FROM tickets WHERE game_schedule_id = %s OR game_schedule_id IS NULL",
        (WRONG_SCHEDULE_ID,)
    )
    bad_count = ticket_cur.fetchone()["count"]
    log(f"Tickets with wrong/missing schedule_id: {bad_count}")

    if bad_count > 0:
        upd = ticket_conn.cursor()
        upd.execute(
            "UPDATE tickets SET game_schedule_id=%s, updated_at=NOW() WHERE game_schedule_id=%s OR game_schedule_id IS NULL",
            (CORRECT_SCHEDULE_ID, WRONG_SCHEDULE_ID)
        )
        ticket_conn.commit()
        upd.close()
        log(f"Fixed schedule_id on {bad_count} ticket(s)")

    # ------------------------------------------------------------------
    # 2. Get all distinct buyers (one row per payment_ref with phone)
    # ------------------------------------------------------------------
    ticket_cur.execute("""
        SELECT DISTINCT ON (payment_ref)
            payment_ref, customer_phone, MIN(created_at) as purchased_at
        FROM tickets
        WHERE customer_phone IS NOT NULL
          AND payment_ref IS NOT NULL
        GROUP BY payment_ref, customer_phone
        ORDER BY payment_ref, purchased_at
    """)
    purchases = ticket_cur.fetchall()
    log(f"Found {len(purchases)} distinct payment_ref(s) to process")

    created_players  = 0
    existing_players = 0
    sessions_logged  = 0
    errors           = 0

    for row in purchases:
        raw_phone  = row["customer_phone"]
        ref        = row["payment_ref"]
        phone      = player_phone(raw_phone)

        try:
            # Upsert player
            player_cur.execute("""
                INSERT INTO players (
                    id, phone_number, password_hash, status, registration_channel,
                    terms_accepted, marketing_consent, created_at, updated_at
                ) VALUES (
                    gen_random_uuid(), %s, 'USSD_NO_PASSWORD', 'ACTIVE', 'USSD',
                    true, false, NOW(), NOW()
                )
                ON CONFLICT (phone_number) DO UPDATE SET updated_at=NOW()
                RETURNING id, (xmax = 0) AS is_new
            """, (phone,))
            result    = player_cur.fetchone()
            player_id = result[0]
            is_new    = result[1]

            if is_new:
                created_players += 1
                log(f"  [NEW PLAYER] phone={phone} id={player_id}")
            else:
                existing_players += 1

            # Log a USSD session for this purchase
            player_cur.execute("""
                INSERT INTO ussd_sessions (
                    id, msisdn, sequence_id, player_id,
                    session_state, current_menu, user_input,
                    started_at, last_activity, completed_at, created_at
                ) VALUES (
                    gen_random_uuid(), %s, %s, %s,
                    'COMPLETED', 'PURCHASE_CONFIRMED', %s,
                    NOW(), NOW(), NOW(), NOW()
                )
            """, (phone, f"backfill-{ref[:8]}", player_id, ref))
            sessions_logged += 1

            player_conn.commit()

        except Exception as e:
            player_conn.rollback()
            log(f"  [ERROR] phone={raw_phone} ref={ref} — {e}")
            errors += 1

    ticket_cur.close()
    ticket_conn.close()
    player_cur.close()
    player_conn.close()

    log("=" * 50)
    log(f"Done. new_players={created_players} existing={existing_players} sessions={sessions_logged} errors={errors}")


if __name__ == "__main__":
    log("Backfill starting")
    run()
    log("Backfill complete")
