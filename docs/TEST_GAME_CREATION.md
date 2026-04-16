# Test Game Creation - WinBig Competition

## Status: ✅ READY TO TEST

All backend changes have been applied successfully:

### ✅ Completed Steps

1. **Database Migration** - DONE
   - Added `prize_details` (TEXT)
   - Added `rules` (TEXT)
   - Added `total_tickets` (INTEGER)
   - Added `start_date` (DATE)
   - Added `end_date` (DATE)
   - Created indexes for date queries

2. **Go Service** - DONE
   - Updated models with new fields
   - Relaxed validation (lottery fields now optional)
   - Updated repository queries (INSERT/SELECT)
   - Service rebuilt and restarted

3. **Frontend** - DONE
   - Updated TypeScript interfaces
   - Wizard sends all competition fields

### 🧪 Test Instructions

1. **Open the Admin Web App**
   ```
   http://localhost:6176
   ```

2. **Navigate to Game Management**
   - Click on "Games" in the sidebar
   - Click "Create Game" button

3. **Fill in the 4-Step Wizard**

   **Step 1: Basic Info**
   - Game Name: `BMW Summer Jackpot`
   - Game Code: `BMW2026`
   - Description: `Win a brand new BMW 3 Series`
   - Status: `Draft`

   **Step 2: Dates & Tickets**
   - Start Date: `2026-05-01`
   - End Date: `2026-08-31`
   - Ticket Price: `10.00` (GHS)
   - Total Tickets: `5000`

   **Step 3: Prize & Rules**
   - Prize Details:
     ```
     1st Prize: BMW 3 Series (Worth GHS 250,000)
     2nd Prize: GHS 50,000 Cash
     3rd Prize: iPhone 15 Pro Max
     ```
   - Rules:
     ```
     1. One entry per ticket purchased
     2. Winner must claim prize within 90 days
     3. Open to Ghana residents only
     4. Draw will be held on September 1, 2026
     ```

   **Step 4: Logo & Review**
   - (Optional) Upload a logo
   - Review all details
   - Click "Create Game"

4. **Verify the Game Appears**
   - Game should appear in the games list immediately
   - Status should show as "Draft"
   - All fields should be populated

5. **Check the Database** (Optional)
   ```bash
   docker exec -it vinne-microservices-service-game-db-1 psql -U game -d game_service
   ```
   
   ```sql
   SELECT 
     name,
     code,
     status,
     start_date,
     end_date,
     total_tickets,
     base_price,
     LEFT(prize_details, 50) as prize_preview,
     LEFT(rules, 50) as rules_preview
   FROM games
   ORDER BY created_at DESC
   LIMIT 1;
   ```

### 🎯 Expected Results

- ✅ Game creates without errors
- ✅ Appears in games list immediately
- ✅ All fields saved correctly:
  - `prize_details` contains the prize description
  - `rules` contains the competition rules
  - `total_tickets` = 5000
  - `start_date` = 2026-05-01
  - `end_date` = 2026-08-31
  - `base_price` = 10.00
- ✅ Status shows as "Draft"
- ✅ Can edit the game
- ✅ Can activate the game (changes status to "Active")

### 🐛 Troubleshooting

**If game doesn't appear:**
1. Check browser console (F12) for errors
2. Check API Gateway logs: `docker logs vinne-microservices-api-gateway-1`
3. Check Game Service logs: `docker logs vinne-microservices-service-game-1`

**If fields are null:**
- The migration may not have run. Check with:
  ```bash
  docker exec vinne-microservices-service-game-db-1 psql -U game -d game_service -c "\d games"
  ```

**If validation errors occur:**
- Check that the wizard is sending all required fields
- Check browser Network tab (F12) to see the request payload

### 📊 Database Connection Info

If you need to connect directly:
- Host: `localhost`
- Port: `5441`
- Database: `game_service`
- User: `game`
- Password: `game123`

Connection string:
```
postgresql://game:game123@localhost:5441/game_service?sslmode=disable
```

---

## Summary

The backend is now fully configured to support WinBig Africa's competition model. The wizard will create games with:
- Competition-specific fields (prize details, rules, total tickets)
- Date ranges (start/end dates)
- Flexible validation (no lottery-specific requirements)
- Real-time persistence to the database

**You can now create your first competition game!** 🎉
