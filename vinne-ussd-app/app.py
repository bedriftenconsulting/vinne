import base64
import hashlib
import json
import os
import random
import string
import threading
import time
import uuid
import requests
import psycopg2
import psycopg2.extras
from datetime import datetime
from flask import Flask, request, jsonify

app = Flask(__name__)

USSD_CODE      = "*899*92"
TEST_MODE      = False
BYPASS_PAYMENT = False
CHARGE_GHS_1   = False

# ---------------------------------------------------------------------------
# Hubtel credentials — set via environment variables on the server
# ---------------------------------------------------------------------------
HUBTEL_CLIENT_ID     = os.environ.get("HUBTEL_CLIENT_ID", "")
HUBTEL_CLIENT_SECRET = os.environ.get("HUBTEL_CLIENT_SECRET", "")
HUBTEL_POS_SALES_ID  = os.environ.get("HUBTEL_POS_SALES_ID", "")
HUBTEL_CALLBACK_URL  = "https://api.winbig.bedriften.xyz/payment/webhook"

NETWORK_CHANNEL = {
    "mtn":        "mtn-gh",
    "vodafone":   "vodafone-gh",
    "airteltigo": "tigo-gh",
    "airtel":     "airtel-gh",
    "tigo":       "tigo-gh",
}

PRICES = {
    "1": {
        "label":              "1-Day Pass",
        "amount":             10000,          # GHS 100 — total charged to customer
        "days":               [("Day 1", "02/05/2026")],
        "access_unit_price":  8000,           # GHS 80 net per ACCESS_PASS (sent to admin)
        "entry_unit_price":   2000,           # GHS 20 per included DRAW_ENTRY
    },
    "2": {
        "label":              "2-Day Pass",
        "amount":             18000,          # GHS 180 — total charged to customer
        "days":               [("Day 1", "02/05/2026"), ("Day 2", "03/05/2026")],
        "access_unit_price":  8000,           # GHS 80 net per ACCESS_PASS per day (sent to admin)
        "entry_unit_price":   1000,           # GHS 10 per included DRAW_ENTRY (2 × 10 = GHS 20)
    },
}
WINBIG_UNIT_PRICE = 2000  # GHS 20 per extra draw entry in pesewas

MNOTIFY_SMS_KEY = "F9XhjQbbJnqKt2fy9lhPIQCSD"
SMS_SENDER_ID   = "CARPARK"

GAME_CODE        = "IPHONE17"
GAME_NAME        = "iPhone 17 Pro Max"
GAME_SCHEDULE_ID = "8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34"
DRAW_NUMBER      = 1
DRAW_DATE        = "2026-05-03"

