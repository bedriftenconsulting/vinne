#!/usr/bin/env python3
"""
SMS Retry Worker — CarPark Ed. 7
Runs every 2 minutes via systemd timer.
Finds completed payments where SMS was not sent (or failed) and retries.
"""

import time
import requests
import psycopg2
import psycopg2.extras
from datetime import datetime

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

TICKET_DB = {
    "host":     "localhost",
    "port":     5442,
    "dbname":   "ticket_service",
    "user":     "ticket",
    "password": "ticket123",
}

MNOTIFY_SMS_KEY = "F9XhjQbbJnqKt2fy9lhPIQCSD"
SMS_SENDER_ID   = "CARPARK"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def log(msg):
    ts = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


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
            log(f"[SMS] to={phone} status=success code={result.get('code')}")
            return True
        else:
            log(f"[SMS FAILED] to={phone} response={result}")
            return False
    except Exception as e:
        log(f"[SMS ERROR] to={msisdn} err={e}")
        return False


# ---------------------------------------------------------------------------
# Main retry logic
# ---------------------------------------------------------------------------

def process():
    try:
        conn = psycopg2.connect(**TICKET_DB, connect_timeout=5)
        cur  = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

        # All payment_refs that are completed but have at least one unsent ticket
        cur.execute("""
            SELECT DISTINCT payment_ref
            FROM tickets
            WHERE payment_status = 'completed'
              AND sms_sent = FALSE
            ORDER BY payment_ref
        """)
        refs = [r["payment_ref"] for r in cur.fetchall()]

        if not refs:
            log("No pending SMS — nothing to do.")
            cur.close()
            conn.close()
            return

        log(f"Found {len(refs)} ref(s) with unsent SMS: {refs}")

        for ref in refs:
            cur.execute("""
                SELECT serial_number, game_type, game_name, customer_phone
                FROM tickets
                WHERE payment_ref = %s
                ORDER BY game_type, game_name
            """, (ref,))
            rows = cur.fetchall()

            if not rows:
                log(f"ref={ref} — no tickets found, skipping")
                continue

            msisdn  = rows[0]["customer_phone"]
            access  = sorted(
                [r for r in rows if r["game_type"] == "ACCESS_PASS"],
                key=lambda r: r["game_name"]
            )
            entries = sorted(
                [r for r in rows if r["game_type"] == "DRAW_ENTRY"],
                key=lambda r: r["serial_number"]
            )

            log(f"ref={ref} phone={msisdn} access={len(access)} entries={len(entries)}")

            all_sent = True

            if access:
                # One SMS per access pass (matches Day 1 / Day 2 structure)
                for i, pass_row in enumerate(access):
                    valid_label  = pass_row["game_name"].split("—")[-1].strip()
                    entry_serial = entries[i]["serial_number"] if i < len(entries) else "—"
                    log(f"  Sending SMS {i+1}/{len(access)}: pass={pass_row['serial_number']} entry={entry_serial}")
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
                        time.sleep(1)
            else:
                # Extra WinBig entries only — chunk into 5 per SMS
                CHUNK = 5
                chunks = [entries[k:k+CHUNK] for k in range(0, len(entries), CHUNK)]
                for j, chunk in enumerate(chunks):
                    entries_text = "\n".join(r["serial_number"] for r in chunk)
                    log(f"  Sending entries SMS {j+1}/{len(chunks)} ({len(chunk)} entries)")
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
                # Use a regular cursor for the UPDATE
                upd = conn.cursor()
                upd.execute(
                    "UPDATE tickets SET sms_sent=TRUE, updated_at=NOW() WHERE payment_ref=%s",
                    (ref,)
                )
                conn.commit()
                upd.close()
                log(f"ref={ref} — all SMS sent, marked sms_sent=TRUE")
            else:
                log(f"ref={ref} — one or more SMS failed, will retry next run")

        cur.close()
        conn.close()

    except Exception as e:
        log(f"[ERROR] {e}")


if __name__ == "__main__":
    log("SMS retry worker starting")
    process()
    log("Done")
