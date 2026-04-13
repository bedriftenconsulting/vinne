#!/bin/bash

# ============================================================================
# RAND LOTTERY - TEST DATA POPULATION SCRIPT
# Purpose: Populate test data across all services for draw system testing
# Date: 2025-10-11
# ============================================================================
#
# RECOMMENDED WINNING NUMBERS FOR UI TESTING:
# ============================================
# Use these 5 numbers when entering winning numbers in the UI: 11, 25, 39, 59, 81
#
# Expected Winners by Game (using winning numbers: 11, 25, 39, 59, 81):
#
# VAGMON (Game 1):
#   - Ticket #1: DIRECT_1 [11] - WINNER! (1 match) - Stake: ₵1, Win: ₵40
#   - Ticket #2: PERM_2 [18,31] - no match
#   - Ticket #3: PERM_3 [25,38,51] - WINNER! (1 match: 25) - need 3 matches for perm_3
#   - Ticket #4: BANKER_1 [32,45,58,71,84] - no match
#   - Ticket #12: BANKER_1 [88,11,24,37,50] - WINNER! (1 match: 11) - need banker match
#   - Ticket #14: PERM_2 [12,25] - WINNER! (1 match: 25) - need 2 matches for perm_2
#   - Ticket #15: PERM_3 [19,32,45] - no match
#
# VAGTUE (Game 2):
#   - Ticket #14: PERM_2 [2,15] - no match
#   - Ticket #15: PERM_3 [9,22,35] - no match
#
# VAGWED (Game 3):
#   - Ticket #2: PERM_2 [88,11] - WINNER! (1 match: 11) - need 2 matches
#   - Ticket #3: PERM_3 [5,18,31] - no match
#   - Ticket #4: BANKER_1 [12,25,38,51,64] - WINNER! (1 match: 25)
#   - Ticket #14: PERM_2 [82,5] - no match
#   - Ticket #15: PERM_3 [89,12,25] - WINNER! (1 match: 25) - need 3 matches
#
# VAGTHU (Game 4):
#   - Ticket #15: PERM_3 [79,2,15] - no match
#
# VAGFRI (Game 5):
#   - Ticket #3: PERM_3 [75,88,11] - WINNER! (1 match: 11) - need 3 matches
#   - Ticket #6: PERM_2 [6,19] - no match
#   - Ticket #7: PERM_3 [13,26,39] - WINNER! (1 match: 39) - need 3 matches
#   - Ticket #15: PERM_3 [69,82,5] - no match
#
# VAGSAT (Game 6):
#   - Ticket #15: PERM_3 [59,72,85] - WINNER! (1 match: 59) - need 3 matches
#
# VAGSUN (Game 7):
#   - Ticket #3: PERM_3 [55,68,81] - WINNER! (1 match: 81) - need 3 matches
#   - Ticket #4: BANKER_1 [62,75,88,11,24] - WINNER! (1 match: 11)
#   - Ticket #14: PERM_2 [42,55] - no match
#   - Ticket #15: PERM_3 [49,62,75] - no match
#
# VAGSPECIAL (Game 8):
#   - Ticket #2: PERM_2 [38,51] - no match
#   - Ticket #3: PERM_3 [45,58,71] - no match
#   - Ticket #10: PERM_2 [4,17] - no match
#   - Ticket #11: PERM_3 [11,24,37] - WINNER! (1 match: 11) - need 3 matches
#   - Ticket #14: PERM_2 [32,45] - no match
#   - Ticket #15: PERM_3 [39,52,65] - WINNER! (1 match: 39) - need 3 matches
#
# NOTE: Most tickets above show only 1 match. For actual wins:
#   - DIRECT_1: needs 1 match (multiplier 40x)
#   - PERM_2: needs 2 matches (multiplier 240x)
#   - PERM_3: needs 3 matches (multiplier 1920x)
#   - BANKER_1: needs banker + 2-banker matches (multiplier 240x)
#
# With winning numbers [11, 25, 39, 59, 81], you'll see:
#   - Several DIRECT_1 winners (ticket #1 in VAGMON: number 11)
#   - Limited PERM_2/PERM_3 winners (need multiple matches)
#   - System will calculate exact winnings at end of Stage 2
#
# ============================================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Docker container names
AGENT_MGMT_CONTAINER="randco-microservices-service-agent-management-db-1"
WALLET_CONTAINER="randco-microservices-service-wallet-db-1"
GAME_CONTAINER="randco-microservices-service-game-db-1"
DRAW_CONTAINER="randco-microservices-service-draw-db-1"
TICKET_CONTAINER="randco-microservices-service-ticket-db-1"

# Function to print colored output
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Function to check if PostgreSQL is running
check_postgres() {
    local container=$1

    if ! docker exec "$container" pg_isready -U postgres > /dev/null 2>&1; then
        return 1
    fi
    return 0
}

