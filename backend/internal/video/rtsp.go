package video

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"webrtc-streaming/internal/config"
)

// detectBestEncoder detects and returns the best available H.264 encoder
// Returns encoder name and encoder-specific parameters
// Also checks if hardware devices are actually accessible
func detectBestEncoder() (string, []string) {
	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Printf("⚠️ FFmpeg not found in PATH, using default encoder")
		return "libx264", getSoftwareEncoderParams()
	}

	osType := runtime.GOOS
	log.Printf("🔍 Detecting hardware encoder on %s...", osType)

	switch osType {
	case "darwin": // macOS
		// Try VideoToolbox (Apple hardware encoder)
		if hasEncoder("h264_videotoolbox") {
			log.Println("✅ Found h264_videotoolbox (Apple hardware encoder)")
			return "h264_videotoolbox", getVideoToolboxParams()
		}
		log.Println("⚠️ h264_videotoolbox not available, falling back to software")

	case "linux":
		// Try VAAPI (Intel/AMD integrated graphics) - but check if device exists first
		if hasEncoder("h264_vaapi") && hasVAAPIDevice() {
			log.Println("✅ Found h264_vaapi (Intel/AMD hardware encoder) with accessible device")
			return "h264_vaapi", getVAAPIParams()
		} else if hasEncoder("h264_vaapi") {
			log.Println("⚠️ h264_vaapi found but device not accessible, falling back to software")
		}
		// Try NVENC (NVIDIA GPU)
		if hasEncoder("h264_nvenc") {
			log.Println("✅ Found h264_nvenc (NVIDIA hardware encoder)")
			return "h264_nvenc", getNVENCParams()
		}
		log.Println("⚠️ No hardware encoder found, falling back to software")

	case "windows":
		// Try NVENC (NVIDIA GPU)
		if hasEncoder("h264_nvenc") {
			log.Println("✅ Found h264_nvenc (NVIDIA hardware encoder)")
			return "h264_nvenc", getNVENCParams()
		}
		// Try AMF (AMD GPU)
		if hasEncoder("h264_amf") {
			log.Println("✅ Found h264_amf (AMD hardware encoder)")
			return "h264_amf", getAMFParams()
		}
		// Try QSV (Intel Quick Sync)
		if hasEncoder("h264_qsv") {
			log.Println("✅ Found h264_qsv (Intel hardware encoder)")
			return "h264_qsv", getQSVParams()
		}
		log.Println("⚠️ No hardware encoder found, falling back to software")
	}

	// Fallback to software encoder
	return "libx264", getSoftwareEncoderParams()
}

// hasVAAPIDevice checks if VAAPI device is accessible and functional
func hasVAAPIDevice() bool {
	// Check if /dev/dri/renderD128 exists (default VAAPI device)
	// Also check for other common render devices
	devices := []string{"/dev/dri/renderD128", "/dev/dri/renderD129", "/dev/dri/renderD130"}
	hasDevice := false
	for _, device := range devices {
		if _, err := os.Stat(device); err == nil {
			hasDevice = true
			break
		}
	}
	
	if !hasDevice {
		return false
	}
	
	// Test if VAAPI actually works by running a simple FFmpeg command
	// This catches cases where device exists but VAAPI driver isn't functional
	testCmd := exec.Command("ffmpeg", "-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=1",
		"-c:v", "h264_vaapi", "-vaapi_device", "/dev/dri/renderD128",
		"-frames:v", "1", "-f", "null", "-")
	testCmd.Stdout = nil
	testCmd.Stderr = nil
	if err := testCmd.Run(); err != nil {
		// VAAPI test failed - device exists but not functional
		log.Printf("⚠️ VAAPI device exists but test encoding failed - will use software encoding")
		return false
	}
	
	return true
}

// hasEncoder checks if FFmpeg has a specific encoder available
func hasEncoder(encoderName string) bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), encoderName)
}

// Encoder-specific parameter functions

