import base64
import hashlib
import json
import os
import random
import re
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

# ---------------------------------------------------------------------------
# Game / Pricing config
# ---------------------------------------------------------------------------
WINBIG_UNIT_PRICE = 2000        # GHS 20 per draw entry (pesewas)
MNOTIFY_SMS_KEY   = "F9XhjQbbJnqKt2fy9lhPIQCSD"
SMS_SENDER_ID     = "CARPARK"

GAME_CODE        = "IPHONE17"
GAME_NAME        = "iPhone 17 Pro Max"
GAME_SCHEDULE_ID = "8aaa6e8d-c01f-4e4e-8a1b-e9668f481e34"
DRAW_NUMBER      = 1
DRAW_DATE        = "2026-05-03"
DRAW_DATE_LABEL  = "03 May 2026"

# ---------------------------------------------------------------------------
# Database config
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# In-memory session state
# ---------------------------------------------------------------------------
# sequenceID  -> list of user inputs accumulated in this session
sessions = {}

# MSISDN -> active sequenceID  (Telecel fallback: gateway may change seq mid-session)
sessions_by_msisdn = {}

# sequenceID -> payment_ref  (idempotency guard against gateway retries)
session_purchases = {}

# SMS retry worker interval (seconds)
SMS_RETRY_INTERVAL = 120


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def ussd_response(msisdn, sequence_id, message, end=False):
    return jsonify({
        "msisdn":       msisdn,
        "sequenceID":   sequence_id,
        "message":      message,
        "timestamp":    datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "continueFlag": 1 if end else 0,
    })


def normalise_phone(phone):
    phone = phone.strip()
    if phone.startswith("0"):
        return "233" + phone[1:]
    if not phone.startswith("233"):
        return "233" + phone
    return phone


def player_phone(msisdn):
    """Return +233XXXXXXXXX format for player_service."""
    return "+" + normalise_phone(msisdn)


def generate_entry_serial():
    """WB-ENT-XXXXXXXX -- WinBig draw entry."""
    return "WB-ENT-" + "".join(random.choices(string.ascii_uppercase + string.digits, k=8))


def make_security_hash(serial_number, payment_ref, customer_phone):
    raw = f"{serial_number}{payment_ref}{customer_phone}"
    return hashlib.sha256(raw.encode()).hexdigest()


def _make_reference():
    """Generate a unique Hubtel-safe payment reference (32 hex chars)."""
    return uuid.uuid4().hex


# ---------------------------------------------------------------------------
# Database connections
# ---------------------------------------------------------------------------

def get_ticket_conn():
    return psycopg2.connect(**TICKET_DB, connect_timeout=5)


def get_player_conn():
    return psycopg2.connect(**PLAYER_DB, connect_timeout=5)


def get_payment_conn():
    return psycopg2.connect(**PAYMENT_DB, connect_timeout=5)


def get_wallet_conn():
    return psycopg2.connect(**WALLET_DB, connect_timeout=5)


# ---------------------------------------------------------------------------
# Player
# ---------------------------------------------------------------------------

def get_or_create_player(msisdn):
    """Upsert a player record. Returns the player UUID or None on error."""
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
        print(f"[PLAYER ERROR] {e}")
        return None


def log_ussd_session(sequence_id, msisdn, player_id, payment_ref):
    """Record a completed USSD purchase session in player_service."""
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
        print(f"[SESSION ERROR] {e}")


# ---------------------------------------------------------------------------
# Ticket creation (Phase 1 -- called on USSD confirmation)
# ---------------------------------------------------------------------------