# Function to cleanup existing test data
cleanup_test_data() {
    print_info "Cleaning up existing test data..."

    # Ticket Service - DELETE FIRST (has FK to draws)
    docker exec $TICKET_CONTAINER psql -U ticket -d ticket_service -c \
        "DELETE FROM tickets WHERE serial_number LIKE 'TKT-%';" 2>/dev/null || true

    # Draw Service - DELETE SECOND (has FK to games)
    docker exec $DRAW_CONTAINER psql -U draw_service -d draw_service -c \
        "DELETE FROM draws WHERE id::text LIKE 'dddddddd-dddd-dddd-dddd-%';
         DELETE FROM draw_schedules WHERE id IN (
            '11111111-8888-8888-8888-111111111111',
            '22222222-8888-8888-8888-222222222222',
            '33333333-8888-8888-8888-333333333333',
            '44444444-8888-8888-8888-444444444444',
            '55555555-8888-8888-8888-555555555555',
            '66666666-8888-8888-8888-666666666666',
            '77777777-8888-8888-8888-777777777777',
            '88888888-8888-8888-8888-888888888888'
         );" 2>/dev/null || true

    # Game Service - DELETE THIRD (has FK constraints)
    docker exec $GAME_CONTAINER psql -U game -d game_service -c \
        "DELETE FROM game_schedules WHERE id IN (
            '11111111-8888-8888-8888-111111111111',
            '22222222-8888-8888-8888-222222222222',
            '33333333-8888-8888-8888-333333333333',
            '44444444-8888-8888-8888-444444444444',
            '55555555-8888-8888-8888-555555555555',
            '66666666-8888-8888-8888-666666666666',
            '77777777-8888-8888-8888-777777777777',
            '88888888-8888-8888-8888-888888888888'
         );
         DELETE FROM game_bet_types WHERE game_id::text LIKE '%-aaaa-bbbb-cccc-%';
         DELETE FROM prize_tiers WHERE prize_structure_id::text LIKE '%-5555-5555-5555-%';
         DELETE FROM prize_structures WHERE id::text LIKE '%-5555-5555-5555-%';
         DELETE FROM game_rules WHERE id::text LIKE '%-4444-4444-4444-%';
         DELETE FROM games WHERE id::text LIKE '%-aaaa-bbbb-cccc-%';" 2>/dev/null || true

    # Wallet Service
    docker exec $WALLET_CONTAINER psql -U wallet -d wallet_service -c \
        "DELETE FROM retailer_winning_wallets WHERE retailer_id::text LIKE '%111111%';
         DELETE FROM retailer_stake_wallets WHERE retailer_id::text LIKE '%111111%' OR retailer_id::text LIKE '%222222%';" 2>/dev/null || true

    # Agent Management Service
    docker exec $AGENT_MGMT_CONTAINER psql -U agent_mgmt -d agent_management -c \
        "DELETE FROM pos_devices WHERE device_code LIKE 'POS-TEST-%';
         DELETE FROM agent_retailers WHERE agent_id = 'a1111111-1111-1111-1111-111111111111';
         DELETE FROM retailers WHERE retailer_code LIKE '9001%';
         DELETE FROM agents WHERE agent_code = '9001';" 2>/dev/null || true

    print_success "Cleanup completed"
}

# Main script
echo "============================================================================"
echo "  RAND LOTTERY - TEST DATA POPULATION"
echo "============================================================================"
echo ""

# Check command line arguments
CLEANUP_MODE=false
if [[ "$1" == "--cleanup" ]]; then
    CLEANUP_MODE=true
    print_info "Running in cleanup mode"
fi

# Check if all databases are running
print_info "Checking database connectivity..."

all_running=true

# Check each database container
if check_postgres "$AGENT_MGMT_CONTAINER"; then
    print_success "Agent Management database is running (port 5435)"
else
    print_error "Agent Management database is NOT running (port 5435)"
    all_running=false
fi

if check_postgres "$WALLET_CONTAINER"; then
    print_success "Wallet Service database is running (port 5438)"
else
    print_error "Wallet Service database is NOT running (port 5438)"
    all_running=false
fi

if check_postgres "$GAME_CONTAINER"; then
    print_success "Game Service database is running (port 5441)"
else
    print_error "Game Service database is NOT running (port 5441)"
    all_running=false
fi

if check_postgres "$DRAW_CONTAINER"; then
    print_success "Draw Service database is running (port 5436)"
else
    print_error "Draw Service database is NOT running (port 5436)"
    all_running=false
fi

if check_postgres "$TICKET_CONTAINER"; then
    print_success "Ticket Service database is running (port 5442)"
else
    print_error "Ticket Service database is NOT running (port 5442)"
    all_running=false
fi

if [ "$all_running" = false ]; then
    echo ""
    print_error "Not all databases are running. Please start infrastructure first:"
    echo "  cd randco-microservices"
    echo "  ./scripts/start-infrastructure.sh"
    exit 1
fi

echo ""

# Cleanup if requested
if [ "$CLEANUP_MODE" = true ]; then
    cleanup_test_data
    echo ""
fi

# Execute the SQL script in sections
print_info "Executing test data population script..."
echo ""

print_info "Step 1: Populating Agent Management Service..."

# STEP 1: Agent Management
docker exec -i $AGENT_MGMT_CONTAINER psql -U agent_mgmt -d agent_management << 'EOF'
-- Create test agent
INSERT INTO agents (
    id, agent_code, business_name, registration_number, tax_id,
    contact_email, contact_phone, primary_contact_name,
    physical_address, city, region,
    bank_name, bank_account_number, bank_account_name,
    status, commission_percentage, onboarding_method,
    created_by, updated_by
) VALUES (
    'a1111111-1111-1111-1111-111111111111',
    '9001',
    'Test Draw Agent',
    'REG-9001',
    'TAX-9001',
    'agent9001@test.com',
    '+233241234567',
    'Test Agent Contact',
    'Test Agent Office, Accra',
    'Accra',
    'Greater Accra',
    'GCB Bank',
    '999000000009001',
    'Test Draw Agent',
    'ACTIVE',
    30.00,
    'RAND_LOTTERY_LTD_DIRECT',
    'system',
    'system'
) ON CONFLICT (id) DO UPDATE SET
    agent_code = EXCLUDED.agent_code,
    business_name = EXCLUDED.business_name,
    registration_number = EXCLUDED.registration_number,
    tax_id = EXCLUDED.tax_id,
    contact_email = EXCLUDED.contact_email,
    contact_phone = EXCLUDED.contact_phone,
    primary_contact_name = EXCLUDED.primary_contact_name,
    physical_address = EXCLUDED.physical_address,
    city = EXCLUDED.city,
    region = EXCLUDED.region,
    bank_name = EXCLUDED.bank_name,
    bank_account_number = EXCLUDED.bank_account_number,
    bank_account_name = EXCLUDED.bank_account_name,
    status = EXCLUDED.status,
    commission_percentage = EXCLUDED.commission_percentage,
    onboarding_method = EXCLUDED.onboarding_method,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW();

