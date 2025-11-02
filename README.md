# WebRTC Real-Time Video Streaming

A complete WebRTC video streaming solution with a Go backend (signaling server + publisher service) and a React + TypeScript frontend (viewer). This system enables real-time video streaming from RTSP sources (IP cameras) or other video sources to web browsers using WebRTC.

## Architecture

```
[ Camera / Video Source (Publisher) ]
        │
        ▼
[ Go WebRTC Publisher Service (pion/webrtc) ]
        │
  (Signaling via WebSocket)
        │
        ▼
[ Signaling Server (Go – WebSocket) ]
        │
  (Offer/Answer + ICE candidates)
        │
        ▼
[ WebRTC Viewer (Browser or Go Client) ]
```

## Project Structure

```
webrtc/
├── backend/
│   ├── cmd/
│   │   ├── signaling/      # Signaling server
│   │   └── publisher/       # Publisher service
│   ├── internal/
│   │   ├── config/          # Configuration management
│   │   ├── signaling/      # WebSocket signaling logic
│   │   └── video/           # Video capture abstraction
│   ├── go.mod
│   └── .env                 # Backend environment variables
├── frontend/
│   ├── src/
│   │   ├── components/      # React components
│   │   ├── hooks/           # Custom React hooks
│   │   ├── config/          # Frontend configuration
│   │   └── App.tsx
│   ├── package.json
│   └── .env                 # Frontend environment variables
└── README.md
```

## Features

- 🎥 Real-time video streaming via WebRTC
- 📡 WebSocket-based signaling server
- 🎬 RTSP stream support (IP cameras) with FFmpeg transcoding
- 🌐 Modern React frontend with TypeScript
- 🔄 Automatic ICE candidate handling and connection management
- 🎨 Beautiful, responsive UI with connection status indicators
- ⚡ H.264 and VP8 codec support
- 🚀 Easy deployment with startup scripts

## Prerequisites

