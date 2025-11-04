export interface SnapshotResult {
  blob: Blob;
  objectUrl: string;
  width: number;
  height: number;
  takenAt: number;
}

export async function captureVideoFrame(video: HTMLVideoElement): Promise<SnapshotResult> {
  if (!video || video.readyState < 2) {
    throw new Error('Video not ready');
  }

  const width = video.videoWidth || video.clientWidth || 0;
  const height = video.videoHeight || video.clientHeight || 0;
  if (!width || !height) {
    throw new Error('Invalid video dimensions');
  }

  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Canvas context not available');

  ctx.drawImage(video, 0, 0, width, height);

  const blob: Blob = await new Promise((resolve, reject) => {
    // Use image/webp if supported for smaller size, fallback to png
    const preferredType = 'image/webp';
    const type = canvas.toDataURL(preferredType).startsWith('data:image/webp') ? preferredType : 'image/png';
    canvas.toBlob((b) => {
      if (b) resolve(b); else reject(new Error('Failed to create snapshot blob'));
    }, type, 0.92);
  });

  const objectUrl = URL.createObjectURL(blob);
  return {
    blob,
    objectUrl,
    width,
    height,
    takenAt: Date.now(),
  };
}

export function revokeSnapshot(result: SnapshotResult) {
  try {
    URL.revokeObjectURL(result.objectUrl);
  } catch {}
}


