import { useEffect, useRef, useState } from 'react';
import config from '../config/config';

interface CandidateMessage {
  type: 'candidate';
  candidate: RTCIceCandidateInit;
  clientId?: string;
}

interface AnswerMessage {
  type: 'answer';
  answer: RTCSessionDescriptionInit;
  clientId?: string;
}

export const useWebRTC = () => {
  const [isConnected, setIsConnected] = useState(false);
  const [connectionState, setConnectionState] = useState<RTCIceConnectionState>('new');
  const [hasTrack, setHasTrack] = useState(false); // Track if we've received a track
  const videoRef = useRef<HTMLVideoElement>(null);
  const peerConnectionRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const candidateQueueRef = useRef<RTCIceCandidateInit[]>([]);
  const remoteDescriptionSetRef = useRef(false);
  const mediaStreamRef = useRef<MediaStream | null>(null);
  const clientIdRef = useRef<string | null>(null); // Track our client ID
  const isConnectingRef = useRef(false); // Prevent concurrent connections
  const isDisconnectingRef = useRef(false); // Prevent race conditions during disconnect

  const createPeerConnection = () => {
    // Get ICE servers from environment or use defaults
    const iceServerUrls = import.meta.env.VITE_ICE_SERVER_URLS?.split(',') || ['stun:stun.l.google.com:19302'];
    
    // Build ICE servers array - add multiple STUN servers for redundancy
    const iceServers: RTCConfiguration = {
      iceServers: [
        ...iceServerUrls.map((url: string) => ({ urls: url.trim() })),
        // Add backup STUN servers
        { urls: 'stun:stun1.l.google.com:19302' },
        { urls: 'stun:stun2.l.google.com:19302' },
      ],
      iceTransportPolicy: 'all', // Allow all transport types (UDP, TCP)
    };

    console.log('🔧 Creating PeerConnection with ICE servers:', iceServers.iceServers);
    const pc = new RTCPeerConnection(iceServers);

    // Handle ICE candidate events
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        console.log('🧊 Local ICE candidate:', {
          candidate: event.candidate.candidate,
          sdpMLineIndex: event.candidate.sdpMLineIndex,
          sdpMid: event.candidate.sdpMid
        });
        
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          const candidateMsg: CandidateMessage = {
            type: 'candidate',
            candidate: event.candidate,
          };
          // Include our client ID if we know it
          if (clientIdRef.current) {
            candidateMsg.clientId = clientIdRef.current;
          }
          wsRef.current.send(JSON.stringify(candidateMsg));
          console.log('📤 Sent ICE candidate to publisher');
        } else {
          console.warn('⚠️ WebSocket not open, cannot send candidate');
        }
      } else {
        console.log('✅ All ICE candidates gathered');
      }
    };

    // Handle ICE connection state changes
    pc.oniceconnectionstatechange = () => {
      const state = pc.iceConnectionState;
      console.log('🧊 ICE connection state:', state);
      
      if (state === 'disconnected') {
        console.warn('⚠️ ICE disconnected - connection may be lost');
        console.warn('   Check: 1) Network connection 2) Firewall settings 3) ICE candidates exchanged');
        setConnectionState(state);
      } else if (state === 'failed') {
        console.error('❌ ICE connection failed!');
        console.error('   This usually means:');
        console.error('   1) No valid ICE candidates to connect');
        console.error('   2) Firewall blocking UDP/TCP');
        console.error('   3) NAT traversal failed');
        setConnectionState(state);
      } else if (state === 'connected') {
        console.log('✅ ICE connected - media should flow now!');
        console.log('   Video should start displaying if track is received');
        setConnectionState(state);
        // Force video to play if it's already received
        if (videoRef.current && videoRef.current.srcObject) {
          setTimeout(() => {
            videoRef.current?.play().catch(err => {
              console.error('Error playing after ICE connect:', err);
            });
          }, 100);
        }
      } else if (state === 'checking') {
        console.log('⏳ ICE checking connectivity...');
        console.log('   This can take 10-30 seconds depending on network');
        console.log('   Video will display once ICE connects');
        setConnectionState(state);
      } else {
        setConnectionState(state);
      }
    };
    
    // Also handle peer connection state
    pc.onconnectionstatechange = () => {
      const state = pc.connectionState;
      const iceState = pc.iceConnectionState;
      console.log('📡 Peer connection state:', state, '(ICE:', iceState + ')');
      
      // Only set connected if both PC and ICE are connected
      // This ensures media can actually flow
      const isFullyConnected = state === 'connected' && iceState === 'connected';
      setIsConnected(isFullyConnected);
      
      if (state === 'connected' && iceState !== 'connected') {
        console.warn('⚠️ PeerConnection is "connected" but ICE is not - media may not flow');
        console.warn('   Waiting for ICE to connect...');
      } else if (isFullyConnected) {
        console.log('✅ Fully connected - both PC and ICE are connected, media should flow!');
      }
    };

    // Handle incoming tracks
    pc.ontrack = async (event) => {
      // Ensure this is the current peer connection
      if (pc !== peerConnectionRef.current) {
        console.warn('⚠️ Received track from stale peer connection, ignoring');
        return;
      }

      console.log('✅ Received track event:', {
        kind: event.track.kind,
        label: event.track.label,
        id: event.track.id,
        enabled: event.track.enabled,
        readyState: event.track.readyState,
        streams: event.streams.length
      });
      
      if (!event.track) {
        console.error('❌ No track in event');
        return;
      }
      
      // Ensure track is enabled
      event.track.enabled = true;
      
      // Always create a fresh stream to avoid stale tracks from previous connections
      // Remove old stream tracks if they exist
      if (mediaStreamRef.current) {
        const oldTracks = mediaStreamRef.current.getTracks();
        oldTracks.forEach(track => {
          track.stop();
          mediaStreamRef.current!.removeTrack(track);
        });
      }
      
      // Create new MediaStream
      mediaStreamRef.current = new MediaStream();
      mediaStreamRef.current.addTrack(event.track);
      console.log('🎥 Created new MediaStream with track:', {
        kind: event.track.kind,
        id: event.track.id,
        enabled: event.track.enabled
      });
      
      // Only set hasTrack for video tracks
      if (event.track.kind === 'video') {
        setHasTrack(true);
        console.log('📹 Video track received, hasTrack set to true');
      }
      
      if (!videoRef.current) {
        console.error('❌ Video element ref is null!');
        return;
      }
      
      // Set video source with the accumulated stream
      const stream = mediaStreamRef.current;
      
      // Verify stream is valid and has active tracks
      if (!stream || stream.getTracks().length === 0) {
        console.error('❌ Stream is invalid or has no tracks');
        return;
      }
      
      const activeTracks = stream.getTracks().filter(t => t.readyState === 'live');
      
      // Define attemptPlay function early so it can be used in event listeners
      const attemptPlay = async () => {
        if (!videoRef.current || !videoRef.current.srcObject) {
          console.warn('⚠️ Video element or srcObject missing during play attempt');
          return;
        }
        
        // Ensure the video element is ready
        if (videoRef.current.readyState === 0) {
          // HAVE_NOTHING - need to load first
          console.log('⏳ Video element needs to load, calling load()...');
          videoRef.current.load();
        }
        
        try {
          // Check if video has metadata
          if (videoRef.current.readyState >= 2) {
            // HAVE_CURRENT_DATA - can play
            console.log('▶️ Video ready, attempting play...');
            await videoRef.current.play();
            console.log('✅ Video playback started successfully!');
            console.log('Video element state:', {
              readyState: videoRef.current.readyState,
              paused: videoRef.current.paused,
              muted: videoRef.current.muted,
              width: videoRef.current.videoWidth,
              height: videoRef.current.videoHeight,
              srcObject: videoRef.current.srcObject ? 'set' : 'not set'
            });
          } else {
            // Wait for metadata with timeout
            console.log('⏳ Waiting for video metadata (readyState:', videoRef.current.readyState, ')');
            
            let metadataResolved = false;
            const playAfterMetadata = async () => {
              if (metadataResolved) return;
              metadataResolved = true;
              console.log('✅ Metadata loaded, attempting play');
              try {
                if (videoRef.current && videoRef.current.srcObject === stream) {
                  await videoRef.current.play();
                  console.log('✅ Video playback started after metadata loaded!');
                }
              } catch (err) {
                console.error('❌ Error playing after metadata:', err);
              }
            };
            
            videoRef.current.addEventListener('loadedmetadata', playAfterMetadata, { once: true });
            videoRef.current.addEventListener('canplay', playAfterMetadata, { once: true });
            videoRef.current.addEventListener('loadeddata', playAfterMetadata, { once: true });
            
            // Fallback: try to play anyway after a short delay
            setTimeout(playAfterMetadata, 500);
          }
        } catch (err) {
          console.error('❌ Error playing video:', err);
          const error = err as Error;
          console.error('Error details:', {
            name: error.name,
            message: error.message
          });
        }
      };
      
      if (activeTracks.length === 0) {
        console.warn('⚠️ Stream has no live tracks yet, will wait for track to become live');
        // Set up listener for when track becomes live
        event.track.addEventListener('started', () => {
          if (videoRef.current && mediaStreamRef.current === stream && pc === peerConnectionRef.current) {
            console.log('✅ Track became live, updating video element');
            videoRef.current.srcObject = stream;
            attemptPlay();
          }
        }, { once: true });
        return;
      }
      
      // CRITICAL: Clear old srcObject first to ensure proper update
      // This is especially important after reconnection
      if (videoRef.current.srcObject) {
        console.log('🧹 Clearing old srcObject before setting new stream');
        const oldStream = videoRef.current.srcObject;
        videoRef.current.srcObject = null;
        // Force a small delay to ensure the old stream is fully cleared
        await new Promise(resolve => setTimeout(resolve, 50));
        // Clean up old stream tracks
        if (oldStream instanceof MediaStream) {
          oldStream.getTracks().forEach(track => {
            if (track.readyState !== 'ended') {
              track.stop();
            }
          });
        }
      }
      
      // Set new stream
      console.log('🎬 Setting new stream on video element:', {
        streamId: stream.id,
        trackCount: stream.getTracks().length,
        activeTrackCount: activeTracks.length
      });
      videoRef.current.srcObject = stream;
      videoRef.current.muted = true; // Required for autoplay
      videoRef.current.playsInline = true; // For mobile
      
      console.log('✅ Video srcObject set, stream:', {
        id: stream.id,
        active: stream.active,
        tracks: stream.getTracks().map(t => ({
          kind: t.kind,
          enabled: t.enabled,
          readyState: t.readyState,
          id: t.id
        }))
      });
      
      // Try immediately, then with delays (attemptPlay is already defined above)
      attemptPlay();
      setTimeout(attemptPlay, 100);
      setTimeout(attemptPlay, 500);
      setTimeout(attemptPlay, 1000);
      
      // Monitor track state changes
      event.track.onended = () => {
        console.log('⚠️ Track ended');
        setHasTrack(false);
        if (mediaStreamRef.current) {
          mediaStreamRef.current.removeTrack(event.track);
        }
      };
      
      event.track.onmute = () => {
        console.log('⚠️ Track muted - video may not display');
      };
      
      event.track.onunmute = () => {
        console.log('✅ Track unmuted');
      };
      
      // Monitor track enabled state
      event.track.addEventListener('ended', () => {
        console.log('📹 Track ended event fired');
        setHasTrack(false);
      });
      
      // Monitor when track becomes active
      const checkTrackActive = () => {
        if (event.track.readyState === 'live') {
          console.log('✅ Track is now live!');
          attemptPlay();
        }
      };
      
      // Check immediately and set up listener
      checkTrackActive();
      if (event.track.readyState === 'live') {
        console.log('✅ Track is already live');
      } else {
        console.log('⏳ Waiting for track to become live (current state:', event.track.readyState, ')');
        event.track.addEventListener('started', checkTrackActive);
      }
    };

    peerConnectionRef.current = pc;
    return pc;
  };

  const connect = async () => {
    // Prevent concurrent connections
    if (isConnectingRef.current || isDisconnectingRef.current) {
      console.log('⚠️ Connection already in progress or disconnecting, skipping...');
      return;
    }

    // If already connected, disconnect first
    if (peerConnectionRef.current || wsRef.current?.readyState === WebSocket.OPEN) {
      console.log('⚠️ Already connected, disconnecting first...');
      disconnect();
      // Wait a bit for cleanup
      await new Promise(resolve => setTimeout(resolve, 200));
    }

    isConnectingRef.current = true;

    try {
      console.log('🔌 Attempting to connect to:', config.signalingServerUrl);
      
      // Reset state before connecting
      setIsConnected(false);
      setConnectionState('new');
      setHasTrack(false);
      clientIdRef.current = null;
      remoteDescriptionSetRef.current = false;
      candidateQueueRef.current = [];
      
      // Ensure old references are cleared
      if (peerConnectionRef.current) {
        try {
          peerConnectionRef.current.close();
        } catch (err) {
          console.warn('Error closing old peer connection:', err);
        }
        peerConnectionRef.current = null;
      }
      
      if (mediaStreamRef.current) {
        try {
          mediaStreamRef.current.getTracks().forEach(track => track.stop());
        } catch (err) {
          console.warn('Error stopping old tracks:', err);
        }
        mediaStreamRef.current = null;
      }
      
      // Connect to signaling server
      const ws = new WebSocket(config.signalingServerUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setIsConnected(true);
        setConnectionState('checking');
        isConnectingRef.current = false;
        console.log('✅ Connected to signaling server');
      };

      ws.onclose = (event) => {
        isConnectingRef.current = false;
        setIsConnected(false);
        setConnectionState('disconnected');
        console.log('❌ Disconnected from signaling server', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean
        });
      };

      ws.onerror = (error) => {
        isConnectingRef.current = false;
        console.error('❌ WebSocket connection error:', error);
        console.error('Failed to connect to:', config.signalingServerUrl);
        console.error('Make sure the signaling server is running on port 8081');
        setIsConnected(false);
        setConnectionState('failed');
      };

      ws.onmessage = async (event) => {
        interface WebRTCMessage {
          type: string;
          offer?: RTCSessionDescriptionInit;
          answer?: RTCSessionDescriptionInit;
          candidate?: RTCIceCandidateInit | { candidate?: string; sdpMLineIndex?: number; sdpMid?: string };
          clientId?: string;
        }
        const message = JSON.parse(event.data) as WebRTCMessage;
        console.log('📥 Received message:', message.type, message);

        // Track our client ID from any message (signaling server adds it)
        if (message.clientId && !clientIdRef.current) {
          clientIdRef.current = message.clientId;
          console.log('✅ Identified client ID:', message.clientId);
        }

        // Ensure peer connection exists
        if (!peerConnectionRef.current) {
          console.log('Creating peer connection...');
          createPeerConnection();
        }

        switch (message.type) {
          case 'offer':
            // Only process offers that are meant for us (if clientId is specified)
            if (message.clientId && clientIdRef.current && message.clientId !== clientIdRef.current) {
              console.log('⚠️ Ignoring offer meant for different client:', message.clientId, '(we are:', clientIdRef.current, ')');
              break;
            }
            console.log('📥 Received offer from publisher:', message.offer);
            if (message.offer && peerConnectionRef.current) {
              try {
                console.log('🔧 Setting remote description (offer)...');
                const offerDesc = new RTCSessionDescription(message.offer);
                console.log('   Offer type:', offerDesc.type, 'SDP length:', offerDesc.sdp?.length || 0);
                await peerConnectionRef.current.setRemoteDescription(offerDesc);
                remoteDescriptionSetRef.current = true;
                console.log('✅ Remote description set');
                
                // Process any buffered candidates
                console.log(`Processing ${candidateQueueRef.current.length} buffered candidates...`);
                for (const candidate of candidateQueueRef.current) {
                  try {
                    await peerConnectionRef.current.addIceCandidate(new RTCIceCandidate(candidate));
                    console.log('✅ Added buffered candidate');
                  } catch (err) {
                    console.error('Error adding buffered candidate:', err);
                  }
                }
                candidateQueueRef.current = [];
                
                console.log('Creating answer...');
                const answer = await peerConnectionRef.current.createAnswer();
                await peerConnectionRef.current.setLocalDescription(answer);
                console.log('✅ Local description set');
                
                console.log('Sending answer to publisher...');
                const answerMsg: AnswerMessage = {
                  type: 'answer',
                  answer: answer,
                };
                // Include our client ID if we know it
                if (clientIdRef.current) {
                  answerMsg.clientId = clientIdRef.current;
                }
                ws.send(JSON.stringify(answerMsg));
                console.log('✅ Answer sent, waiting for ICE connection...');
              } catch (error) {
                console.error('Error handling offer:', error);
              }
            }
            break;

          case 'answer':
            if (message.answer && peerConnectionRef.current) {
              await peerConnectionRef.current.setRemoteDescription(
                new RTCSessionDescription(message.answer)
              );
            }
            break;

          case 'candidate':
            // Only process candidates that are meant for us (if clientId is specified)
            if (message.clientId && clientIdRef.current && message.clientId !== clientIdRef.current) {
              console.log('⚠️ Ignoring candidate meant for different client:', message.clientId);
              break;
            }

            if (message.candidate && peerConnectionRef.current) {
              try {
                // Handle both formats: direct candidate object or nested candidate
                let candidateData = message.candidate;
                if (typeof candidateData === 'object' && candidateData.candidate) {
                  candidateData = {
                    candidate: candidateData.candidate,
                    sdpMLineIndex: candidateData.sdpMLineIndex,
                    sdpMid: candidateData.sdpMid,
                  };
                }
                
                console.log('🧊 Received remote ICE candidate:', {
                  candidate: candidateData.candidate?.substring(0, 50) + '...',
                  sdpMLineIndex: candidateData.sdpMLineIndex,
                  sdpMid: candidateData.sdpMid
                });
                
                // If remote description is not set yet, buffer the candidate
                if (!remoteDescriptionSetRef.current) {
                  console.log('⏳ Buffering candidate (remote description not set yet)');
                  candidateQueueRef.current.push(candidateData);
                } else {
                  await peerConnectionRef.current.addIceCandidate(
                    new RTCIceCandidate(candidateData)
                  );
                  console.log('✅ Added remote ICE candidate');
                }
              } catch (error) {
                console.error('❌ Error adding ICE candidate:', error);
              }
            }
            break;
        }
      };

      // Create peer connection immediately when WebSocket connects
      createPeerConnection();
      remoteDescriptionSetRef.current = false;
      candidateQueueRef.current = [];
      console.log('Peer connection created, waiting for offer...');
    } catch (error) {
      isConnectingRef.current = false;
      console.error('Error connecting:', error);
      setIsConnected(false);
      setConnectionState('failed');
    }
  };

  const disconnect = () => {
    // Prevent concurrent disconnect calls
    if (isDisconnectingRef.current) {
      return;
    }
    isDisconnectingRef.current = true;
    isConnectingRef.current = false;

    console.log('🔌 Disconnecting...');

    // Close and clean up peer connection
    if (peerConnectionRef.current) {
      try {
        peerConnectionRef.current.close();
      } catch (err) {
        console.error('Error closing peer connection:', err);
      }
      peerConnectionRef.current = null;
    }

    // Close and clean up WebSocket
    if (wsRef.current) {
      try {
        // Remove all event listeners by replacing with no-op handlers
        wsRef.current.onopen = null;
        wsRef.current.onclose = null;
        wsRef.current.onerror = null;
        wsRef.current.onmessage = null;
        
        // Close WebSocket properly with close code
        if (wsRef.current.readyState === WebSocket.OPEN || wsRef.current.readyState === WebSocket.CONNECTING) {
          wsRef.current.close(1000, 'Normal closure');
        } else {
          wsRef.current.close();
        }
      } catch (err) {
        console.error('Error closing WebSocket:', err);
      }
      wsRef.current = null;
    }

    // Clean up video element
    if (videoRef.current) {
      try {
        videoRef.current.pause();
        // Clear srcObject but don't call load() - it can interfere with future streams
        // The load() will be called when we set a new stream if needed
        videoRef.current.srcObject = null;
      } catch (err) {
        console.error('Error cleaning up video element:', err);
      }
    }

    // Clean up media stream and tracks
    if (mediaStreamRef.current) {
      try {
        mediaStreamRef.current.getTracks().forEach(track => {
          track.stop();
          track.enabled = false;
        });
      } catch (err) {
        console.error('Error stopping tracks:', err);
      }
      mediaStreamRef.current = null;
    }

    // Reset all state
    setIsConnected(false);
    setConnectionState('closed');
    setHasTrack(false);
    
    // Reset all refs
    clientIdRef.current = null;
    remoteDescriptionSetRef.current = false;
    candidateQueueRef.current = [];

    console.log('✅ Disconnected and cleaned up');
    
    // Reset disconnecting flag after a short delay to allow cleanup to complete
    setTimeout(() => {
      isDisconnectingRef.current = false;
    }, 100);
  };

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, []);

  return {
    isConnected,
    connectionState,
    hasTrack,
    videoRef,
    connect,
    disconnect,
  };
};

