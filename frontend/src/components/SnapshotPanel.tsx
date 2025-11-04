import React from 'react';
import type { SnapshotResult } from '../utils/snapshot';

interface SnapshotPanelProps {
  snapshot: SnapshotResult | null;
  onClear: () => void;
}

const SnapshotPanel: React.FC<SnapshotPanelProps> = ({ snapshot, onClear }) => {
  if (!snapshot) {
    return null;
  }

  const formatTime = (timestamp: number) => {
    return new Date(timestamp).toLocaleTimeString();
  };

  return (
    <div style={{
      backgroundColor: '#374151',
      padding: '20px 24px',
      borderTop: '1px solid #4b5563'
    }}>
      <div style={{
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '16px',
        marginBottom: '16px'
      }}>
        <h2 style={{
          fontSize: '18px',
          fontWeight: '600',
          color: '#ffffff',
          margin: 0
        }}>
          Captured Snapshot
        </h2>
        <button
          onClick={onClear}
          style={{
            padding: '8px 16px',
            backgroundColor: '#dc2626',
            color: '#ffffff',
            borderRadius: '6px',
            border: 'none',
            cursor: 'pointer',
            fontWeight: '500',
            fontSize: '14px',
            transition: 'background-color 0.2s'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = '#b91c1c';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = '#dc2626';
          }}
        >
          Clear
        </button>
      </div>
      
      <div style={{
        display: 'flex',
        flexDirection: 'row',
        gap: '20px',
        alignItems: 'flex-start'
      }}>
        <div style={{
          position: 'relative',
          borderRadius: '8px',
          overflow: 'hidden',
          border: '2px solid #4b5563',
          backgroundColor: '#000000',
          flexShrink: 0
        }}>
          <img
            src={snapshot.objectUrl}
            alt="Captured snapshot"
            style={{
              maxWidth: '400px',
              maxHeight: '300px',
              width: 'auto',
              height: 'auto',
              display: 'block'
            }}
          />
        </div>
        
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          flex: 1,
          fontSize: '14px'
        }}>
          <div style={{
            display: 'flex',
            flexDirection: 'row',
            gap: '8px',
            alignItems: 'center'
          }}>
            <span style={{ color: '#9ca3af' }}>Dimensions:</span>
            <span style={{ color: '#d1d5db', fontWeight: '500' }}>
              {snapshot.width} × {snapshot.height}
            </span>
          </div>
          
          <div style={{
            display: 'flex',
            flexDirection: 'row',
            gap: '8px',
            alignItems: 'center'
          }}>
            <span style={{ color: '#9ca3af' }}>Captured at:</span>
            <span style={{ color: '#d1d5db', fontWeight: '500' }}>
              {formatTime(snapshot.takenAt)}
            </span>
          </div>
          
          <div style={{
            display: 'flex',
            flexDirection: 'row',
            gap: '8px',
            alignItems: 'center'
          }}>
            <span style={{ color: '#9ca3af' }}>Size:</span>
            <span style={{ color: '#d1d5db', fontWeight: '500' }}>
              {(snapshot.blob.size / 1024).toFixed(2)} KB
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SnapshotPanel;
