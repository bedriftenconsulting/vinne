#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "📊 Running database migrations..."

# Function to run migrations for a service
run_migration() {
    local service_name=$1
    local db_container=$2
    local db_user=$3
    local db_name=$4
    local migration_dir=$5
    
    echo -e "${YELLOW}Running migrations for ${service_name}...${NC}"
    
    # Check if migration directory exists
    if [ ! -d "$migration_dir" ]; then
        echo -e "${RED}❌ Migration directory not found: $migration_dir${NC}"
        return 1
    fi
    
    # Check if container is running
    if ! docker ps | grep -q "$db_container"; then
        echo -e "${RED}❌ Database container $db_container is not running${NC}"
        return 1
    fi
    
    # Run each migration file in order
    for migration_file in $(ls -1 "$migration_dir"/*.sql 2>/dev/null | sort); do
        if [ -f "$migration_file" ]; then
            echo "  Applying: $(basename $migration_file)"
            
            # Extract only the UP migration (between -- +goose Up and -- +goose Down)
            cat "$migration_file" | \
                sed -n '/-- +goose Up/,/-- +goose Down/p' | \
                sed '/-- +goose/d' | \
                docker exec -i "$db_container" psql -U "$db_user" -d "$db_name" 2>&1 | \
                grep -E "(ERROR|NOTICE|WARNING)" || true
        fi
    done
    
    echo -e "${GREEN}✅ ${service_name} migrations complete${NC}"
}

# Get the project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Wait for databases to be ready
echo "⏳ Waiting for databases to be ready..."
sleep 5

# Run migrations for each service
run_migration \
    "Admin Management" \
    "randco-microservices-service-admin-management-db-1" \
    "admin_mgmt" \
    "admin_management" \
    "services/service-admin-management/migrations"

run_migration \
    "Admin Auth" \
    "randco-microservices-service-admin-auth-db-1" \
    "admin" \
    "admin_auth" \
    "services/service-admin-auth/migrations"

run_migration \
    "Agent Management" \
    "randco-microservices-service-agent-management-db-1" \
    "agent_mgmt" \
    "agent_management" \
    "services/service-agent-management/migrations"

run_migration \
    "Agent Auth" \
    "randco-microservices-service-agent-auth-db-1" \
    "agent" \
    "agent_auth" \
    "services/service-agent-auth/migrations"

# Apply seed data
echo -e "${YELLOW}Applying seed data...${NC}"

# Admin user seed
if [ -f "scripts/seed_admin_user.sql" ]; then
    echo "  Seeding admin user..."
    cat scripts/seed_admin_user.sql | \
        docker exec -i randco-microservices-service-admin-management-db-1 \
        psql -U admin_mgmt -d admin_management 2>&1 | \
        grep -E "(ERROR|INSERT|already exists)" | head -5 || true
    echo -e "${GREEN}✅ Admin user seeded${NC}"
fi

echo -e "${GREEN}🎉 All migrations complete!${NC}"