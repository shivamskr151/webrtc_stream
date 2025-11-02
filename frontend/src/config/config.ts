interface Config {
  signalingServerUrl: string;
}

// Get WebSocket URL - use same origin in production, or configured URL in development
const getSignalingUrl = () => {
  // If running in production (same port as backend), use relative WebSocket URL
  if (import.meta.env.PROD) {
    // Check if we have a valid host (not file:// protocol or empty host)
    if (window.location.host && window.location.protocol !== 'file:') {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      return `${protocol}//${window.location.host}/ws`;
    }
    
    // Fallback for production builds opened via file:// or when host is not available
    // Use environment variable if available, otherwise default to localhost:8081
    if (import.meta.env.VITE_SIGNALING_SERVER_URL) {
      return import.meta.env.VITE_SIGNALING_SERVER_URL;
    }
    
    // Default fallback for production (matches backend default configuration)
    return 'ws://localhost:8081/ws';
  }
  
  // In development, use environment variable or default to port 8081
  if (import.meta.env.VITE_SIGNALING_SERVER_URL) {
    return import.meta.env.VITE_SIGNALING_SERVER_URL;
  }
  
  // Default fallback to port 8081 (matches backend configuration)
  return 'ws://localhost:8081/ws';
};

export const config: Config = {
  signalingServerUrl: getSignalingUrl(),
};

export default config;