TICKET_DB = {
    "host":     os.environ.get("TICKET_DB_HOST", "localhost"),
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

PAYMENT_DB = {
    "host":     "localhost",
    "port":     5440,
    "dbname":   "payment_service",
    "user":     "payment",
    "password": "payment123",
}

WALLET_DB = {
    "host":     "localhost",
    "port":     5438,
    "dbname":   "wallet_service",
    "user":     "wallet",
    "password": "wallet123",
}

# In-memory session state: sequenceID -> list of user inputs
sessions = {}

# Idempotency guard: session_id -> payment_ref
# Prevents duplicate ticket creation if the USSD gateway retries the same step
session_purchases = {}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def ussd_response(msisdn, sequence_id, message, end=False):
    return jsonify({
        "msisdn":       msisdn,
        "sequenceID":   sequence_id,
        "message":      message,
        "timestamp":    datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "continueFlag": 1 if end else 0
    })


def normalise_phone(phone):
    phone = phone.strip()
    if phone.startswith("0"):
        return "233" + phone[1:]
    if not phone.startswith("233"):
        return "233" + phone
    return phone


def generate_access_serial():
    """CP-ACC-XXXXXXXX — Car Park access pass."""
    return "CP-ACC-" + "".join(random.choices(string.ascii_uppercase + string.digits, k=8))


def generate_entry_serial():
    """WB-ENT-XXXXXXXX — Draw entry."""
    return "WB-ENT-" + "".join(random.choices(string.ascii_uppercase + string.digits, k=8))


def make_security_hash(serial_number, payment_ref, customer_phone):
    """SHA256(serial_number + payment_ref + customer_phone)"""
    raw = f"{serial_number}{payment_ref}{customer_phone}"
    return hashlib.sha256(raw.encode()).hexdigest()


def _make_reference():
    """Generate a unique, Hubtel-safe reference (32 hex chars, <= 36)."""
    return uuid.uuid4().hex


# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

def get_ticket_conn():
    return psycopg2.connect(**TICKET_DB, connect_timeout=2)


def get_player_conn():
    return psycopg2.connect(**PLAYER_DB, connect_timeout=2)


def player_phone(msisdn):
    """Return phone in +233XXXXXXXXX format for player_service."""
    return "+" + normalise_phone(msisdn)


def get_or_create_player(msisdn):
    """Upsert a player record in player_service. Returns the player UUID or None on error."""
    phone = player_phone(msisdn)
    try:
        conn = get_player_conn()
        cur  = conn.cursor()
        cur.execute("""
            INSERT INTO players (
                id, phone_number, password_hash, status, registration_channel,
                terms_accepted, marketing_consent, created_at, updated_at
            ) VALUES (
                gen_random_uuid(), %s, 'USSD_NO_PASSWORD', 'ACTIVE', 'USSD',
                true, false, NOW(), NOW()
            )
            ON CONFLICT (phone_number) DO UPDATE SET updated_at=NOW()
            RETURNING id
        """, (phone,))
        player_id = str(cur.fetchone()[0])
        conn.commit()
        cur.close(); conn.close()
        print(f"[PLAYER] upserted phone={phone} id={player_id}")
        return player_id
    except Exception as e:
        print(f"[PLAYER ERROR] get_or_create_player: {e}")
        return None


def log_ussd_session(sequence_id, msisdn, player_id, payment_ref):
    """Insert a completed USSD purchase session into player_service.ussd_sessions."""
    if not player_id:
        return
    phone = player_phone(msisdn)
    try:
        conn = get_player_conn()
        cur  = conn.cursor()
        cur.execute("""
            INSERT INTO ussd_sessions (
                id, msisdn, sequence_id, player_id,
                session_state, current_menu, user_input,
                started_at, last_activity, completed_at, created_at
            ) VALUES (
                gen_random_uuid(), %s, %s, %s,
                'COMPLETED', 'PURCHASE_CONFIRMED', %s,
                NOW(), NOW(), NOW(), NOW()
            )
        """, (phone, sequence_id, player_id, payment_ref))
        conn.commit()
        cur.close(); conn.close()
        print(f"[SESSION] logged seq={sequence_id} player={player_id} ref={payment_ref}")
    except Exception as e:
        print(f"[SESSION ERROR] log_ussd_session: {e}")


def create_tickets_from_ussd(session_data):
    """
    Phase 1: Creates tickets immediately on USSD confirmation.
    All tickets are inserted with payment_status='pending'.

    session_data keys:
      session_id    : str   — USSD sequenceID (used for idempotency)
      msisdn        : str   — raw MSISDN from USSD gateway
      purchase_type : str   — "day_pass" | "extra_entries"
      ticket_key    : str   — "1" | "2"  (day_pass only)
      qty           : int   — number of extra entries (extra_entries only)

    Returns:
      {"reference": str, "access": [serials], "entries": [serials]}

    Idempotency:
      If session_id already maps to a payment_ref (e.g. gateway retry),
      the existing tickets are fetched from DB and returned — no new rows created.
    """
    session_id = session_data["session_id"]
    msisdn     = session_data["msisdn"]
    phone      = normalise_phone(msisdn)

    # ------------------------------------------------------------------
    # Idempotency check — return existing tickets if session already used
    # ------------------------------------------------------------------
    if session_id in session_purchases:
        existing_ref = session_purchases[session_id]
        print(f"[IDEMPOTENCY] session={session_id} already has ref={existing_ref}, returning existing")
        try:
            conn = get_ticket_conn()
            cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            cur.execute(
                "SELECT serial_number, game_type FROM tickets WHERE payment_ref=%s ORDER BY game_type, serial_number",
                (existing_ref,)
            )
            rows = cur.fetchall()
            cur.close(); conn.close()
            return {
                "reference": existing_ref,
                "access":    [r["serial_number"] for r in rows if r["game_type"] == "ACCESS_PASS"],
                "entries":   [r["serial_number"] for r in rows if r["game_type"] == "DRAW_ENTRY"],
            }
        except Exception as e:
            print(f"[IDEMPOTENCY ERROR] {e}")
            # Fall through to create fresh (safety net)

    reference = _make_reference()
    rows_to_insert  = []   # list of tuples for executemany
    access_serials  = []
    entry_serials   = []

    # ------------------------------------------------------------------
    # Build ticket rows
    # ------------------------------------------------------------------
    if session_data["purchase_type"] == "day_pass":
        ticket     = PRICES[session_data["ticket_key"]]
        num_days   = len(ticket["days"])

        for day_label, day_date in ticket["days"]:
            # --- ACCESS_PASS (one per day) — net price sent to admin ---
            acc_serial = generate_access_serial()
            access_serials.append(acc_serial)
            acc_hash = make_security_hash(acc_serial, reference, phone)
            acc_bet_lines = json.dumps([{"type": "ACCESS_PASS", "days": day_label}])
            rows_to_insert.append((
                acc_serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, f"{ticket['label']} — {day_label} ({day_date})", "ACCESS_PASS",
                acc_bet_lines, 1,
                ticket["access_unit_price"], ticket["access_unit_price"],
                "USSD", msisdn,
                phone,
                "mobile_money", reference, "pending",
                acc_hash, "issued", DRAW_DATE,
            ))

            # --- DRAW_ENTRY (one per day, paired with access pass) ---
            ent_serial = generate_entry_serial()
            entry_serials.append(ent_serial)
            ent_hash = make_security_hash(ent_serial, reference, phone)

            if num_days == 1:
                ent_bet_lines = json.dumps([{"type": "DRAW_ENTRY", "source": "1-Day Pass"}])
            else:
                ent_bet_lines = json.dumps([{"type": "DRAW_ENTRY", "source": "2-Day Pass", "day": day_label}])

            rows_to_insert.append((
                ent_serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, GAME_NAME, "DRAW_ENTRY",
                ent_bet_lines, 1,
                ticket["entry_unit_price"], ticket["entry_unit_price"],
                "USSD", msisdn,
                phone,
                "mobile_money", reference, "pending",
                ent_hash, "issued", DRAW_DATE,
            ))

    elif session_data["purchase_type"] == "extra_entries":
        qty       = session_data["qty"]
        total_amt = qty * WINBIG_UNIT_PRICE

        for _ in range(qty):
            ent_serial = generate_entry_serial()
            entry_serials.append(ent_serial)
            ent_hash = make_security_hash(ent_serial, reference, phone)
            ent_bet_lines = json.dumps([{"type": "DRAW_ENTRY", "source": "Extra WinBig"}])
            rows_to_insert.append((
                ent_serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, GAME_NAME, "DRAW_ENTRY",
                ent_bet_lines, 1,
                WINBIG_UNIT_PRICE, total_amt,
                "USSD", msisdn,
                phone,
                "mobile_money", reference, "pending",
                ent_hash, "issued", DRAW_DATE,
            ))

    # ------------------------------------------------------------------
    # Register / fetch player before inserting tickets
    # ------------------------------------------------------------------
    player_id = get_or_create_player(msisdn)

    # ------------------------------------------------------------------
    # Bulk insert — single round-trip for all tickets
    # ------------------------------------------------------------------
    conn = get_ticket_conn()
    cur  = conn.cursor()
    cur.executemany("""
        INSERT INTO tickets (
            serial_number, game_code, game_schedule_id,
            draw_number, game_name, game_type,
            bet_lines, number_of_lines,
            unit_price, total_amount,
            issuer_type, issuer_id,
            customer_phone,
            payment_method, payment_ref, payment_status,
            security_hash, status, draw_date,
            created_at, updated_at
        ) VALUES (
            %s, %s, %s,
            %s, %s, %s,
            %s::jsonb, %s,
            %s, %s,
            %s, %s,
            %s,
            %s, %s, %s,
            %s, %s, %s,
            NOW(), NOW()
        )
    """, rows_to_insert)
    conn.commit()
    cur.close()
    conn.close()

    # Lock session to prevent duplicate creation on gateway retry
    session_purchases[session_id] = reference
    print(f"[TICKETS] {len(rows_to_insert)} ticket(s) created ref={reference} access={access_serials} entries={entry_serials}")

    # Log the USSD purchase session in player_service (non-blocking — failure won't break flow)
    log_ussd_session(session_id, msisdn, player_id, reference)

    return {"reference": reference, "access": access_serials, "entries": entry_serials}


def handle_payment_webhook(payload):
    """
    Phase 2: Webhook handler — updates existing tickets only, never creates new ones.

    ResponseCode:
      "0000" → completed (payment success)
      "2001" → failed    (user rejected / timeout)
      other  → pending/ignored
    """
    data          = payload.get("Data", {})
    response_code = payload.get("ResponseCode", "")
    reference     = data.get("ClientReference", "")
    amount        = data.get("Amount", 0)
    phone         = data.get("CustomerMsisdn", "")

    if not reference:
        print("[WEBHOOK] missing ClientReference, ignoring")
        return

    if response_code == "0000":
        momo_tx_id = data.get("TransactionId", "") or data.get("OrderId", "")
        print(f"[PAYMENT SUCCESS] ref={reference} amount=GHS{amount} phone={phone} momo_tx={momo_tx_id}")
        _update_payment_status(reference, "completed", momo_tx_id=momo_tx_id)
        send_confirmation_sms(reference)
        threading.Thread(target=push_to_payment_service, args=(reference,), daemon=True).start()

    elif response_code == "2001":
        print(f"[PAYMENT FAILED] ref={reference} code={response_code}")
        _update_payment_status(reference, "failed")

    else:
        print(f"[PAYMENT PENDING/OTHER] ref={reference} code={response_code}")


def _update_payment_status(reference, status, momo_tx_id=None):
    """Update payment_status (and paid_at / payment_reference for completed) on all tickets."""
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor()
        if status == "completed":
            cur.execute(
                """UPDATE tickets
                   SET payment_status=%s, paid_at=NOW(), updated_at=NOW(),
                       payment_reference=%s
                   WHERE payment_ref=%s""",
                (status, momo_tx_id or "", reference)
            )
        else:
            cur.execute(
                "UPDATE tickets SET payment_status=%s, updated_at=NOW() WHERE payment_ref=%s",
                (status, reference)
            )
        conn.commit()
        cur.close()
        conn.close()
        print(f"[DB] payment_status={status} momo_tx={momo_tx_id} for ref={reference}")
    except Exception as e:
        print(f"[DB ERROR] _update_payment_status: {e}")


def _get_payment_db_conn():
    return psycopg2.connect(**PAYMENT_DB, connect_timeout=5)


def _get_wallet_db_conn():
    return psycopg2.connect(**WALLET_DB, connect_timeout=5)


def push_to_payment_service(reference):
    """
    After a successful Hubtel webhook, insert ONE transaction row per payment_ref
    into payment_service.transactions and ensure player wallet exists.

    Amount = SUM(unit_price) across all tickets for the payment_ref:
      1-day pass  → 8000 + 2000           = 10000 (GHS 100)
      2-day pass  → 16000 + 2000          = 18000 (GHS 180)
      5 extra WB  → 5 × 2000             = 10000 (GHS 100)
    """
    try:
        # ── 1. Fetch all tickets for this payment_ref ──────────────────────
        tconn = get_ticket_conn()
        tcur  = tconn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        tcur.execute("""
            SELECT serial_number, game_type, unit_price, customer_phone,
                   payment_reference, paid_at, created_at
            FROM tickets
            WHERE payment_ref = %s
            ORDER BY game_type DESC
        """, (reference,))
        tickets = tcur.fetchall()
        tcur.close(); tconn.close()

        if not tickets:
            print(f"[PAYMENT SVC] no tickets found for ref={reference[:8]}, skipping")
            return

        customer_phone = tickets[0]["customer_phone"]
        phone_fmt      = player_phone(customer_phone)   # +233XXXXXXXXX
        momo_tx_id     = tickets[0]["payment_reference"] or ""
        completed      = tickets[0]["paid_at"] or tickets[0]["created_at"]

        # Total amount = SUM of all unit prices for this purchase
        total_amount   = sum(int(t["unit_price"] or 0) for t in tickets)

        # Describe the bundle type for the narration
        has_access = any(t["game_type"] == "ACCESS_PASS" for t in tickets)
        n_access   = sum(1 for t in tickets if t["game_type"] == "ACCESS_PASS")
        n_entries  = sum(1 for t in tickets if t["game_type"] == "DRAW_ENTRY")
        if has_access and n_access == 1:
            label = "1-Day CarPark Pass"
        elif has_access and n_access == 2:
            label = "2-Day CarPark Pass"
        else:
            label = f"{n_entries} Extra WinBig Entr{'y' if n_entries == 1 else 'ies'}"
        narration = f"CarPark {label} via Hubtel MoMo"

        serials  = [t["serial_number"] for t in tickets]
        metadata = json.dumps({
            "source":      "ussd",
            "user_role":   "player",
            "game":        GAME_CODE,
            "payment_ref": reference,
            "momo_tx_id":  momo_tx_id,
            "serials":     serials,
        })

        # ── 2. Resolve player_id ───────────────────────────────────────────
        pconn = get_player_conn()
        pcur  = pconn.cursor()
        pcur.execute("SELECT id FROM players WHERE phone_number = %s", (phone_fmt,))
        row = pcur.fetchone()
        pcur.close(); pconn.close()
        if not row:
            print(f"[PAYMENT SVC] no player for {phone_fmt}, skipping ref={reference[:8]}")
            return
        player_id = str(row[0])

        # ── 3. Insert ONE transaction per payment_ref ──────────────────────
        pyconn = _get_payment_db_conn()
        pycur  = pyconn.cursor()

        pycur.execute("""
            INSERT INTO transactions (
                reference, type, status, amount, currency, narration,
                provider_name, source_type, source_identifier, source_name,
                destination_type, destination_identifier, destination_name,
                user_id, metadata,
                requested_at, completed_at, created_at, updated_at
            ) VALUES (
                %s, 'DEPOSIT', 'SUCCESS', %s, 'GHS', %s,
                'HUBTEL', 'mobile_money', %s, %s,
                'stake_wallet', %s, 'CarPark Player',
                %s, %s::jsonb,
                %s, %s, %s, NOW()
            )
            ON CONFLICT (reference) DO NOTHING
        """, (
            reference, total_amount, narration,
            phone_fmt, phone_fmt,
            player_id,
            player_id, metadata,
            completed, completed, completed,
        ))
        inserted = pycur.rowcount
        pyconn.commit()
        pycur.close(); pyconn.close()

        if inserted:
            print(f"[PAYMENT SVC] inserted tx ref={reference[:8]} amount={total_amount} ({label})")
        else:
            print(f"[PAYMENT SVC] tx already exists for ref={reference[:8]}, skipped")

        # ── 4. Ensure wallet row exists ────────────────────────────────────
        try:
            wconn = _get_wallet_db_conn()
            wcur  = wconn.cursor()
            wcur.execute("""
                INSERT INTO player_wallets (player_id, balance, pending_balance, available_balance, currency, status)
                VALUES (%s, 0, 0, 0, 'GHS', 'ACTIVE')
                ON CONFLICT (player_id) DO NOTHING
            """, (player_id,))
            wconn.commit(); wcur.close(); wconn.close()
        except Exception as we:
            print(f"[PAYMENT SVC] wallet upsert error: {we}")

        print(f"[PAYMENT SVC] ref={reference[:8]} done — amount={total_amount} tickets={len(tickets)}")

    except Exception as e:
        print(f"[PAYMENT SVC ERROR] ref={reference[:8]}: {e}")


def get_access_passes(msisdn):
    phone = normalise_phone(msisdn)
    rows  = []
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute("""
            SELECT serial_number, game_name, payment_status, created_at
            FROM tickets
            WHERE customer_phone=%s AND game_type='ACCESS_PASS'
              AND payment_status='completed'
            ORDER BY created_at DESC LIMIT 5
        """, (phone,))
        rows = cur.fetchall()
        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] get_access_passes: {e}")
    return rows


