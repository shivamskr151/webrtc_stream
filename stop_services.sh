#!/bin/bash

# Stop WebRTC Streaming Services

echo "🛑 Stopping WebRTC Streaming Services..."

# Kill publisher
pkill -f "go run cmd/publisher/main.go"
if [ $? -eq 0 ]; then
    echo "✅ Stopped Publisher service"
else
    echo "⚠️  Publisher service was not running"
fi

# Kill signaling server
pkill -f "go run cmd/signaling/main.go"
if [ $? -eq 0 ]; then
    echo "✅ Stopped Signaling server"
else
    echo "⚠️  Signaling server was not running"
fi

# Free ports
echo "🔄 Freeing ports..."
for port in 8081 8082; do
    PID=$(lsof -ti:$port 2>/dev/null)
    if [ ! -z "$PID" ]; then
        kill -9 $PID 2>/dev/null
        echo "   Freed port $port"
    fi
done

echo ""
echo "✅ All services stopped"
echo ""

