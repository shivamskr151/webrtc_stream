import { useEffect, useRef, useState } from 'react';
import config from '../config/config';

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
          wsRef.current.send(
            JSON.stringify({
              type: 'candidate',
              candidate: event.candidate,
            })
          );
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
    pc.ontrack = (event) => {
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
      
      // Create or get existing stream
      if (!mediaStreamRef.current) {
        mediaStreamRef.current = new MediaStream();
        console.log('🎥 Created new MediaStream');
      }
      
      // Add track to stream (avoid duplicates)
      const existingTrack = mediaStreamRef.current.getTracks().find(t => t.id === event.track.id);
      if (!existingTrack) {
        mediaStreamRef.current.addTrack(event.track);
        console.log('✅ Added track to stream:', {
          kind: event.track.kind,
          id: event.track.id,
          enabled: event.track.enabled
        });
      } else {
        console.log('⚠️ Track already in stream:', event.track.id);
      }
      
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
      
      // Force video to load and play
      const attemptPlay = async () => {
        if (!videoRef.current || !videoRef.current.srcObject) {
          console.warn('⚠️ Video element or srcObject missing during play attempt');
          return;
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
            // Wait for metadata
            console.log('⏳ Waiting for video metadata (readyState:', videoRef.current.readyState, ')');
            videoRef.current.addEventListener('loadedmetadata', async () => {
              console.log('✅ Metadata loaded, attempting play');
              try {
                await videoRef.current!.play();
                console.log('✅ Video playback started after metadata loaded!');
              } catch (err) {
                console.error('❌ Error playing after metadata:', err);
              }
            }, { once: true });
            
            // Also try on canplay
            videoRef.current.addEventListener('canplay', async () => {
              console.log('✅ Can play event, attempting play');
              try {
                await videoRef.current!.play();
                console.log('✅ Video playback started after canplay!');
              } catch (err) {
                console.error('❌ Error playing after canplay:', err);
              }
            }, { once: true });
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
      
      // Try immediately
      attemptPlay();
      
      // Also try after delays to handle async stream setup
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
    try {
      console.log('Attempting to connect to:', config.signalingServerUrl);
      
      // Connect to signaling server
      const ws = new WebSocket(config.signalingServerUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setIsConnected(true);
        setConnectionState('checking');
        console.log('✅ Connected to signaling server');
      };

      ws.onclose = (event) => {
        setIsConnected(false);
        setConnectionState('disconnected');
        console.log('❌ Disconnected from signaling server', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean
        });
      };

      ws.onerror = (error) => {
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

        // Ensure peer connection exists
        if (!peerConnectionRef.current) {
          console.log('Creating peer connection...');
          createPeerConnection();
        }

        switch (message.type) {
          case 'offer':
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
                ws.send(
                  JSON.stringify({
                    type: 'answer',
                    answer: answer,
                  })
                );
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
      console.error('Error connecting:', error);
      setIsConnected(false);
      setConnectionState('failed');
    }
  };

  const disconnect = () => {
    if (peerConnectionRef.current) {
      peerConnectionRef.current.close();
      peerConnectionRef.current = null;
    }
    if (wsRef.current) {
      // Close WebSocket properly with close code
      if (wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.close(1000, 'Normal closure');
      } else {
        wsRef.current.close();
      }
      wsRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
    if (mediaStreamRef.current) {
      mediaStreamRef.current.getTracks().forEach(track => track.stop());
      mediaStreamRef.current = null;
    }
    setIsConnected(false);
    setConnectionState('closed');
    setHasTrack(false);
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

