#!/bin/bash
set -e

echo "🚀 Starting microservices..."

# Start all services in parallel - they will load their own .env files

# 1. Admin Management Service (includes auth functionality)
echo "Starting admin-management service on port 50057..."
(
  cd ./services/service-admin-management
  go run cmd/server/main.go
) &
ADMIN_MGMT_PID=$!
echo "Admin-management PID: $ADMIN_MGMT_PID"

# Wait a moment for admin-management to start (other services depend on it)
sleep 3

# 2. Agent Auth Service
echo "Starting agent-auth service on port 50052..."
(
  cd ./services/service-agent-auth
  go run cmd/server/main.go
) &
AGENT_AUTH_PID=$!
echo "Agent-auth PID: $AGENT_AUTH_PID"

# 3. Agent Management Service
echo "Starting agent-management service on port 50058..."
(
  cd ./services/service-agent-management
  go run cmd/server/main.go
) &
AGENT_MGMT_PID=$!
echo "Agent-management PID: $AGENT_MGMT_PID"

# 4. Wallet Service
echo "Starting wallet service on port 50059..."
(
  cd ./services/service-wallet
  go run cmd/server/main.go
) &
WALLET_PID=$!
echo "Wallet service PID: $WALLET_PID"

# 5. Terminal Management Service
echo "Starting terminal service on port 50054..."
(
  cd ./services/service-terminal
  go run cmd/server/main.go
) &
TERMINAL_PID=$!
echo "Terminal service PID: $TERMINAL_PID"

# 6. Payment Service
echo "Starting payment service on port 50061..."
(
  cd ./services/service-payment
  go run cmd/server/main.go
) &
PAYMENT_PID=$!
echo "Payment service PID: $PAYMENT_PID"

# 7. Game Service
echo "Starting game service on port 50053..."
(
  cd ./services/service-game
  go run cmd/server/main.go
) &
GAME_PID=$!
echo "Game service PID: $GAME_PID"

# 8. Draw Service
echo "Starting draw service on port 50060..."
(
  cd ./services/service-draw
  go run cmd/server/main.go
) &
DRAW_PID=$!
echo "Draw service PID: $DRAW_PID"

# 9. Ticket Service
echo "Starting ticket service on port 50062..."
(
  cd ./services/service-ticket
  go run cmd/server/main.go
) &
TICKET_PID=$!
echo "Ticket service PID: $TICKET_PID"

# Wait a moment for auth services to start
sleep 2

# 10. API Gateway
echo "Starting api-gateway service on port 4000..."
(
  cd ./services/api-gateway
  go run cmd/server/main.go
) &
GATEWAY_PID=$!
echo "API Gateway PID: $GATEWAY_PID"


echo ""
echo "✅ All services started successfully!"
echo "🌐 API Gateway: http://localhost:4000"
echo "👤 Admin Management (with Auth): grpc://localhost:50057"
echo "🏢 Agent Auth: grpc://localhost:50052"
echo "🏪 Agent Management: grpc://localhost:50058"
echo "💰 Wallet Service: grpc://localhost:50059"
echo "📱 Terminal Management: grpc://localhost:50054"
echo "💳 Payment Service: grpc://localhost:50061"
echo "🎮 Game Service: grpc://localhost:50053"
echo "🎲 Draw Service: grpc://localhost:50060"
echo "🎫 Ticket Service: grpc://localhost:50062"
echo ""
echo "📋 Service Status:"
echo "  Admin Management: RUNNING (PID: $ADMIN_MGMT_PID)"
echo "  Agent Auth:       RUNNING (PID: $AGENT_AUTH_PID)"
echo "  Agent Management: RUNNING (PID: $AGENT_MGMT_PID)"
echo "  Wallet Service:   RUNNING (PID: $WALLET_PID)"
echo "  Terminal Service: RUNNING (PID: $TERMINAL_PID)"
echo "  Payment Service:  RUNNING (PID: $PAYMENT_PID)"
echo "  Game Service:     RUNNING (PID: $GAME_PID)"
echo "  Draw Service:     RUNNING (PID: $DRAW_PID)"
echo "  Ticket Service:   RUNNING (PID: $TICKET_PID)"
echo "  API Gateway:      RUNNING (PID: $GATEWAY_PID)"
echo ""
echo "🛑 To stop all services:"
echo "kill $ADMIN_MGMT_PID $AGENT_AUTH_PID $AGENT_MGMT_PID $WALLET_PID $TERMINAL_PID $PAYMENT_PID $GAME_PID $DRAW_PID $TICKET_PID $GATEWAY_PID $NOTIFICATION_PID"