-- Create test retailers (5 retailers)
INSERT INTO retailers (
    id, retailer_code, business_name, owner_name,
    contact_email, contact_phone,
    physical_address, city, region, gps_coordinates,
    business_license, shop_type,
    status, onboarding_method, parent_agent_id,
    created_by, updated_by
) VALUES
    ('11111111-1111-1111-1111-111111111111', '90010001', 'Test Shop 1', 'John Doe',
     'shop1@test.com', '+233241234568',
     'Test Shop 1 Location, Accra', 'Accra', 'Greater Accra', '5.6037,-0.1870',
     'LIC-90010001', 'PHYSICAL_STORE',
     'ACTIVE', 'AGENT_ONBOARDED', 'a1111111-1111-1111-1111-111111111111',
     'system', 'system'),
    ('22222222-2222-2222-2222-222222222222', '90010002', 'Test Shop 2', 'Jane Smith',
     'shop2@test.com', '+233241234569',
     'Test Shop 2 Location, Accra', 'Accra', 'Greater Accra', '5.6040,-0.1875',
     'LIC-90010002', 'PHYSICAL_STORE',
     'ACTIVE', 'AGENT_ONBOARDED', 'a1111111-1111-1111-1111-111111111111',
     'system', 'system'),
    ('33333331-3333-3333-3333-333333333331', '90010003', 'Test Shop 3', 'Kwame Mensah',
     'shop3@test.com', '+233241234570',
     'Test Shop 3 Location, Kumasi', 'Kumasi', 'Ashanti', '6.6885,-1.6244',
     'LIC-90010003', 'PHYSICAL_STORE',
     'ACTIVE', 'AGENT_ONBOARDED', 'a1111111-1111-1111-1111-111111111111',
     'system', 'system'),
    ('44444441-4444-4444-4444-444444444441', '90010004', 'Test Shop 4', 'Ama Agyei',
     'shop4@test.com', '+233241234571',
     'Test Shop 4 Location, Accra', 'Accra', 'Greater Accra', '5.6050,-0.1880',
     'LIC-90010004', 'PHYSICAL_STORE',
     'ACTIVE', 'AGENT_ONBOARDED', 'a1111111-1111-1111-1111-111111111111',
     'system', 'system'),
    ('55555551-5555-5555-5555-555555555551', '90010005', 'Test Shop 5', 'Kofi Asante',
     'shop5@test.com', '+233241234572',
     'Test Shop 5 Location, Takoradi', 'Takoradi', 'Western', '4.8970,-1.7533',
     'LIC-90010005', 'PHYSICAL_STORE',
     'ACTIVE', 'AGENT_ONBOARDED', 'a1111111-1111-1111-1111-111111111111',
     'system', 'system')
ON CONFLICT (id) DO UPDATE SET
    retailer_code = EXCLUDED.retailer_code,
    business_name = EXCLUDED.business_name,
    owner_name = EXCLUDED.owner_name,
    contact_email = EXCLUDED.contact_email,
    contact_phone = EXCLUDED.contact_phone,
    physical_address = EXCLUDED.physical_address,
    city = EXCLUDED.city,
    region = EXCLUDED.region,
    gps_coordinates = EXCLUDED.gps_coordinates,
    business_license = EXCLUDED.business_license,
    shop_type = EXCLUDED.shop_type,
    status = EXCLUDED.status,
    onboarding_method = EXCLUDED.onboarding_method,
    parent_agent_id = EXCLUDED.parent_agent_id,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW();

-- Create agent-retailer relationships
INSERT INTO agent_retailers (agent_id, retailer_id, relationship_type, is_active, assigned_by)
SELECT 'a1111111-1111-1111-1111-111111111111'::uuid, r.id, 'MANAGED', true, 'system'
FROM retailers r WHERE r.retailer_code LIKE '9001%'
ON CONFLICT ON CONSTRAINT agent_retailers_agent_id_retailer_id_is_active_key DO NOTHING;

-- Create POS devices
INSERT INTO pos_devices (
    device_code, imei, model, manufacturer, assigned_retailer_id, status, assigned_by, created_by
) VALUES
    ('POS-TEST-000001', 'IMEI-TEST-001', 'Sunmi T2', 'Sunmi', '11111111-1111-1111-1111-111111111111', 'ACTIVE', 'system', 'system'),
    ('POS-TEST-000002', 'IMEI-TEST-002', 'Sunmi T2', 'Sunmi', '22222222-2222-2222-2222-222222222222', 'ACTIVE', 'system', 'system'),
    ('POS-TEST-000003', 'IMEI-TEST-003', 'Sunmi T2', 'Sunmi', '33333331-3333-3333-3333-333333333331', 'ACTIVE', 'system', 'system'),
    ('POS-TEST-000004', 'IMEI-TEST-004', 'Sunmi T2', 'Sunmi', '44444441-4444-4444-4444-444444444441', 'ACTIVE', 'system', 'system'),
    ('POS-TEST-000005', 'IMEI-TEST-005', 'Sunmi T2', 'Sunmi', '55555551-5555-5555-5555-555555555551', 'ACTIVE', 'system', 'system')
ON CONFLICT (device_code) DO UPDATE SET status = 'ACTIVE', updated_at = NOW();
EOF

if [ $? -eq 0 ]; then
    print_success "Agent Management data populated (1 agent, 5 retailers, 5 POS devices)"
else
    print_error "Failed to populate Agent Management data"
    exit 1
fi

print_info "Step 2: Populating Wallet Service..."

# STEP 2: Wallet Service
docker exec -i $WALLET_CONTAINER psql -U wallet -d wallet_service << 'EOF'
-- Create retailer stake wallets
INSERT INTO retailer_stake_wallets (retailer_id, balance, pending_balance, available_balance, currency, status) VALUES
    ('11111111-1111-1111-1111-111111111111', 5000000, 0, 5000000, 'GHS', 'ACTIVE'),
    ('22222222-2222-2222-2222-222222222222', 3000000, 0, 3000000, 'GHS', 'ACTIVE'),
    ('33333331-3333-3333-3333-333333333331', 4000000, 0, 4000000, 'GHS', 'ACTIVE'),
    ('44444441-4444-4444-4444-444444444441', 2000000, 0, 2000000, 'GHS', 'ACTIVE'),
    ('55555551-5555-5555-5555-555555555551', 6000000, 0, 6000000, 'GHS', 'ACTIVE')
