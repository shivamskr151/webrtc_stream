package video

import (
	"fmt"
	"time"

	"webrtc-streaming/internal/config"

	"github.com/pion/webrtc/v4/pkg/media"
)

// VideoSource represents a video source (camera, file, etc.)
type VideoSource interface {
	Start() error
	ReadFrame() ([]byte, error)
	Close() error
	GetFrameRate() int // Get the actual frame rate of the source
}

// MockVideoSource is a placeholder for actual video capture
// In production, replace this with actual camera capture using platform-specific libraries
// (e.g., v4l2 on Linux, AVFoundation on macOS, DirectShow on Windows)
type MockVideoSource struct {
	width      int
	height     int
	fps        int
	frameCount int
}

func NewVideoSource() (VideoSource, error) {
	// Use RTSP if URL is provided
	if config.AppConfig.Video.RTSPURL != "" {
		return NewRTSPVideoSource(config.AppConfig.Video.RTSPURL)
	}

	// Otherwise use mock source
	return &MockVideoSource{
		width:  config.AppConfig.Video.Width,
		height: config.AppConfig.Video.Height,
		fps:    config.AppConfig.Video.FPS,
	}, nil
}

func (m *MockVideoSource) Start() error {
	return nil
}

func (m *MockVideoSource) ReadFrame() ([]byte, error) {
	// Generate a simple test pattern
	// In production, this would read actual video frames from a camera
	frameSize := m.width * m.height * 3 // RGB
	frame := make([]byte, frameSize)

	// Simple pattern: alternating colors
	m.frameCount++
	for i := 0; i < len(frame); i += 3 {
		if (m.frameCount/30)%2 == 0 {
			frame[i] = 255 // R
			frame[i+1] = 0 // G
			frame[i+2] = 0 // B
		} else {
			frame[i] = 0     // R
			frame[i+1] = 255 // G
			frame[i+2] = 0   // B
		}
	}

	return frame, nil
}

func (m *MockVideoSource) Close() error {
	return nil
}

func (m *MockVideoSource) GetFrameRate() int {
	return m.fps
}

// VideoCapturer handles video capture and encoding
type VideoCapturer struct {
	source    VideoSource
	frameRate time.Duration
}

func NewVideoCapturer() (*VideoCapturer, error) {
	source, err := NewVideoSource()
	if err != nil {
		return nil, fmt.Errorf("failed to create video source: %w", err)
	}

	if err := source.Start(); err != nil {
		return nil, fmt.Errorf("failed to start video source: %w", err)
	}

	// Get actual frame rate from source (may differ from config)
	actualFPS := source.GetFrameRate()
	if actualFPS <= 0 {
		actualFPS = config.AppConfig.Video.FPS
	}

	return &VideoCapturer{
		source:    source,
		frameRate: time.Second / time.Duration(actualFPS),
	}, nil
}

func (vc *VideoCapturer) CaptureFrame() (media.Sample, error) {
	frameData, err := vc.source.ReadFrame()
	if err != nil {
		return media.Sample{}, fmt.Errorf("failed to read frame from source: %w", err)
	}

	// For H264 (from RTSP), frameData is already in Annex-B format with access units
	// For VP8 (mock), we would need encoding, but that's not implemented
	// The data format is correct for H264 - Pion WebRTC will handle RTP packetization

	if len(frameData) == 0 {
		return media.Sample{}, fmt.Errorf("empty frame data received")
	}

	// Update frame rate dynamically if source frame rate changed
	actualFPS := vc.source.GetFrameRate()
	if actualFPS > 0 {
		vc.frameRate = time.Second / time.Duration(actualFPS)
	}

	sample := media.Sample{
		Data:     frameData,
		Duration: vc.frameRate,
	}

	return sample, nil
}

func (vc *VideoCapturer) Close() error {
	return vc.source.Close()
}

// GetFrameRate returns the actual frame rate from the video source
func (vc *VideoCapturer) GetFrameRate() int {
	return vc.source.GetFrameRate()
}
