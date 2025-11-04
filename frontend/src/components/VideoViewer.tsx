import React, { useEffect, useState } from 'react';
import { useWebRTC } from '../hooks/useWebRTC';
import { captureVideoFrame, revokeSnapshot } from '../utils/snapshot';
import type { SnapshotResult } from '../utils/snapshot';

const VideoViewer: React.FC = () => {
  const { isConnected, connectionState, hasTrack, videoRef, connect, disconnect } = useWebRTC();
  const [lastSnapshot, setLastSnapshot] = useState<SnapshotResult | null>(null);
  const [isCapturing, setIsCapturing] = useState(false);

  useEffect(() => {
    return () => {
      if (lastSnapshot) revokeSnapshot(lastSnapshot);
    };
  }, [lastSnapshot]);

  const onCapture = async () => {
    if (!videoRef.current || isCapturing) return;
    setIsCapturing(true);
    try {
      const shot = await captureVideoFrame(videoRef.current);
      if (lastSnapshot) revokeSnapshot(lastSnapshot);
      setLastSnapshot(shot);
    } catch (e) {
      console.error('Failed to capture snapshot', e);
    } finally {
      setIsCapturing(false);
    }
  };

  const getConnectionStatusColor = () => {
    switch (connectionState) {
      case 'connected':
        return '#10b981';
      case 'checking':
        return '#f59e0b';
      case 'disconnected':
      case 'failed':
      case 'closed':
        return '#ef4444';
      default:
        return '#6b7280';
    }
  };

  return (
    <div style={{
      width: '100vw',
      height: '100vh',
      minHeight: '100vh',
      backgroundColor: '#f9fafb',
      display: 'flex',
      flexDirection: 'column',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      overflow: 'hidden'
    }}>
      {/* Main Container */}
      <div style={{
        width: '100%',
        height: '100%',
        backgroundColor: '#ffffff',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden'
      }}>
        {/* Header */}
        <div style={{
          backgroundColor: '#ffffff',
          padding: '28px 36px',
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '20px',
          borderBottom: '1px solid #e5e7eb',
          boxShadow: '0 2px 8px rgba(0, 0, 0, 0.04)',
          position: 'relative',
          zIndex: 10,
          backdropFilter: 'blur(10px)',
          background: 'linear-gradient(to bottom, #ffffff 0%, #fafafa 100%)'
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '12px'
          }}>
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 16px rgba(102, 126, 234, 0.4)',
              transition: 'transform 0.2s ease',
              cursor: 'pointer'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'scale(1.05) rotate(5deg)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'scale(1) rotate(0deg)';
            }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M15 10L19.553 7.276C19.8325 7.10759 20.1692 7.06481 20.4822 7.15765C20.7952 7.2505 21.0579 7.47162 21.2097 7.77086C21.3615 8.07011 21.3891 8.42095 21.2868 8.742C21.1845 9.06304 20.9609 9.32705 20.664 9.48L15 13M5 18H13C13.5304 18 14.0391 17.7893 14.4142 17.4142C14.7893 17.0391 15 16.5304 15 16V8C15 7.46957 14.7893 6.96086 14.4142 6.58579C14.0391 6.21071 13.5304 6 13 6H5C4.46957 6 3.96086 6.21071 3.58579 6.58579C3.21071 6.96086 3 7.46957 3 8V16C3 16.5304 3.21071 17.0391 3.58579 17.4142C3.96086 17.7893 4.46957 18 5 18Z" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </div>
            <h1 style={{
              fontSize: '28px',
              fontWeight: '700',
              color: '#111827',
              margin: 0,
              padding: 0,
              letterSpacing: '-0.5px',
              background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
              backgroundClip: 'text'
            }}>
              WebRTC Video Stream
            </h1>
          </div>
          
          <div style={{
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
            gap: '20px',
            flexWrap: 'wrap'
          }}>
            {/* Status Indicator */}
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              alignItems: 'center',
              gap: '10px',
              padding: '11px 20px',
              backgroundColor: '#ffffff',
              borderRadius: '12px',
              border: `2px solid ${getConnectionStatusColor()}40`,
              boxShadow: `0 3px 12px ${getConnectionStatusColor()}20, 0 1px 3px rgba(0, 0, 0, 0.1)`,
              transition: 'all 0.3s ease',
              backdropFilter: 'blur(10px)'
            }}>
              <div style={{
                width: '12px',
                height: '12px',
                borderRadius: '50%',
                backgroundColor: getConnectionStatusColor(),
                flexShrink: 0,
                boxShadow: `0 0 12px ${getConnectionStatusColor()}60`,
                animation: connectionState === 'connected' ? 'pulse 2s infinite' : 'none'
              }}></div>
              <span style={{
                fontSize: '14px',
                color: '#111827',
                textTransform: 'capitalize',
                whiteSpace: 'nowrap',
                fontWeight: '600',
                letterSpacing: '0.3px'
              }}>
                {connectionState}
              </span>
            </div>

            {/* Connect/Disconnect Button */}
            <button
              onClick={isConnected ? disconnect : connect}
              style={{
                padding: '12px 24px',
                backgroundColor: isConnected ? '#dc2626' : '#2563eb',
                color: '#ffffff',
                borderRadius: '12px',
                border: 'none',
                cursor: 'pointer',
                fontWeight: '600',
                fontSize: '15px',
                whiteSpace: 'nowrap',
                transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
                boxShadow: isConnected ? '0 4px 16px rgba(220, 38, 38, 0.35), 0 2px 4px rgba(220, 38, 38, 0.2)' : '0 4px 16px rgba(37, 99, 235, 0.35), 0 2px 4px rgba(37, 99, 235, 0.2)',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                position: 'relative',
                overflow: 'hidden'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = isConnected ? '#b91c1c' : '#1d4ed8';
                e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
                e.currentTarget.style.boxShadow = isConnected ? '0 8px 24px rgba(220, 38, 38, 0.45), 0 4px 8px rgba(220, 38, 38, 0.3)' : '0 8px 24px rgba(37, 99, 235, 0.45), 0 4px 8px rgba(37, 99, 235, 0.3)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = isConnected ? '#dc2626' : '#2563eb';
                e.currentTarget.style.transform = 'translateY(0) scale(1)';
                e.currentTarget.style.boxShadow = isConnected ? '0 4px 16px rgba(220, 38, 38, 0.35), 0 2px 4px rgba(220, 38, 38, 0.2)' : '0 4px 16px rgba(37, 99, 235, 0.35), 0 2px 4px rgba(37, 99, 235, 0.2)';
              }}
              onMouseDown={(e) => {
                e.currentTarget.style.transform = 'translateY(0) scale(0.98)';
              }}
              onMouseUp={(e) => {
                e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
              }}
            >
              {isConnected ? (
                <>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  Disconnect
                </>
              ) : (
                <>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  Connect
                </>
              )}
            </button>

            {/* Capture Snapshot Button */}
            <button
              onClick={onCapture}
              disabled={!hasTrack || isCapturing}
              style={{
                padding: '12px 24px',
                backgroundColor: hasTrack && !isCapturing ? '#059669' : '#9ca3af',
                color: '#ffffff',
                borderRadius: '12px',
                border: 'none',
                cursor: hasTrack && !isCapturing ? 'pointer' : 'not-allowed',
                fontWeight: '600',
                fontSize: '15px',
                whiteSpace: 'nowrap',
                transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
                boxShadow: hasTrack && !isCapturing ? '0 4px 16px rgba(5, 150, 105, 0.35), 0 2px 4px rgba(5, 150, 105, 0.2)' : 'none',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                position: 'relative',
                overflow: 'hidden'
              }}
              onMouseEnter={(e) => {
                if (hasTrack && !isCapturing) {
                  e.currentTarget.style.backgroundColor = '#047857';
                  e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
                  e.currentTarget.style.boxShadow = '0 8px 24px rgba(5, 150, 105, 0.45), 0 4px 8px rgba(5, 150, 105, 0.3)';
                }
              }}
              onMouseLeave={(e) => {
                if (hasTrack && !isCapturing) {
                  e.currentTarget.style.backgroundColor = '#059669';
                  e.currentTarget.style.transform = 'translateY(0) scale(1)';
                  e.currentTarget.style.boxShadow = '0 4px 16px rgba(5, 150, 105, 0.35), 0 2px 4px rgba(5, 150, 105, 0.2)';
                }
              }}
              onMouseDown={(e) => {
                if (hasTrack && !isCapturing) {
                  e.currentTarget.style.transform = 'translateY(0) scale(0.98)';
                }
              }}
              onMouseUp={(e) => {
                if (hasTrack && !isCapturing) {
                  e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
                }
              }}
            >
              {isCapturing ? (
                <>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ animation: 'spin 1s linear infinite' }}>
                    <path d="M12 2V6M12 18V22M6 12H2M22 12H18M19.07 19.07L16.24 16.24M19.07 4.93L16.24 7.76M4.93 19.07L7.76 16.24M4.93 4.93L7.76 7.76" stroke="currentColor" strokeWidth="2" strokeLinecap="round"/>
                  </svg>
                  Capturing...
                </>
              ) : (
                <>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M23 19C23 19.5304 22.7893 20.0391 22.4142 20.4142C22.0391 20.7893 21.5304 21 21 21H3C2.46957 21 1.96086 20.7893 1.58579 20.4142C1.21071 20.0391 1 19.5304 1 19V8C1 7.46957 1.21071 6.96086 1.58579 6.58579C1.96086 6.21071 2.46957 6 3 6H7L9 4H15L17 6H21C21.5304 6 22.0391 6.21071 22.4142 6.58579C22.7893 6.96086 23 7.46957 23 8V19Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    <circle cx="12" cy="13" r="4" stroke="currentColor" strokeWidth="2"/>
                  </svg>
                  Capture Snapshot
                </>
              )}
            </button>
          </div>
        </div>

        {/* Video and Snapshot Container - Side by side when snapshot exists */}
        <div style={{
          display: 'flex',
          flexDirection: 'row',
          width: '100%',
          flex: 1,
          overflow: 'hidden',
          gap: lastSnapshot ? '16px' : '0'
        }}>
          {/* Video Container */}
          <div style={{
            position: 'relative',
            width: lastSnapshot ? '60%' : '100%',
            flex: lastSnapshot ? '0 0 60%' : '1',
            backgroundColor: '#000000',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            overflow: 'hidden',
            transition: 'width 0.4s cubic-bezier(0.4, 0, 0.2, 1)',
            borderRadius: lastSnapshot ? '0 0 0 0' : '0',
            boxShadow: lastSnapshot ? 'inset -1px 0 0 rgba(255, 255, 255, 0.1)' : 'none'
          }}>
            <video
              ref={videoRef}
              autoPlay
              playsInline
              muted
              controls={false}
              preload="none"
              style={{
                width: '100%',
                height: '100%',
                objectFit: 'contain',
                display: hasTrack ? 'block' : 'none',
                backgroundColor: '#000000',
              }}
              onLoadedMetadata={() => {
                // Configure video element for low-latency playback
                if (videoRef.current) {
                  // Reduce buffering by setting small buffer sizes
                  // This helps with real-time streaming
                  try {
                    // Set playback rate to 1.0 to ensure smooth playback
                    videoRef.current.playbackRate = 1.0;
                  } catch {
                    // Ignore errors if property is not supported
                  }
                }
                console.log('✅ Video metadata loaded:', {
                  videoWidth: videoRef.current?.videoWidth,
                  videoHeight: videoRef.current?.videoHeight,
                  duration: videoRef.current?.duration,
                  srcObject: videoRef.current?.srcObject ? 'set' : 'not set'
                });
              }}
              onCanPlay={() => {
                console.log('✅ Video can play');
                if (videoRef.current) {
                  videoRef.current.play().catch(err => {
                    console.error('Auto-play prevented:', err);
                  });
                }
              }}
              onPlay={() => {
                console.log('✅ Video is playing');
              }}
              onPlaying={() => {
                console.log('✅ Video started playing');
              }}
              onError={(e) => {
                console.error('❌ Video element error:', e);
                if (videoRef.current) {
                  console.error('Video error details:', {
                    error: videoRef.current.error,
                    networkState: videoRef.current.networkState,
                    readyState: videoRef.current.readyState,
                    srcObject: videoRef.current.srcObject ? 'set' : 'not set'
                  });
                }
              }}
            />
            
            {!hasTrack && (
              <div style={{
                position: 'absolute',
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'linear-gradient(135deg, #1f2937 0%, #111827 100%)',
                flexDirection: 'column',
                gap: '20px',
                padding: '60px 40px'
              }}>
                <div style={{
                  width: '80px',
                  height: '80px',
                  borderRadius: '50%',
                  backgroundColor: '#374151',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  marginBottom: '8px',
                  boxShadow: '0 8px 24px rgba(0, 0, 0, 0.3)'
                }}>
                  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15 10L19.553 7.276C19.8325 7.10759 20.1692 7.06481 20.4822 7.15765C20.7952 7.2505 21.0579 7.47162 21.2097 7.77086C21.3615 8.07011 21.3891 8.42095 21.2868 8.742C21.1845 9.06304 20.9609 9.32705 20.664 9.48L15 13M5 18H13C13.5304 18 14.0391 17.7893 14.4142 17.4142C14.7893 17.0391 15 16.5304 15 16V8C15 7.46957 14.7893 6.96086 14.4142 6.58579C14.0391 6.21071 13.5304 6 13 6H5C4.46957 6 3.96086 6.21071 3.58579 6.58579C3.21071 6.96086 3 7.46957 3 8V16C3 16.5304 3.21071 17.0391 3.58579 17.4142C3.96086 17.7893 4.46957 18 5 18Z" stroke="#9ca3af" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </div>
                <div style={{
                  fontSize: '28px',
                  fontWeight: '700',
                  color: '#ffffff',
                  marginBottom: '4px',
                  letterSpacing: '-0.5px'
                }}>
                  No Stream
                </div>
                <div style={{
                  fontSize: '16px',
                  color: '#d1d5db',
                  textAlign: 'center',
                  lineHeight: '1.6',
                  maxWidth: '400px'
                }}>
                  Click the <strong style={{ color: '#ffffff', fontWeight: '600' }}>"Connect"</strong> button above to start receiving video stream
                </div>
                <div style={{
                  fontSize: '14px',
                  color: '#9ca3af',
                  textAlign: 'center',
                  marginTop: '8px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  justifyContent: 'center'
                }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  Make sure the publisher service is running
                </div>
              </div>
            )}
          </div>

          {/* Snapshot Panel - Side by side with video */}
          {lastSnapshot && (
            <div style={{
              width: '40%',
              flex: '0 0 40%',
              backgroundColor: '#ffffff',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'auto',
              borderLeft: '1px solid #e5e7eb',
              boxShadow: '-8px 0 24px rgba(0, 0, 0, 0.08), inset 1px 0 0 rgba(255, 255, 255, 0.5)',
              animation: 'slideIn 0.4s cubic-bezier(0.4, 0, 0.2, 1)',
              background: 'linear-gradient(to bottom, #ffffff 0%, #fafafa 100%)'
            }}>
              <div style={{
                padding: '28px 36px',
                borderBottom: '1px solid #e5e7eb',
                backgroundColor: 'rgba(255, 255, 255, 0.8)',
                backdropFilter: 'blur(10px)',
                boxShadow: '0 1px 3px rgba(0, 0, 0, 0.05)'
              }}>
                <div style={{
                  display: 'flex',
                  flexDirection: 'row',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '20px'
                }}>
                  <h2 style={{
                    fontSize: '22px',
                    fontWeight: '700',
                    color: '#111827',
                    margin: 0,
                    letterSpacing: '-0.3px',
                    background: 'linear-gradient(135deg, #111827 0%, #374151 100%)',
                    WebkitBackgroundClip: 'text',
                    WebkitTextFillColor: 'transparent',
                    backgroundClip: 'text'
                  }}>
                    Captured Snapshot
                  </h2>
                  <button
                    onClick={() => {
                      const prev = lastSnapshot;
                      setLastSnapshot(null);
                      if (prev) revokeSnapshot(prev);
                    }}
                    style={{
                      padding: '10px 20px',
                      backgroundColor: '#dc2626',
                      color: '#ffffff',
                      borderRadius: '10px',
                      border: 'none',
                      cursor: 'pointer',
                      fontWeight: '600',
                      fontSize: '14px',
                      transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
                      boxShadow: '0 3px 12px rgba(220, 38, 38, 0.3), 0 1px 3px rgba(220, 38, 38, 0.2)',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '6px'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.backgroundColor = '#b91c1c';
                      e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
                      e.currentTarget.style.boxShadow = '0 6px 20px rgba(220, 38, 38, 0.4), 0 2px 6px rgba(220, 38, 38, 0.3)';
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.backgroundColor = '#dc2626';
                      e.currentTarget.style.transform = 'translateY(0) scale(1)';
                      e.currentTarget.style.boxShadow = '0 3px 12px rgba(220, 38, 38, 0.3), 0 1px 3px rgba(220, 38, 38, 0.2)';
                    }}
                    onMouseDown={(e) => {
                      e.currentTarget.style.transform = 'translateY(0) scale(0.98)';
                    }}
                    onMouseUp={(e) => {
                      e.currentTarget.style.transform = 'translateY(-2px) scale(1.02)';
                    }}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                    Clear
                  </button>
                </div>
              </div>
              
              <div style={{
                flex: 1,
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                padding: '32px',
                overflow: 'auto',
                background: 'linear-gradient(135deg, #f9fafb 0%, #ffffff 100%)'
              }}>
                <div style={{
                  position: 'relative',
                  borderRadius: '16px',
                  overflow: 'hidden',
                  border: '2px solid #e5e7eb',
                  backgroundColor: '#000000',
                  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.2), 0 2px 8px rgba(0, 0, 0, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.1)',
                  maxWidth: '100%',
                  maxHeight: '100%',
                  transition: 'transform 0.3s ease, box-shadow 0.3s ease'
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.transform = 'scale(1.02)';
                  e.currentTarget.style.boxShadow = '0 12px 40px rgba(0, 0, 0, 0.25), 0 4px 12px rgba(0, 0, 0, 0.15), inset 0 1px 0 rgba(255, 255, 255, 0.1)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.transform = 'scale(1)';
                  e.currentTarget.style.boxShadow = '0 8px 32px rgba(0, 0, 0, 0.2), 0 2px 8px rgba(0, 0, 0, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.1)';
                }}>
                  <img
                    src={lastSnapshot.objectUrl}
                    alt="Captured snapshot"
                    style={{
                      maxWidth: '100%',
                      maxHeight: '100%',
                      width: 'auto',
                      height: 'auto',
                      display: 'block'
                    }}
                  />
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Info Panel */}
        <div style={{
          backgroundColor: '#f9fafb',
          padding: '28px 36px',
          borderTop: '1px solid #e5e7eb',
          boxShadow: '0 -2px 8px rgba(0, 0, 0, 0.04)',
          background: 'linear-gradient(to top, #f9fafb 0%, #ffffff 100%)'
        }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
            gap: '24px',
            fontSize: '15px'
          }}>
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '12px',
              alignItems: 'center',
              padding: '16px 20px',
              backgroundColor: '#ffffff',
              borderRadius: '12px',
              border: `2px solid ${isConnected ? '#10b98140' : '#ef444440'}`,
              boxShadow: `0 4px 12px ${isConnected ? '#10b98120' : '#ef444420'}, 0 2px 4px rgba(0, 0, 0, 0.05)`,
              transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
              backdropFilter: 'blur(10px)'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(-2px)';
              e.currentTarget.style.boxShadow = `0 6px 20px ${isConnected ? '#10b98130' : '#ef444430'}, 0 4px 8px rgba(0, 0, 0, 0.08)`;
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = `0 4px 12px ${isConnected ? '#10b98120' : '#ef444420'}, 0 2px 4px rgba(0, 0, 0, 0.05)`;
            }}>
              <div style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                backgroundColor: isConnected ? '#10b981' : '#ef4444',
                flexShrink: 0,
                boxShadow: `0 0 8px ${isConnected ? '#10b981' : '#ef4444'}40`
              }} />
              <span style={{ color: '#6b7280', fontWeight: '500', fontSize: '14px' }}>Connection:</span>
              <span style={{
                color: isConnected ? '#10b981' : '#ef4444',
                fontWeight: '600',
                fontSize: '15px'
              }}>
                {isConnected ? 'Connected' : 'Disconnected'}
              </span>
            </div>
            
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '12px',
              alignItems: 'center',
              padding: '16px 20px',
              backgroundColor: '#ffffff',
              borderRadius: '12px',
              border: '2px solid #e5e7eb',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.06), 0 2px 4px rgba(0, 0, 0, 0.04)',
              transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
              backdropFilter: 'blur(10px)'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(-2px)';
              e.currentTarget.style.boxShadow = '0 6px 20px rgba(0, 0, 0, 0.1), 0 4px 8px rgba(0, 0, 0, 0.06)';
              e.currentTarget.style.borderColor = '#d1d5db';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.06), 0 2px 4px rgba(0, 0, 0, 0.04)';
              e.currentTarget.style.borderColor = '#e5e7eb';
            }}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ flexShrink: 0 }}>
                <path d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22Z" stroke="#6b7280" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M12 6V12L16 14" stroke="#6b7280" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              <span style={{ color: '#6b7280', fontWeight: '500', fontSize: '14px' }}>ICE State:</span>
              <span style={{
                color: '#111827',
                textTransform: 'capitalize',
                fontWeight: '600',
                fontSize: '15px'
              }}>
                {connectionState}
              </span>
            </div>
            
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '12px',
              alignItems: 'center',
              padding: '16px 20px',
              backgroundColor: '#ffffff',
              borderRadius: '12px',
              border: '2px solid #e5e7eb',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.06), 0 2px 4px rgba(0, 0, 0, 0.04)',
              transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
              backdropFilter: 'blur(10px)'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(-2px)';
              e.currentTarget.style.boxShadow = '0 6px 20px rgba(0, 0, 0, 0.1), 0 4px 8px rgba(0, 0, 0, 0.06)';
              e.currentTarget.style.borderColor = '#d1d5db';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.06), 0 2px 4px rgba(0, 0, 0, 0.04)';
              e.currentTarget.style.borderColor = '#e5e7eb';
            }}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ flexShrink: 0 }}>
                <path d="M15 10L19.553 7.276C19.8325 7.10759 20.1692 7.06481 20.4822 7.15765C20.7952 7.2505 21.0579 7.47162 21.2097 7.77086C21.3615 8.07011 21.3891 8.42095 21.2868 8.742C21.1845 9.06304 20.9609 9.32705 20.664 9.48L15 13M5 18H13C13.5304 18 14.0391 17.7893 14.4142 17.4142C14.7893 17.0391 15 16.5304 15 16V8C15 7.46957 14.7893 6.96086 14.4142 6.58579C14.0391 6.21071 13.5304 6 13 6H5C4.46957 6 3.96086 6.21071 3.58579 6.58579C3.21071 6.96086 3 7.46957 3 8V16C3 16.5304 3.21071 17.0391 3.58579 17.4142C3.96086 17.7893 4.46957 18 5 18Z" stroke="#6b7280" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              <span style={{ color: '#6b7280', fontWeight: '500', fontSize: '14px' }}>Status:</span>
              <span style={{
                color: '#111827',
                fontWeight: '600',
                fontSize: '15px'
              }}>
                {isConnected ? 'Streaming' : 'Waiting'}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default VideoViewer;