def create_winbig_tickets(session_id, msisdn, qty):
    """
    Create N DRAW_ENTRY tickets with payment_status=pending.
    Idempotent: if session_id already has a payment_ref, returns existing tickets.
    Returns {"reference": str, "entries": [serial, ...]}
    """
    # -- Idempotency guard ------------------------------------------------
    if session_id in session_purchases:
        existing_ref = session_purchases[session_id]
        print(f"[IDEMPOTENCY] session={session_id} ref={existing_ref} already exists")
        try:
            conn = get_ticket_conn()
            cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            cur.execute(
                "SELECT serial_number FROM tickets WHERE payment_ref=%s ORDER BY serial_number",
                (existing_ref,)
            )
            rows = cur.fetchall()
            cur.close(); conn.close()
            return {"reference": existing_ref, "entries": [r["serial_number"] for r in rows]}
        except Exception as e:
            print(f"[IDEMPOTENCY ERROR] {e}")

    phone      = normalise_phone(msisdn)
    reference  = _make_reference()
    total_amt  = qty * WINBIG_UNIT_PRICE
    player_id  = get_or_create_player(msisdn)

    rows_to_insert = []
    entry_serials  = []

    for _ in range(qty):
        serial    = generate_entry_serial()
        entry_serials.append(serial)
        sec_hash  = make_security_hash(serial, reference, phone)
        bet_lines = json.dumps([{"type": "DRAW_ENTRY", "source": "WinBig USSD"}])
        rows_to_insert.append((
            serial, GAME_CODE, GAME_SCHEDULE_ID,
            DRAW_NUMBER, GAME_NAME, "DRAW_ENTRY",
            bet_lines, 1,
            WINBIG_UNIT_PRICE, total_amt,
            "USSD", msisdn,
            phone,
            "mobile_money", reference, "pending",
            sec_hash, "issued", DRAW_DATE,
        ))

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
    cur.close(); conn.close()

    session_purchases[session_id] = reference
    print(f"[TICKETS] {qty} DRAW_ENTRY ticket(s) created ref={reference} entries={entry_serials}")

    log_ussd_session(session_id, msisdn, player_id, reference)

    return {"reference": reference, "entries": entry_serials}


# ---------------------------------------------------------------------------
# Payment status update
# ---------------------------------------------------------------------------

def _update_payment_status(reference, status, momo_tx_id=None):
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
        cur.close(); conn.close()
        print(f"[DB] payment_status={status} momo_tx={momo_tx_id} ref={reference}")
    except Exception as e:
        print(f"[DB ERROR] _update_payment_status: {e}")


# ---------------------------------------------------------------------------
# Payment service sync (background thread after successful webhook)
# ---------------------------------------------------------------------------

def push_to_payment_service(reference):
    """
    Insert ONE transaction row into payment_service and ensure wallet exists.
    Amount = SUM(unit_price) across all tickets for the payment_ref.
    """
    try:
        # 1. Fetch tickets
        tconn = get_ticket_conn()
        tcur  = tconn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        tcur.execute("""
            SELECT serial_number, unit_price, customer_phone,
                   payment_reference, paid_at, created_at
            FROM tickets
            WHERE payment_ref = %s
        """, (reference,))
        tickets = tcur.fetchall()
        tcur.close(); tconn.close()

        if not tickets:
            print(f"[PAYMENT SVC] no tickets for ref={reference[:8]}")
            return

        customer_phone = tickets[0]["customer_phone"]
        phone_fmt      = player_phone(customer_phone)
        momo_tx_id     = tickets[0]["payment_reference"] or ""
        completed      = tickets[0]["paid_at"] or tickets[0]["created_at"]
        total_amount   = sum(int(t["unit_price"] or 0) for t in tickets)
        n_entries      = len(tickets)
        narration      = (
            f"CarPark Ed. 7 -- {n_entries} WinBig "
            f"Entr{'y' if n_entries == 1 else 'ies'} via Hubtel MoMo"
        )
        metadata = json.dumps({
            "source":      "ussd",
            "user_role":   "player",
            "game":        GAME_CODE,
            "payment_ref": reference,
            "momo_tx_id":  momo_tx_id,
            "serials":     [t["serial_number"] for t in tickets],
        })

        # 2. Resolve player
        pconn = get_player_conn()
        pcur  = pconn.cursor()
        pcur.execute("SELECT id FROM players WHERE phone_number = %s", (phone_fmt,))
        row = pcur.fetchone()
        pcur.close(); pconn.close()
        if not row:
            print(f"[PAYMENT SVC] no player for {phone_fmt}, skipping ref={reference[:8]}")
            return
        player_id = str(row[0])

        # 3. Insert transaction (one per payment_ref, idempotent)
        pyconn = get_payment_conn()
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
            print(f"[PAYMENT SVC] tx inserted ref={reference[:8]} amount={total_amount} ({n_entries} entr{'y' if n_entries==1 else 'ies'})")
        else:
            print(f"[PAYMENT SVC] tx already exists ref={reference[:8]}, skipped")

        # 4. Ensure wallet row exists
        try:
            wconn = get_wallet_conn()
            wcur  = wconn.cursor()
            wcur.execute("""
                INSERT INTO player_wallets (player_id, balance, pending_balance, available_balance, currency, status)
                VALUES (%s, 0, 0, 0, 'GHS', 'ACTIVE')
                ON CONFLICT (player_id) DO NOTHING
            """, (player_id,))
            wconn.commit(); wcur.close(); wconn.close()
        except Exception as we:
            print(f"[PAYMENT SVC] wallet upsert error: {we}")

    except Exception as e:
        print(f"[PAYMENT SVC ERROR] ref={reference[:8]}: {e}")