def get_draw_entries(msisdn):
    phone = normalise_phone(msisdn)
    rows  = []
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute("""
            SELECT serial_number, payment_status, created_at
            FROM tickets
            WHERE customer_phone=%s AND game_type='DRAW_ENTRY'
              AND payment_status='completed'
            ORDER BY created_at DESC LIMIT 10
        """, (phone,))
        rows = cur.fetchall()
        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] get_draw_entries: {e}")
    return rows


# ---------------------------------------------------------------------------
# SMS
# ---------------------------------------------------------------------------

def send_sms(msisdn, message):
    phone = msisdn.strip()
    if phone.startswith("233"):
        phone = "0" + phone[3:]
    try:
        resp = requests.post(
            f"https://api.mnotify.com/api/sms/quick?key={MNOTIFY_SMS_KEY}",
            json={
                "recipient":     [phone],
                "sender":        SMS_SENDER_ID,
                "message":       message,
                "is_schedule":   False,
                "schedule_date": "",
            },
            timeout=10
        )
        result = resp.json()
        if result.get("status") == "success":
            print(f"[SMS] to={phone} status=success code={result.get('code')}")
            return True
        else:
            print(f"[SMS FAILED] to={phone} response={result}")
            return False
    except Exception as e:
        print(f"[SMS ERROR] {e}")
        return False


