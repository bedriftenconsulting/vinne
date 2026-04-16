# Game Winner Configuration Implementation

## Overview
Implemented a comprehensive game configuration system that allows setting the number of winners and prize structure for each position, integrated with Google RNG for transparent winner selection.

## Key Features Implemented

### 1. Game Configuration System (`GameConfiguration.tsx`)

#### Core Configuration Options:
- **Game Details**: Name, description, type (instant win, draw-based, raffle)
- **Ticket Settings**: Price, max tickets per player
- **Winner Configuration**: Total number of winners (1-100)
- **Prize Structure**: Individual prize amounts for each winner position
- **Selection Method**: Google RNG or Cryptographic RNG
- **Payout Settings**: Auto-payout threshold and big win limits

#### Winner Position Management:
- **Dynamic Prize Structure**: Automatically creates prize positions based on winner count
- **Position Names**: Customizable names (1st Place, 2nd Place, etc.)
- **Prize Amounts**: Individual prize configuration for each position
- **Big Win Detection**: Automatic flagging of prizes above threshold
- **Prize Pool Summary**: Real-time calculation of total prize pool

### 2. Updated Draw Dashboard

#### Changed "Total Stakes" to "Total Tickets":
```typescript
// Before: Total Stakes (GHS amount)
// After: Total Tickets (count of tickets)
<CardTitle className="text-sm font-medium">Total Tickets</CardTitle>
<div className="text-2xl font-bold">
  {totalTickets.toLocaleString()}
</div>
<p className="text-xs text-muted-foreground">
  Total ticket entries
</p>
```

### 3. Enhanced Winner Selection System

#### Updated Winner Selection Result:
```typescript
export interface WinnerSelectionResult {
  draw_id: string
  selection_method: string
  random_seed: string
  google_request_id?: string
  selected_winners: SelectedWinner[]  // New: Position-based winners
  audit_log: AuditLogEntry[]
  selection_timestamp: string
  cryptographic_proof?: string
}

export interface SelectedWinner {
  ticket_id: string
  ticket_number: string
  player_id?: string
  position: number          // Winner position (1st, 2nd, 3rd, etc.)
  position_name: string     // Display name for position
  prize_amount: number      // Prize amount for this position
  selection_rank: number    // Order selected by RNG
  is_big_win: boolean      // Requires manual approval
}
```

### 4. Game Configuration Interface

#### Prize Structure Configuration:
- **Position-Based Prizes**: Each winner position has its own prize amount
- **Automatic Position Generation**: Creates positions based on winner count
- **Prize Validation**: Ensures prize structure matches winner count
- **Big Win Alerts**: Visual indicators for prizes requiring approval

#### Configuration Preview:
- **Winner Count**: Visual display of total winners
- **Prize Pool**: Sum of all position prizes
- **Ticket Price**: Individual ticket cost
- **Max per Player**: Purchase limits

### 5. Winner Selection Process Flow

#### Game Creation:
1. **Set Winner Count**: Configure how many winners (1-100)
2. **Configure Prizes**: Set prize amount for each position
3. **Choose Selection Method**: Google RNG or Cryptographic RNG
4. **Set Thresholds**: Big win threshold for manual approval

#### Draw Execution:
1. **Pre-Draw Email**: Send ticket manifest to administrators
2. **Google RNG Selection**: Select N winners based on game configuration
3. **Position Assignment**: Assign winners to positions (1st, 2nd, 3rd, etc.)
4. **Prize Calculation**: Apply position-specific prize amounts
5. **Payout Processing**: Process payouts based on position prizes

### 6. User Experience Enhancements

#### Game Configuration UX:
- **Real-time Updates**: Prize pool updates as positions are configured
- **Validation Feedback**: Immediate validation of configuration
- **Visual Indicators**: Clear display of big wins and thresholds
- **Position Management**: Easy addition/removal of winner positions

#### Draw Management UX:
- **Winner Position Display**: Clear visualization of winner positions
- **Prize Amount Display**: Show prize for each position
- **Selection Status**: Real-time status of winner selection process

## Technical Implementation

### 7. API Endpoints Required

```typescript
// Game Configuration
POST /admin/games                    // Create game with winner config
PUT  /admin/games/{id}              // Update game configuration
GET  /admin/games/{id}/config       // Get game configuration

// Winner Selection
POST /admin/draws/{id}/execute-winner-selection
GET  /admin/draws/{id}/winners      // Get selected winners with positions
POST /admin/draws/{id}/assign-positions  // Assign prizes to positions
```

### 8. Database Schema Updates

#### games table:
```sql
ALTER TABLE games ADD COLUMN total_winners INTEGER DEFAULT 1;
ALTER TABLE games ADD COLUMN winner_selection_method VARCHAR(50) DEFAULT 'google_rng';
ALTER TABLE games ADD COLUMN auto_payout_enabled BOOLEAN DEFAULT true;
ALTER TABLE games ADD COLUMN big_win_threshold INTEGER DEFAULT 10000;
```

#### game_prize_structure table:
```sql
CREATE TABLE game_prize_structure (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  position INTEGER NOT NULL,
  position_name VARCHAR(100) NOT NULL,
  prize_amount INTEGER NOT NULL,
  prize_type VARCHAR(20) DEFAULT 'fixed',
  created_at TIMESTAMP DEFAULT NOW()
);
```

#### draw_winners table:
```sql
CREATE TABLE draw_winners (
  id UUID PRIMARY KEY,
  draw_id UUID REFERENCES draws(id),
  ticket_id UUID REFERENCES tickets(id),
  position INTEGER NOT NULL,
  position_name VARCHAR(100) NOT NULL,
  prize_amount INTEGER NOT NULL,
  selection_rank INTEGER NOT NULL,
  is_big_win BOOLEAN DEFAULT false,
  selected_at TIMESTAMP DEFAULT NOW()
);
```

## Benefits

### 1. **Flexible Prize Structure**
- Support for multiple winners per game
- Customizable prize amounts for each position
- Easy configuration of winner hierarchy

### 2. **Transparent Selection Process**
- Google RNG integration for verifiable randomness
- Position-based winner assignment
- Complete audit trail for compliance

### 3. **Enhanced User Experience**
- Clear visualization of winner positions
- Real-time prize pool calculations
- Intuitive game configuration interface

### 4. **Compliance & Audit**
- Pre-draw email notifications with ticket manifests
- Cryptographic proof of winner selection
- Position-based prize tracking for regulatory compliance

## Example Game Configuration

```typescript
const exampleGame: GameConfig = {
  name: "BMW Car Raffle",
  description: "Win a brand new BMW or cash prizes",
  game_type: "raffle",
  ticket_price: 500, // 5 GHS
  max_tickets_per_player: 20,
  total_winners: 3,
  prize_structure: [
    {
      position: 1,
      position_name: "Grand Prize - BMW Car",
      prize_amount: 5000000, // 50,000 GHS
      prize_type: "fixed"
    },
    {
      position: 2,
      position_name: "Second Prize - Cash",
      prize_amount: 1000000, // 10,000 GHS
      prize_type: "fixed"
    },
    {
      position: 3,
      position_name: "Third Prize - Cash",
      prize_amount: 500000, // 5,000 GHS
      prize_type: "fixed"
    }
  ],
  winner_selection_method: "google_rng",
  auto_payout_enabled: false, // Manual approval for all prizes
  big_win_threshold: 100000, // 1,000 GHS
  status: "active"
}
```

This implementation provides a complete solution for configuring games with multiple winners, position-based prizes, and transparent winner selection using Google's RNG service.