ON CONFLICT (retailer_id) DO UPDATE SET balance = EXCLUDED.balance, available_balance = EXCLUDED.available_balance, status = 'ACTIVE', updated_at = NOW();

-- Create retailer winning wallets
INSERT INTO retailer_winning_wallets (retailer_id, balance, pending_balance, available_balance, currency, status) VALUES
    ('11111111-1111-1111-1111-111111111111', 0, 0, 0, 'GHS', 'ACTIVE'),
    ('22222222-2222-2222-2222-222222222222', 0, 0, 0, 'GHS', 'ACTIVE'),
    ('33333331-3333-3333-3333-333333333331', 0, 0, 0, 'GHS', 'ACTIVE'),
    ('44444441-4444-4444-4444-444444444441', 0, 0, 0, 'GHS', 'ACTIVE'),
    ('55555551-5555-5555-5555-555555555551', 0, 0, 0, 'GHS', 'ACTIVE')
ON CONFLICT (retailer_id) DO UPDATE SET status = 'ACTIVE', updated_at = NOW();
EOF

if [ $? -eq 0 ]; then
    print_success "Wallet data populated (5 stake wallets ₵200K total, 5 winning wallets)"
else
    print_error "Failed to populate Wallet data"
    exit 1
fi

print_info "Step 3: Populating Game Service..."

# STEP 3: Game Service
docker exec -i $GAME_CONTAINER psql -U game -d game_service << 'EOF'
-- Create games (8 different 5/90 games)

-- 1. VAG MONDAY (Morning 10:00am)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '11111111-aaaa-bbbb-cccc-111111111111',
    'VAGMON',
    'VAG Monday',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'VAG Monday - Morning Draw',
    'WEEKLY',
    '10:00',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 2. THURSDAY NOONRUSH (Afternoon 1:30pm)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '22222222-aaaa-bbbb-cccc-222222222222',
    'THUNR',
    'Thursday Noonrush',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Thursday Noonrush - Afternoon Draw',
    'WEEKLY',
    '13:30',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 3. MONDAY SPECIAL (Evening 7:15pm)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '33333333-aaaa-bbbb-cccc-333333333333',
    'MONSPC',
    'Monday Special',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Monday Special - Evening Draw',
    'WEEKLY',
    '19:15',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 4. FRIDAY BONANZA (Evening 7:15pm)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '44444444-aaaa-bbbb-cccc-444444444444',
    'FRIBON',
    'Friday Bonanza',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Friday Bonanza - Evening Draw',
    'WEEKLY',
    '19:15',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 5. NATIONAL SATURDAY (Evening 7:15pm)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '55555555-aaaa-bbbb-cccc-555555555555',
    'NATSAT',
    'National Saturday',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'National Saturday - Evening Draw',
    'WEEKLY',
    '19:15',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 6. ASEDA SUNDAY (Evening 7:15pm)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '66666666-aaaa-bbbb-cccc-666666666666',
    'ASEDSUN',
    'Aseda Sunday',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Aseda Sunday - Evening Draw',
    'WEEKLY',
    '19:15',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 7. BINGO4 MONDAY (Early Morning 6:50am)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '77777777-aaaa-bbbb-cccc-777777777777',
    'BING4',
    'Bingo4 Monday',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Bingo4 Monday - Early Morning Draw',
    'WEEKLY',
    '06:50',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- 8. SIKA KESE FRIDAY (Early Morning 6:50am)
INSERT INTO games (
    id, code, name, type, game_type, game_format, game_category, organizer,
    min_stake_amount, max_stake_amount, base_price, status,
    description, draw_frequency, draw_time_str, sales_cutoff_minutes,
    number_range_min, number_range_max, selection_count,
    multi_draw_enabled, max_draws_advance, max_tickets_per_player
) VALUES (
    '88888888-aaaa-bbbb-cccc-888888888888',
    'SIKAKES',
    'Sika Kese Friday',
    'standard',
    'GAME_TYPE_5_90',
    'NUMBER_SELECTION',
    'LOTTO',
    'ORGANIZER_NLA',
    0.50, 5000.00, 1.00, 'ACTIVE',
    'Sika Kese Friday - Early Morning Draw',
    'WEEKLY',
    '06:50',
    30,
    1, 90, 5,
    true, 10, 100
) ON CONFLICT (id) DO UPDATE SET
    code = EXCLUDED.code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    game_type = EXCLUDED.game_type,
    game_format = EXCLUDED.game_format,
    game_category = EXCLUDED.game_category,
    organizer = EXCLUDED.organizer,
    min_stake_amount = EXCLUDED.min_stake_amount,
    max_stake_amount = EXCLUDED.max_stake_amount,
    base_price = EXCLUDED.base_price,
    status = EXCLUDED.status,
    description = EXCLUDED.description,
    draw_frequency = EXCLUDED.draw_frequency,
    draw_time_str = EXCLUDED.draw_time_str,
    sales_cutoff_minutes = EXCLUDED.sales_cutoff_minutes,
    number_range_min = EXCLUDED.number_range_min,
    number_range_max = EXCLUDED.number_range_max,
    selection_count = EXCLUDED.selection_count,
    multi_draw_enabled = EXCLUDED.multi_draw_enabled,
    max_draws_advance = EXCLUDED.max_draws_advance,
    max_tickets_per_player = EXCLUDED.max_tickets_per_player,
    updated_at = NOW();