def send_confirmation_sms(reference):
    """
    Send confirmation SMS after payment is completed.
    - ACCESS_PASS tickets → 1 SMS per pass (paired with its draw entry)
    - Extra WinBig only  → chunk entries into groups of 5
    Marks sms_sent=TRUE on full success so the retry worker skips this ref.
    """
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute(
            "SELECT serial_number, game_type, game_name, customer_phone FROM tickets WHERE payment_ref=%s",
            (reference,)
        )
        rows = cur.fetchall()

        if not rows:
            print(f"[SMS] no tickets found for ref={reference}")
            cur.close(); conn.close()
            return

        msisdn  = rows[0]["customer_phone"]
        access  = sorted(
            [r for r in rows if r["game_type"] == "ACCESS_PASS"],
            key=lambda r: r["game_name"]   # "...Day 1..." sorts before "...Day 2..."
        )
        entries = sorted(
            [r for r in rows if r["game_type"] == "DRAW_ENTRY"],
            key=lambda r: r["serial_number"]
        )

        all_sent = True

        if access:
            # One SMS per access pass — 1-Day Pass sends 1, 2-Day Pass sends 2
            print(f"[SMS] {len(access)} pass SMS(es) for ref={reference}")
            for i, pass_row in enumerate(access):
                # game_name format: "1-Day Pass — Day 1 (02/05/2026)"
                valid_label  = pass_row["game_name"].split("—")[-1].strip()
                entry_serial = entries[i]["serial_number"] if i < len(entries) else "—"
                print(f"[SMS] {i+1}/{len(access)} pass={pass_row['serial_number']} entry={entry_serial}")
                ok = send_sms(
                    msisdn,
                    f"CarPark payment confirmed!\n"
                    f"Pass: {pass_row['serial_number']}\n"
                    f"Valid: {valid_label}\n"
                    f"WinBig Entry: {entry_serial}\n"
                    f"Draw: 03 May 2026"
                )
                if not ok:
                    all_sent = False
                if i < len(access) - 1:
                    time.sleep(1)   # avoid mNotify rate-limit between back-to-back sends
        else:
            # Extra WinBig entries only — chunk into groups of 5
            CHUNK = 5
            chunks = [entries[i:i+CHUNK] for i in range(0, len(entries), CHUNK)]
            print(f"[SMS] {len(chunks)} entries SMS chunk(s) for ref={reference}")
            for j, chunk in enumerate(chunks):
                entries_text = "\n".join(r["serial_number"] for r in chunk)
                ok = send_sms(
                    msisdn,
                    f"CarPark payment confirmed!\n"
                    f"WinBig Entries:\n{entries_text}\n"
                    f"Draw: 03 May 2026"
                )
                if not ok:
                    all_sent = False
                if j < len(chunks) - 1:
                    time.sleep(1)

        if all_sent:
            upd = conn.cursor()
            upd.execute(
                "UPDATE tickets SET sms_sent=TRUE, updated_at=NOW() WHERE payment_ref=%s",
                (reference,)
            )
            conn.commit()
            upd.close()
            print(f"[SMS] sms_sent=TRUE for ref={reference}")
        else:
            print(f"[SMS] some sends failed for ref={reference} — retry worker will pick up")

        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] send_confirmation_sms: {e}")


