package video

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// RTSPVideoSource handles RTSP stream using ffmpeg
type RTSPVideoSource struct {
	rtspURL     string
	cmd         *exec.Cmd
	stdout      io.ReadCloser
	frameChan   chan []byte
	errChan     chan error
	mu          sync.Mutex
	closed      bool
	accessUnit  []byte // Accumulator for complete access units
	spsPpsFound bool   // Track if we've received SPS/PPS
}

func NewRTSPVideoSource(rtspURL string) (*RTSPVideoSource, error) {
	return &RTSPVideoSource{
		rtspURL:     rtspURL,
		frameChan:   make(chan []byte, 30),
		errChan:     make(chan error, 1),
		accessUnit:  make([]byte, 0, 512*1024),
		spsPpsFound: false,
	}, nil
}

func (r *RTSPVideoSource) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("RTSP source already closed")
	}

	log.Printf("Starting RTSP stream from: %s", r.rtspURL)

	// Build ffmpeg command to decode RTSP and output raw H264 frames
	// IMPORTANT: The stream might be HEVC/H.265, so we need to transcode to H.264
	// Browser support for H.264 is universal, but HEVC support is limited
	ffmpegArgs := []string{
		"-rtsp_transport", "tcp", // Use TCP for more reliable connection
		"-fflags", "nobuffer+flush_packets", // Reduce latency and flush immediately
		"-flags", "low_delay",
		"-strict", "experimental",
		"-analyzeduration", "1000000", // Reduce analysis time (1 second)
		"-probesize", "1000000", // Reduce probe size
		"-i", r.rtspURL,
		// Transcode to H.264 (works for both H.264 and HEVC input)
		// Use hardware acceleration if available (videotoolbox on macOS)
		"-c:v", "libx264", // Encode to H.264 (transcodes HEVC/H.265 to H.264)
		"-preset", "ultrafast", // Fastest encoding for low latency
		"-tune", "zerolatency", // Zero latency tuning
		"-profile:v", "baseline", // Baseline profile for maximum compatibility
		"-level", "3.1", // Level 3.1 for good compatibility
		"-pix_fmt", "yuv420p", // Pixel format for compatibility
		"-bf", "0", // No B-frames (WebRTC requirement)
		"-g", "30", // GOP size (keyframe every 30 frames, ~1 second at 30fps)
		"-bsf:v", "h264_mp4toannexb", // Convert to Annex-B format (required for raw H264)
		"-f", "h264", // Raw H264 format
		"-flush_packets", "1", // Flush packets immediately
		"-", // Output to stdout
	}

	log.Println("⚠️ Note: Transcoding to H.264 (may introduce latency)")
	log.Println("   If source is HEVC/H.265, it will be transcoded to H.264 for browser compatibility")

	log.Printf("Running ffmpeg with args: %v", ffmpegArgs)

	cmd := exec.Command("ffmpeg", ffmpegArgs...)
	r.cmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	r.stdout = stdout

	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Log stderr in a goroutine for debugging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Log important ffmpeg messages
			if len(line) > 0 {
				log.Printf("ffmpeg: %s", line)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		stdout.Close()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Start reading frames in a goroutine
	go r.readFrames()

	return nil
}

