# WebRTC Frontend - Video Viewer

A modern React + TypeScript frontend application for viewing WebRTC video streams. This frontend connects to the signaling server via WebSocket and establishes a peer-to-peer connection with the publisher service to receive and display video streams.

## Features

- 🎥 Real-time video streaming via WebRTC
- 🔄 Automatic WebRTC connection management
- 📊 Connection status indicators
- 🎨 Modern, responsive UI built with React and TypeScript
- 🔌 WebSocket signaling integration
- 🧊 ICE candidate handling
- 📱 Mobile-friendly interface

## Tech Stack

- **React 19** - UI framework
- **TypeScript** - Type-safe JavaScript
- **Vite** - Build tool and dev server
- **Tailwind CSS** - Styling (via PostCSS)
- **WebRTC API** - Real-time video streaming

## Project Structure

```
frontend/
├── src/
│   ├── components/
│   │   └── VideoViewer.tsx      # Main video viewer component
│   ├── hooks/
│   │   └── useWebRTC.ts         # Custom hook for WebRTC logic
│   ├── config/
│   │   └── config.ts             # Configuration (signaling URL)
│   ├── App.tsx                  # Root app component
│   ├── main.tsx                 # Application entry point
│   └── index.css                # Global styles
├── dist/                        # Built production files (generated)
├── public/                      # Static assets
├── index.html                   # HTML template
├── package.json                 # Dependencies and scripts
├── vite.config.ts               # Vite configuration
├── tsconfig.json                # TypeScript configuration
└── tailwind.config.js           # Tailwind CSS configuration
```

## Setup

### Prerequisites

- Node.js 18+ and npm

### Installation

1. Install dependencies:
```bash
npm install
```

## Development

### Running the Development Server

```bash
npm run dev
```

The frontend will be available at `http://localhost:5173` (or another port if 5173 is in use).

**Note:** In development mode, you'll need to:
1. Run the signaling server: `cd ../backend && go run cmd/signaling/main.go`
2. Run the publisher service: `cd ../backend && go run cmd/publisher/main.go`
3. Make sure the signaling server WebSocket URL matches your configuration (default: `ws://localhost:8080/ws`)

### Environment Variables

Create a `.env` file in the `frontend` directory (optional for development):

```env
VITE_SIGNALING_SERVER_URL=ws://localhost:8080/ws
VITE_ICE_SERVER_URLS=stun:stun.l.google.com:19302
```

**Note:** In production mode (when built and served by the backend), the frontend automatically uses the same origin for WebSocket connections, so these environment variables are not needed.

## Building for Production

```bash
npm run build
```

This will create a `dist` directory with the production-ready build. The backend will serve these files.

The build output includes:
- Optimized JavaScript bundles
- Minified CSS
- Static assets
- Source maps (for debugging)

## Usage

1. **Build the frontend** (if not already built):
   ```bash
   npm run build
   ```

2. **Start the backend services**:
   - Signaling server: `cd ../backend && go run cmd/signaling/main.go`
   - Publisher service: `cd ../backend && go run cmd/publisher/main.go`

3. **Open the application**:
   - Production: `http://localhost:8080` (served by backend)
   - Development: `http://localhost:5173` (Vite dev server)

4. **Connect to stream**:
   - Click the "Connect" button
   - Wait for WebRTC connection to establish (ICE negotiation may take 10-30 seconds)
   - Video stream will appear once connection is established

## Components

### VideoViewer

The main component that displays the video stream and connection controls.

**Features:**
- Connection status indicator
- Connect/Disconnect button
- Video player with autoplay
- Connection state display (ICE state, connection status)
- Responsive layout

**Props:** None (uses `useWebRTC` hook internally)

### useWebRTC Hook

Custom React hook that manages WebRTC connection lifecycle.

**Responsibilities:**
- WebSocket connection to signaling server
- PeerConnection creation and management
- ICE candidate handling
- Offer/Answer SDP exchange
- Track reception and video playback
- Connection state management

**Returns:**
```typescript
{
  isConnected: boolean;
  connectionState: RTCIceConnectionState;
  hasTrack: boolean;
  videoRef: RefObject<HTMLVideoElement>;
  connect: () => void;
  disconnect: () => void;
}
```

## Configuration

The frontend configuration is handled in `src/config/config.ts`:

- **Production mode**: Automatically uses same origin for WebSocket (`ws://${host}/ws`)
- **Development mode**: Uses `VITE_SIGNALING_SERVER_URL` environment variable or defaults to `ws://localhost:8080/ws`

## WebRTC Flow

1. User clicks "Connect"
2. WebSocket connection established to signaling server
3. PeerConnection created with ICE servers
4. When publisher connects, offer is received
5. Answer is created and sent back
6. ICE candidates are exchanged
7. Once ICE connects, video track is received
8. Video element displays the stream

## Troubleshooting

### Video Not Appearing

1. **Check browser console** for errors
2. **Verify connection state**: Should show "connected" in the UI
3. **Check ICE state**: Should be "connected" (not "checking" or "failed")
4. **Ensure publisher is running**: The publisher service must be active
5. **Check WebSocket connection**: Should be connected to signaling server

### Connection Fails

1. **Verify signaling server URL**: Check `src/config/config.ts`
2. **Check CORS settings**: Ensure frontend URL is in backend `ALLOWED_ORIGINS`
3. **Test WebSocket manually**: Use browser console or a WebSocket client
4. **Check firewall**: Ensure ports aren't blocked

### ICE Connection Fails

1. **Check STUN/TURN servers**: Verify they're accessible
2. **Check network**: Firewall may be blocking UDP/TCP
3. **Try different network**: Mobile hotspot can help test NAT traversal
4. **Check browser**: Some browsers have better WebRTC support (Chrome, Firefox, Edge)

### Development Issues

1. **Port conflicts**: If port 5173 is in use, Vite will use another port
2. **CORS errors**: Add `http://localhost:5173` to backend `ALLOWED_ORIGINS`
3. **Hot reload not working**: Restart dev server
4. **Type errors**: Run `npm run build` to check TypeScript errors

## Browser Support

Recommended browsers:
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest, with limitations)

**Note:** Some browsers have better WebRTC and codec support. H.264 streams work best in Chrome/Edge.

## Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build locally
- `npm run lint` - Run ESLint

## License

MIT
