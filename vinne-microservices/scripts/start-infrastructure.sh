#!/bin/bash
set -e

echo "🚀 Starting microservices infrastructure..."

# Start infrastructure services
docker-compose up -d \
  service-admin-management-db \
  service-agent-auth-db \
  service-agent-management-db \
  service-wallet-db \
  service-terminal-db \
  service-payment-db \
  service-game-db \
  service-draw-db \
  service-ticket-db \
  service-admin-management-redis \
  service-agent-auth-redis \
  service-agent-management-redis \
  service-wallet-redis \
  service-terminal-redis \
  service-payment-redis \
  service-game-redis \
  service-draw-redis \
  service-ticket-redis \
  redis-shared \
  kafka \
  jaeger

# Wait for databases to be ready
echo "⏳ Waiting for databases to be ready..."
sleep 8

# Apply migrations
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/run-migrations.sh" ]; then
    "$SCRIPT_DIR/run-migrations.sh"
else
    echo "⚠️  Migration script not found, trying goose..."
    goose -dir services/service-admin-management/migrations postgres "postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-agent-auth/migrations postgres "postgresql://agent:agent123@localhost:5434/agent_auth?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-agent-management/migrations postgres "postgresql://agent_mgmt:agent_mgmt123@localhost:5435/agent_management?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-wallet/migrations postgres "postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-terminal/migrations postgres "postgresql://terminal:terminal123@localhost:5439/terminal_service?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-payment/migrations postgres "postgresql://payment:payment123@localhost:5440/payment_service?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-game/migrations postgres "postgresql://game:game123@localhost:5441/game_service?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
    goose -dir services/service-ticket/migrations postgres "postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable" up 2>/dev/null || echo "Goose not installed"
fi

echo "✅ Infrastructure started successfully!"
echo "📊 Database Services:"
echo "  Admin Management DB: localhost:5437 (admin_mgmt/admin_mgmt123)"  
echo "  Agent Auth DB:       localhost:5434 (agent/agent123)"
echo "  Agent Management DB: localhost:5435 (agent_mgmt/agent_mgmt123)"
echo "  Wallet Service DB:   localhost:5438 (wallet/wallet123)"
echo "  Terminal Service DB: localhost:5439 (terminal/terminal123)"
echo "  Payment Service DB:  localhost:5440 (payment/payment123)"
echo "  Game Service DB:     localhost:5441 (game/game123)"
echo "  Draw Service DB:     localhost:5436 (draw_service/draw_service123)"
echo "  Ticket Service DB:   localhost:5442 (ticket/ticket123)"
echo ""
echo "🔄 Cache Services:"
echo "  Admin Mgmt Redis:    localhost:6384"
echo "  Agent Auth Redis:    localhost:6381"
echo "  Agent Mgmt Redis:    localhost:6382"
echo "  Wallet Redis:        localhost:6385"
echo "  Terminal Redis:      localhost:6386"
echo "  Payment Redis:       localhost:6387"
echo "  Game Redis:          localhost:6388"
echo "  Draw Redis:          localhost:6383"
echo "  Ticket Redis:        localhost:6389"
echo "  Shared Redis:        localhost:6379"
echo ""
echo "📡 Other Services:"
echo "  Kafka:               localhost:9092"
echo "  Jaeger UI:           localhost:16686"