# ---------------------------------------------------------------------------
# Payment
# ---------------------------------------------------------------------------

def trigger_momo_payment(msisdn, amount_pesewas, reference, network="mtn"):
    """Initiate a Hubtel Direct Receive Money request."""
    phone      = "0551234987" if TEST_MODE else normalise_phone(msisdn)
    amount_ghs = 1.00 if CHARGE_GHS_1 else round(amount_pesewas / 100, 2)
    channel    = NETWORK_CHANNEL.get(network.lower(), "mtn-gh")

    credentials = base64.b64encode(
        f"{HUBTEL_CLIENT_ID}:{HUBTEL_CLIENT_SECRET}".encode()
    ).decode()

    payload = {
        "Amount":             amount_ghs,
        "Title":              "CarPark Ed. 7",
        "Description":        "Car Park ticket / WinBig draw entry",
        "PrimaryCallbackUrl": HUBTEL_CALLBACK_URL,
        "ClientReference":    reference,
        "CustomerName":       phone,
        "CustomerMsisdn":     phone,
        "Channel":            channel,
    }
    headers = {
        "Authorization": f"Basic {credentials}",
        "Content-Type":  "application/json",
        "Cache-Control": "no-cache",
    }
    url = f"https://rmp.hubtel.com/merchantaccount/merchants/{HUBTEL_POS_SALES_ID}/receive/mobilemoney"
    resp = requests.post(url, json=payload, headers=headers, timeout=15)
    print(f"[HUBTEL] POST {url} → {resp.status_code} {resp.text[:300]}")
    return resp.json()


def _fire_momo_async(msisdn, amount_pesewas, reference, network="mtn"):
    """Trigger Hubtel MoMo prompt in a background thread after a short delay
    so the USSD session can close first."""
    def _call():
        time.sleep(3)
        try:
            result    = trigger_momo_payment(msisdn, amount_pesewas, reference, network=network)
            resp_code = result.get("ResponseCode", "?")
            print(f"[ASYNC PAYMENT] ref={reference} network={network} "
                  f"ResponseCode={resp_code} msg={result.get('Data', {}).get('Description', '')}")
        except Exception as e:
            print(f"[ASYNC PAYMENT ERROR] {e}")
    threading.Thread(target=_call, daemon=True).start()