func (r *RTSPVideoSource) readFrames() {
	defer close(r.frameChan)
	defer close(r.errChan)

	// H264 NAL Unit start codes: 0x00000001 or 0x000001
	buffer := make([]byte, 0, 512*1024)              // 512KB initial buffer
	reader := bufio.NewReaderSize(r.stdout, 64*1024) // 64KB read buffer
	chunk := make([]byte, 32*1024)                   // Read 32KB chunks

	for {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()

		// Read chunk
		n, err := reader.Read(chunk)
		if err != nil {
			if err != io.EOF {
				r.errChan <- fmt.Errorf("failed to read chunk: %w", err)
			}
			// Process remaining buffer before returning
			if len(buffer) > 0 {
				select {
				case r.frameChan <- buffer:
				default:
				}
			}
			return
		}

		if n == 0 {
			continue
		}

		buffer = append(buffer, chunk[:n]...)

		// Log when we start receiving data (log once, then periodically)
		if !ffmpegDataLogged && len(buffer) > 100 {
			log.Printf("📥 FFmpeg started producing data: received %d bytes", len(buffer))
			log.Printf("   Transcoding is working - parsing H.264 NAL units...")

			// Debug: Check first bytes for H.264 start codes
			if len(buffer) >= 100 {
				firstBytesLen := 100
				if len(buffer) < firstBytesLen {
					firstBytesLen = len(buffer)
				}
				firstBytes := buffer[:firstBytesLen]
				log.Printf("   First %d bytes (hex): %x", firstBytesLen, firstBytes)
				// Look for start codes in first bytes
				hasStartCode := false
				for i := 0; i < len(firstBytes)-3; i++ {
					if (firstBytes[i] == 0x00 && firstBytes[i+1] == 0x00 && firstBytes[i+2] == 0x00 && firstBytes[i+3] == 0x01) ||
						(firstBytes[i] == 0x00 && firstBytes[i+1] == 0x00 && firstBytes[i+2] == 0x01) {
						hasStartCode = true
						log.Printf("   ✅ Found H.264 start code at position %d", i)
						break
					}
				}
				if !hasStartCode {
					log.Printf("   ⚠️ No H.264 start codes found in first 100 bytes!")
				}
			}
			ffmpegDataLogged = true
		} else if len(buffer) > 50000 && len(buffer)%50000 < 32768 { // Log roughly every 50KB
			log.Printf("📥 FFmpeg buffer: %d bytes (parsing NAL units...) - still looking for complete frames", len(buffer))
		}

		// Parse NAL units from buffer
		// IMPORTANT: We need at least one complete start code + NAL unit to extract
		// If buffer is too small, read more data first
		if len(buffer) < 20 {
			// Buffer too small, continue reading
			continue
		}

		for {
			found := false

			// Look for start codes (0x00000001 or 0x000001)
			// Try 4-byte first (more common), then 3-byte
			startCodeIdx := -1
			startCodeLen := 0

			if idx := findStartCode4(buffer); idx >= 0 {
				startCodeIdx = idx
				startCodeLen = 4
			} else if idx := findStartCode3(buffer); idx >= 0 {
				startCodeIdx = idx
				startCodeLen = 3
			}

			if startCodeIdx >= 0 {
				// Find the previous start code to determine NAL unit boundaries
				prevStartCodeIdx := -1
				prevStartCodeLen := 0

				// Look for previous start code before current one
				for i := startCodeIdx - 1; i >= 0; i-- {
					if i >= 3 && buffer[i-3] == 0x00 && buffer[i-2] == 0x00 && buffer[i-1] == 0x00 && buffer[i] == 0x01 {
						prevStartCodeIdx = i - 3
						prevStartCodeLen = 4
						break
					} else if i >= 2 && buffer[i-2] == 0x00 && buffer[i-1] == 0x00 && buffer[i] == 0x01 {
						prevStartCodeIdx = i - 2
						prevStartCodeLen = 3
						break
					}
				}

				var nalUnit []byte
				var nalStart, nalEnd int

				if prevStartCodeIdx >= 0 {
					// Extract NAL unit between previous and current start code
					nalStart = prevStartCodeIdx
					nalEnd = startCodeIdx
				} else {
					// This is the FIRST start code in buffer - extract everything up to next start code
					// Find the next start code after this one
					nextStartCodeIdx := -1

					// Use the detected start code length
					currentStartCodeLen := startCodeLen

					// Search for next start code after current one
					for i := startCodeIdx + currentStartCodeLen; i < len(buffer)-3; i++ {
						if i+3 < len(buffer) && buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x00 && buffer[i+3] == 0x01 {
							nextStartCodeIdx = i
							break
						} else if i+2 < len(buffer) && buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x01 {
							// Make sure it's not part of 4-byte code
							if i == 0 || buffer[i-1] != 0x00 {
								nextStartCodeIdx = i
								break
							}
						}
					}

					if nextStartCodeIdx >= 0 {
						// Extract NAL unit from first start code to next
						nalStart = startCodeIdx
						nalEnd = nextStartCodeIdx
						prevStartCodeLen = currentStartCodeLen
					} else {
						// No next start code found yet - need more data
						// Keep everything including first start code in buffer
						// Don't process anything yet, break and read more
						found = false // Don't mark as found, continue reading
						break
					}
				}

				if nalEnd > nalStart && nalEnd-nalStart > prevStartCodeLen {
					nalUnit = make([]byte, nalEnd-nalStart)
					copy(nalUnit, buffer[nalStart:nalEnd])

					// Only process non-empty NAL units
					if len(nalUnit) > prevStartCodeLen+1 {
						// Extract NAL unit type
						nalTypeByte := byte(0)
						if len(nalUnit) >= 5 && nalUnit[0] == 0x00 && nalUnit[1] == 0x00 && nalUnit[2] == 0x00 && nalUnit[3] == 0x01 {
							nalTypeByte = nalUnit[4] & 0x1F // 4-byte start code
						} else if len(nalUnit) >= 4 && nalUnit[0] == 0x00 && nalUnit[1] == 0x00 && nalUnit[2] == 0x01 {
							nalTypeByte = nalUnit[3] & 0x1F // 3-byte start code
						}

						// NAL unit types:
						// 7 = SPS (Sequence Parameter Set)
						// 8 = PPS (Picture Parameter Set)
						// 5 = IDR (Instantaneous Decoder Refresh) - keyframe
						// 1 = Non-IDR slice (P/B frame)

						// Debug first few NAL units
						if frameQueueCounter == 0 {
							log.Printf("🔍 First NAL unit extracted: type=%d, size=%d bytes", nalTypeByte, len(nalUnit))
						}

						// Check if this is a picture frame (IDR or P/B)
						isPictureFrame := nalTypeByte == 5 || nalTypeByte == 1

						if isPictureFrame {
							// Picture frame - send complete access unit
							// For IDR frames (keyframes), ALWAYS include SPS/PPS if available
							// For P/B frames, include SPS/PPS only on first frame
							var completeAccessUnit []byte

							if nalTypeByte == 5 {
								// IDR frame (keyframe) - MUST include SPS/PPS
								if len(r.accessUnit) > 0 {
									// Prepend SPS/PPS before IDR frame
									spsPpsLen := len(r.accessUnit)
									completeAccessUnit = make([]byte, spsPpsLen+len(nalUnit))
									copy(completeAccessUnit, r.accessUnit)
									copy(completeAccessUnit[spsPpsLen:], nalUnit)
									r.accessUnit = r.accessUnit[:0] // Clear accumulator
									log.Printf("📦 IDR frame with SPS/PPS: %d + %d = %d bytes",
										spsPpsLen, len(nalUnit), len(completeAccessUnit))
								} else {
									// No SPS/PPS available yet, send IDR alone (might work if sent before)
									completeAccessUnit = nalUnit
									log.Printf("⚠️ IDR frame without SPS/PPS (may cause decoding issues)")
								}
							} else if nalTypeByte == 1 {
								// P/B frame - include SPS/PPS only if not sent before
								if !r.spsPpsFound && len(r.accessUnit) > 0 {
									// First frame, include SPS/PPS
									completeAccessUnit = make([]byte, len(r.accessUnit)+len(nalUnit))
									copy(completeAccessUnit, r.accessUnit)
									copy(completeAccessUnit[len(r.accessUnit):], nalUnit)
									r.accessUnit = r.accessUnit[:0]
									r.spsPpsFound = true // Mark as sent
									log.Printf("📦 First P/B frame with SPS/PPS: %d bytes", len(completeAccessUnit))
								} else {
									// Regular P/B frame, send alone
									completeAccessUnit = nalUnit
								}
							}

							// Always log first few frames being queued for debugging
							frameQueueCounter++

							select {
							case r.frameChan <- completeAccessUnit:
								// Successfully queued
								if frameQueueCounter <= 10 {
									log.Printf("📤 Queued frame #%d to channel: type=%d, size=%d bytes (channel has %d frames)",
										frameQueueCounter, nalTypeByte, len(completeAccessUnit), len(r.frameChan))
								} else if frameQueueCounter%30 == 0 {
									log.Printf("📤 Queued frame #%d: type=%d, size=%d bytes", frameQueueCounter, nalTypeByte, len(completeAccessUnit))
								}
							default:
								// Channel full - this means ReadFrame() is not being called fast enough
								// This could mean: 1) Connection not established yet, 2) Streaming not started
								if frameQueueCounter%100 == 0 {
									log.Printf("⚠️ Frame channel full (capacity 30, %d frames queued), dropping frame (type %d, size %d)", len(r.frameChan), nalTypeByte, len(completeAccessUnit))
									log.Printf("   This suggests frames are being produced faster than consumed")
									log.Printf("   Check if WebRTC connection is established and streaming has started")
								}
							}
						} else if nalTypeByte == 7 || nalTypeByte == 8 {
							// SPS/PPS - accumulate for next access unit
							r.accessUnit = append(r.accessUnit, nalUnit...)
							if !r.spsPpsFound {
								// Don't mark as found until we actually send it with a frame
								log.Printf("📋 Received %s parameter set (size: %d bytes) - will include with next frame",
									map[byte]string{7: "SPS", 8: "PPS"}[nalTypeByte], len(nalUnit))
							}
						} else {
							// Other NAL unit types (AUD=9, SEI=6, etc.) - include in access unit
							r.accessUnit = append(r.accessUnit, nalUnit...)
						}
					}

					// Remove processed data from buffer
					if prevStartCodeIdx >= 0 {
						// Remove from previous start code to current start code
						buffer = buffer[startCodeIdx:]
					} else {
						// Remove from first start code to next start code
						buffer = buffer[nalEnd:]
					}
				} else {
					// NAL unit too small or invalid, remove start code and continue
					buffer = buffer[startCodeIdx+4:] // Skip the start code
				}
				found = true
			}

			if !found {
				// No more start codes found in current buffer
				// If buffer is large but no frames found, log for debugging
				if len(buffer) > 100000 && frameQueueCounter == 0 {
					log.Printf("⚠️ Large buffer (%d bytes) but no NAL units extracted yet - may need more data", len(buffer))
					// Check if there are any start codes at all
					hasAnyStartCode := false
					checkLen := 1000
					if len(buffer) < checkLen {
						checkLen = len(buffer)
					}
					for i := 0; i < checkLen-3; i++ {
						if (buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x00 && buffer[i+3] == 0x01) ||
							(i < len(buffer)-2 && buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x01) {
							hasAnyStartCode = true
							break
						}
					}
					if !hasAnyStartCode {
						log.Printf("   ❌ No H.264 start codes found in buffer - FFmpeg output may not be in Annex-B format!")
						log.Printf("   This suggests the h264_mp4toannexb bitstream filter may not be working")
					} else {
						log.Printf("   ✅ Start codes found but not forming complete NAL units - continuing to read...")
					}
				}
				break
			}

			// Prevent buffer from growing too large
			if len(buffer) > 2*1024*1024 { // 2MB max
				// Keep only last 1MB
				buffer = buffer[len(buffer)-1024*1024:]
			}
		}
	}
}

