#!/bin/bash

# WebRTC Streaming - Start Script
# This script helps you start the WebRTC streaming services

echo "🚀 Starting WebRTC Streaming Services..."
echo ""

# Read port from .env or use default
PORT=$(grep SIGNALING_SERVER_PORT backend/.env 2>/dev/null | cut -d '=' -f2 | tr -d ' ' || echo "8081")
if [ -z "$PORT" ] || [ "$PORT" = "" ]; then
    PORT=8081
fi

# Check if port is in use and free it automatically
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "⚠️  Port $PORT is already in use!"
    PID=$(lsof -ti:$PORT)
    
    # Check if it's nginx
    if ps -p $PID -o comm= 2>/dev/null | grep -q nginx; then
        echo "🔍 Detected nginx is using port $PORT"
        echo "ℹ️  You can either:"
        echo "   1. Stop nginx: brew services stop nginx-full (or nginx)"
        echo "   2. Change port in backend/.env: SIGNALING_SERVER_PORT=8081"
        echo ""
        read -p "Try to free port $PORT anyway? (y/n): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    if [ ! -z "$PID" ]; then
        echo "🛑 Freeing port $PORT (killing process $PID)..."
        kill -9 $PID 2>/dev/null
        sleep 2
        
        if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1 ; then
            echo "❌ Failed to free port $PORT (may require sudo)"
            echo "ℹ️  Try: sudo lsof -ti:$PORT | xargs sudo kill -9"
            echo "   Or change SIGNALING_SERVER_PORT in backend/.env"
            exit 1
        else
            echo "✅ Port $PORT is now free"
        fi
    fi
fi

# Verify Go dependencies
echo "📦 Checking Go dependencies..."
cd backend
if ! go mod tidy; then
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

# Start services
echo "🔧 Starting services..."
echo ""
echo "Starting Signaling Server (http://localhost:$PORT)..."
echo "Open another terminal and run: cd backend && go run cmd/publisher/main.go"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

# Start signaling server (blocks)
cd backend && go run cmd/signaling/main.go