def handle_day_pass(session_id, msisdn, ticket_key, network="mtn"):
    ticket = PRICES[ticket_key]

    # ------------------------------------------------------------------
    # BYPASS MODE
    # ------------------------------------------------------------------
    if BYPASS_PAYMENT:
        result = create_tickets_from_ussd({
            "session_id":    session_id,
            "msisdn":        msisdn,
            "purchase_type": "day_pass",
            "ticket_key":    ticket_key,
        })
        _update_payment_status(result["reference"], "completed")
        send_confirmation_sms(result["reference"])
        passes_display  = "\r\n".join(result["access"])
        entries_display = "\r\n".join(result["entries"])
        return (
            f"Purchase Confirmed!\r\n"
            f"Pass(es):\r\n{passes_display}\r\n\r\n"
            f"Draw Entries:\r\n{entries_display}\r\n\r\n"
            f"Good luck!",
            True
        )

    # ------------------------------------------------------------------
    # LIVE MODE — create tickets (pending), fire MoMo in background
    # ------------------------------------------------------------------
    try:
        result = create_tickets_from_ussd({
            "session_id":    session_id,
            "msisdn":        msisdn,
            "purchase_type": "day_pass",
            "ticket_key":    ticket_key,
        })
    except Exception as e:
        print(f"[ERROR] ticket creation failed: {e}")
        return ("Sorry, an error occurred.\r\nPlease try again later.", True)

    _fire_momo_async(msisdn, ticket["amount"], result["reference"], network=network)
    return (
        f"Processing payment...\r\n"
        f"{ticket['label']} - GHS {ticket['amount'] // 100}\r\n"
        "\r\n"
        "Enter PIN when prompted.\r\n"
        "Go to My Approvals if\r\n"
        "payment prompt delays.\r\n"
        "SMS with tickets.",
        True
    )


def handle_extra_entries(session_id, msisdn, qty, network="mtn"):
    amount = qty * WINBIG_UNIT_PRICE

    # ------------------------------------------------------------------
    # BYPASS MODE
    # ------------------------------------------------------------------
    if BYPASS_PAYMENT:
        result = create_tickets_from_ussd({
            "session_id":    session_id,
            "msisdn":        msisdn,
            "purchase_type": "extra_entries",
            "qty":           qty,
        })
        _update_payment_status(result["reference"], "completed")
        send_confirmation_sms(result["reference"])
        entries_display = "\r\n".join(result["entries"])
        return (
            f"Entries Confirmed!\r\n"
            f"{qty} Draw Entry(ies):\r\n"
            f"{entries_display}\r\n\r\n"
            f"Good luck!",
            True
        )

    # ------------------------------------------------------------------
    # LIVE MODE — create tickets (pending), fire MoMo in background
    # ------------------------------------------------------------------
    try:
        result = create_tickets_from_ussd({
            "session_id":    session_id,
            "msisdn":        msisdn,
            "purchase_type": "extra_entries",
            "qty":           qty,
        })
    except Exception as e:
        print(f"[ERROR] ticket creation failed: {e}")
        return ("Sorry, an error occurred.\r\nPlease try again later.", True)

    _fire_momo_async(msisdn, amount, result["reference"], network=network)
    return (
        f"Processing payment...\r\n"
        f"{qty} WinBig Entry(ies)\r\n"
        f"GHS {qty * 20}\r\n"
        "\r\n"
        "Enter PIN when prompted.\r\n"
        "Go to My Approvals if\r\n"
        "payment prompt delays.\r\n"
        "SMS with tickets.",
        True
    )


# ---------------------------------------------------------------------------
# USSD route
# ---------------------------------------------------------------------------

