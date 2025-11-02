#!/bin/bash

# WebRTC Streaming - Background Start Script
# This script starts all services in the background

echo "🚀 Starting WebRTC Streaming Services in Background..."
echo ""

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Read port from .env or use default
PORT=$(grep SIGNALING_SERVER_PORT backend/.env 2>/dev/null | cut -d '=' -f2 | tr -d ' ' || echo "8081")
if [ -z "$PORT" ] || [ "$PORT" = "" ]; then
    PORT=8081
fi

# Kill existing services
echo "🛑 Stopping existing services..."
pkill -f "go run cmd/publisher/main.go" 2>/dev/null
pkill -f "go run cmd/signaling/main.go" 2>/dev/null
sleep 2

# Free ports
echo "🔄 Freeing ports..."
for port in 8081 8082; do
    PID=$(lsof -ti:$port 2>/dev/null)
    if [ ! -z "$PID" ]; then
        kill -9 $PID 2>/dev/null
        echo "   Freed port $port"
    fi
done
sleep 1

# Verify Go dependencies
echo "📦 Checking Go dependencies..."
cd backend
if ! go mod tidy > /dev/null 2>&1; then
    echo "❌ Error: Failed to tidy Go modules"
    exit 1
fi
cd ..

# Check if frontend is built
if [ ! -d "frontend/dist" ]; then
    echo "📦 Building frontend..."
    cd frontend && npm run build && cd ..
    if [ $? -ne 0 ]; then
        echo "❌ Error: Failed to build frontend"
        exit 1
    fi
    echo ""
fi

# Create logs directory
mkdir -p logs

# Start signaling server first (publisher needs it)
echo "📡 Starting Signaling Server..."
cd backend
nohup go run cmd/signaling/main.go > ../logs/signaling.log 2>&1 &
SIGNALING_PID=$!
cd ..
sleep 3

# Wait for signaling server to be ready
echo "⏳ Waiting for signaling server to be ready..."
for i in {1..10}; do
    if curl -s http://localhost:$PORT/health > /dev/null 2>&1; then
        echo "✅ Signaling server is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "⚠️  Signaling server may not be ready yet, continuing anyway..."
    else
        sleep 1
    fi
done

# Start publisher in background
echo "🎥 Starting Publisher service..."
cd backend
nohup go run cmd/publisher/main.go > ../logs/publisher.log 2>&1 &
PUBLISHER_PID=$!
cd ..
sleep 3

# Check if services started successfully
echo ""
echo "⏳ Checking services..."
sleep 2

if ps -p $PUBLISHER_PID > /dev/null 2>&1; then
    echo "✅ Publisher service running (PID: $PUBLISHER_PID)"
else
    echo "❌ Publisher service failed to start"
    echo "   Check logs/publisher.log for errors"
    exit 1
fi

if ps -p $SIGNALING_PID > /dev/null 2>&1; then
    echo "✅ Signaling server running (PID: $SIGNALING_PID)"
else
    echo "❌ Signaling server failed to start"
    echo "   Check logs/signaling.log for errors"
    exit 1
fi

# Check if server is responding
if curl -s http://localhost:$PORT/health > /dev/null 2>&1; then
    echo "✅ Server is responding on port $PORT"
else
    echo "⚠️  Server may still be starting up..."
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ All services started in background!"
echo ""
echo "📊 Service Status:"
echo "   • Publisher:    PID $PUBLISHER_PID"
echo "   • Signaling:     PID $SIGNALING_PID"
echo "   • Frontend:      http://localhost:$PORT"
echo ""
echo "📝 Logs:"
echo "   • Publisher:    tail -f logs/publisher.log"
echo "   • Signaling:       tail -f logs/signaling.log"
echo ""
echo "🛑 To stop services:"
echo "   ./stop_services.sh"
echo "   or"
echo "   kill $PUBLISHER_PID $SIGNALING_PID"
echo ""
echo "🌐 Open http://localhost:$PORT in your browser"
echo "   Click 'Connect' to start receiving the stream"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

