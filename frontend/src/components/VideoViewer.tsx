import React from 'react';
import { useWebRTC } from '../hooks/useWebRTC';

const VideoViewer: React.FC = () => {
  const { isConnected, connectionState, hasTrack, videoRef, connect, disconnect } = useWebRTC();

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
      backgroundColor: '#111827',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '20px',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    }}>
      {/* Main Container */}
      <div style={{
        width: '100%',
        maxWidth: '1200px',
        backgroundColor: '#1f2937',
        borderRadius: '12px',
        boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column'
      }}>
        {/* Header */}
        <div style={{
          backgroundColor: '#374151',
          padding: '20px 24px',
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '16px',
          borderBottom: '1px solid #4b5563'
        }}>
          <h1 style={{
            fontSize: '24px',
            fontWeight: 'bold',
            color: '#ffffff',
            margin: 0,
            padding: 0
          }}>
            WebRTC Video Stream
          </h1>
          
          <div style={{
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
            gap: '16px'
          }}>
            {/* Status Indicator */}
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              alignItems: 'center',
              gap: '8px'
            }}>
              <div style={{
                width: '12px',
                height: '12px',
                borderRadius: '50%',
                backgroundColor: getConnectionStatusColor(),
                flexShrink: 0
              }}></div>
              <span style={{
                fontSize: '14px',
                color: '#d1d5db',
                textTransform: 'capitalize',
                whiteSpace: 'nowrap'
              }}>
                {connectionState}
              </span>
            </div>

            {/* Connect/Disconnect Button */}
            <button
              onClick={isConnected ? disconnect : connect}
              style={{
                padding: '10px 20px',
                backgroundColor: isConnected ? '#dc2626' : '#2563eb',
                color: '#ffffff',
                borderRadius: '8px',
                border: 'none',
                cursor: 'pointer',
                fontWeight: '600',
                fontSize: '14px',
                whiteSpace: 'nowrap',
                transition: 'background-color 0.2s'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = isConnected ? '#b91c1c' : '#1d4ed8';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = isConnected ? '#dc2626' : '#2563eb';
              }}
            >
              {isConnected ? 'Disconnect' : 'Connect'}
            </button>
          </div>
        </div>

        {/* Video Container */}
        <div style={{
          position: 'relative',
          width: '100%',
          backgroundColor: '#000000',
          aspectRatio: '16/9',
          minHeight: '450px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center'
        }}>
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            controls={false}
            style={{
              width: '100%',
              height: '100%',
              objectFit: 'contain',
              display: hasTrack ? 'block' : 'none', // Show video only when track is received
              backgroundColor: '#000000',
            }}
            onLoadedMetadata={() => {
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
              backgroundColor: '#111827',
              flexDirection: 'column',
              gap: '12px',
              padding: '40px'
            }}>
              <div style={{
                fontSize: '24px',
                fontWeight: '600',
                color: '#9ca3af',
                marginBottom: '8px'
              }}>
                No Stream
              </div>
              <div style={{
                fontSize: '16px',
                color: '#6b7280',
                textAlign: 'center',
                lineHeight: '1.5'
              }}>
                Click the <strong style={{ color: '#d1d5db' }}>"Connect"</strong> button above to start receiving video stream
              </div>
              <div style={{
                fontSize: '14px',
                color: '#4b5563',
                textAlign: 'center',
                marginTop: '8px'
              }}>
                Make sure the publisher service is running
              </div>
            </div>
          )}
        </div>

        {/* Info Panel */}
        <div style={{
          backgroundColor: '#374151',
          padding: '20px 24px',
          borderTop: '1px solid #4b5563'
        }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
            gap: '20px',
            fontSize: '14px'
          }}>
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '8px',
              alignItems: 'center'
            }}>
              <span style={{ color: '#9ca3af' }}>Connection:</span>
              <span style={{
                color: isConnected ? '#34d399' : '#f87171',
                fontWeight: '500'
              }}>
                {isConnected ? 'Connected' : 'Disconnected'}
              </span>
            </div>
            
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '8px',
              alignItems: 'center'
            }}>
              <span style={{ color: '#9ca3af' }}>ICE State:</span>
              <span style={{
                color: '#d1d5db',
                textTransform: 'capitalize',
                fontWeight: '500'
              }}>
                {connectionState}
              </span>
            </div>
            
            <div style={{
              display: 'flex',
              flexDirection: 'row',
              gap: '8px',
              alignItems: 'center'
            }}>
              <span style={{ color: '#9ca3af' }}>Status:</span>
              <span style={{
                color: '#d1d5db',
                fontWeight: '500'
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