func getVideoToolboxParams() []string {
	// VideoToolbox (macOS) - very low latency hardware encoding
	// Note: VideoToolbox doesn't support -rc, -maxrate, -bufsize options
	// Use -b:v for target bitrate instead
	return []string{
		"-allow_sw", "1", // Allow software fallback
		"-realtime", "1", // Real-time encoding mode
		"-b:v", "2M", // Target bitrate (VideoToolbox doesn't use -rc or -maxrate)
		"-prio_speed", "1", // Prioritize encoding speed for low latency
	}
}

func getNVENCParams() []string {
	// NVENC (NVIDIA) - low latency hardware encoding
	return []string{
		"-preset", "p1", // P1 = fastest, lowest latency
		"-rc", "vbr", // Variable bitrate
		"-tune", "ll", // Low latency tuning
		"-zerolatency", "1", // Zero latency mode
		"-gpu", "0", // Use first GPU
		"-delay", "0", // No delay
		"-no-scenecut", "1", // Disable scene cut detection (low latency)
		"-rc-lookahead", "0", // Disable lookahead for minimal buffering
		"-surfaces", "1", // Minimal surface count for lowest latency
		"-cbr", "0", // Disable constant bitrate mode
	}
}

func getVAAPIParams() []string {
	// VAAPI (Intel/AMD Linux) - low latency hardware encoding
	return []string{
		"-vaapi_device", "/dev/dri/renderD128", // Default render device
		"-b:v", "2M", // Bitrate
		"-maxrate", "2M", // Max bitrate
		"-bufsize", "2M", // Buffer size
		"-rc_mode", "VBR", // Variable bitrate
		"-low_power", "1", // Low power mode (lower latency)
	}
}

func getAMFParams() []string {
	// AMF (AMD Windows) - low latency hardware encoding
	return []string{
		"-quality", "speed", // Speed over quality
		"-rc", "vbr_peak", // Variable bitrate peak
		"-usage", "ultralowlatency", // Ultra low latency usage
	}
}

func getQSVParams() []string {
	// Quick Sync (Intel Windows) - low latency hardware encoding
	return []string{
		"-preset", "veryfast", // Fast preset
		"-async_depth", "1", // Minimal async depth (low latency)
		"-ref", "1", // Single reference frame
	}
}

func getSoftwareEncoderParams() []string {
	// Software encoder (libx264) - optimized for low latency
	return []string{
		"-preset", "ultrafast", // Fastest encoding
		"-tune", "zerolatency", // Zero latency tuning
		"-x264-params", "keyint=10:scenecut=0:force-cfr=1:sync-lookahead=0:sliced-threads=1:threads=auto", // Optimized for low latency
	}
}

// RTSPVideoSource handles RTSP stream using ffmpeg
type RTSPVideoSource struct {
	rtspURL      string
	cmd          *exec.Cmd
	stdout       io.ReadCloser
	frameChan    chan []byte
	errChan      chan error
	mu           sync.Mutex
	closed       bool
	accessUnit   []byte // Accumulator for SPS/PPS
	spsPps       []byte // Persistent copy of SPS/PPS for IDR frames
	spsPpsFound  bool   // Track if we've received SPS/PPS
	currentFrame []byte // Accumulator for all NAL units in current access unit
	frameRate    int    // Detected frame rate from stream (FPS)
}

func NewRTSPVideoSource(rtspURL string) (*RTSPVideoSource, error) {
	return &RTSPVideoSource{
		rtspURL:      rtspURL,
		frameChan:    make(chan []byte, 5), // Buffer 5 frames to prevent drops during network jitter
		errChan:      make(chan error, 1),
		accessUnit:   make([]byte, 0, 128*1024), // Further reduced for minimal latency
		spsPps:       make([]byte, 0, 1024),
		spsPpsFound:  false,
		currentFrame: make([]byte, 0, 64*1024),   // Minimal frame buffer
		frameRate:    config.AppConfig.Video.FPS, // Default to config, will be updated from stream
	}, nil
}

