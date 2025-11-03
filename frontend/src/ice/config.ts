/**
 * Centralized WebRTC ICE/STUN/TURN configuration
 */

// Default STUN servers for redundancy (Google's public STUN servers)
const DEFAULT_STUN_SERVERS = [
  'stun:stun1.l.google.com:19302',
  'stun:stun2.l.google.com:19302',
];

// Default fallback STUN server
const DEFAULT_STUN_SERVER = 'stun:stun.l.google.com:19302';

// Parses ICE server URLs from environment variable or returns defaults
const parseICEServerURLs = (): string[] => {
  const envUrls = import.meta.env.VITE_ICE_SERVER_URLS;
  if (envUrls) {
    return envUrls.split(',').map((url: string) => url.trim()).filter(Boolean);
  }
  return [DEFAULT_STUN_SERVER];
};

// Creates an RTCConfiguration object with optimized ICE/STUN/TURN settings
export const getWebRTCConfiguration = (): RTCConfiguration => {
  const iceServerUrls = parseICEServerURLs();

  const iceServers: RTCIceServer[] = [
    ...iceServerUrls.map((url: string) => ({ urls: url })),
    ...DEFAULT_STUN_SERVERS
      .filter(stun => !iceServerUrls.some(url => url.includes(stun.split(':')[1])))
      .map(url => ({ urls: url })),
  ];

  const config: RTCConfiguration = {
    iceServers,
    iceTransportPolicy: 'all',
  };

  console.log('🔧 WebRTC Configuration:', {
    iceServers: config.iceServers?.map(s => s.urls).flat() || [],
    iceTransportPolicy: config.iceTransportPolicy,
  });

  return config;
};

export const getDefaultSTUNServer = (): string => DEFAULT_STUN_SERVER;


