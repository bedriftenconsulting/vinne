# This script patches app.py to add player DB lookup
import re

with open('/home/suraj/ussd-app/app.py', 'r') as f:
    content = f.read()

# 1. Add PLAYER_DB config after TICKET_DB block
player_db_config = '''
PLAYER_DB = {
    host:     os.environ.get(PLAYER_DB_HOST, localhost),
    port:     5444,
    dbname:   player_service,
    user:     player,
    password: player123,
}

# Cache: phone -> player_id (avoid repeated DB lookups per session)
_player_id_cache = {}

def get_player_id_for_phone(phone):
    "Look up registered player UUID by phone number. Returns None if not found."
    normalised = phone.strip()
    if not normalised.startswith(+):
        normalised = + + normalised
    if normalised in _player_id_cache:
        return _player_id_cache[normalised]
    try:
        conn = psycopg2.connect(**PLAYER_DB, connect_timeout=2)
        cur  = conn.cursor()
        cur.execute(SELECT id FROM players WHERE phone_number = %s LIMIT 1, (normalised,))
        row = cur.fetchone()
        cur.close()
        conn.close()
        player_id = str(row[0]) if row else None
        _player_id_cache[normalised] = player_id
        if player_id:
            print(f[PLAYER LINK] phone={normalised} -> player_id={player_id})
        return player_id
    except Exception as e:
        print(f[PLAYER LOOKUP ERROR] {e})
        return None
'''

# Insert after TICKET_DB block
content = content.replace(
    '# In-memory session state: sequenceID -> list of user inputs',
    player_db_config + '\n# In-memory session state: sequenceID -> list of user inputs'
)

# 2. Update _insert_row to accept and use player_id
old_insert = '''def _insert_row(cur, conn, serial, game_type, game_name, unit_price, total_amount,     
                msisdn, phone, reference, bet_lines_data):
    "Insert a single ticket row, retrying on serial collision."
    for _ in range(5):
        try:
            security_hash = make_security_hash(serial, phone, total_amount, reference) 
            cur.execute("
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
 ", (
                serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, game_name, game_type,
                json.dumps(bet_lines_data), 1,
                unit_price, total_amount,
                USSD, msisdn,
                phone,
                mobile_money, reference, pending,
                security_hash, issued, DRAW_DATE,
            ))'''

new_insert = '''def _insert_row(cur, conn, serial, game_type, game_name, unit_price, total_amount,
                msisdn, phone, reference, bet_lines_data, player_id=None):
    "Insert a single ticket row, retrying on serial collision."
    # Use player UUID if registered, else fall back to msisdn
    effective_issuer_type = player if player_id else USSD
    effective_issuer_id   = player_id if player_id else msisdn
    for _ in range(5):
        try:
            security_hash = make_security_hash(serial, phone, total_amount, reference)
            cur.execute("
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
 ", (
                serial, GAME_CODE, GAME_SCHEDULE_ID,
                DRAW_NUMBER, game_name, game_type,
                json.dumps(bet_lines_data), 1,
                unit_price, total_amount,
                effective_issuer_type, effective_issuer_id,
                phone,
                mobile_money, reference, pending,
                security_hash, issued, DRAW_DATE,
            ))'''

content = content.replace(old_insert, new_insert)

with open('/home/suraj/ussd-app/app.py', 'w') as f:
    f.write(content)

print(Patch applied successfully)