func (r *RTSPVideoSource) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("RTSP source already closed")
	}

	log.Printf("Starting RTSP stream from: %s", r.rtspURL)

	// Detect and use hardware acceleration for best performance
	encoder, encoderParams := detectBestEncoder()
	log.Printf("🎬 Using encoder: %s", encoder)

	// Build ffmpeg command to decode RTSP and output raw H264 frames
	// IMPORTANT: The stream might be HEVC/H.265, so we need to transcode to H.264
	// Browser support for H.264 is universal, but HEVC support is limited
	ffmpegArgs := []string{
		"-rtsp_transport", "tcp", // Use TCP for more reliable connection
		"-fflags", "nobuffer+flush_packets", // Reduce latency and flush immediately
		"-flags", "low_delay",
		"-strict", "experimental",
		"-analyzeduration", "200000", // Reduce analysis time (0.2 second) - faster startup
		"-probesize", "200000", // Reduce probe size - faster startup
		"-err_detect", "ignore_err", // Ignore non-critical decoding errors
		"-i", r.rtspURL,
		// Transcode to H.264 with optimized settings
		"-c:v", encoder, // Use detected best encoder (hardware or software)
		"-profile:v", "baseline", // Baseline profile for maximum compatibility
		"-level", "3.1", // Level 3.1 for good compatibility
		"-pix_fmt", "yuv420p", // Pixel format for compatibility
		"-color_range", "pc", // Use PC range (full range 0-255) for proper color
		"-colorspace", "bt709", // BT.709 color space (standard for web)
		"-color_primaries", "bt709", // BT.709 color primaries
		"-color_trc", "bt709", // BT.709 transfer characteristics
		"-bf", "0", // No B-frames (WebRTC requirement for low latency)
		"-g", "15", // GOP size (keyframe every 15 frames, ~1 second at 15fps) - matches actual frame rate for better buffering
		"-bsf:v", "h264_mp4toannexb", // Convert to Annex-B format (required for raw H264)
		"-f", "h264", // Raw H264 format
		"-flush_packets", "1", // Flush packets immediately
	}

	// Add encoder-specific parameters
	ffmpegArgs = append(ffmpegArgs, encoderParams...)
	ffmpegArgs = append(ffmpegArgs, "-") // Output to stdout

	if encoder != "libx264" {
		log.Printf("✅ Hardware acceleration enabled - latency reduced by ~60-80%%")
		log.Println("   If source is HEVC/H.265, it will be transcoded to H.264 for browser compatibility")
	} else {
		log.Println("⚠️ Using software encoding (libx264) - consider hardware acceleration for lower latency")
		log.Println("   If source is HEVC/H.265, it will be transcoded to H.264 for browser compatibility")
	}

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

	// Track if we've detected a critical FFmpeg error
	var ffmpegError error
	var ffmpegErrorMutex sync.Mutex

	// Log stderr in a goroutine for debugging and extract frame rate
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Filter out common HEVC decoder warnings that are expected when joining mid-stream
			// These are non-critical and just create log spam
			if len(line) > 0 {
				// Skip repetitive HEVC decoder warnings that are normal when joining mid-stream
				// These warnings occur because FFmpeg needs to wait for a keyframe to decode properly
				// They are non-critical - FFmpeg will automatically skip undecodable frames until it finds a keyframe
				lowerLine := strings.ToLower(line)
				isHEVCWarning := strings.Contains(lowerLine, "[hevc @") && (strings.Contains(lowerLine, "could not find ref with poc") ||
					strings.Contains(lowerLine, "error constructing the frame rps") ||
					strings.Contains(lowerLine, "skipping invalid undecodable nalu") ||
					strings.Contains(lowerLine, "pps id out of range"))

				// Also filter swscaler warnings about color conversion (non-critical performance info)
				isScalerWarning := strings.Contains(lowerLine, "[swscaler @") &&
					strings.Contains(lowerLine, "no accelerated colorspace conversion")

				// Skip non-critical warnings but log important messages
				if !isHEVCWarning && !isScalerWarning {
					log.Printf("ffmpeg: %s", line)
				}

				// Detect critical errors early (404, connection refused, hardware encoder failures, etc.)
				// (lowerLine already defined above)
				isHardwareEncoderError := strings.Contains(lowerLine, "vaapi") && (strings.Contains(lowerLine, "failed") ||
					strings.Contains(lowerLine, "error") ||
					strings.Contains(lowerLine, "device creation failed") ||
					strings.Contains(lowerLine, "failed to initialise") ||
					strings.Contains(lowerLine, "input/output error"))
				
				if strings.Contains(lowerLine, "404 not found") ||
					strings.Contains(lowerLine, "connection refused") ||
					strings.Contains(lowerLine, "failed") ||
					strings.Contains(lowerLine, "error opening input") ||
					isHardwareEncoderError {
					ffmpegErrorMutex.Lock()
					if ffmpegError == nil {
						ffmpegError = fmt.Errorf("FFmpeg critical error: %s", line)
						log.Printf("❌ FFmpeg error detected: %s", line)
						if isHardwareEncoderError {
							log.Printf("⚠️ Hardware encoder (VAAPI) failed - this may indicate hardware acceleration is not available")
							log.Printf("   Consider using software encoding (libx264) if this persists")
						}
					}
					ffmpegErrorMutex.Unlock()
				}

				// Extract frame rate from ffmpeg output (e.g., "15 fps")
				if strings.Contains(line, " fps") || strings.Contains(line, " tbr") {
					// Look for patterns like "15 fps" or "15 tbr"
					parts := strings.Fields(line)
					for i, part := range parts {
						if (part == "fps" || part == "tbr") && i > 0 {
							if fps, err := strconv.Atoi(strings.TrimSuffix(parts[i-1], ",")); err == nil && fps > 0 {
								r.mu.Lock()
								if r.frameRate != fps {
									log.Printf("📊 Detected frame rate from stream: %d FPS (was configured: %d FPS)", fps, r.frameRate)
									r.frameRate = fps
								}
								r.mu.Unlock()
								break
							}
						}
					}
				}
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		stdout.Close()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Monitor FFmpeg process exit in a separate goroutine
	go func() {
		err := cmd.Wait()
		ffmpegErrorMutex.Lock()
		hasError := ffmpegError != nil
		storedError := ffmpegError
		ffmpegErrorMutex.Unlock()

		if err != nil {
			// FFmpeg exited with error
			if !hasError {
				// If we didn't already capture an error from stderr, use the exit error
				storedError = fmt.Errorf("FFmpeg process exited with error: %w", err)
			}
			log.Printf("❌ FFmpeg process exited: %v", storedError)
		} else {
			log.Printf("⚠️ FFmpeg process exited normally (unexpected)")
			if !hasError {
				storedError = fmt.Errorf("FFmpeg process exited unexpectedly")
			}
		}

		// Send error to errChan so ReadFrame can detect it
		// Use recover to prevent panic if channel is already closed
		if storedError != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Channel was closed - this is expected if readFrames exited first
						log.Printf("⚠️ Could not send FFmpeg error to channel (already closed): %v", storedError)
					}
				}()

				// Check if source is closed before attempting to send
				r.mu.Lock()
				isClosed := r.closed
				r.mu.Unlock()

				if !isClosed {
					select {
					case r.errChan <- storedError:
						// Successfully sent error
					default:
						// Channel might be full or closed (non-blocking)
					}
				}
			}()
		}

		// Close stdout to signal readFrames that input is done
		if r.stdout != nil {
			r.stdout.Close()
		}
	}()

	// Start reading frames in a goroutine
	go r.readFrames()

	return nil
}

