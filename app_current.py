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
import redis
from datetime import datetime
from flask import Flask, request, jsonify

app = Flask(__name__)

USSD_CODE      = "*899*92"
TEST_MODE      = False
BYPASS_PAYMENT = False
CHARGE_GHS_1   = True   # send GHS 1 to Hubtel regardless of ticket price (testing)

# ---------------------------------------------------------------------------
# Hubtel credentials — set via environment variables on the server
# ---------------------------------------------------------------------------
HUBTEL_CLIENT_ID     = os.environ.get("HUBTEL_CLIENT_ID", "")
HUBTEL_CLIENT_SECRET = os.environ.get("HUBTEL_CLIENT_SECRET", "")
HUBTEL_POS_SALES_ID  = os.environ.get("HUBTEL_POS_SALES_ID", "")
HUBTEL_CALLBACK_URL  = "https://api.winbig.bedriften.xyz/payment/webhook"

# Network code → Hubtel channel mapping
NETWORK_CHANNEL = {
    "mtn":       "mtn-gh",
    "vodafone":  "vodafone-gh",
    "airteltigo":"tigo-gh",
    "airtel":    "airtel-gh",
    "tigo":      "tigo-gh",
}

PRICES = {
    "1": {"label": "1-Day Pass",  "amount": 10000, "days": "Day 1",       "entries": 1},
    "2": {"label": "2-Day Pass",  "amount": 18000, "days": "Day 1 & 2",   "entries": 2},
}
WINBIG_UNIT_PRICE = 2000  # GHS 20 per extra draw entry in pesewas

MNOTIFY_SMS_KEY = "F9XhjQbbJnqKt2fy9lhPIQCSD"
SMS_SENDER_ID   = "CARPARK"

# Game constants
GAME_CODE        = "IPHONE17"
GAME_NAME        = "iPhone 17 Pro Max"
GAME_SCHEDULE_ID = "aa5d0c8b-d7b8-4148-9955-e8453b88198f"
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
    "host":     os.environ.get("PLAYER_DB_HOST", "localhost"),
    "port":     5444,
    "dbname":   "player_service",
    "user":     "player",
    "password": "player123",
}

# In-memory session state: sequenceID -> list of user inputs
sessions = {}


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


def make_security_hash(serial, phone, amount, reference):
    raw = f"{serial}{phone}{amount}{reference}"
    return hashlib.sha256(raw.encode()).hexdigest()


# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

def get_ticket_conn():
    return psycopg2.connect(**TICKET_DB, connect_timeout=2)


def _insert_row(cur, conn, serial, game_type, game_name, unit_price, total_amount,
                msisdn, phone, reference, bet_lines_data):
    """Insert a single ticket row, retrying on serial collision."""
    for _ in range(5):
        try:
            security_hash = make_security_hash(serial, phone, total_amount, reference)
            cur.execute("""
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
            """, (
                serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, game_name, game_type,
                json.dumps(bet_lines_data), 1,
                unit_price, total_amount,
                "USSD", msisdn,
                phone,
                "mobile_money", reference, "pending",
                security_hash, "issued", DRAW_DATE,
            ))
            return serial
        except psycopg2.errors.UniqueViolation:
            conn.rollback()
            serial = generate_access_serial() if serial.startswith("WB-ACC") else generate_entry_serial()
    return serial


def save_day_pass(msisdn, ticket_key, reference):
    """Create 1 access pass + N draw entries for a day pass purchase.
    Returns {"access": serial, "entries": [serials]}."""
    ticket = PRICES[ticket_key]
    phone  = normalise_phone(msisdn)
    access_serial  = generate_access_serial()
    entry_serials  = [generate_entry_serial() for _ in range(ticket["entries"])]

    try:
        conn = get_ticket_conn()
        cur  = conn.cursor()

        # Access pass row
        _insert_row(
            cur, conn,
            serial      = access_serial,
            game_type   = "ACCESS_PASS",
            game_name   = f"{ticket['label']} — {ticket['days']}",
            unit_price  = ticket["amount"],
            total_amount= ticket["amount"],
            msisdn      = msisdn,
            phone       = phone,
            reference   = reference,
            bet_lines_data = [{"type": "ACCESS_PASS", "days": ticket["days"]}],
        )

        # Draw entry rows
        for serial in entry_serials:
            _insert_row(
                cur, conn,
                serial      = serial,
                game_type   = "DRAW_ENTRY",
                game_name   = GAME_NAME,
                unit_price  = 0,
                total_amount= 0,
                msisdn      = msisdn,
                phone       = phone,
                reference   = reference,
                bet_lines_data = [{"type": "DRAW_ENTRY", "source": ticket["label"]}],
            )

        conn.commit()
        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] save_day_pass: {e}")

    return {"access": access_serial, "entries": entry_serials}