@app.route("/ussd/callback", methods=["POST", "GET"])
def ussd():
    msisdn      = request.form.get("msisdn", "")
    sequence_id = request.form.get("sequenceID", "")
    raw_data    = request.form.get("data", "").strip().rstrip("#")
    network     = request.form.get("network", "")

    print(f"[REQUEST] msisdn={msisdn} seq={sequence_id} data={repr(raw_data)} net={network}")

    is_initial = (raw_data == USSD_CODE or raw_data.startswith(USSD_CODE + "*"))

    if is_initial:
        prefix = USSD_CODE + "*"
        text   = "" if raw_data == USSD_CODE else raw_data[len(prefix):]
        sessions[sequence_id] = text.split("*") if text else []
    else:
        history = sessions.get(sequence_id, [])
        if raw_data:
            history.append(raw_data)
        sessions[sequence_id] = history
        text = "*".join(history)

    print(f"[SESSION] seq={sequence_id} text={repr(text)}")

    def resp(message, end=False):
        if end:
            sessions.pop(sequence_id, None)
        return ussd_response(msisdn, sequence_id, message, end)

    # ---- Main menu ----
    if text == "":
        return resp(
            "Welcome to CarPark Ed. 7\r\n"
            "Buy a ticket & Stand a\r\n"
            "chance to win an\r\n"
            "iPhone 17 Pro\r\n\r\n"
            "1. Buy Ticket\r\n"
            "2. My Tickets\r\n"
            "3. Help\r\n"
            "0. Exit"
        )

    # ---- Buy Ticket menu ----
    elif text == "1":
        return resp(
            "Car Park Tickets\r\n\r\n"
            "1. 1-Day Pass - GHS 100\r\n"
            "2. 2-Day Pass - GHS 180\r\n"
            "3. Extra WinBig Entry\r\n"
            "   - GHS 20 each\r\n"
            "0. Back"
        )

    # ---- Day pass confirm screens ----
    elif text == "1*1":
        return resp(
            "Confirm Purchase:\r\n"
            "1-Day Pass = GHS 100\r\n"
            "Includes: 1 draw entry\r\n\r\n"
            "1. Confirm & Pay\r\n"
            "0. Cancel"
        )
    elif text == "1*2":
        return resp(
            "Confirm Purchase:\r\n"
            "2-Day Pass = GHS 180\r\n"
            "Includes: 2 draw entries\r\n\r\n"
            "1. Confirm & Pay\r\n"
            "0. Cancel"
        )

    # ---- Day pass payments ----
    elif text == "1*1*1":
        msg, end = handle_day_pass(sequence_id, msisdn, "1", network=network)
        return resp(msg, end)
    elif text == "1*2*1":
        msg, end = handle_day_pass(sequence_id, msisdn, "2", network=network)
        return resp(msg, end)
    elif text in ["1*1*0", "1*2*0"]:
        return resp("Transaction cancelled.\r\n\r\n1. Buy Ticket\r\n0. Back", end=True)

    # ---- Extra WinBig Entry flow ----
    elif text == "1*3":
        return resp(
            "Extra WinBig Entries\r\n"
            "GHS 20 each\r\n\r\n"
            "How many entries?\r\n"
            "Enter a number:"
        )
    elif text.startswith("1*3*"):
        parts   = text.split("*")
        qty_str = parts[2] if len(parts) > 2 else ""

        if len(parts) == 3:
            if not qty_str.isdigit() or int(qty_str) < 1:
                return resp("Invalid number.\r\nPlease enter a\r\npositive number:")
            qty   = int(qty_str)
            total = qty * 20
            return resp(
                f"Confirm Purchase:\r\n"
                f"{qty} WinBig Entry(ies)\r\n"
                f"Total = GHS {total}\r\n\r\n"
                "1. Confirm & Pay\r\n"
                "0. Cancel"
            )
        elif len(parts) == 4:
            action = parts[3]
            if action == "0":
                return resp("Transaction cancelled.\r\n\r\n1. Buy Ticket\r\n0. Back", end=True)
            elif action == "1":
                if not qty_str.isdigit() or int(qty_str) < 1:
                    return resp("Invalid number.\r\nPlease try again.", end=True)
                msg, end = handle_extra_entries(sequence_id, msisdn, int(qty_str), network=network)
                return resp(msg, end)
            else:
                return resp("Invalid choice.\r\nPlease try again.")
        else:
            return resp("Invalid choice.\r\nPlease try again.")

    # ---- My Tickets — sub-menu ----
    elif text == "2":
        return resp(
            "My Tickets\r\n\r\n"
            "1. Car Park Tickets\r\n"
            "2. Win Big Entries\r\n"
            "0. Back"
        )

    elif text == "2*1":
        passes = get_access_passes(msisdn)
        if not passes:
            return resp(
                "Car Park Tickets\r\n\r\n"
                "No passes yet.\r\n\r\n"
                "1. Buy Ticket\r\n"
                "0. Back"
            )
        lines = ["Car Park Tickets\r\n"]
        for p in passes:
            lines.append(f"{p['serial_number']}\r\n{p['game_name']}")
        lines.append("\r\n0. Back")
        return resp("\r\n".join(lines))

    elif text == "2*2":
        entries = get_draw_entries(msisdn)
        if not entries:
            return resp(
                "Win Big Entries\r\n\r\n"
                "No entries yet.\r\n\r\n"
                "1. Buy Ticket\r\n"
                "0. Back"
            )
        lines = ["Win Big Entries\r\n"]
        for e in entries:
            lines.append(e["serial_number"])
        lines.append(f"\r\nTotal: {len(entries)}\r\n0. Back")
        return resp("\r\n".join(lines))

    elif text in ["2*0", "2*1*0", "2*2*0"]:
        if text == "2*0":
            return resp(
                "Welcome to CarPark Ed. 7\r\n"
                "Buy a ticket & Stand a\r\n"
                "chance to win an\r\n"
                "iPhone 17 Pro\r\n\r\n"
                "1. Buy Ticket\r\n"
                "2. My Tickets\r\n"
                "3. Help\r\n"
                "0. Exit"
            )
        return resp(
            "My Tickets\r\n\r\n"
            "1. Car Park Tickets\r\n"
            "2. Win Big Entries\r\n"
            "0. Back"
        )

    # ---- Help ----
    elif text == "3":
        return resp(
            "Help & Support\r\n\r\n"
            "Call: 020 XXX XXXX\r\n"
            "Email: support@carpark.com\r\n\r\n"
            "0. Back"
        )

    # ---- Exit ----
    elif text == "0":
        return resp("Thank you for using Car Park.", end=True)

    else:
        return resp("Invalid choice.\r\nPlease try again.")


# ---------------------------------------------------------------------------
# Hubtel webhook
# ---------------------------------------------------------------------------

@app.route("/payment/webhook", methods=["POST"])
def payment_webhook():
    """Hubtel calls this after a payment attempt.
    Delegates to handle_payment_webhook — only updates tickets, never creates them.
    """
    event = request.get_json(force=True, silent=True) or {}
    print(f"[WEBHOOK] {event}")
    handle_payment_webhook(event)
    return jsonify({"status": "ok"}), 200


