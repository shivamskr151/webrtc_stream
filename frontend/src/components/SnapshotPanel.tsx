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


  return (
    <div style={{
      backgroundColor: '#ffffff',
      padding: '28px 32px',
      borderTop: '2px solid #e5e7eb'
    }}>
      <div style={{
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '20px',
        marginBottom: '24px'
      }}>
        <h2 style={{
          fontSize: '22px',
          fontWeight: '700',
          color: '#111827',
          margin: 0,
          letterSpacing: '-0.3px'
        }}>
          Captured Snapshot
        </h2>
        <button
          onClick={onClear}
          style={{
            padding: '10px 20px',
            backgroundColor: '#dc2626',
            color: '#ffffff',
            borderRadius: '8px',
            border: 'none',
            cursor: 'pointer',
            fontWeight: '600',
            fontSize: '14px',
            transition: 'all 0.2s ease',
            boxShadow: '0 2px 8px rgba(220, 38, 38, 0.25)'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = '#b91c1c';
            e.currentTarget.style.transform = 'translateY(-1px)';
            e.currentTarget.style.boxShadow = '0 4px 12px rgba(220, 38, 38, 0.35)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = '#dc2626';
            e.currentTarget.style.transform = 'translateY(0)';
            e.currentTarget.style.boxShadow = '0 2px 8px rgba(220, 38, 38, 0.25)';
          }}
        >
          Clear
        </button>
      </div>
      
      <div style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center'
      }}>
        <div style={{
          position: 'relative',
          borderRadius: '12px',
          overflow: 'hidden',
          border: '3px solid #e5e7eb',
          backgroundColor: '#000000',
          boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)'
        }}>
          <img
            src={snapshot.objectUrl}
            alt="Captured snapshot"
            style={{
              maxWidth: '100%',
              maxHeight: '500px',
              width: 'auto',
              height: 'auto',
              display: 'block'
            }}
          />
        </div>
      </div>
    </div>
  );
};

export default SnapshotPanel;