def save_extra_entries(msisdn, qty, reference):
    """Create N draw entries for extra WinBig ticket purchase.
    Returns list of entry serials."""
    phone  = normalise_phone(msisdn)
    serials = [generate_entry_serial() for _ in range(qty)]

    try:
        conn = get_ticket_conn()
        cur  = conn.cursor()

        for serial in serials:
            _insert_row(
                cur, conn,
                serial      = serial,
                game_type   = "DRAW_ENTRY",
                game_name   = GAME_NAME,
                unit_price  = WINBIG_UNIT_PRICE,
                total_amount= qty * WINBIG_UNIT_PRICE,
                msisdn      = msisdn,
                phone       = phone,
                reference   = reference,
                bet_lines_data = [{"type": "DRAW_ENTRY", "source": "Extra WinBig"}],
            )

        conn.commit()
        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] save_extra_entries: {e}")

    return serials


def update_payment_status(reference, status):
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor()
        cur.execute(
            "UPDATE tickets SET payment_status=%s, updated_at=NOW() WHERE payment_ref=%s",
            (status, reference)
        )
        conn.commit()
        cur.close()
        conn.close()
    except Exception as e:
        print(f"[DB ERROR] update_payment_status: {e}")


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
        else:
            print(f"[SMS FAILED] to={phone} response={result}")
    except Exception as e:
        print(f"[SMS ERROR] {e}")


def sms_day_pass(msisdn, ticket, result):
    """Build and send SMS for a day pass purchase."""
    entry_count = len(result["entries"])
    send_sms(
        msisdn,
        f"CarPark confirmed!\n"
        f"Pass: {result['access']}\n"
        f"Valid: {ticket['days']}\n"
        f"Draw entries: {entry_count}\n"
        f"Draw: 03 May 2026\n"
        f"Good luck!"
    )


def sms_extra_entries(msisdn, serials):
    count = len(serials)
    send_sms(
        msisdn,
        f"WinBig entries confirmed!\n"
        f"{count} draw entry(ies) added.\n"
        f"Check entries: dial *899*92\n"
        f"Draw: 03 May 2026\n"
        f"Good luck!"
    )


# ---------------------------------------------------------------------------
# Payment
# ---------------------------------------------------------------------------

def _fire_momo_async(msisdn, amount_pesewas, reference, network="mtn"):
    """Trigger Hubtel MoMo prompt in a background thread after a short delay,
    so the USSD session can close first."""
    def _call():
        time.sleep(3)
        try:
            result = trigger_momo_payment(msisdn, amount_pesewas, reference, network=network)
            resp_code = result.get("ResponseCode", "?")
            print(f"[ASYNC PAYMENT] ref={reference} network={network} "
                  f"ResponseCode={resp_code} msg={result.get('Data', {}).get('Description', '')}")
        except Exception as e:
            print(f"[ASYNC PAYMENT ERROR] {e}")
    threading.Thread(target=_call, daemon=True).start()


def send_confirmation_sms(reference):
    """Called by webhook after payment confirmed — looks up tickets by reference and SMSes customer."""
    try:
        conn = get_ticket_conn()
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute(
            "SELECT serial_number, game_type, game_name, customer_phone FROM tickets WHERE payment_ref=%s",
            (reference,)
        )
        rows = cur.fetchall()
        cur.close()
        conn.close()

        if not rows:
            print(f"[SMS] no tickets found for ref={reference}")
            return

        msisdn  = rows[0]["customer_phone"]
        access  = [r for r in rows if r["game_type"] == "ACCESS_PASS"]
        entries = [r for r in rows if r["game_type"] == "DRAW_ENTRY"]

        if access:
            send_sms(
                msisdn,
                f"CarPark payment confirmed!\n"
                f"Pass: {access[0]['serial_number']}\n"
                f"Valid: {access[0]['game_name'].split('—')[-1].strip()}\n"
                f"Draw entries: {len(entries)}\n"
                f"Draw: 03 May 2026\nGood luck!"
            )
        else:
            send_sms(
                msisdn,
                f"WinBig payment confirmed!\n"
                f"{len(entries)} draw entry(ies) added.\n"
                f"Dial *899*92 to view entries.\n"
                f"Draw: 03 May 2026\nGood luck!"
            )
    except Exception as e:
        print(f"[DB ERROR] send_confirmation_sms: {e}")


