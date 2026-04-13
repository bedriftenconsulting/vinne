#!/bin/bash
set -e

echo "🛑 Stopping all microservices..."

# Gracefully stop Go processes with SIGTERM
echo "Sending SIGTERM to Go service processes for graceful shutdown..."
pkill -TERM -f "go run.*cmd/server/main.go" || echo "No Go processes found"

# Wait for graceful shutdown (30 seconds to match queue drain timeout)
echo "Waiting for graceful shutdown (up to 30 seconds)..."
WAIT_TIME=0
MAX_WAIT=30

while [ $WAIT_TIME -lt $MAX_WAIT ]; do
    if ! pgrep -f "go run.*cmd/server/main.go" > /dev/null; then
        echo "✅ All services stopped gracefully"
        break
    fi
    sleep 1
    WAIT_TIME=$((WAIT_TIME + 1))
    if [ $((WAIT_TIME % 5)) -eq 0 ]; then
        echo "  ... still waiting ($WAIT_TIME/$MAX_WAIT seconds)"
    fi
done

# Force kill if still running after timeout
if pgrep -f "go run.*cmd/server/main.go" > /dev/null; then
    echo "⚠️  Some processes didn't stop gracefully, forcing shutdown..."
    pkill -KILL -f "go run.*cmd/server/main.go" || echo "Processes already terminated"
fi

# Stop Docker containers
echo "Stopping infrastructure containers..."
docker-compose down

# Clean up any remaining processes with SIGTERM first, then SIGKILL if needed
echo "Cleaning up remaining processes..."
if pgrep -f "main" > /dev/null; then
    pkill -TERM -f "main" || true
    sleep 2
    pkill -KILL -f "main" || echo "No remaining processes found"
fi

echo "✅ All services stopped successfully!"
echo ""
echo "To restart everything:"
echo "  1. ./scripts/start-infrastructure.sh"
echo "  2. ./scripts/start-services.sh"