# ---------------------------------------------------------------------------
# SMS
# ---------------------------------------------------------------------------

def send_sms(msisdn, message):
    """Send a single SMS via mNotify. Returns True on success."""
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
            timeout=10,
        )
        result = resp.json()
        if result.get("status") == "success":
            print(f"[SMS] to={phone} status=success code={result.get('code')}")
            return True
        print(f"[SMS FAILED] to={phone} response={result}")
        return False
    except Exception as e:
        print(f"[SMS ERROR] {e}")
        return False


def send_winbig_sms(reference):
    """
    Send confirmation SMS(es) for a completed WinBig payment.
    Entries are chunked 5 per SMS.
    Marks sms_sent=TRUE on all tickets when every SMS succeeds.
    """
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute(
            "SELECT serial_number, customer_phone FROM tickets "
            "WHERE payment_ref=%s ORDER BY serial_number",
            (reference,)
        )
        rows = cur.fetchall()
        cur.close()

        if not rows:
            print(f"[SMS] no tickets for ref={reference}")
            conn.close()
            return

        msisdn  = rows[0]["customer_phone"]
        serials = [r["serial_number"] for r in rows]

        CHUNK    = 5
        chunks   = [serials[i:i+CHUNK] for i in range(0, len(serials), CHUNK)]
        all_sent = True

        print(f"[SMS] {len(chunks)} chunk(s) for ref={reference} to={msisdn}")
        for j, chunk in enumerate(chunks):
            entries_text = "\n".join(chunk)
            label = "Entry" if len(chunk) == 1 else "Entries"
            ok = send_sms(
                msisdn,
                f"WinBig {label}:\n{entries_text}\n"
                f"CarPark Ed. 7 Draw: {DRAW_DATE_LABEL}\n"
                f"Good luck!"
            )
            if not ok:
                all_sent = False
            if j < len(chunks) - 1:
                time.sleep(1)   # avoid mNotify rate-limit between back-to-back sends

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
            print(f"[SMS] some sends failed for ref={reference} -- retry worker will pick up")

        conn.close()
    except Exception as e:
        print(f"[SMS ERROR] send_winbig_sms ref={reference}: {e}")


# ---------------------------------------------------------------------------
# Background SMS retry worker
# ---------------------------------------------------------------------------