-- Create game rules for all 8 games
INSERT INTO game_rules (
    id, game_id, numbers_to_pick, total_numbers, min_selections, max_selections, allow_quick_pick
) VALUES
    ('11111111-4444-4444-4444-111111111111', '11111111-aaaa-bbbb-cccc-111111111111', 5, 90, 1, 10, true),
    ('22222222-4444-4444-4444-222222222222', '22222222-aaaa-bbbb-cccc-222222222222', 5, 90, 1, 10, true),
    ('33333333-4444-4444-4444-333333333333', '33333333-aaaa-bbbb-cccc-333333333333', 5, 90, 1, 10, true),
    ('44444444-4444-4444-4444-444444444444', '44444444-aaaa-bbbb-cccc-444444444444', 5, 90, 1, 10, true),
    ('55555555-4444-4444-4444-555555555555', '55555555-aaaa-bbbb-cccc-555555555555', 5, 90, 1, 10, true),
    ('66666666-4444-4444-4444-666666666666', '66666666-aaaa-bbbb-cccc-666666666666', 5, 90, 1, 10, true),
    ('77777777-4444-4444-4444-777777777777', '77777777-aaaa-bbbb-cccc-777777777777', 5, 90, 1, 10, true),
    ('88888888-4444-4444-4444-888888888888', '88888888-aaaa-bbbb-cccc-888888888888', 5, 90, 1, 10, true)
ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    numbers_to_pick = EXCLUDED.numbers_to_pick,
    total_numbers = EXCLUDED.total_numbers,
    min_selections = EXCLUDED.min_selections,
    max_selections = EXCLUDED.max_selections,
    allow_quick_pick = EXCLUDED.allow_quick_pick,
    updated_at = NOW();

-- Create prize structures for all 8 games
INSERT INTO prize_structures (
    id, game_id, total_prize_pool, house_edge_percentage
) VALUES
    ('11111111-5555-5555-5555-111111111111', '11111111-aaaa-bbbb-cccc-111111111111', 0, 30.00),
    ('22222222-5555-5555-5555-222222222222', '22222222-aaaa-bbbb-cccc-222222222222', 0, 30.00),
    ('33333333-5555-5555-5555-333333333333', '33333333-aaaa-bbbb-cccc-333333333333', 0, 30.00),
    ('44444444-5555-5555-5555-444444444444', '44444444-aaaa-bbbb-cccc-444444444444', 0, 30.00),
    ('55555555-5555-5555-5555-555555555555', '55555555-aaaa-bbbb-cccc-555555555555', 0, 30.00),
    ('66666666-5555-5555-5555-666666666666', '66666666-aaaa-bbbb-cccc-666666666666', 0, 30.00),
    ('77777777-5555-5555-5555-777777777777', '77777777-aaaa-bbbb-cccc-777777777777', 0, 30.00),
    ('88888888-5555-5555-5555-888888888888', '88888888-aaaa-bbbb-cccc-888888888888', 0, 30.00)
ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    total_prize_pool = EXCLUDED.total_prize_pool,
    house_edge_percentage = EXCLUDED.house_edge_percentage,
    updated_at = NOW();

