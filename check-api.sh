#!/bin/bash
# Login and check game data
TOKEN=$(curl -sk -X POST https://api.winbig.bedriften.xyz/api/v1/admin/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@winbig.com","password":"admin123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

echo "Token: ${TOKEN:0:40}..."

echo ""
echo "=== Games from admin API ==="
curl -sk https://api.winbig.bedriften.xyz/api/v1/admin/games \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool 2>/dev/null | grep -E '"(name|prize_details|logo_url|id)"' | head -30

echo ""
echo "=== Player login test ==="
curl -sk -X POST https://api.winbig.bedriften.xyz/api/v1/players/login \
  -H 'Content-Type: application/json' \
  -d '{"phone":"0200000000","password":"test123"}' | head -c 300