def _sms_retry_worker():
    """
    Runs every SMS_RETRY_INTERVAL seconds.
    Finds completed DRAW_ENTRY tickets where sms_sent=FALSE and retries sending.
    Grouped by payment_ref so each purchase gets one attempt per cycle.
    """
    print(f"[SMS WORKER] started -- interval={SMS_RETRY_INTERVAL}s")
    while True:
        time.sleep(SMS_RETRY_INTERVAL)
        try:
            conn = get_ticket_conn()
            cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            cur.execute("""
                SELECT DISTINCT payment_ref
                FROM tickets
                WHERE payment_status = 'completed'
                  AND (sms_sent = FALSE OR sms_sent IS NULL)
                  AND game_type = 'DRAW_ENTRY'
                ORDER BY payment_ref
            """)
            pending_refs = [r["payment_ref"] for r in cur.fetchall()]
            cur.close(); conn.close()

            if pending_refs:
                print(f"[SMS WORKER] {len(pending_refs)} ref(s) pending SMS")
                for ref in pending_refs:
                    send_winbig_sms(ref)
                    time.sleep(1)
        except Exception as e:
            print(f"[SMS WORKER ERROR] {e}")


# ---------------------------------------------------------------------------
# Hubtel payment
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
        "Title":              "CarPark Ed. 7 -- WinBig",
        "Description":        "WinBig draw entry",
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
    url  = f"https://rmp.hubtel.com/merchantaccount/merchants/{HUBTEL_POS_SALES_ID}/receive/mobilemoney"
    resp = requests.post(url, json=payload, headers=headers, timeout=15)
    print(f"[HUBTEL] POST {url} -> {resp.status_code} {resp.text[:300]}")
    return resp.json()


def _fire_momo_async(msisdn, amount_pesewas, reference, network="mtn"):
    """Trigger Hubtel MoMo in a background thread after a short delay
    so the USSD session can close before the STK push arrives."""
    def _call():
        time.sleep(3)
        try:
            result    = trigger_momo_payment(msisdn, amount_pesewas, reference, network=network)
            resp_code = result.get("ResponseCode", "?")
            print(f"[ASYNC PAYMENT] ref={reference} net={network} "
                  f"ResponseCode={resp_code} msg={result.get('Data', {}).get('Description', '')}")
        except Exception as e:
            print(f"[ASYNC PAYMENT ERROR] {e}")
    threading.Thread(target=_call, daemon=True).start()


def handle_buy_entries(session_id, msisdn, qty, network="mtn"):
    """
    Orchestrates the confirm->create->pay flow for WinBig entries.
    Returns (message, end_session).
    """
    amount = qty * WINBIG_UNIT_PRICE

    # BYPASS MODE -- for testing without real payments
    if BYPASS_PAYMENT:
        result = create_winbig_tickets(session_id, msisdn, qty)
        _update_payment_status(result["reference"], "completed")
        send_winbig_sms(result["reference"])
        entries_text = "\r\n".join(result["entries"])
        return (
            f"Entries Confirmed!\r\n"
            f"{qty} WinBig Entr{'y' if qty == 1 else 'ies'}:\r\n"
            f"{entries_text}\r\n\r\n"
            f"Good luck!",
            True,
        )

    # LIVE MODE
    try:
        result = create_winbig_tickets(session_id, msisdn, qty)
    except Exception as e:
        print(f"[ERROR] ticket creation failed: {e}")
        return ("Sorry, an error occurred.\r\nPlease try again later.", True)

    _fire_momo_async(msisdn, amount, result["reference"], network=network)
    return (
        f"Processing payment...\r\n"
        f"{qty} WinBig Entr{'y' if qty == 1 else 'ies'}\r\n"
        f"GHS {qty * 20}\r\n"
        "\r\n"
        "Enter PIN when prompted.\r\n"
        "Go to My Approvals if\r\n"
        "payment prompt delays.\r\n"
        "SMS with entries.",
        True,
    )


# ---------------------------------------------------------------------------
# My Entries query
# ---------------------------------------------------------------------------