@app.route("/ussd/sessions", methods=["GET"])
def ussd_sessions():
    """
    Admin endpoint — returns paginated USSD session records.
    Query params: page (default 1), limit (default 20), msisdn (optional filter)
    """
    try:
        page   = max(1, int(request.args.get("page",  1)))
        limit  = max(1, min(100, int(request.args.get("limit", 20))))
        msisdn = request.args.get("msisdn", "").strip()
        offset = (page - 1) * limit

        conn = get_player_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

        where  = "WHERE msisdn = %s" if msisdn else ""
        params = (msisdn,) if msisdn else ()

        cur.execute(f"SELECT COUNT(*) as total FROM ussd_sessions {where}", params)
        total = cur.fetchone()["total"]

        cur.execute(
            f"""
            SELECT id, msisdn, sequence_id, player_id,
                   session_state, current_menu, user_input,
                   started_at, last_activity, completed_at, created_at
            FROM ussd_sessions
            {where}
            ORDER BY created_at DESC
            LIMIT %s OFFSET %s
            """,
            params + (limit, offset)
        )
        rows = cur.fetchall()
        cur.close()
        conn.close()

        sessions = []
        for r in rows:
            sessions.append({
                "id":            str(r["id"]),
                "msisdn":        r["msisdn"],
                "sequence_id":   r["sequence_id"],
                "player_id":     str(r["player_id"]) if r["player_id"] else None,
                "session_state": r["session_state"],
                "current_menu":  r["current_menu"],
                "user_input":    r["user_input"],
                "started_at":    r["started_at"].isoformat()   if r["started_at"]   else None,
                "last_activity": r["last_activity"].isoformat() if r["last_activity"] else None,
                "completed_at":  r["completed_at"].isoformat() if r["completed_at"] else None,
                "created_at":    r["created_at"].isoformat()   if r["created_at"]   else None,
            })

        return jsonify({
            "data":  sessions,
            "total": total,
            "page":  page,
            "limit": limit,
            "pages": (total + limit - 1) // limit,
        })
    except Exception as e:
        print(f"[ERROR] ussd_sessions: {e}")
        return jsonify({"error": str(e)}), 500


@app.route("/ussd/tickets", methods=["GET"])
def ussd_tickets():
    """
    Admin endpoint — returns paginated USSD tickets with full payment fields.
    Query params:
      page, limit, payment_status, game_type, search, start_date, end_date
    """
    try:
        page           = max(1, int(request.args.get("page",   1)))
        limit          = max(1, min(200, int(request.args.get("limit", 50))))
        offset         = (page - 1) * limit
        payment_status = request.args.get("payment_status", "").strip()
        game_type      = request.args.get("game_type",      "").strip()
        search         = request.args.get("search",         "").strip()
        start_date     = request.args.get("start_date",     "").strip()
        end_date       = request.args.get("end_date",       "").strip()

        conditions = ["issuer_type = 'USSD'"]
        params     = []

        if payment_status:
            conditions.append("payment_status = %s")
            params.append(payment_status)
        if game_type:
            conditions.append("game_type = %s")
            params.append(game_type)
        if search:
            conditions.append("(serial_number ILIKE %s OR customer_phone ILIKE %s OR payment_ref ILIKE %s)")
            params += [f"%{search}%", f"%{search}%", f"%{search}%"]
        if start_date:
            conditions.append("created_at >= %s")
            params.append(start_date)
        if end_date:
            conditions.append("created_at <= %s")
            params.append(end_date + " 23:59:59")

        where = "WHERE " + " AND ".join(conditions)

        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

        cur.execute(f"SELECT COUNT(*) as total FROM tickets {where}", params)
        total = cur.fetchone()["total"]

        cur.execute(
            f"""
            SELECT id, serial_number, game_type, game_name, game_code,
                   customer_phone, unit_price, total_amount,
                   payment_status, payment_ref, payment_reference,
                   payment_method, status, draw_date,
                   created_at, updated_at, paid_at
            FROM tickets
            {where}
            ORDER BY created_at DESC
            LIMIT %s OFFSET %s
            """,
            params + [limit, offset]
        )
        rows = cur.fetchall()
        cur.close()
        conn.close()

        def fmt_phone(raw):
            if not raw:
                return raw
            p = str(raw).strip()
            if p.startswith("233") and len(p) == 12:
                return "+233" + p[3:]
            if p.startswith("0"):
                return "+233" + p[1:]
            return p

        tickets = []
        for r in rows:
            tickets.append({
                "id":                 str(r["id"]),
                "serial_number":      r["serial_number"],
                "game_type":          r["game_type"],
                "game_name":          r["game_name"],
                "game_code":          r["game_code"],
                "customer_phone":     fmt_phone(r["customer_phone"]),
                "unit_price":         int(r["unit_price"]) if r["unit_price"] else 0,
                "total_amount":       int(r["total_amount"]) if r["total_amount"] else 0,
                "payment_status":     r["payment_status"] or "pending",
                "payment_ref":        r["payment_ref"],
                "payment_reference":  r["payment_reference"],
                "payment_method":     r["payment_method"],
                "status":             r["status"],
                "draw_date":          r["draw_date"].isoformat() if r["draw_date"] else None,
                "created_at":         r["created_at"].isoformat() if r["created_at"] else None,
                "updated_at":         r["updated_at"].isoformat() if r["updated_at"] else None,
                "paid_at":            r["paid_at"].isoformat() if r["paid_at"] else None,
            })

        return jsonify({
            "data":  tickets,
            "total": total,
            "page":  page,
            "limit": limit,
            "pages": (total + limit - 1) // limit,
        })
    except Exception as e:
        print(f"[ERROR] ussd_tickets: {e}")
        return jsonify({"error": str(e)}), 500


@app.route("/")
def home():
    return jsonify({"status": "WinBig USSD API is running"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001)