func (r *RTSPVideoSource) readFrames() {
	defer close(r.frameChan)
	defer close(r.errChan)

	// H264 NAL Unit start codes: 0x00000001 or 0x000001
	buffer := make([]byte, 0, 128*1024)              // 128KB initial buffer (minimal for zero-latency)
	reader := bufio.NewReaderSize(r.stdout, 16*1024) // 16KB read buffer (minimal)
	chunk := make([]byte, 8*1024)                    // Read 8KB chunks (minimal for real-time)

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
			// Helper function to safely send to error channel
			sendErrorSafely := func(errMsg string) {
				defer func() {
					if r := recover(); r != nil {
						// Channel was closed - this is expected during shutdown
					}
				}()
				select {
				case r.errChan <- fmt.Errorf("%s", errMsg):
				default:
					// Channel might be full or closed
				}
			}

			// Helper function to safely send frame
			sendFrameSafely := func(frame []byte) {
				defer func() {
					if r := recover(); r != nil {
						// Channel was closed - this is expected during shutdown
					}
				}()
				select {
				case r.frameChan <- frame:
				default:
					// Channel might be full or closed
				}
			}

			if err != io.EOF {
				// Only send error if channel is not already closed
				sendErrorSafely(fmt.Sprintf("failed to read from FFmpeg stdout: %v (FFmpeg may have exited)", err))
			} else {
				// EOF means FFmpeg closed its stdout
				// Send error to indicate FFmpeg exited
				sendErrorSafely("FFmpeg stdout closed (EOF) - process may have exited")
			}
			// Process remaining buffer before returning
			if len(buffer) > 0 {
				sendFrameSafely(buffer)
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

						// Check if this is a picture frame (IDR or P/B) or AUD delimiter
						isPictureFrame := nalTypeByte == 5 || nalTypeByte == 1
						isIDR := nalTypeByte == 5
						isAUD := nalTypeByte == 9 // Access Unit Delimiter

						// Send frame when:
						// 1. AUD encountered (marks end of access unit)
						// 2. New picture frame encountered AND we have a previous frame to send
						// 3. IDR frame encountered (always send IDRs immediately, even if first frame)
						shouldSendFrame := false
						if isAUD {
							// AUD marks end of access unit - send current frame if we have one
							shouldSendFrame = len(r.currentFrame) > 0
						} else if isPictureFrame {
							// For picture frames:
							// - Always send if it's an IDR (first frame or keyframe)
							// - Send previous frame if we have one accumulated
							if isIDR {
								// IDR frames should be sent immediately (especially the first one)
								// If we have accumulated data, send it first, then start accumulating the IDR
								if len(r.currentFrame) > 0 {
									shouldSendFrame = true
								} else {
									// No previous frame, but we should still send this IDR once accumulated
									// Don't send yet - add the IDR to currentFrame first
								}
							} else if len(r.currentFrame) > 0 {
								// P/B frame encountered - send previous frame
								shouldSendFrame = true
							}
						}

						if shouldSendFrame {
							// Send the accumulated frame
							var frameToSend []byte

							// Check if current frame contains an IDR (NAL type 5) anywhere
							hasIDR := false
							if len(r.spsPps) > 0 {
								// Scan through currentFrame to find IDR NAL unit
								i := 0
								for i < len(r.currentFrame) {
									var nalStart int
									var nalType byte

									// Find start code
									if i+4 <= len(r.currentFrame) && r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x00 && r.currentFrame[i+3] == 0x01 {
										nalStart = i + 4
										if nalStart < len(r.currentFrame) {
											nalType = r.currentFrame[nalStart] & 0x1F
										}
										i = nalStart + 1
									} else if i+3 <= len(r.currentFrame) && r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x01 {
										nalStart = i + 3
										if nalStart < len(r.currentFrame) {
											nalType = r.currentFrame[nalStart] & 0x1F
										}
										i = nalStart + 1
									} else {
										i++
										continue
									}

									if nalType == 5 {
										hasIDR = true
										break
									}
								}
							}

							if hasIDR && len(r.spsPps) > 0 {
								// Prepend SPS/PPS to IDR frame
								frameToSend = make([]byte, len(r.spsPps)+len(r.currentFrame))
								copy(frameToSend, r.spsPps)
								copy(frameToSend[len(r.spsPps):], r.currentFrame)
							} else {
								frameToSend = make([]byte, len(r.currentFrame))
								copy(frameToSend, r.currentFrame)
							}

							frameQueueCounter++
							// Send frame with buffering to handle network jitter
							// Buffer allows smooth playback during temporary network delays
							select {
							case r.frameChan <- frameToSend:
								// Frame sent successfully
								if frameQueueCounter <= 10 {
									log.Printf("📤 Queued complete access unit #%d: %d bytes", frameQueueCounter, len(frameToSend))
								}
							default:
								// Channel is full - drop oldest frame to prevent excessive buffering
								// This prevents buffer buildup while maintaining smooth playback
								select {
								case oldFrame := <-r.frameChan: // Remove and discard old frame
									_ = oldFrame // Explicitly discard old frame
									select {
									case r.frameChan <- frameToSend: // Add newest frame
										// Successfully replaced old frame with new one
										if frameQueueCounter%100 == 0 {
											log.Printf("⚡ Buffer full: Replaced old frame #%d with latest", frameQueueCounter)
										}
									default:
										// Extremely rare - channel filled between operations
										log.Printf("⚠️ Warning: Frame channel still full after drop")
									}
								default:
									// Channel became empty between checks - send new frame
									r.frameChan <- frameToSend
								}
							}

							// Clear current frame accumulator
							r.currentFrame = r.currentFrame[:0]
						}

						if isAUD {
							// AUD marks end of access unit - frame should have been sent above
							// Don't add AUD to frame, it's just a delimiter
							// However, if we have an accumulated frame but haven't sent it, send it now
							if len(r.currentFrame) > 0 && !shouldSendFrame {
								// We have a frame but AUD didn't trigger send - send it now
								var frameToSend []byte
								if len(r.spsPps) > 0 {
									// Check if currentFrame has IDR
									hasIDR := false
									for i := 0; i < len(r.currentFrame)-3; i++ {
										if r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x00 && r.currentFrame[i+3] == 0x01 {
											if i+4 < len(r.currentFrame) {
												nalType := r.currentFrame[i+4] & 0x1F
												if nalType == 5 {
													hasIDR = true
													break
												}
											}
										} else if i+2 < len(r.currentFrame) && r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x01 {
											if i+3 < len(r.currentFrame) {
												nalType := r.currentFrame[i+3] & 0x1F
												if nalType == 5 {
													hasIDR = true
													break
												}
											}
										}
									}
									if hasIDR {
										frameToSend = make([]byte, len(r.spsPps)+len(r.currentFrame))
										copy(frameToSend, r.spsPps)
										copy(frameToSend[len(r.spsPps):], r.currentFrame)
									} else {
										frameToSend = make([]byte, len(r.currentFrame))
										copy(frameToSend, r.currentFrame)
									}
								} else {
									frameToSend = make([]byte, len(r.currentFrame))
									copy(frameToSend, r.currentFrame)
								}
								// Send the frame
								select {
								case r.frameChan <- frameToSend:
									frameQueueCounter++
									if frameQueueCounter <= 10 {
										log.Printf("📤 Queued complete access unit #%d (via AUD): %d bytes", frameQueueCounter, len(frameToSend))
									}
								default:
									// Channel full, replace
									select {
									case <-r.frameChan:
										r.frameChan <- frameToSend
										frameQueueCounter++
									default:
										r.frameChan <- frameToSend
										frameQueueCounter++
									}
								}
								r.currentFrame = r.currentFrame[:0]
							}
						} else if isPictureFrame {
							// Start accumulating NAL units for this new frame
							wasEmpty := len(r.currentFrame) == 0
							r.currentFrame = append(r.currentFrame, nalUnit...)

							// Special case: If this is the first IDR frame and we have SPS/PPS,
							// we should send it after accumulating (when next NAL or timeout)
							// But for now, if we just started accumulating an IDR and we have SPS/PPS,
							// mark that we should check on next iteration
							if wasEmpty && isIDR && len(r.spsPps) > 0 {
								// First IDR frame with SPS/PPS - will be sent when next NAL arrives
								// or after a short delay if no more NALs come
							}
						} else if nalTypeByte == 7 || nalTypeByte == 8 {
							// SPS/PPS - accumulate for next access unit
							r.accessUnit = append(r.accessUnit, nalUnit...)

							// Check if we have both SPS and PPS now
							hasSPS := false
							hasPPS := false
							for i := 0; i < len(r.accessUnit)-3; i++ {
								var nalType byte
								if r.accessUnit[i] == 0x00 && r.accessUnit[i+1] == 0x00 && r.accessUnit[i+2] == 0x00 && r.accessUnit[i+3] == 0x01 {
									if i+4 < len(r.accessUnit) {
										nalType = r.accessUnit[i+4] & 0x1F
									} else {
										continue
									}
								} else if r.accessUnit[i] == 0x00 && r.accessUnit[i+1] == 0x00 && r.accessUnit[i+2] == 0x01 {
									if i+3 < len(r.accessUnit) {
										nalType = r.accessUnit[i+3] & 0x1F
									} else {
										continue
									}
								} else {
									continue
								}

								if nalType == 7 {
									hasSPS = true
								} else if nalType == 8 {
									hasPPS = true
								}
							}

							if hasSPS && hasPPS {
								// Save persistent copy of SPS/PPS
								r.spsPps = make([]byte, len(r.accessUnit))
								copy(r.spsPps, r.accessUnit)
								log.Printf("📋 Saved SPS/PPS: %d bytes", len(r.spsPps))
							}

							if !r.spsPpsFound {
								log.Printf("📋 Received %s parameter set (size: %d bytes)",
									map[byte]string{7: "SPS", 8: "PPS"}[nalTypeByte], len(nalUnit))
							}
						} else {
							// Other NAL unit types (AUD=9, SEI=6, etc.) - add to current frame if we're building one
							if len(r.currentFrame) > 0 {
								r.currentFrame = append(r.currentFrame, nalUnit...)
							} else {
								// Not building a frame yet, accumulate in accessUnit
								r.accessUnit = append(r.accessUnit, nalUnit...)
							}
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
				// Before breaking, check if we have an accumulated IDR frame that should be sent
				// This handles the case where we've accumulated a complete IDR but haven't encountered
				// an AUD or next picture frame yet
				if len(r.currentFrame) > 0 && len(r.spsPps) > 0 && frameQueueCounter == 0 {
					// Check if currentFrame contains an IDR
					hasIDR := false
					for i := 0; i < len(r.currentFrame)-3; i++ {
						if r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x00 && r.currentFrame[i+3] == 0x01 {
							if i+4 < len(r.currentFrame) {
								nalType := r.currentFrame[i+4] & 0x1F
								if nalType == 5 {
									hasIDR = true
									break
								}
							}
						} else if i+2 < len(r.currentFrame) && r.currentFrame[i] == 0x00 && r.currentFrame[i+1] == 0x00 && r.currentFrame[i+2] == 0x01 {
							if i+3 < len(r.currentFrame) {
								nalType := r.currentFrame[i+3] & 0x1F
								if nalType == 5 {
									hasIDR = true
									break
								}
							}
						}
					}
					// If we have an IDR frame with sufficient size, send it
					if hasIDR && len(r.currentFrame) > 100 {
						frameToSend := make([]byte, len(r.spsPps)+len(r.currentFrame))
						copy(frameToSend, r.spsPps)
						copy(frameToSend[len(r.spsPps):], r.currentFrame)
						select {
						case r.frameChan <- frameToSend:
							frameQueueCounter++
							log.Printf("📤 Queued first IDR frame (complete): %d bytes", len(frameToSend))
						default:
							// Channel full, but this is first frame so clear and send
							select {
							case <-r.frameChan:
								r.frameChan <- frameToSend
								frameQueueCounter++
								log.Printf("📤 Queued first IDR frame (complete): %d bytes", len(frameToSend))
							default:
								r.frameChan <- frameToSend
								frameQueueCounter++
								log.Printf("📤 Queued first IDR frame (complete): %d bytes", len(frameToSend))
							}
						}
						r.currentFrame = r.currentFrame[:0]
					}
				}

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

			// Prevent buffer from growing - aggressive limit for zero-latency
			if len(buffer) > 512*1024 { // 512KB max (minimal for real-time)
				// Keep only last 256KB (immediate processing)
				buffer = buffer[len(buffer)-256*1024:]
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
	// Check if source is closed first
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()

	if closed {
		return nil, fmt.Errorf("RTSP source is closed")
	}

	// Real-time streaming: Try to get frame with reasonable timeout
	// Increased timeout to prevent frame drops during network jitter
	// Frame rate is 15fps (66ms per frame), so 200ms timeout allows for some buffering
	timeout := time.After(200 * time.Millisecond)

	select {
	case frame, ok := <-r.frameChan:
		if !ok {
			// Channel closed - source has failed
			return nil, fmt.Errorf("frame channel closed - FFmpeg may have failed or exited")
		}
		return r.processFrame(frame)
	case err, ok := <-r.errChan:
		if !ok {
			// Error channel closed - source has failed
			return nil, fmt.Errorf("error channel closed - FFmpeg may have failed or exited")
		}
		if err != nil {
			log.Printf("RTSP error: %v", err)
			return nil, err
		}
		// err is nil but channel is open - this shouldn't happen, but handle it
		return nil, fmt.Errorf("unexpected nil error from error channel")
	case <-timeout:
		// Very short timeout - check if channel has frame now (non-blocking check)
		select {
		case frame, ok := <-r.frameChan:
			if !ok {
				// Channel closed while checking
				return nil, fmt.Errorf("frame channel closed - FFmpeg may have failed or exited")
			}
			// Got frame immediately after timeout - process it
			return r.processFrame(frame)
		case err, ok := <-r.errChan:
			if !ok {
				// Error channel closed
				return nil, fmt.Errorf("error channel closed - FFmpeg may have failed or exited")
			}
			if err != nil {
				log.Printf("RTSP error: %v", err)
				return nil, err
			}
			// err is nil but channel is open - this shouldn't happen, but handle it
			return nil, fmt.Errorf("unexpected nil error from error channel")
		default:
			// No frame available and no error - check if channels are closed
			r.mu.Lock()
			isClosed := r.closed
			r.mu.Unlock()

			if isClosed {
				return nil, fmt.Errorf("RTSP source is closed")
			}

			// No frame available yet - retry once more with a brief sleep to avoid busy-waiting
			// but don't recurse infinitely
			time.Sleep(33 * time.Millisecond) // ~1 frame at 30fps, prevents excessive retries
			select {
			case frame, ok := <-r.frameChan:
				if !ok {
					return nil, fmt.Errorf("frame channel closed - FFmpeg may have failed or exited")
				}
				return r.processFrame(frame)
			case err, ok := <-r.errChan:
				if !ok {
					return nil, fmt.Errorf("error channel closed - FFmpeg may have failed or exited")
				}
				if err != nil {
					return nil, err
				}
				// err is nil but channel is open - this shouldn't happen, but handle it
				return nil, fmt.Errorf("unexpected nil error from error channel")
			default:
				// Still no frame - return error instead of infinite recursion
				return nil, fmt.Errorf("no frame available - FFmpeg may still be initializing or stream may be unavailable")
			}
		}
	}
}

// processFrame handles frame processing logic separately for reusability
func (r *RTSPVideoSource) processFrame(frame []byte) ([]byte, error) {
	frameReadCount++
	if frame == nil {
		return nil, fmt.Errorf("frame channel closed")
	}
	if len(frame) == 0 {
		// Skip empty frames, try again immediately
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

// GetFrameRate returns the detected frame rate from the stream
func (r *RTSPVideoSource) GetFrameRate() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.frameRate
}