def get_my_entries(msisdn):
    """Return up to 10 completed DRAW_ENTRY serials for this MSISDN, newest first."""
    phone = normalise_phone(msisdn)
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute("""
            SELECT serial_number
            FROM tickets
            WHERE customer_phone = %s
              AND game_type = 'DRAW_ENTRY'
              AND payment_status = 'completed'
            ORDER BY created_at DESC
            LIMIT 10
        """, (phone,))
        rows = cur.fetchall()
        cur.close(); conn.close()
        return [r["serial_number"] for r in rows]
    except Exception as e:
        print(f"[DB ERROR] get_my_entries: {e}")
        return []


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

    # -- Telecel/gateway session-end signal ---------------------------------
    if raw_data.lower() == "release":
        sessions.pop(sequence_id, None)
        sessions_by_msisdn.pop(msisdn, None)
        print(f"[RELEASE] msisdn={msisdn} seq={sequence_id}")
        return ussd_response(msisdn, sequence_id, "Session ended.", end=True)

    # Hubtel gateway error codes e.g. '03020340-UNKNOWN_ERROR' -- not user input
    _GATEWAY_ERR = bool(re.match(r'^[0-9A-Fa-f]+-[A-Z_]+$', raw_data))

    is_initial = (raw_data == USSD_CODE or raw_data.startswith(USSD_CODE + "*"))

    if is_initial:
        prefix = USSD_CODE + "*"
        text   = "" if raw_data == USSD_CODE else raw_data[len(prefix):]
        sessions[sequence_id] = text.split("*") if text else []
        sessions_by_msisdn[msisdn] = sequence_id
    else:
        # -- MSISDN fallback: Telecel may change sequenceID mid-session -----
        existing_seq = sessions_by_msisdn.get(msisdn)
        if existing_seq and existing_seq != sequence_id and sequence_id not in sessions:
            old_history = sessions.pop(existing_seq, [])
            sessions[sequence_id] = old_history
            sessions_by_msisdn[msisdn] = sequence_id
            print(f"[SESSION MIGRATE] msisdn={msisdn} {existing_seq} -> {sequence_id}")

        history = sessions.get(sequence_id, [])
        if _GATEWAY_ERR:
            print(f"[GATEWAY ERROR] ignored: {repr(raw_data)}")
        elif raw_data:
            history.append(raw_data)
        sessions[sequence_id] = history
        text = "*".join(history)

    print(f"[SESSION] seq={sequence_id} text={repr(text)}")

    def resp(message, end=False):
        if end:
            sessions.pop(sequence_id, None)
            sessions_by_msisdn.pop(msisdn, None)
        return ussd_response(msisdn, sequence_id, message, end)

    # -- Main menu ----------------------------------------------------------
    if text == "":
        return resp(
            "CarPark Ed. 7 -- WinBig\r\n"
            "Buy entries & stand a\r\n"
            "chance to win an\r\n"
            "iPhone 17 Pro\r\n\r\n"
            "1. Buy WinBig Entries\r\n"
            "2. My Entries\r\n"
            "3. Help\r\n"
            "0. Exit"
        )

    # -- Buy WinBig Entries -------------------------------------------------
    elif text == "1":
        return resp(
            "WinBig Entries\r\n"
            "GHS 20 each\r\n\r\n"
            "How many entries?\r\n"
            "Enter a number:"
        )

    elif text.startswith("1*"):
        parts   = text.split("*")
        qty_str = parts[1] if len(parts) > 1 else ""

        if len(parts) == 2:
            # User entered a quantity -- show confirm screen
            if not qty_str.isdigit() or int(qty_str) < 1:
                return resp(
                    "Invalid number.\r\n"
                    "Please enter a\r\n"
                    "positive number:"
                )
            qty   = int(qty_str)
            total = qty * 20
            return resp(
                f"Confirm Purchase:\r\n"
                f"{qty} WinBig Entr{'y' if qty == 1 else 'ies'}\r\n"
                f"Total = GHS {total}\r\n\r\n"
                "1. Confirm & Pay\r\n"
                "0. Cancel"
            )

        elif len(parts) == 3:
            action = parts[2]
            if action == "0":
                return resp(
                    "Transaction cancelled.\r\n\r\n"
                    "1. Buy WinBig Entries\r\n"
                    "0. Back",
                    end=True,
                )
            elif action == "1":
                if not qty_str.isdigit() or int(qty_str) < 1:
                    return resp("Invalid number.\r\nPlease try again.", end=True)
                msg, end = handle_buy_entries(sequence_id, msisdn, int(qty_str), network=network)
                return resp(msg, end)
            else:
                return resp("Invalid choice.\r\nPlease try again.")

        else:
            return resp("Invalid choice.\r\nPlease try again.")

    # -- My Entries ---------------------------------------------------------
    elif text == "2":
        entries = get_my_entries(msisdn)
        if not entries:
            return resp(
                "My WinBig Entries\r\n\r\n"
                "No entries yet.\r\n\r\n"
                "1. Buy WinBig Entries\r\n"
                "0. Back"
            )
        lines = ["My WinBig Entries\r\n"]
        for serial in entries:
            lines.append(serial)
        lines.append(f"\r\nTotal: {len(entries)}\r\n0. Back")
        return resp("\r\n".join(lines))

    elif text in ("2*0", "2*1"):
        if text == "2*1":
            # "1. Buy WinBig Entries" from My Entries empty state
            return resp(
                "WinBig Entries\r\n"
                "GHS 20 each\r\n\r\n"
                "How many entries?\r\n"
                "Enter a number:"
            )
        # "0. Back" -- return to main menu
        return resp(
            "CarPark Ed. 7 -- WinBig\r\n"
            "Buy entries & stand a\r\n"
            "chance to win an\r\n"
            "iPhone 17 Pro\r\n\r\n"
            "1. Buy WinBig Entries\r\n"
            "2. My Entries\r\n"
            "3. Help\r\n"
            "0. Exit"
        )

    # -- Help ---------------------------------------------------------------
    elif text == "3":
        return resp(
            "Help & Support\r\n\r\n"
            "Call: 020 XXX XXXX\r\n"
            "Email: support@carpark.com\r\n\r\n"
            "0. Back"
        )

    elif text == "3*0":
        return resp(
            "CarPark Ed. 7 -- WinBig\r\n"
            "Buy entries & stand a\r\n"
            "chance to win an\r\n"
            "iPhone 17 Pro\r\n\r\n"
            "1. Buy WinBig Entries\r\n"
            "2. My Entries\r\n"
            "3. Help\r\n"
            "0. Exit"
        )

    # -- Exit ---------------------------------------------------------------
    elif text == "0":
        return resp("Thank you for using\r\nCarPark Ed. 7 WinBig.", end=True)

    else:
        return resp("Invalid choice.\r\nPlease try again.")