// findStartCode4 finds 4-byte start code (0x00000001) in buffer
// Returns index where start code begins, or -1 if not found
func findStartCode4(buffer []byte) int {
	for i := 0; i <= len(buffer)-4; i++ {
		if buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x00 && buffer[i+3] == 0x01 {
			return i
		}
	}
	return -1
}

// findStartCode3 finds 3-byte start code (0x000001) in buffer
// Returns index where start code begins, or -1 if not found
// Only checks after position 0 to avoid false positives from 4-byte codes
func findStartCode3(buffer []byte) int {
	start := 1 // Start from position 1 to avoid matching 4-byte codes
	if start > len(buffer)-3 {
		return -1
	}
	for i := start; i <= len(buffer)-3; i++ {
		// Make sure it's not part of a 4-byte code
		if i > 0 && buffer[i-1] == 0x00 {
			continue
		}
		if buffer[i] == 0x00 && buffer[i+1] == 0x00 && buffer[i+2] == 0x01 {
			return i
		}
	}
	return -1
}

var (
	frameReadCount    int64
	firstFrameSent    bool
	ffmpegDataLogged  bool // Track if we've logged first data receipt
	frameQueueCounter int  // Track frames queued to channel (package-level for access from goroutine)
)

func (r *RTSPVideoSource) ReadFrame() ([]byte, error) {
	// Add timeout to avoid blocking forever
	// Increased timeout since transcoding may take longer
	timeout := time.After(10 * time.Second)

	select {
	case frame := <-r.frameChan:
		frameReadCount++
		if frame == nil {
			return nil, fmt.Errorf("frame channel closed")
		}
		if len(frame) == 0 {
			// Skip empty frames, try again
			return r.ReadFrame()
		}

		// For the very first frame, ensure it has SPS/PPS
		// Pion WebRTC needs SPS/PPS before the first IDR frame
		if !firstFrameSent && len(frame) >= 8 {
			// Check if this frame starts with SPS/PPS
			hasSpsPps := false
			if len(frame) >= 8 {
				// Look for SPS (type 7) or PPS (type 8) in the first 500 bytes
				checkLen := len(frame)
				if checkLen > 500 {
					checkLen = 500
				}
				for i := 0; i < checkLen-4; i++ {
					if frame[i] == 0x00 && frame[i+1] == 0x00 && frame[i+2] == 0x00 && frame[i+3] == 0x01 {
						if i+4 < len(frame) {
							nalType := frame[i+4] & 0x1F
							if nalType == 7 || nalType == 8 {
								hasSpsPps = true
								break
							}
						}
					} else if frame[i] == 0x00 && frame[i+1] == 0x00 && frame[i+2] == 0x01 {
						if i+3 < len(frame) {
							nalType := frame[i+3] & 0x1F
							if nalType == 7 || nalType == 8 {
								hasSpsPps = true
								break
							}
						}
					}
				}
			}

			if !hasSpsPps {
				// First frame doesn't have SPS/PPS
				// After transcoding starts, libx264 should produce frames with SPS/PPS
				// But if we wait too long, skip the check after 5 attempts
				if frameReadCount <= 5 {
					log.Printf("⚠️ First frame doesn't contain SPS/PPS (attempt %d), skipping and waiting...", frameReadCount)
					return r.ReadFrame()
				} else {
					log.Printf("⚠️ No SPS/PPS found after 5 attempts, sending frame anyway (transcoding may still be initializing)")
					// Continue anyway - transcoding might need more time
				}
			}
			firstFrameSent = true
			log.Printf("✅ First frame ready: %d bytes (SPS/PPS: %v)", len(frame), hasSpsPps)
		}

		if len(frame) < 8 {
			// Skip very small frames (not valid NAL units)
			if frameReadCount%100 == 0 {
				log.Printf("Skipping small frame: %d bytes", len(frame))
			}
			return r.ReadFrame()
		}

		// Log first few frames for debugging
		if frameReadCount <= 5 || frameReadCount%100 == 0 {
			nalTypes := []string{}
			// Parse all NAL units in the access unit
			i := 0
			for i < len(frame) {
				if i+4 <= len(frame) && frame[i] == 0x00 && frame[i+1] == 0x00 && frame[i+2] == 0x00 && frame[i+3] == 0x01 {
					// 4-byte start code
					if i+4 < len(frame) {
						nalTypeByte := frame[i+4] & 0x1F
						switch nalTypeByte {
						case 1:
							nalTypes = append(nalTypes, "P/B")
						case 5:
							nalTypes = append(nalTypes, "IDR")
						case 7:
							nalTypes = append(nalTypes, "SPS")
						case 8:
							nalTypes = append(nalTypes, "PPS")
						default:
							nalTypes = append(nalTypes, fmt.Sprintf("NAL%d", nalTypeByte))
						}
					}
					i += 4
				} else if i+3 <= len(frame) && frame[i] == 0x00 && frame[i+1] == 0x00 && frame[i+2] == 0x01 {
					// 3-byte start code
					if i+3 < len(frame) {
						nalTypeByte := frame[i+3] & 0x1F
						switch nalTypeByte {
						case 1:
							nalTypes = append(nalTypes, "P/B")
						case 5:
							nalTypes = append(nalTypes, "IDR")
						case 7:
							nalTypes = append(nalTypes, "SPS")
						case 8:
							nalTypes = append(nalTypes, "PPS")
						default:
							nalTypes = append(nalTypes, fmt.Sprintf("NAL%d", nalTypeByte))
						}
					}
					i += 3
				} else {
					i++
				}
			}
			log.Printf("📹 RTSP access unit #%d: %d bytes, NALs: %v", frameReadCount, len(frame), nalTypes)
		}

		return frame, nil
	case err := <-r.errChan:
		if err != nil {
			log.Printf("RTSP error: %v", err)
			return nil, err
		}
		return nil, fmt.Errorf("error channel closed")
	case <-timeout:
		log.Printf("⚠️ Timeout waiting for RTSP frame (waited 10s)")
		log.Printf("   Frame channel length: %d", len(r.frameChan))
		log.Printf("   This might mean:")
		log.Printf("   1) FFmpeg is still transcoding first frame")
		log.Printf("   2) NAL unit parsing needs more data")
		log.Printf("   3) Frame channel may be empty")
		return nil, fmt.Errorf("timeout waiting for frame")
	}
}

func (r *RTSPVideoSource) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
		r.cmd.Wait()
	}

	if r.stdout != nil {
		r.stdout.Close()
	}

	return nil
}