-- Create prize tiers for all 8 games (4 tiers each)
INSERT INTO prize_tiers (
    prize_structure_id, tier_number, name, matches_required, prize_percentage, estimated_winners
) VALUES
    -- Game 1: VAG MONDAY
    ('11111111-5555-5555-5555-111111111111', 1, 'First Prize', 5, 50.00, 1),
    ('11111111-5555-5555-5555-111111111111', 2, 'Second Prize', 4, 30.00, 10),
    ('11111111-5555-5555-5555-111111111111', 3, 'Third Prize', 3, 15.00, 50),
    ('11111111-5555-5555-5555-111111111111', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 2: THURSDAY NOONRUSH
    ('22222222-5555-5555-5555-222222222222', 1, 'First Prize', 5, 50.00, 1),
    ('22222222-5555-5555-5555-222222222222', 2, 'Second Prize', 4, 30.00, 10),
    ('22222222-5555-5555-5555-222222222222', 3, 'Third Prize', 3, 15.00, 50),
    ('22222222-5555-5555-5555-222222222222', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 3: MONDAY SPECIAL
    ('33333333-5555-5555-5555-333333333333', 1, 'First Prize', 5, 50.00, 1),
    ('33333333-5555-5555-5555-333333333333', 2, 'Second Prize', 4, 30.00, 10),
    ('33333333-5555-5555-5555-333333333333', 3, 'Third Prize', 3, 15.00, 50),
    ('33333333-5555-5555-5555-333333333333', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 4: FRIDAY BONANZA
    ('44444444-5555-5555-5555-444444444444', 1, 'First Prize', 5, 50.00, 1),
    ('44444444-5555-5555-5555-444444444444', 2, 'Second Prize', 4, 30.00, 10),
    ('44444444-5555-5555-5555-444444444444', 3, 'Third Prize', 3, 15.00, 50),
    ('44444444-5555-5555-5555-444444444444', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 5: NATIONAL SATURDAY
    ('55555555-5555-5555-5555-555555555555', 1, 'First Prize', 5, 50.00, 1),
    ('55555555-5555-5555-5555-555555555555', 2, 'Second Prize', 4, 30.00, 10),
    ('55555555-5555-5555-5555-555555555555', 3, 'Third Prize', 3, 15.00, 50),
    ('55555555-5555-5555-5555-555555555555', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 6: ASEDA SUNDAY
    ('66666666-5555-5555-5555-666666666666', 1, 'First Prize', 5, 50.00, 1),
    ('66666666-5555-5555-5555-666666666666', 2, 'Second Prize', 4, 30.00, 10),
    ('66666666-5555-5555-5555-666666666666', 3, 'Third Prize', 3, 15.00, 50),
    ('66666666-5555-5555-5555-666666666666', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 7: BINGO4 MONDAY
    ('77777777-5555-5555-5555-777777777777', 1, 'First Prize', 5, 50.00, 1),
    ('77777777-5555-5555-5555-777777777777', 2, 'Second Prize', 4, 30.00, 10),
    ('77777777-5555-5555-5555-777777777777', 3, 'Third Prize', 3, 15.00, 50),
    ('77777777-5555-5555-5555-777777777777', 4, 'Fourth Prize', 2, 5.00, 200),
    -- Game 8: SIKA KESE FRIDAY
    ('88888888-5555-5555-5555-888888888888', 1, 'First Prize', 5, 50.00, 1),
    ('88888888-5555-5555-5555-888888888888', 2, 'Second Prize', 4, 30.00, 10),
    ('88888888-5555-5555-5555-888888888888', 3, 'Third Prize', 3, 15.00, 50),
    ('88888888-5555-5555-5555-888888888888', 4, 'Fourth Prize', 2, 5.00, 200)
ON CONFLICT (prize_structure_id, tier_number) DO NOTHING;

-- Create bet types (skip if already exist)
INSERT INTO bet_types (id, name, description, base_multiplier)
SELECT gen_random_uuid(), 'Direct 1', 'Pick 1 number', 80.0
WHERE NOT EXISTS (SELECT 1 FROM bet_types WHERE name = 'Direct 1');

INSERT INTO bet_types (id, name, description, base_multiplier)
SELECT gen_random_uuid(), 'Perm 2', 'Pick 2 numbers', 40.0
WHERE NOT EXISTS (SELECT 1 FROM bet_types WHERE name = 'Perm 2');

INSERT INTO bet_types (id, name, description, base_multiplier)
SELECT gen_random_uuid(), 'Perm 3', 'Pick 3 numbers', 25.0
WHERE NOT EXISTS (SELECT 1 FROM bet_types WHERE name = 'Perm 3');

INSERT INTO bet_types (id, name, description, base_multiplier)
SELECT gen_random_uuid(), 'Banker 1', 'One banker number', 15.0
WHERE NOT EXISTS (SELECT 1 FROM bet_types WHERE name = 'Banker 1');

-- Create game schedules for all 8 games
-- Each game gets a schedule based on its draw frequency and time
-- All schedules will be for the upcoming week to match the draws created later

-- 1. VAG MONDAY (Monday 10:00am)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '11111111-8888-8888-8888-111111111111',
    '11111111-aaaa-bbbb-cccc-111111111111',
    'VAG Monday',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,  -- Inactive so scheduler won't pick it up
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 2. THURSDAY NOONRUSH (Thursday 1:30pm)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '22222222-8888-8888-8888-222222222222',
    '22222222-aaaa-bbbb-cccc-222222222222',
    'Thursday Noonrush',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,  -- Inactive so scheduler won't pick it up
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 3. MONDAY SPECIAL (Monday 7:15pm)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '33333333-8888-8888-8888-333333333333',
    '33333333-aaaa-bbbb-cccc-333333333333',
    'Monday Special',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 4. FRIDAY BONANZA (Friday 7:15pm)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '44444444-8888-8888-8888-444444444444',
    '44444444-aaaa-bbbb-cccc-444444444444',
    'Friday Bonanza',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 5. NATIONAL SATURDAY (Saturday 7:15pm)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '55555555-8888-8888-8888-555555555555',
    '55555555-aaaa-bbbb-cccc-555555555555',
    'National Saturday',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 6. ASEDA SUNDAY (Sunday 7:15pm)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '66666666-8888-8888-8888-666666666666',
    '66666666-aaaa-bbbb-cccc-666666666666',
    'Aseda Sunday',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 7. BINGO4 MONDAY (Monday 6:50am)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '77777777-8888-8888-8888-777777777777',
    '77777777-aaaa-bbbb-cccc-777777777777',
    'Bingo4 Monday',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();

-- 8. SIKA KESE FRIDAY (Friday 6:50am)
INSERT INTO game_schedules (
    id, game_id, game_name, scheduled_start, scheduled_end, scheduled_draw, frequency, is_active, status
) VALUES (
    '88888888-8888-8888-8888-888888888888',
    '88888888-aaaa-bbbb-cccc-888888888888',
    'Sika Kese Friday',
    NOW() - INTERVAL '7 days',  -- Sales started 7 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Sales ended 5 days ago (in the past)
    NOW() - INTERVAL '5 days',  -- Draw was 5 days ago (in the past)
    'WEEKLY',
    false,
    'COMPLETED'
) ON CONFLICT (id) DO UPDATE SET
    game_id = EXCLUDED.game_id,
    game_name = EXCLUDED.game_name,
    scheduled_start = EXCLUDED.scheduled_start,
    scheduled_end = EXCLUDED.scheduled_end,
    scheduled_draw = EXCLUDED.scheduled_draw,
    frequency = EXCLUDED.frequency,
    is_active = EXCLUDED.is_active,
    status = EXCLUDED.status,
    updated_at = NOW();
EOF

if [ $? -eq 0 ]; then
    print_success "Game data populated (8 games, 32 prize tiers, 4 bet types, 8 schedules)"
else
    print_error "Failed to populate Game data"
    exit 1
fi

print_info "Step 4: Populating Draw Service..."

# STEP 4: Draw Service - Create scheduled draws for each game

# Game configuration arrays
GAME_CODES=("VAGMON" "THUNR" "MONSPC" "FRIBON" "NATSAT" "ASEDSUN" "BING4" "SIKAKES")
GAME_IDS=("11111111-aaaa-bbbb-cccc-111111111111" "22222222-aaaa-bbbb-cccc-222222222222" "33333333-aaaa-bbbb-cccc-333333333333" "44444444-aaaa-bbbb-cccc-444444444444" "55555555-aaaa-bbbb-cccc-555555555555" "66666666-aaaa-bbbb-cccc-666666666666" "77777777-aaaa-bbbb-cccc-777777777777" "88888888-aaaa-bbbb-cccc-888888888888")
GAME_NAMES=("VAG Monday" "Thursday Noonrush" "Monday Special" "Friday Bonanza" "National Saturday" "Aseda Sunday" "Bingo4 Monday" "Sika Kese Friday")
GAME_SCHEDULE_IDS=("11111111-8888-8888-8888-111111111111" "22222222-8888-8888-8888-222222222222" "33333333-8888-8888-8888-333333333333" "44444444-8888-8888-8888-444444444444" "55555555-8888-8888-8888-555555555555" "66666666-8888-8888-8888-666666666666" "77777777-8888-8888-8888-777777777777" "88888888-8888-8888-8888-888888888888")

DRAW_COUNT=0

for i in "${!GAME_CODES[@]}"; do
    game_code="${GAME_CODES[$i]}"
    game_id="${GAME_IDS[$i]}"
    game_name="${GAME_NAMES[$i]}"

    draw_number=$((i + 1))

    # Generate unique draw ID
    draw_uuid="dddddddd-dddd-dddd-dddd-$(printf '%012d' $draw_number)"

    # Calculate ticket stats for this draw (50 tickets per game)
    # With new stake amounts, average total will be much higher (around 175000 pesewas = 1750 GHS)
    # This is more realistic for a lottery draw
    tickets_sold=50
    total_stakes=175000

    game_schedule_id="${GAME_SCHEDULE_IDS[$i]}"

    docker exec -i $DRAW_CONTAINER psql -U draw_service -d draw_service <<EOF
INSERT INTO draws (
    id, game_id, draw_number, game_name, game_code, game_schedule_id, draw_name, status, scheduled_time, draw_location,
    total_tickets_sold, total_prize_pool
) VALUES (
    '$draw_uuid',
    '$game_id',
    $draw_number,
    '$game_name',
    '$game_code',
    '$game_schedule_id',
    '$game_name - Draw #$draw_number',
    'scheduled',
    NOW() - INTERVAL '5 days',
    'Accra Draw Center',
    $tickets_sold,
    $total_stakes
) ON CONFLICT (game_id, draw_number) DO UPDATE SET
    id = EXCLUDED.id,
    game_name = EXCLUDED.game_name,
    game_code = EXCLUDED.game_code,
    game_schedule_id = EXCLUDED.game_schedule_id,
    draw_name = EXCLUDED.draw_name,
    status = EXCLUDED.status,
    scheduled_time = EXCLUDED.scheduled_time,
    draw_location = EXCLUDED.draw_location,
    total_tickets_sold = EXCLUDED.total_tickets_sold,
    total_prize_pool = EXCLUDED.total_prize_pool,
    updated_at = NOW();
EOF

    # Store draw ID for ticket generation
    eval "DRAW_${game_code}=$draw_uuid"

    DRAW_COUNT=$((DRAW_COUNT + 1))
done

if [ $DRAW_COUNT -eq 8 ]; then
    print_success "Draw data populated ($DRAW_COUNT scheduled draws for 8 games)"
else
    print_error "Failed to populate Draw data"
    exit 1
fi

print_info "Step 5: Populating Ticket Service..."

# STEP 5: Ticket Service - Generate 50 tickets per game with varying bet types

# Retailer codes
RETAILER_IDS=("90010001" "90010002" "90010003" "90010004" "90010005")

# Bet type configurations (must match bet_rules_engine.go format)
BET_TYPE_NAMES=("Direct 1" "Perm 2" "Perm 3" "Banker All")
BET_TYPE_MULTIPLIERS=("80.0" "40.0" "25.0" "15.0")

# Stake amounts (in pesewas) - 100 pesewas = 1 GHS
# DIRECT_1: 1-10 GHS, PERM_2: 5-25 GHS, PERM_3: 10-50 GHS, BANKER_1: 20-100 GHS
STAKE_AMOUNTS_DIRECT=(100 200 300 500 1000)       # 1, 2, 3, 5, 10 GHS
STAKE_AMOUNTS_PERM2=(500 1000 1500 2000 2500)     # 5, 10, 15, 20, 25 GHS
STAKE_AMOUNTS_PERM3=(1000 2000 3000 4000 5000)    # 10, 20, 30, 40, 50 GHS
STAKE_AMOUNTS_BANKER=(2000 4000 6000 8000 10000)  # 20, 40, 60, 80, 100 GHS

# Function to generate random numbers
generate_numbers() {
    local count=$1
    local seed=$2
    local numbers=""

    for ((n=1; n<=count; n++)); do
        local num=$(( (seed * 7 + n * 13) % 90 + 1 ))
        numbers="$numbers$num"
        if [ $n -lt $count ]; then
            numbers="$numbers,"
        fi
    done

    echo "$numbers"
}

TICKET_START=1
TOTAL_TICKETS=0

for i in "${!GAME_CODES[@]}"; do
    game_code="${GAME_CODES[$i]}"
    game_id="${GAME_IDS[$i]}"
    game_name="${GAME_NAMES[$i]}"
    game_schedule_id="${GAME_SCHEDULE_IDS[$i]}"
    draw_var="DRAW_${game_code}"
    draw_id="${!draw_var}"
    draw_num=$((i + 1))

    # Generate 50 tickets for this game
    for ((ticket_num=0; ticket_num<50; ticket_num++)); do
        global_ticket_num=$((TICKET_START + ticket_num))

        # Select retailer (rotate through 5)
        retailer_idx=$((ticket_num % 5))
        retailer_code="${RETAILER_IDS[$retailer_idx]}"

        # Select bet type (rotate through 4 types)
        bet_type_idx=$((ticket_num % 4))
        bet_type="${BET_TYPE_NAMES[$bet_type_idx]}"
        multiplier="${BET_TYPE_MULTIPLIERS[$bet_type_idx]}"

        # Select stake amount based on bet type (rotate through 5 amounts)
        stake_idx=$((ticket_num % 5))
        if [ "$bet_type" == "Direct 1" ]; then
            stake="${STAKE_AMOUNTS_DIRECT[$stake_idx]}"
        elif [ "$bet_type" == "Perm 2" ]; then
            stake="${STAKE_AMOUNTS_PERM2[$stake_idx]}"
        elif [ "$bet_type" == "Perm 3" ]; then
            stake="${STAKE_AMOUNTS_PERM3[$stake_idx]}"
        else  # Banker All
            stake="${STAKE_AMOUNTS_BANKER[$stake_idx]}"
        fi
        total=$stake

        # Calculate potential win
        potential_win=$(echo "$stake * $multiplier / 100" | bc)

        # Generate numbers based on bet type
        if [ "$bet_type" == "Direct 1" ]; then
            numbers=$(generate_numbers 1 $global_ticket_num)
            bet_line="{\"bet_type\": \"$bet_type\", \"numbers\": [$numbers], \"amount\": $stake, \"potential_win\": $potential_win}"
            selected_nums="ARRAY[$numbers]"
        elif [ "$bet_type" == "Perm 2" ]; then
            numbers=$(generate_numbers 2 $global_ticket_num)
            bet_line="{\"bet_type\": \"$bet_type\", \"numbers\": [$numbers], \"amount\": $stake, \"potential_win\": $potential_win}"
            selected_nums="ARRAY[$numbers]"
        elif [ "$bet_type" == "Perm 3" ]; then
            numbers=$(generate_numbers 3 $global_ticket_num)
            bet_line="{\"bet_type\": \"$bet_type\", \"numbers\": [$numbers], \"amount\": $stake, \"potential_win\": $potential_win}"
            selected_nums="ARRAY[$numbers]"
        else  # Banker All
            numbers=$(generate_numbers 5 $global_ticket_num)
            bet_line="{\"bet_type\": \"$bet_type\", \"numbers\": [$numbers], \"amount\": $stake, \"potential_win\": $potential_win}"
            selected_nums="ARRAY[$numbers]"
        fi

        # Generate serial number
        serial_num="TKT-${game_code}-$(printf '%05d' $((ticket_num + 1)))"

        # Insert ticket
        docker exec -i $TICKET_CONTAINER psql -U ticket -d ticket_service <<EOF > /dev/null 2>&1
INSERT INTO tickets (
    serial_number, game_code, game_schedule_id, draw_id, draw_number,
    game_name, game_type,
    selected_numbers, banker_numbers, opposed_numbers,
    bet_lines, number_of_lines,
    unit_price, total_amount,
    issuer_type, issuer_id,
    customer_phone, customer_name, customer_email,
    payment_method, payment_ref, payment_status,
    security_hash, status,
    draw_date, draw_time, issued_at
) VALUES (
    '$serial_num',
    '$game_code',
    '$game_schedule_id',
    '$draw_id',
    $draw_num,
    '$game_name',
    'GAME_TYPE_5_90',
    $selected_nums,
    ARRAY[]::integer[],
    ARRAY[]::integer[],
    '[$bet_line]'::jsonb,
    1,
    $stake, $total,
    'pos',
    '$retailer_code',
    '+233200$(printf '%06d' $global_ticket_num)',
    'Customer $global_ticket_num',
    'customer$global_ticket_num@test.com',
    'cash',
    'CASH-$(printf '%08d' $global_ticket_num)',
    'completed',
    md5('$serial_num'),
    'issued',
    NOW() - INTERVAL '5 days',
    '18:00',
    NOW()
) ON CONFLICT (serial_number) DO UPDATE SET
    game_code = EXCLUDED.game_code,
    game_schedule_id = EXCLUDED.game_schedule_id,
    draw_id = EXCLUDED.draw_id,
    draw_number = EXCLUDED.draw_number,
    game_name = EXCLUDED.game_name,
    game_type = EXCLUDED.game_type,
    selected_numbers = EXCLUDED.selected_numbers,
    bet_lines = EXCLUDED.bet_lines,
    unit_price = EXCLUDED.unit_price,
    total_amount = EXCLUDED.total_amount,
    issuer_id = EXCLUDED.issuer_id,
    updated_at = NOW();
EOF
    done

    TICKET_START=$((TICKET_START + 50))
    TOTAL_TICKETS=$((TOTAL_TICKETS + 50))
done

if [ $TOTAL_TICKETS -eq 400 ]; then
    print_success "Ticket data populated ($TOTAL_TICKETS tickets: 50 per game, 10 per retailer per game)"
else
    print_error "Failed to populate Ticket data"
    exit 1
fi

echo ""
print_success "Test data population completed successfully!"
echo ""
echo "============================================================================"
echo "  TEST DATA SUMMARY"
echo "============================================================================"
echo "  Agents:              1 (agent code: 9001)"
echo "  Retailers:           5 (retailer codes: 90010001-90010005)"
echo "  POS Devices:         5 (POS-TEST-000001 to POS-TEST-000005)"
echo "  Stake Wallets:       5 (₵20K-₵60K each, ₵200K total)"
echo "  Winning Wallets:     5 (₵0 balance, ready for payouts)"
echo "  Games:               8 (VAGMON, THUNR, MONSPC, FRIBON, NATSAT, ASEDSUN, BING4, SIKAKES)"
echo "  Game Schedules:      8 (one per game, all WEEKLY frequency)"
echo "  Prize Tiers:         32 total (4 tiers per game: Match 5, 4, 3, 2)"
echo "  Bet Types:           4 (Direct 1, Perm 2, Perm 3, Banker 1)"
echo "  Draws:               8 (all scheduled for 3 days from now)"
echo "  Tickets:             400 total (50 per game, 10 per retailer per game)"
echo "============================================================================"
echo ""
echo "Ticket Details:"
echo "  - Serial Numbers:    TKT-{GAMECODE}-00001 to TKT-{GAMECODE}-00050 per game"
echo "  - Bet Types:         Rotating (Direct 1, Perm 2, Perm 3, Banker 1)"
echo "  - Stake Amounts by Type:"
echo "      Direct 1:        ₵1, ₵2, ₵3, ₵5, ₵10"
echo "      Perm 2:          ₵5, ₵10, ₵15, ₵20, ₵25"
echo "      Perm 3:          ₵10, ₵20, ₵30, ₵40, ₵50"
echo "      Banker 1:        ₵20, ₵40, ₵60, ₵80, ₵100"
echo "  - Total Stakes:      ₵1,750 per draw (175,000 pesewas)"
echo "  - Retailers:         Each retailer has 10 tickets per game (80 tickets total per retailer)"
echo "============================================================================"
echo ""
echo "Next Steps:"
echo "  1. Open Admin Web:    http://localhost:6176/draws"
echo "  2. Login with:        admin@randlottery.com / Admin123!"
echo "  3. Navigate to Draws page"
echo "  4. Select Draw #2 (in-progress) to test workflow"
echo "  5. Execute 4-stage draw workflow:"
echo "     - Stage 1: Complete Preparation"
echo "     - Stage 2: Record Winning Numbers (triple-entry)"
echo "     - Stage 3: Calculate Results"
echo "     - Stage 4: Process Payouts"
echo ""
echo "View Traces:          http://localhost:16686 (Jaeger)"
echo "============================================================================"