# ---------------------------------------------------------------------------
# Hubtel payment webhook
# ---------------------------------------------------------------------------

@app.route("/payment/webhook", methods=["POST"])
def payment_webhook():
    """
    Hubtel calls this after a MoMo payment attempt.
    ResponseCode "0000" = success, "2001" = failed.
    Only updates existing tickets -- never creates new ones.
    """
    event = request.get_json(force=True, silent=True) or {}
    print(f"[WEBHOOK] {event}")

    data          = event.get("Data", {})
    response_code = event.get("ResponseCode", "")
    reference     = data.get("ClientReference", "")
    amount        = data.get("Amount", 0)
    phone         = data.get("CustomerMsisdn", "")

    if not reference:
        print("[WEBHOOK] missing ClientReference, ignoring")
        return jsonify({"status": "ok"}), 200

    if response_code == "0000":
        momo_tx_id = data.get("TransactionId", "") or data.get("OrderId", "")
        print(f"[PAYMENT SUCCESS] ref={reference} amount=GHS{amount} phone={phone} momo_tx={momo_tx_id}")
        _update_payment_status(reference, "completed", momo_tx_id=momo_tx_id)
        threading.Thread(target=send_winbig_sms,         args=(reference,), daemon=True).start()
        threading.Thread(target=push_to_payment_service,  args=(reference,), daemon=True).start()

    elif response_code == "2001":
        print(f"[PAYMENT FAILED] ref={reference} code={response_code}")
        _update_payment_status(reference, "failed")

    else:
        print(f"[PAYMENT PENDING/OTHER] ref={reference} code={response_code}")

    return jsonify({"status": "ok"}), 200