- Go 1.21 or higher
- Node.js 18+ and npm
- FFmpeg (required for RTSP stream support)
  - **macOS**: `brew install ffmpeg`
  - **Linux**: `sudo apt-get install ffmpeg` (Ubuntu/Debian) or `sudo yum install ffmpeg` (RHEL/CentOS)
  - **Windows**: Download from [ffmpeg.org](https://ffmpeg.org/download.html)
- RTSP stream URL (optional - for IP camera streaming)

## Setup

### Backend Setup

1. Navigate to the backend directory:
```bash
cd backend
```

2. Create a `.env` file in the `backend` directory:
```bash
touch .env
```

3. Edit `.env` file with your configuration:
```env
SIGNALING_SERVER_HOST=localhost
SIGNALING_SERVER_PORT=8080
PUBLISHER_SERVER_HOST=localhost
PUBLISHER_SERVER_PORT=8081
ICE_SERVER_URLS=stun:stun.l.google.com:19302
VIDEO_DEVICE_INDEX=0
VIDEO_WIDTH=1280
VIDEO_HEIGHT=720
VIDEO_FPS=30
RTSP_URL=rtsp://admin:Tatva%40321@183.82.113.87:554/Streaming/Channels/301
ALLOWED_ORIGINS=http://localhost:8080,http://localhost:5173,http://localhost:3000
STATIC_FILES_PATH=../frontend/dist
```

**Note:** The `RTSP_URL` is optional. If provided, the system will stream from the RTSP source. If not provided, it will use a mock video source for testing. URL-encode special characters in the RTSP URL (e.g., `@` becomes `%40`).

**Important:** Make sure the `.env` file is in the `backend` directory, not the root directory.

4. Install Go dependencies:
```bash
go mod download
```

### Frontend Setup

1. Navigate to the frontend directory:
```bash
cd frontend
```

2. Install dependencies:
```bash
npm install
```

3. Build the frontend for production:
```bash
npm run build
```

This will create a `dist` directory that the backend will serve.

### Running the Application

**Option 1: Using Startup Scripts (Recommended)**

1. Build the frontend (if not already built):
```bash
cd frontend
npm run build
cd ..
```

2. Start services using the startup script:
```bash
# Foreground mode (single terminal)
./start.sh

# OR background mode (services run in background)
./start_background.sh
```

3. In another terminal, start the publisher service:
```bash
cd backend
go run cmd/publisher/main.go
```

4. Open your browser and navigate to `http://localhost:8080` (or the port configured in `.env`)

**Option 2: Manual Start (Production Mode - Single Port)**

1. Build the frontend (if not already built):
```bash
cd frontend
npm run build
cd ..
```

2. Run the backend server (serves both API and frontend on port 8080):
```bash
cd backend
go run cmd/signaling/main.go
```

3. Run the publisher service (in another terminal):
```bash
cd backend
go run cmd/publisher/main.go
```

4. Open your browser and navigate to:
```
http://localhost:8080
```

The backend server serves both:
- WebSocket signaling at `ws://localhost:8080/ws`
- Frontend application at `http://localhost:8080`

**Option 3: Development Mode (Separate Ports)**

If you want to run the frontend dev server separately during development:

1. Run the signaling server:
```bash
cd backend
go run cmd/signaling/main.go
```

2. Run the publisher service (in another terminal):
```bash
cd backend
go run cmd/publisher/main.go
```

3. Run the frontend dev server (in a third terminal):
```bash
cd frontend
npm run dev
```

4. The frontend will be available at `http://localhost:5173` (or the port shown in the terminal)

## Usage

1. **Configure the backend**: Edit `backend/.env` with your settings (especially `RTSP_URL` if using an IP camera)
2. **Build the frontend**: `cd frontend && npm run build`
3. **Start the signaling server**: `cd backend && go run cmd/signaling/main.go`
4. **Start the publisher service**: In another terminal, run `cd backend && go run cmd/publisher/main.go`
5. **Open the frontend**: Navigate to `http://localhost:8080` in your browser
6. **Connect**: Click the "Connect" button to establish the WebRTC connection
7. **Watch**: The video stream should appear in the viewer once the connection is established

**Quick Start (Using Scripts):**
```bash
# Build frontend
cd frontend && npm run build && cd ..

# Start services
./start_background.sh

# In another terminal, start publisher
cd backend && go run cmd/publisher/main.go
```

**Stopping Services:**
```bash
./stop_services.sh
```

## Configuration

All configurations are loaded from `.env` files:

### Backend Configuration

- **SIGNALING_SERVER_HOST**: Host for the signaling server
- **SIGNALING_SERVER_PORT**: Port for the signaling server
- **PUBLISHER_SERVER_HOST**: Host for the publisher service
- **PUBLISHER_SERVER_PORT**: Port for the publisher service
- **ICE_SERVER_URLS**: Comma-separated list of STUN/TURN server URLs
- **ICE_SERVER_USERNAME**: Optional username for TURN server
- **ICE_SERVER_CREDENTIAL**: Optional credential for TURN server
- **VIDEO_DEVICE_INDEX**: Camera device index (default: 0)
- **VIDEO_WIDTH**: Video width in pixels (default: 1280)
- **VIDEO_HEIGHT**: Video height in pixels (default: 720)
- **VIDEO_FPS**: Frames per second (default: 30)
- **RTSP_URL**: RTSP stream URL for IP camera streaming (optional)
- **ALLOWED_ORIGINS**: Comma-separated list of allowed CORS origins

### Frontend Configuration (Optional - for development only)

- **VITE_SIGNALING_SERVER_URL**: WebSocket URL for the signaling server (only needed in dev mode)
- **VITE_ICE_SERVER_URLS**: Comma-separated list of STUN/TURN server URLs

**Note:** In production mode (single port), the frontend automatically uses the same origin for WebSocket connections, so these environment variables are not needed.

## Video Sources

### RTSP Stream (IP Camera)

The system supports RTSP streams from IP cameras. To use an RTSP stream:

1. Add `RTSP_URL` to your `.env` file:
   ```env
   RTSP_URL=rtsp://username:password@ip:port/path
   ```

2. Make sure FFmpeg is installed (see Prerequisites)

3. URL-encode special characters in the RTSP URL:
   - `@` becomes `%40`
   - `:` becomes `%3A` (if needed in password)
   - Example: `rtsp://admin:Tatva%40321@183.82.113.87:554/Streaming/Channels/301`

The system will automatically use the RTSP source when `RTSP_URL` is configured. If not provided, it falls back to a mock video source for testing.

### Other Video Sources

To add support for other video sources (USB camera, file, etc.):

1. Implement the `VideoSource` interface in `backend/internal/video/capture.go`
2. Modify `NewVideoSource()` to return your implementation based on configuration
3. For encoding, you can use:
   - FFmpeg (as shown in RTSP implementation)
   - libvpx for VP8/VP9
   - x264 for H264
   - Platform-specific libraries (v4l2, AVFoundation, DirectShow)

## Development

### Project Structure

```
webrtc/
├── backend/
│   ├── cmd/
│   │   ├── signaling/          # Signaling server (WebSocket + HTTP)
│   │   └── publisher/          # Publisher service (WebRTC publisher)
│   ├── internal/
│   │   ├── config/             # Configuration management
│   │   ├── signaling/          # WebSocket signaling logic
│   │   └── video/              # Video capture and RTSP handling
│   ├── go.mod                  # Go dependencies
│   ├── Makefile                # Build shortcuts
│   └── .env                    # Backend configuration
├── frontend/
│   ├── src/
│   │   ├── components/         # React components (VideoViewer)
│   │   ├── hooks/              # Custom React hooks (useWebRTC)
│   │   ├── config/             # Frontend configuration
│   │   └── App.tsx             # Main app component
│   ├── dist/                   # Built frontend (served by backend)
│   ├── package.json
│   └── vite.config.ts          # Vite configuration
├── logs/                       # Service logs (when running in background)
├── start.sh                    # Start script (foreground)
├── start_background.sh         # Start script (background)
├── stop_services.sh            # Stop all services
└── package.json                # Root package.json with npm scripts
```

### Building Backend

```bash
cd backend

# Build signaling server
go build -o bin/signaling cmd/signaling/main.go

# Build publisher
go build -o bin/publisher cmd/publisher/main.go

# OR use Makefile
make build
```

### Building Frontend

```bash
cd frontend
npm run build
```

The built files will be in `frontend/dist/`, which the backend serves statically.

## Troubleshooting

### Common Issues

1. **Frontend Not Loading**: 
   - Make sure you've built the frontend first: `cd frontend && npm run build`
   - Check that `STATIC_FILES_PATH` in `backend/.env` points to the correct frontend `dist` directory (default: `../frontend/dist`)

2. **Static Files Not Found**: 
   - Verify the `frontend/dist` directory exists
   - Check that `STATIC_FILES_PATH` in `.env` uses the correct relative or absolute path
   - Ensure the path uses forward slashes `/` even on Windows

3. **Connection Issues**: 
   - Ensure both services (signaling server and publisher) are running
   - Check that ports are not blocked by firewall
   - Verify ports in `.env` match what you're trying to access

4. **Video Not Appearing**: 
   - Open browser console (F12) and check for WebRTC errors
   - Verify the publisher service is running and connected
   - Check that ICE connection state is "connected" (not "checking" or "failed")
   - Ensure browser supports WebRTC (Chrome, Firefox, Edge recommended)

5. **ICE Connection Fails**: 
   - Verify STUN/TURN servers are accessible
   - Check firewall isn't blocking UDP/TCP traffic
   - Try adding more STUN servers in `ICE_SERVER_URLS`
   - For strict NATs, consider using a TURN server

6. **CORS Errors**: 
   - Add your frontend URL to `ALLOWED_ORIGINS` in backend `.env`
   - In development, include `http://localhost:5173` if using Vite dev server

7. **WebSocket Connection Fails**: 
   - In production mode, ensure you're accessing via `http://localhost:8080` (same port as backend)
   - Check that the signaling server is running
   - Verify WebSocket URL in browser console

8. **RTSP Stream Not Working**: 
   - Verify FFmpeg is installed: `ffmpeg -version`
   - Check RTSP URL is correct and accessible
   - Ensure URL encoding is correct (e.g., `@` as `%40`)
   - Test RTSP stream with: `ffplay rtsp://your-stream-url`
   - Check firewall/network connectivity to the RTSP server
   - Look for errors in publisher logs

9. **Services Won't Start**: 
   - Check if ports are already in use: `lsof -i :8080` (or your configured port)
   - Use `./stop_services.sh` to stop any running instances
   - Verify Go and Node.js are installed correctly

10. **Video Freezes or Drops**: 
    - Check network bandwidth
    - Verify video codec (H.264 recommended for RTSP)
    - Monitor CPU usage (FFmpeg transcoding can be CPU-intensive)
    - Check publisher logs for frame capture errors

### Debugging Tips

- **Check Logs**: When running in background, check `logs/signaling.log` and `logs/publisher.log`
- **Browser DevTools**: Use `chrome://webrtc-internals/` in Chrome to inspect WebRTC connections
- **Network Tab**: Check WebSocket messages in browser Network tab
- **Console Logs**: Frontend includes detailed logging in browser console

### Getting Help

If issues persist:
1. Check all services are running: `ps aux | grep -E "(signaling|publisher)"`
2. Verify environment variables: `cat backend/.env`
3. Test WebSocket manually: Use a WebSocket client to connect to `ws://localhost:8080/ws`
4. Check FFmpeg: Test RTSP stream directly with `ffplay`

## License

MIT

