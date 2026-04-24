import re

with open('/home/suraj/ussd-app/app.py', 'r') as f:
    content = f.read()

# 1. Add PLAYER_DB config + lookup function after TICKET_DB block
player_db_block = '''
PLAYER_DB = {
    "host":     os.environ.get("PLAYER_DB_HOST", "localhost"),
    "port":     5444,
    "dbname":   "player_service",
    "user":     "player",
    "password": "player123",
}

# Cache: phone -> player_id (avoid repeated DB lookups per session)
_player_id_cache = {}

def get_player_id_for_phone(phone):
    """Look up registered player UUID by phone. Returns None if not found."""
    normalised = phone.strip()
    if not normalised.startswith("+"):
        normalised = "+" + normalised
    if normalised in _player_id_cache:
        return _player_id_cache[normalised]
    try:
        conn = psycopg2.connect(**PLAYER_DB, connect_timeout=2)
        cur  = conn.cursor()
        cur.execute("SELECT id FROM players WHERE phone_number = %s LIMIT 1", (normalised,))
        row = cur.fetchone()
        cur.close()
        conn.close()
        player_id = str(row[0]) if row else None
        _player_id_cache[normalised] = player_id
        if player_id:
            print(f"[PLAYER LINK] phone={normalised} -> player_id={player_id}")
        else:
            print(f"[PLAYER LINK] phone={normalised} -> no account found, using USSD")
        return player_id
    except Exception as e:
        print(f"[PLAYER LOOKUP ERROR] {e}")
        return None

'''

# Insert before the sessions dict
content = content.replace(
    '# In-memory session state: sequenceID -> list of user inputs\nsessions = {}',
    player_db_block + '# In-memory session state: sequenceID -> list of user inputs\nsessions = {}'
)

# 2. Update _insert_row signature to accept player_id and use it
old_sig = 'def _insert_row(cur, conn, serial, game_type, game_name, unit_price, total_amount,     \n                msisdn, phone, reference, bet_lines_data):'
new_sig = 'def _insert_row(cur, conn, serial, game_type, game_name, unit_price, total_amount,\n                msisdn, phone, reference, bet_lines_data, player_id=None):'

content = content.replace(old_sig, new_sig)

# 3. Replace the issuer fields in the INSERT
old_issuer = '                "USSD", msisdn,'
new_issuer = '''                "player" if player_id else "USSD", player_id if player_id else msisdn,'''

content = content.replace(old_issuer, new_issuer)

# 4. Update save_day_pass to look up player and pass to _insert_row
old_save_day = '    try:\n        conn = get_ticket_conn()\n        cur  = conn.cursor()\n\n        # Access pass row\n        _insert_row(\n            cur, conn,\n            serial      = access_serial,'
new_save_day = '    player_id = get_player_id_for_phone(msisdn)\n    try:\n        conn = get_ticket_conn()\n        cur  = conn.cursor()\n\n        # Access pass row\n        _insert_row(\n            cur, conn,\n            serial      = access_serial,'

content = content.replace(old_save_day, new_save_day)

# 5. Pass player_id into all _insert_row calls inside save_day_pass
# access pass call
content = content.replace(
    "            bet_lines_data = [{\"type\": \"ACCESS_PASS\", \"days\": ticket[\"days\"]}],        \n        )",
    "            bet_lines_data = [{\"type\": \"ACCESS_PASS\", \"days\": ticket[\"days\"]}],\n            player_id   = player_id,\n        )"
)
# draw entry call inside save_day_pass
content = content.replace(
    "                bet_lines_data = [{\"type\": \"DRAW_ENTRY\", \"source\": ticket[\"label\"]}],  \n            )",
    "                bet_lines_data = [{\"type\": \"DRAW_ENTRY\", \"source\": ticket[\"label\"]}],\n                player_id   = player_id,\n            )"
)

# 6. Update save_extra_entries similarly
old_save_extra = '    phone  = normalise_phone(msisdn)\n    serials = [generate_entry_serial() for _ in range(qty)]\n\n    try:\n        conn = get_ticket_conn()\n        cur  = conn.cursor()\n\n\n\n        for serial in serials:'
new_save_extra = '    phone  = normalise_phone(msisdn)\n    serials = [generate_entry_serial() for _ in range(qty)]\n    player_id = get_player_id_for_phone(msisdn)\n\n    try:\n        conn = get_ticket_conn()\n        cur  = conn.cursor()\n\n        for serial in serials:'

content = content.replace(old_save_extra, new_save_extra)

content = content.replace(
    "                bet_lines_data = [{\"type\": \"DRAW_ENTRY\", \"source\": \"Extra WinBig\"}],   \n            )",
    "                bet_lines_data = [{\"type\": \"DRAW_ENTRY\", \"source\": \"Extra WinBig\"}],\n                player_id   = player_id,\n            )"
)

with open('/home/suraj/ussd-app/app.py', 'w') as f:
    f.write(content)

print("Patch applied successfully")