# ---------------------------------------------------------------------------
# Admin API endpoints
# ---------------------------------------------------------------------------

@app.route("/ussd/sessions", methods=["GET"])
def ussd_sessions_api():
    """
    Paginated USSD session records from player_service.
    Query params: page, limit, msisdn
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
            params + (limit, offset),
        )
        rows = cur.fetchall()
        cur.close(); conn.close()

        result = []
        for r in rows:
            result.append({
                "id":            str(r["id"]),
                "msisdn":        r["msisdn"],
                "sequence_id":   r["sequence_id"],
                "player_id":     str(r["player_id"]) if r["player_id"] else None,
                "session_state": r["session_state"],
                "current_menu":  r["current_menu"],
                "user_input":    r["user_input"],
                "started_at":    r["started_at"].isoformat()    if r["started_at"]    else None,
                "last_activity": r["last_activity"].isoformat() if r["last_activity"] else None,
                "completed_at":  r["completed_at"].isoformat()  if r["completed_at"]  else None,
                "created_at":    r["created_at"].isoformat()    if r["created_at"]    else None,
            })

        return jsonify({
            "data":  result,
            "total": total,
            "page":  page,
            "limit": limit,
            "pages": (total + limit - 1) // limit,
        })
    except Exception as e:
        print(f"[ERROR] ussd_sessions_api: {e}")
        return jsonify({"error": str(e)}), 500


@app.route("/ussd/tickets", methods=["GET"])
def ussd_tickets_api():
    """
    Paginated ticket records from ticket_service.
    Query params: page, limit, payment_status, game_type, search, start_date, end_date
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
            conditions.append(
                "(serial_number ILIKE %s OR customer_phone ILIKE %s OR payment_ref ILIKE %s)"
            )
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
            params + [limit, offset],
        )
        rows = cur.fetchall()
        cur.close(); conn.close()

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
                "id":                str(r["id"]),
                "serial_number":     r["serial_number"],
                "game_type":         r["game_type"],
                "game_name":         r["game_name"],
                "game_code":         r["game_code"],
                "customer_phone":    fmt_phone(r["customer_phone"]),
                "unit_price":        int(r["unit_price"])    if r["unit_price"]    else 0,
                "total_amount":      int(r["total_amount"])  if r["total_amount"]  else 0,
                "payment_status":    r["payment_status"] or "pending",
                "payment_ref":       r["payment_ref"],
                "payment_reference": r["payment_reference"],
                "payment_method":    r["payment_method"],
                "status":            r["status"],
                "draw_date":         r["draw_date"].isoformat()  if r["draw_date"]  else None,
                "created_at":        r["created_at"].isoformat() if r["created_at"] else None,
                "updated_at":        r["updated_at"].isoformat() if r["updated_at"] else None,
                "paid_at":           r["paid_at"].isoformat()    if r["paid_at"]    else None,
            })

        return jsonify({
            "data":  tickets,
            "total": total,
            "page":  page,
            "limit": limit,
            "pages": (total + limit - 1) // limit,
        })
    except Exception as e:
        print(f"[ERROR] ussd_tickets_api: {e}")
        return jsonify({"error": str(e)}), 500


@app.route("/")
def home():
    return jsonify({"status": "WinBig USSD API is running -- CarPark Ed. 7"})


# ---------------------------------------------------------------------------
# Startup -- launch background SMS retry worker
# ---------------------------------------------------------------------------

_worker_thread = threading.Thread(target=_sms_retry_worker, daemon=True)
_worker_thread.start()

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001)