def trigger_momo_payment(msisdn, amount_pesewas, reference, network="mtn"):
    """Initiate a Hubtel Direct Receive Money request.
    amount_pesewas: internal amount in pesewas (divide by 100 → GHS float for Hubtel).
    reference: must be ≤ 36 alphanumeric chars (used as ClientReference and DB key).
    """
    phone      = "0551234987" if TEST_MODE else normalise_phone(msisdn)
    amount_ghs = 1.00 if CHARGE_GHS_1 else round(amount_pesewas / 100, 2)
    channel = NETWORK_CHANNEL.get(network.lower(), "mtn-gh")

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


def _make_reference():
    """Generate a unique, Hubtel-safe reference (32 hex chars, ≤ 36)."""
    return uuid.uuid4().hex  # e.g. "a3f2c1d0e4b5..."


def handle_day_pass(msisdn, ticket_key, network="mtn"):
    ticket    = PRICES[ticket_key]
    reference = _make_reference()

    # ------------------------------------------------------------------
    # BYPASS MODE
    # ------------------------------------------------------------------
    if BYPASS_PAYMENT:
        result = save_day_pass(msisdn, ticket_key, reference)
        update_payment_status(reference, "completed")
        print(f"[BYPASS] day pass: access={result['access']} entries={result['entries']}")
        sms_day_pass(msisdn, ticket, result)
        entries_display = "\r\n".join(result["entries"])
        return (
            f"Purchase Confirmed!\r\n"
            f"Pass: {result['access']}\r\n"
            f"({ticket['days']})\r\n\r\n"
            f"Draw Entries:\r\n{entries_display}\r\n\r\n"
            f"Good luck!",
            True
        )

    # ------------------------------------------------------------------
    # LIVE MODE — save tickets immediately, fire MoMo in background
    # ------------------------------------------------------------------
    result = save_day_pass(msisdn, ticket_key, reference)
    _fire_momo_async(msisdn, ticket["amount"], reference, network=network)
    return (
        f"Processing payment...\r\n"
        f"{ticket['label']} - GHS {ticket['amount'] // 100}\r\n\r\n"
        "Enter MoMo PIN when\r\n"
        "prompted.\r\n\r\n"
        "Ticket details will\r\n"
        "be sent via SMS.",
        True
    )


