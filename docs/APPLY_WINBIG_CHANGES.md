# Apply WinBig Competition Changes

## Quick Start

The backend has been updated to support WinBig Africa's competition model. Follow these steps to apply the changes:

### 1. Run the Database Migration

```bash
cd vinne-microservices/services/service-game

# Find your database connection string from your .env or config
# Example format: postgresql://user:password@localhost:5432/game_db

# Run the migration
goose -dir migrations postgres "YOUR_CONNECTION_STRING_HERE" up
```

**Example with default local settings:**
```bash
goose -dir migrations postgres "postgresql://game_user:game_pass@localhost:5433/game_db" up
```

### 2. Rebuild the Game Service

```bash
cd vinne-microservices/services/service-game

# Build the service
go build -o bin/service-game ./cmd/server/main.go
```

### 3. Restart the Service

**If running locally:**
```bash
# Stop the current service (Ctrl+C if running in terminal)
# Then start it again:
./bin/service-game
```

**If using Docker:**
```bash
cd vinne-microservices
docker-compose restart service-game
```

**If using Kubernetes:**
```bash
kubectl rollout restart deployment/service-game -n microservices
```

### 4. Verify the Changes

1. Open the admin web app at `http://localhost:6176` (or your configured URL)
2. Navigate to **Game Management**
3. Click **Create Game**
4. Fill in the 4-step wizard:
   - Step 1: Basic Info (name, code, description, status)
   - Step 2: Dates & Tickets (start/end dates, ticket price, total tickets)
   - Step 3: Prize & Rules (prize details, rules)
   - Step 4: Logo & Review
5. Click **Create Game**
6. The game should appear in the games list immediately

### 5. Check the Database

Verify the new fields were saved:

```sql
SELECT 
  id, 
  name, 
  code, 
  status,
  start_date,
  end_date,
  total_tickets,
  base_price,
  prize_details,
  rules
FROM games
ORDER BY created_at DESC
LIMIT 1;
```

## Troubleshooting

### Migration Fails

**Error: "relation already exists"**
- The migration may have already run. Check with:
  ```bash
  goose -dir migrations postgres "YOUR_CONNECTION_STRING" status
  ```

**Error: "connection refused"**
- Verify your database is running:
  ```bash
  docker ps | grep postgres
  ```
- Check your connection string matches your database config

### Service Won't Start

**Error: "address already in use"**
- Another instance is running. Find and kill it:
  ```bash
  lsof -i :51053  # Game service gRPC port
  kill -9 <PID>
  ```

**Error: "cannot connect to database"**
- Verify database connection in your service config
- Check `services/service-game/config.yaml` or environment variables

### Game Not Appearing in List

1. **Check browser console** for API errors (F12 → Console tab)
2. **Check API Gateway logs** - should show the POST request
3. **Check Game Service logs** - should show game creation
4. **Verify API Gateway is routing to Game Service**:
   ```bash
   curl http://localhost:4000/api/v1/admin/games
   ```

### Fields Not Saving

**Symptom:** Game creates but prize_details/rules/total_tickets are null

**Solution:** The migration didn't run. Re-run step 1 above.

## What Changed

### Database
- Added 5 new columns to `games` table:
  - `prize_details` (TEXT)
  - `rules` (TEXT)
  - `total_tickets` (INTEGER)
  - `start_date` (DATE)
  - `end_date` (DATE)

### Backend (Go)
- Relaxed validation to make lottery-specific fields optional
- Added defaults for required fields (organizer, game_format, etc.)
- Updated all repository queries to include new fields

### Frontend (TypeScript)
- Updated interfaces to include new fields
- Wizard now sends competition-specific data

## Need Help?

Check the detailed documentation:
- `vinne-microservices/services/service-game/WINBIG_COMPETITION_UPDATE.md`
- `vinne-microservices/README.md` for service architecture
