# Wins Module Updates

## Overview
Updated the Wins Module to focus on ticket-based structure with proper field organization as requested, removing stakes references and emphasizing total tickets instead.

## Key Changes Made

### 1. Updated Data Structure

#### UnpaidWin Interface
```typescript
export interface UnpaidWin {
  ticket_id: string
  ticket_number: string
  player_id?: string
  player_name?: string
  game_id: string
  game_name: string
  won_at: string
  winning_amount: number
  payment_status: 'pending' | 'processing' | 'failed'
  prize_delivery_status: 'not_delivered' | 'in_transit' | 'delivered'
  is_big_win: boolean
  approval_required: boolean
}
```

#### PaidWin Interface
```typescript
export interface PaidWin {
  ticket_id: string
  ticket_number: string
  player_id?: string
  player_name?: string
  game_id: string
  game_name: string
  won_at: string
  paid_at: string
  winning_amount: number
  payment_status: 'completed'
  prize_delivery_status: 'delivered' | 'collected'
  payout_method: 'wallet' | 'bank_transfer' | 'cash' | 'pos'
  transaction_id: string
  processed_by: string
}
```

### 2. Updated Fields Structure

#### Core Fields (as requested):
- ✅ **Ticket ID**: Primary identifier for each winning ticket
- ✅ **Player**: Player ID and name information
- ✅ **Game**: Game ID and name for context
- ✅ **Date**: Won date and paid date tracking
- ✅ **Payment Status**: Pending, processing, failed, completed
- ✅ **Prize Delivery Status**: Not delivered, in transit, delivered, collected

#### Additional Fields:
- **Winning Amount**: Prize amount in currency
- **Big Win Flag**: Identifies wins requiring manual approval
- **Transaction ID**: Unique identifier for payment transactions
- **Processed By**: Administrator who processed the payout
- **Payout Method**: Wallet, bank transfer, cash, POS

### 3. Updated Summary Cards

#### Removed Stakes Focus:
- ❌ Removed "Total Stakes" references
- ❌ Removed retailer-focused metrics

#### Added Ticket Focus:
- ✅ **Total Unpaid Wins**: Count of unpaid winning tickets
- ✅ **Total Paid Wins**: Count of paid winning tickets  
- ✅ **Big Wins Pending**: Count of big wins requiring approval
- ✅ **Total Winning Tickets**: Combined count of all winning tickets

### 4. Updated Table Structure

#### Unpaid Wins Table Columns:
1. Ticket ID
2. Player (Name/ID)
3. Game (Name/ID)
4. Date Won
5. Amount
6. Payment Status
7. Prize Delivery Status
8. Type (Big Win, Approval Required)
9. Actions

#### Paid Wins Table Columns:
1. Ticket ID
2. Player (Name/ID)
3. Game (Name/ID)
4. Date Won
5. Date Paid
6. Amount
7. Payment Status
8. Prize Delivery Status
9. Method
10. Transaction ID
11. Actions

### 5. Updated Filtering System

#### Filter Options:
- **Search**: Ticket number, player ID/name, game name
- **Game**: Filter by game name or ID
- **Player**: Filter by player ID or name (replaced retailer filter)
- **Big Win**: Filter by big win status
- **Date Range**: From/to date filtering for paid wins

### 6. Enhanced Detail Views

#### Unpaid Win Details:
- Ticket ID (monospace)
- Player information
- Game details
- Winning amount
- Won date/time
- Payment status badge
- Prize delivery status badge

#### Paid Win Details:
- All unpaid win fields plus:
- Paid date/time
- Processed by administrator
- Transaction ID (monospace)
- Payout method badge

### 7. Updated Service Methods

#### API Parameter Changes:
- `retailer_id` → `player_id`
- Added `player_name` support
- Added `game_name` support
- Enhanced prize delivery status tracking

#### WinsModule Interface:
```typescript
export interface WinsModule {
  unpaid_wins: UnpaidWin[]
  paid_wins: PaidWin[]
  total_unpaid_amount: number
  total_paid_amount: number
  total_unpaid_tickets: number    // New field
  total_paid_tickets: number     // New field
}
```

## Benefits of Changes

### 1. **Ticket-Centric Focus**
- Clear emphasis on individual winning tickets
- Better tracking of ticket lifecycle from win to payout
- Removed confusing stakes references

### 2. **Enhanced Player Experience**
- Player-focused instead of retailer-focused
- Clear prize delivery status tracking
- Better payment status visibility

### 3. **Improved Administration**
- Comprehensive game information display
- Enhanced filtering and search capabilities
- Better big win management workflow

### 4. **Compliance & Audit**
- Complete transaction tracking
- Prize delivery status monitoring
- Payment method documentation

## API Endpoints Required

```
GET  /admin/wins
GET  /admin/wins/unpaid?game_id&player_id&is_big_win
GET  /admin/wins/paid?game_id&player_id&from_date&to_date
POST /admin/wins/process-payout
POST /admin/wins/{ticketId}/approve-big-win
```

## Database Schema Updates

### wins_unpaid table:
- `ticket_id` (primary key)
- `ticket_number`
- `player_id`, `player_name`
- `game_id`, `game_name`
- `won_at`
- `winning_amount`
- `payment_status`
- `prize_delivery_status`
- `is_big_win`
- `approval_required`

### wins_paid table:
- `ticket_id` (primary key)
- `ticket_number`
- `player_id`, `player_name`
- `game_id`, `game_name`
- `won_at`, `paid_at`
- `winning_amount`
- `payment_status`
- `prize_delivery_status`
- `payout_method`
- `transaction_id`
- `processed_by`

This updated structure provides a cleaner, more focused approach to managing winning tickets with emphasis on the player experience and ticket lifecycle rather than stakes and retailer metrics.