def handle_extra_entries(msisdn, qty, network="mtn"):
    amount    = qty * WINBIG_UNIT_PRICE
    reference = _make_reference()

    # ------------------------------------------------------------------
    # BYPASS MODE
    # ------------------------------------------------------------------
    if BYPASS_PAYMENT:
        serials = save_extra_entries(msisdn, qty, reference)
        update_payment_status(reference, "completed")
        print(f"[BYPASS] extra entries: {serials}")
        sms_extra_entries(msisdn, serials)
        entries_display = "\r\n".join(serials)
        return (
            f"Entries Confirmed!\r\n"
            f"{qty} Draw Entry(ies):\r\n"
            f"{entries_display}\r\n\r\n"
            f"Good luck!",
            True
        )

    # ------------------------------------------------------------------
    # LIVE MODE — save entries immediately, fire MoMo in background
    # ------------------------------------------------------------------
    serials = save_extra_entries(msisdn, qty, reference)
    _fire_momo_async(msisdn, amount, reference, network=network)
    return (
        f"Processing payment...\r\n"
        f"{qty} WinBig Entry(ies)\r\n"
        f"GHS {qty * 20}\r\n\r\n"
        "Enter MoMo PIN when\r\n"
        "prompted.\r\n\r\n"
        "Ticket details will\r\n"
        "be sent via SMS.",
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
        msg, end = handle_day_pass(msisdn, "1", network=network)
        return resp(msg, end)
    elif text == "1*2*1":
        msg, end = handle_day_pass(msisdn, "2", network=network)
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
                msg, end = handle_extra_entries(msisdn, int(qty_str), network=network)
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
    """Hubtel calls this URL after a payment attempt completes.
    ResponseCode "0000" = success, "2001" = failed/cancelled.
    """
    event = request.get_json(force=True, silent=True) or {}
    print(f"[WEBHOOK] {event}")

    data          = event.get("Data", {})
    response_code = event.get("ResponseCode", "")
    reference     = data.get("ClientReference", "")
    amount        = data.get("Amount", 0)
    phone         = data.get("CustomerMsisdn", "")

    if response_code == "0000":
        # Payment successful
        print(f"[PAYMENT SUCCESS] ref={reference} amount=GHS{amount} phone={phone}")
        update_payment_status(reference, "completed")
        send_confirmation_sms(reference)
    elif response_code == "2001":
        # Payment failed or cancelled by user
        print(f"[PAYMENT FAILED] ref={reference} code={response_code}")
        update_payment_status(reference, "failed")
    else:
        print(f"[PAYMENT PENDING/OTHER] ref={reference} code={response_code}")

    return jsonify({"status": "ok"}), 200


@app.route("/")
def home():
    return jsonify({"status": "WinBig USSD API is running"})


# ---------------------------------------------------------------------------
# Featured game config — admin sets, website reads
# ---------------------------------------------------------------------------

import threading as _threading
_config_lock = _threading.Lock()
_config_file = os.path.join(os.path.dirname(__file__), "config.json")

def _read_config():
    try:
        with open(_config_file, "r") as f:
            return json.load(f)
    except Exception:
        return {}

def _write_config(data):
    with _config_lock:
        with open(_config_file, "w") as f:
            json.dump(data, f)

@app.route("/config", methods=["GET", "OPTIONS"])
def get_config():
    if request.method == "OPTIONS":
        resp = jsonify({})
        resp.headers["Access-Control-Allow-Origin"] = "*"
        resp.headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
        resp.headers["Access-Control-Allow-Headers"] = "Authorization, Content-Type"
        return resp, 204
    cfg = _read_config()
    resp = jsonify(cfg)
    resp.headers["Access-Control-Allow-Origin"] = "*"
    return resp

@app.route("/config", methods=["POST"])
def set_config():
    auth = request.headers.get("Authorization", "")
    if auth != "Bearer winbig-admin-config-2026":
        return jsonify({"error": "unauthorized"}), 401
    data = request.get_json(force=True, silent=True) or {}
    cfg = _read_config()
    cfg.update(data)
    _write_config(cfg)
    resp = jsonify({"status": "ok", "config": cfg})
    resp.headers["Access-Control-Allow-Origin"] = "*"
    return resp



# ---------------------------------------------------------------------------
# OTP — Phone & Email Verification
# ---------------------------------------------------------------------------

PLAYER_REDIS = redis.Redis(host="localhost", port=6391, db=1, decode_responses=True)
OTP_TTL      = 300   # 5 minutes
OTP_MAX_ATTEMPTS = 3
MNOTIFY_KEY  = "F9XhjQbbJnqKt2fy9lhPIQCSD"

def _generate_otp():
    return str(random.randint(100000, 999999))

def _otp_key(player_id, channel):
    return f"otp:{channel}:{player_id}"

def _attempts_key(player_id, channel):
    return f"otp_attempts:{channel}:{player_id}"

def _send_sms_otp(phone, otp):
    """Send OTP via mNotify SMS."""
    local = phone.strip()
    if local.startswith("233"):
        local = "0" + local[3:]
    elif local.startswith("+233"):
        local = "0" + local[4:]
    try:
        resp = requests.post(
            f"https://api.mnotify.com/api/sms/quick?key={MNOTIFY_KEY}",
            data={
                "recipient[]": local,
                "sender":      "CARPARK",
                "message":     f"Your WinBig verification code is: {otp}. Valid for 5 minutes. Do not share.",
                "is_schedule": "false",
                "schedule_date": "",
            },
            timeout=10
        )
        result = resp.json()
        print(f"[OTP SMS] to={local} result={result.get('status')} code={result.get('code')}")
        return result.get("status") == "success"
    except Exception as e:
        print(f"[OTP SMS ERROR] {e}")
        return False

def _update_player_verified(player_id, field):
    """Set phone_verified or email_verified = true in player DB."""
    try:
        conn = psycopg2.connect(**PLAYER_DB, connect_timeout=3)
        cur  = conn.cursor()
        cur.execute(f"UPDATE players SET {field} = true, updated_at = NOW() WHERE id = %s", (player_id,))
        conn.commit()
        cur.close()
        conn.close()
        print(f"[OTP] player {player_id} {field} set to true")
        return True
    except Exception as e:
        print(f"[OTP DB ERROR] {e}")
        return False

def _cors(resp):
    resp.headers["Access-Control-Allow-Origin"]  = "*"
    resp.headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
    resp.headers["Access-Control-Allow-Headers"] = "Authorization, Content-Type"
    return resp

@app.route("/otp/send", methods=["POST", "OPTIONS"])
def otp_send():
    if request.method == "OPTIONS":
        return _cors(jsonify({})), 204
    data      = request.get_json(force=True, silent=True) or {}
    print(f"[OTP SEND] raw data: {request.data} parsed: {data}")
    player_id = (data.get("player_id") or "").strip()
    channel   = (data.get("channel") or "phone").strip()
    contact   = (data.get("contact") or "").strip()

    if not player_id or not contact:
        return _cors(jsonify({"error": "player_id and contact required"})), 400

    # Rate limit — max 3 sends per hour
    att_key = _attempts_key(player_id, channel)
    attempts = int(PLAYER_REDIS.get(att_key) or 0)
    if attempts >= OTP_MAX_ATTEMPTS:
        ttl = PLAYER_REDIS.ttl(att_key)
        return _cors(jsonify({"error": f"Too many attempts. Try again in {ttl//60+1} minutes."})), 429

    otp = _generate_otp()
    PLAYER_REDIS.setex(_otp_key(player_id, channel), OTP_TTL, otp)
    pipe = PLAYER_REDIS.pipeline()
    pipe.incr(att_key)
    pipe.expire(att_key, 3600)
    pipe.execute()

    sent = False
    if channel == "phone":
        sent = _send_sms_otp(contact, otp)
    else:
        # Email — stub for now, log the OTP
        print(f"[OTP EMAIL] to={contact} otp={otp} (email sending not yet configured)")
        sent = True  # treat as sent, user won't get email yet

    if sent:
        return _cors(jsonify({"status": "sent", "channel": channel}))
    else:
        return _cors(jsonify({"error": "Failed to send OTP. Please try again."})), 500

@app.route("/otp/verify", methods=["POST", "OPTIONS"])
def otp_verify():
    if request.method == "OPTIONS":
        return _cors(jsonify({})), 204
    data      = request.get_json(force=True, silent=True) or request.form or {}
    player_id = (data.get("player_id") or "").strip()
    channel   = (data.get("channel") or "phone").strip()
    code      = (data.get("code") or "").strip()

    if not player_id or not code:
        return _cors(jsonify({"error": "player_id and code required"})), 400

    stored = PLAYER_REDIS.get(_otp_key(player_id, channel))
    if not stored:
        return _cors(jsonify({"error": "OTP expired or not found. Please request a new one."})), 400
    if stored != code:
        return _cors(jsonify({"error": "Incorrect code. Please try again."})), 400

    # OTP correct — mark as verified in DB
    field = "phone_verified" if channel == "phone" else "email_verified"
    ok = _update_player_verified(player_id, field)
    PLAYER_REDIS.delete(_otp_key(player_id, channel))
    PLAYER_REDIS.delete(_attempts_key(player_id, channel))

    if ok:
        return _cors(jsonify({"status": "verified", "channel": channel}))
    else:
        return _cors(jsonify({"error": "Verified but failed to update profile. Contact support."})), 500

@app.route("/otp/status/<player_id>", methods=["GET", "OPTIONS"])
def otp_status(player_id):
    if request.method == "OPTIONS":
        return _cors(jsonify({})), 204
    try:
        conn = psycopg2.connect(**PLAYER_DB, connect_timeout=3)
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
        cur.execute("SELECT phone_verified, email_verified FROM players WHERE id = %s", (player_id,))
        row = cur.fetchone()
        cur.close()
        conn.close()
        if not row:
            return _cors(jsonify({"error": "Player not found"})), 404
        return _cors(jsonify({"phone_verified": row["phone_verified"], "email_verified": row["email_verified"]}))
    except Exception as e:
        return _cors(jsonify({"error": str(e)})), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001)
