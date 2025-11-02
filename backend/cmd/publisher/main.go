package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"webrtc-streaming/internal/config"
	"webrtc-streaming/internal/video"

	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type Publisher struct {
	pc           *webrtc.PeerConnection
	wsConn       *websocket.Conn
	signalingURL string
	track        *webrtc.TrackLocalStaticSample
	capturer     *video.VideoCapturer
}

func NewPublisher() (*Publisher, error) {
	// Create WebRTC configuration with improved ICE settings
	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
		// Allow all ICE transport types (UDP, TCP, relay)
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
		// Enable ICE candidate gathering for all types
		ICECandidatePoolSize: 0, // Let Pion manage this
	}

	// Add ICE servers from config
	for _, iceURL := range config.AppConfig.WebRTC.ICEServerURLs {
		iceServer := webrtc.ICEServer{
			URLs: []string{iceURL},
		}
		if config.AppConfig.WebRTC.ICEServerUsername != "" {
			iceServer.Username = config.AppConfig.WebRTC.ICEServerUsername
			iceServer.Credential = config.AppConfig.WebRTC.ICEServerCredential
		}
		webrtcConfig.ICEServers = append(webrtcConfig.ICEServers, iceServer)
	}

	// Add multiple STUN servers for redundancy (if only one configured)
	if len(webrtcConfig.ICEServers) <= 1 {
		log.Println("Adding additional STUN servers for better connectivity...")
		additionalSTUN := []string{
			"stun:stun1.l.google.com:19302",
			"stun:stun2.l.google.com:19302",
			"stun:stun3.l.google.com:19302",
		}
		for _, stunURL := range additionalSTUN {
			webrtcConfig.ICEServers = append(webrtcConfig.ICEServers, webrtc.ICEServer{
				URLs: []string{stunURL},
			})
		}
	}

	log.Printf("✅ Configured %d ICE server(s) for connectivity", len(webrtcConfig.ICEServers))

	// Create peer connection with proper codec support
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	// H264 codec is already registered by RegisterDefaultCodecs()
	// No need to manually register it
	if config.AppConfig.Video.RTSPURL != "" {
		log.Println("H264 codec support enabled for RTSP stream")
	}

	// Create interceptor registry
	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		return nil, err
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
	)
	pc, err := api.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	capturer, err := video.NewVideoCapturer()
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to create video capturer: %w", err)
	}

	publisher := &Publisher{
		pc:           pc,
		signalingURL: fmt.Sprintf("ws://%s:%d/ws", config.AppConfig.SignalingServer.Host, config.AppConfig.SignalingServer.Port),
		capturer:     capturer,
	}

	// Determine codec based on video source
	// Use H264 if RTSP is configured, otherwise VP8
	mimeType := webrtc.MimeTypeVP8
	if config.AppConfig.Video.RTSPURL != "" {
		mimeType = webrtc.MimeTypeH264
		log.Println("Using H264 codec for RTSP stream")
	} else {
		log.Println("Using VP8 codec for mock stream")
	}

	// Create video track with proper codec configuration
	codecCapability := webrtc.RTPCodecCapability{
		MimeType: mimeType,
	}

	// For H264, ensure proper clock rate
	if mimeType == webrtc.MimeTypeH264 {
		codecCapability.ClockRate = 90000
		log.Println("Configured H264 track with 90000 Hz clock rate")
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		codecCapability,
		"video",
		"publisher",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}
	publisher.track = videoTrack
	log.Printf("✅ Created video track with codec: %s", mimeType)

	// Add track to peer connection
	sender, err := pc.AddTrack(videoTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add track: %w", err)
	}
	log.Println("✅ Added video track to peer connection")

	// Handle RTCP packets from the receiver
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
				if rtcpErr != io.EOF {
					log.Printf("RTCP read error: %v", rtcpErr)
				}
				return
			}
			// Log RTCP packets for debugging (optional - can be verbose)
			// log.Printf("RTCP packet received")
		}
	}()

	// Verify track is ready
	log.Printf("Track ID: %s, Kind: %s", videoTrack.ID(), videoTrack.Kind())

	// Set up ICE candidate handling
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			log.Printf("🧊 Local ICE candidate generated: %s", candidate.String())
			publisher.sendICECandidate(candidate)
		} else {
			log.Println("✅ All local ICE candidates gathered")
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("📡 Peer connection state changed: %s", state.String())
		if state == webrtc.PeerConnectionStateConnected {
			log.Println("✅ WebRTC peer connection established!")
		} else if state == webrtc.PeerConnectionStateFailed {
			log.Printf("❌ Peer connection failed - check network and ICE configuration")
		} else if state == webrtc.PeerConnectionStateClosed {
			log.Printf("❌ Peer connection closed")
		}
	})

	// Add track event handler to verify track is being sent
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("📹 Received remote track: %s, kind: %s", track.ID(), track.Kind())
	})

	// Monitor data channel state if needed
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📨 Data channel received: %s", dc.Label())
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("🧊 ICE connection state changed: %s", state.String())
		if state == webrtc.ICEConnectionStateConnected {
			log.Println("✅ ICE connection established - media should flow now!")
			log.Printf("   🎬 Video frames can now be transmitted to viewer")
		} else if state == webrtc.ICEConnectionStateFailed {
			log.Println("❌ ICE connection failed!")
			log.Printf("   Troubleshooting:")
			log.Printf("   1) Check firewall - ensure UDP ports are not blocked")
			log.Printf("   2) Check network - try different network (mobile hotspot)")
			log.Printf("   3) Consider TURN server for strict NAT/firewall")
			log.Printf("   4) Check STUN server accessibility")
		} else if state == webrtc.ICEConnectionStateChecking {
			log.Printf("   ⏳ ICE checking connectivity (this can take 10-30 seconds)")
			log.Printf("   If stuck >30s: Network/firewall may be blocking UDP")
		} else if state == webrtc.ICEConnectionStateDisconnected {
			log.Printf("   ⚠️ ICE disconnected!")
			log.Printf("   Possible causes:")
			log.Printf("   1) ICE candidates not exchanged properly")
			log.Printf("   2) Network interruption")
			log.Printf("   3) Firewall blocking connection")
			log.Printf("   Checking connection state...")

			// Log current state for debugging
			pcState := pc.ConnectionState()
			log.Printf("   PeerConnection state: %s", pcState.String())

			// If we're still connected at PC level but ICE disconnected, this is unusual
			if pcState == webrtc.PeerConnectionStateConnected {
				log.Printf("   ⚠️ PC is connected but ICE is disconnected - media won't flow!")
			}
		} else if state == webrtc.ICEConnectionStateClosed {
			log.Printf("   ❌ ICE connection closed")
		}
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("📹 Received remote track: %s, kind: %s", track.ID(), track.Kind())
	})

	return publisher, nil
}

func (p *Publisher) Connect() error {
	log.Println("Connecting to signaling server...")
	// Connect to signaling server
	conn, _, err := websocket.DefaultDialer.Dial(p.signalingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to signaling server: %w", err)
	}
	p.wsConn = conn
	log.Println("Connected to signaling server")

	// Start reading messages
	go p.readMessages()

	// Send initial offer after connection is established
	// Note: We'll wait for "viewer_connected" message instead
	log.Println("   Waiting for viewer to connect...")
	log.Println("   (Offer will be sent when viewer connects)")

	return nil
}

func (p *Publisher) sendOffer() error {
	// Create and send offer
	log.Println("Creating WebRTC offer...")
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := p.pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	// Send offer through signaling server
	log.Println("Sending offer to signaling server...")
	// Serialize offer to match browser's RTCSessionDescription format
	offerMsg := map[string]interface{}{
		"type": "offer",
		"offer": map[string]interface{}{
			"type": offer.Type.String(),
			"sdp":  offer.SDP,
		},
	}
	if err := p.sendMessage(offerMsg); err != nil {
		return fmt.Errorf("failed to send offer: %w", err)
	}
	log.Println("✅ Offer sent successfully to viewer")
	log.Printf("   Waiting for answer... (offer SDP length: %d bytes)", len(offer.SDP))
	return nil
}

func (p *Publisher) readMessages() {
	// Set read deadline and pong handler
	p.wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
	p.wsConn.SetPongHandler(func(string) error {
		p.wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := p.wsConn.ReadMessage()
		if err != nil {
			// Check if it's a close error
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket read error: %v", err)
			} else {
				log.Printf("WebSocket closed: %v", err)
			}
			return
		}

		// Reset read deadline on successful read
		p.wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		switch msg["type"] {
		case "viewer_connected":
			log.Println("New viewer connected, sending offer...")
			p.sendOffer()

		case "answer":
			log.Println("📥 Received answer from viewer!")
			answerSDP := msg["answer"].(map[string]interface{})
			sdpStr, ok := answerSDP["sdp"].(string)
			if !ok {
				log.Printf("❌ Answer SDP is not a string: %T", answerSDP["sdp"])
				return
			}

			answer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeAnswer,
				SDP:  sdpStr,
			}

			log.Printf("   Answer SDP length: %d bytes", len(sdpStr))

			// CRITICAL: Set remote description BEFORE adding ICE candidates
			if err := p.pc.SetRemoteDescription(answer); err != nil {
				log.Printf("❌ Error setting remote description: %v", err)
				return
			}

			log.Println("✅ Remote description (answer) set successfully")
			log.Printf("   Connection should start establishing now...")

			// Check if video codec is negotiated
			if strings.Contains(answer.SDP, "H264") || strings.Contains(answer.SDP, "h264") {
				log.Println("✅ H264 codec detected in answer SDP")
			} else {
				log.Println("⚠️ H264 codec not found in answer SDP - check codec negotiation")
			}

			log.Println("✅ WebRTC connection should establish now - ICE will negotiate")
			log.Printf("   Current connection state: PC=%s, ICE=%s",
				p.pc.ConnectionState().String(), p.pc.ICEConnectionState().String())

			// Now that remote description is set, ICE can start gathering candidates
			log.Println("   📡 Waiting for ICE candidates to be exchanged...")

		case "candidate":
			log.Println("🧊 Received ICE candidate from viewer")
			candidateMap := msg["candidate"].(map[string]interface{})
			candidate := webrtc.ICECandidateInit{
				Candidate: candidateMap["candidate"].(string),
			}
			if sdpMLineIndex, ok := candidateMap["sdpMLineIndex"].(float64); ok {
				idx := uint16(sdpMLineIndex)
				candidate.SDPMLineIndex = &idx
			}
			if sdpMid, ok := candidateMap["sdpMid"].(string); ok {
				candidate.SDPMid = &sdpMid
			}

			// Check if remote description is set (required before adding candidates)
			if p.pc.RemoteDescription() == nil {
				log.Printf("⚠️ Remote description not set yet, buffering candidate...")
				// Buffer candidate - but this shouldn't happen if signaling is correct
			}

			// Extract candidate type for logging
			candidateStr := candidate.Candidate
			candidateType := "unknown"
			if strings.Contains(candidateStr, " typ host ") {
				candidateType = "host (localhost)"
			} else if strings.Contains(candidateStr, " typ srflx ") {
				candidateType = "srflx (STUN)"
			} else if strings.Contains(candidateStr, " typ relay ") {
				candidateType = "relay (TURN)"
			}

			if err := p.pc.AddICECandidate(candidate); err != nil {
				log.Printf("❌ Error adding ICE candidate (%s): %v", candidateType, err)
				candidatePreview := candidateStr
				if len(candidatePreview) > 80 {
					candidatePreview = candidatePreview[:80]
				}
				log.Printf("   Candidate: %s", candidatePreview)
				// This is critical - if we can't add candidates, connection will fail
			} else {
				candidateStrShort := candidateStr
				if len(candidateStrShort) > 80 {
					candidateStrShort = candidateStrShort[:80] + "..."
				}
				log.Printf("✅ Added remote ICE candidate (%s): %s", candidateType, candidateStrShort)
			}
		}
	}
}

func (p *Publisher) sendMessage(msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Set write deadline
	p.wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return p.wsConn.WriteMessage(websocket.TextMessage, data)
}

func (p *Publisher) sendICECandidate(candidate *webrtc.ICECandidate) {
	if p.wsConn == nil {
		log.Printf("⚠️ Cannot send ICE candidate - WebSocket not connected")
		return
	}

	candidateJSON := candidate.ToJSON()
	msg := map[string]interface{}{
		"type": "candidate",
		"candidate": map[string]interface{}{
			"candidate":     candidateJSON.Candidate,
			"sdpMLineIndex": candidateJSON.SDPMLineIndex,
			"sdpMid":        candidateJSON.SDPMid,
		},
	}

	if err := p.sendMessage(msg); err != nil {
		log.Printf("❌ Error sending ICE candidate: %v", err)
	} else {
		// Log candidate type for debugging (but limit logging to avoid spam)
		candidateStr := candidateJSON.Candidate
		if strings.Contains(candidateStr, " typ host ") {
			log.Printf("📤 Sent host ICE candidate (localhost)")
		} else if strings.Contains(candidateStr, " typ srflx ") {
			log.Printf("📤 Sent srflx ICE candidate (STUN)")
		}
	}
}

func (p *Publisher) StartStreaming() error {
	log.Println("🎬 Starting video stream...")

	// Wait for WebRTC connection to establish
	log.Println("⏳ Waiting for WebRTC connection to establish...")
	log.Println("   (Make sure frontend/viewer is connected and has sent answer)")

	maxWait := 60 // seconds
	connected := false
	startTime := time.Now()

	for i := 0; i < maxWait; i++ {
		connState := p.pc.ConnectionState()
		iceState := p.pc.ICEConnectionState()

		// Check if we're fully connected
		if connState == webrtc.PeerConnectionStateConnected &&
			iceState == webrtc.ICEConnectionStateConnected {
			log.Println("✅ WebRTC connection fully established!")
			log.Printf("   PC: %s, ICE: %s (waited %.1f seconds)",
				connState.String(), iceState.String(), time.Since(startTime).Seconds())
			connected = true
			break
		}

		// Check for failed states
		if connState == webrtc.PeerConnectionStateFailed ||
			iceState == webrtc.ICEConnectionStateFailed {
			log.Printf("❌ Connection failed! PC: %s, ICE: %s", connState.String(), iceState.String())
			log.Printf("   Check: 1) Is viewer connected? 2) Are ICE candidates exchanged? 3) Network/Firewall?")
			break
		}

		if i%5 == 0 && i > 0 {
			log.Printf("⏳ Still waiting... PC: %s, ICE: %s (waited %d/%d seconds)",
				connState.String(), iceState.String(), i, maxWait)
			log.Printf("   Make sure viewer has clicked 'Connect' and sent answer")
		}
		time.Sleep(1 * time.Second)
	}

	// Check final state
	connState := p.pc.ConnectionState()
	iceState := p.pc.ICEConnectionState()

	if !connected {
		if connState == webrtc.PeerConnectionStateFailed ||
			iceState == webrtc.ICEConnectionStateFailed {
			log.Printf("❌ Cannot start streaming - connection failed")
			return fmt.Errorf("connection failed: PC=%s, ICE=%s", connState.String(), iceState.String())
		}
		log.Printf("⚠️ Warning: Connection not fully established (PC: %s, ICE: %s)",
			connState.String(), iceState.String())
		log.Printf("   Will attempt to stream anyway, but frames may not be transmitted")
		log.Printf("   Ensure viewer is connected and WebRTC negotiation is complete")
	} else {
		log.Printf("✅ Connection ready - starting stream (PC: %s, ICE: %s)",
			connState.String(), iceState.String())
	}

	// Additional wait to ensure everything is ready
	log.Println("⏳ Final connection check (2 seconds)...")
	time.Sleep(2 * time.Second)

	finalConnState := p.pc.ConnectionState()
	finalIceState := p.pc.ICEConnectionState()
	log.Printf("Final state before streaming: PC=%s, ICE=%s", finalConnState.String(), finalIceState.String())

	// Get actual frame rate from capturer (detected from stream)
	actualFPS := p.capturer.GetFrameRate()
	if actualFPS <= 0 {
		actualFPS = config.AppConfig.Video.FPS
	}

	// Calculate frame interval for precise timing
	frameRate := time.Second / time.Duration(actualFPS)
	// Use high-precision ticker for perfect real-time streaming
	ticker := time.NewTicker(frameRate)
	defer ticker.Stop()

	log.Printf("⏱️ Frame rate: %d FPS (interval: %v) - Real-time streaming enabled", actualFPS, frameRate)

	frameCount := 0
	errorCount := 0
	lastLogTime := time.Now()

	log.Println("🎥 Starting frame capture loop...")
	log.Println("   IMPORTANT: Transcoding HEVC→H.264 may take 5-15 seconds to produce first frame")
	log.Println("   IMPORTANT: ICE negotiation may take 10-30 seconds to complete")
	log.Println("   Total wait time: 15-45 seconds before video appears")
	log.Println("   Connection will be checked continuously - frames will buffer if not ready")

	startedStreaming := false
	lastFrameTime := time.Now()
	maxFrameWait := 15 * time.Second // Max time to wait for first frame

	for range ticker.C {
		// Check connection state periodically
		connState := p.pc.ConnectionState()
		iceState := p.pc.ICEConnectionState()

		// Log connection state changes
		if connState == webrtc.PeerConnectionStateConnected && iceState == webrtc.ICEConnectionStateConnected {
			if !startedStreaming {
				log.Println("🎉 Connection fully established! Starting to stream frames...")
				startedStreaming = true
			}
		} else if connState == webrtc.PeerConnectionStateFailed || iceState == webrtc.ICEConnectionStateFailed {
			log.Printf("❌ Connection failed! PC: %s, ICE: %s", connState.String(), iceState.String())
			log.Printf("   Will continue attempting to stream...")
		}

		// Stream regardless of connection state - WebRTC will buffer
		// But log warnings if not connected after initial wait
		if !startedStreaming && connState != webrtc.PeerConnectionStateConnected && connState != webrtc.PeerConnectionStateConnecting {
			if time.Since(lastLogTime) > 5*time.Second {
				log.Printf("⏳ Waiting for connection... (PC: %s, ICE: %s) - frames will be buffered",
					connState.String(), iceState.String())
				lastLogTime = time.Now()
			}
		} else if startedStreaming && connState != webrtc.PeerConnectionStateConnected && connState != webrtc.PeerConnectionStateConnecting {
			if time.Since(lastLogTime) > 10*time.Second {
				log.Printf("⚠️ Connection lost while streaming (PC: %s, ICE: %s)",
					connState.String(), iceState.String())
				lastLogTime = time.Now()
			}
		}

		sample, err := p.capturer.CaptureFrame()
		if err != nil {
			errorCount++

			// Check if error indicates FFmpeg has failed permanently
			// Only treat as fatal if it's an actual FFmpeg process failure, not temporary "no frame available"
			errStr := err.Error()
			isFatalError := false

			// Only treat these specific error patterns as fatal (actual FFmpeg failures):
			// - "FFmpeg process exited"
			// - "FFmpeg critical error"
			// - "FFmpeg stdout closed"
			// - "FFmpeg may have failed or exited" (from channel closed errors)
			// - "channel closed" (when it indicates FFmpeg failure)
			// BUT NOT: "no frame available" which is just a temporary condition
			if strings.Contains(errStr, "FFmpeg process exited") ||
				strings.Contains(errStr, "FFmpeg critical error") ||
				strings.Contains(errStr, "FFmpeg stdout closed") ||
				strings.Contains(errStr, "FFmpeg may have failed") ||
				(strings.Contains(errStr, "channel closed") &&
					!strings.Contains(errStr, "no frame available")) {
				isFatalError = true
			}

			// For fatal errors, log immediately and check if we should stop
			if isFatalError {
				log.Printf("❌ Fatal error detected (count: %d): %v", errorCount, err)
				log.Printf("   FFmpeg process appears to have failed - check RTSP stream availability")

				// If we haven't received any frames and error persists, stop after threshold
				if frameCount == 0 && errorCount >= 60 { // ~2 seconds at 30fps
					log.Printf("❌ Stopping stream: FFmpeg failed and no frames received after %d attempts", errorCount)
					return fmt.Errorf("FFmpeg failed: %w (no frames captured after %d attempts)", err, errorCount)
				}
			}

			// Check if we've been waiting too long for first frame
			if frameCount == 0 && time.Since(lastFrameTime) > maxFrameWait {
				log.Printf("⚠️ No frames captured after %.0f seconds", maxFrameWait.Seconds())
				log.Printf("   This might mean:")
				log.Printf("   1) FFmpeg transcoding is still initializing (HEVC→H.264 takes time)")
				log.Printf("   2) RTSP stream is not accessible")
				log.Printf("   3) FFmpeg encountered an error")
				log.Printf("   Error: %v", err)

				// If it's a fatal error and we've waited too long, stop
				if isFatalError {
					log.Printf("❌ FFmpeg fatal error persists - stopping stream")
					return fmt.Errorf("FFmpeg fatal error after waiting %.0f seconds: %w", maxFrameWait.Seconds(), err)
				}

				log.Printf("   Will continue waiting...")
				lastFrameTime = time.Now() // Reset timer
			}

			// For continuous streaming, don't log every error (reduces spam)
			// Log fatal errors immediately, but for temporary errors (like "no frame available"),
			// only log periodically to avoid spam, especially if we're already streaming successfully
			if isFatalError {
				// Always log fatal errors immediately
				log.Printf("❌ Error capturing frame (count: %d): %v", errorCount, err)
			} else if frameCount == 0 {
				// During initialization, log temporary errors periodically
				if errorCount%30 == 0 {
					log.Printf("⚠️ Temporary error capturing frame (count: %d): %v", errorCount, err)
					log.Printf("   Continuing stream - will retry next frame...")
				}
			} else {
				// After we've successfully streamed frames, suppress "no frame available" errors
				// as they're just temporary gaps between frames
				if !strings.Contains(errStr, "no frame available") {
					// Log other temporary errors periodically
					if errorCount%60 == 0 {
						log.Printf("⚠️ Temporary error capturing frame (count: %d): %v", errorCount, err)
						log.Printf("   Continuing stream - will retry next frame...")
					}
				}
				// Completely suppress "no frame available" errors when already streaming
			}
			// Don't skip ticker - continue immediately to keep frame rate consistent
			continue
		}

		// Reset frame timer on success
		if frameCount == 0 {
			lastFrameTime = time.Now()
		}

		// Verify sample data is valid
		if len(sample.Data) == 0 {
			if errorCount%30 == 0 {
				log.Printf("⚠️ Empty sample received (frame %d)", frameCount)
			}
			errorCount++
			continue
		}

		// Log first frame details
		if frameCount == 0 {
			log.Printf("🎉 FIRST FRAME CAPTURED! %d bytes, duration: %v", len(sample.Data), sample.Duration)
			log.Printf("   ✅ RTSP→FFmpeg→H.264 parsing pipeline is WORKING!")
			log.Printf("   Next step: Frame will be written to WebRTC track")
		}

		// Write sample to track (non-blocking, zero-latency real-time streaming)
		// Always attempt write - WebRTC handles buffering internally
		writeErr := p.track.WriteSample(sample)
		if writeErr != nil {
			errorCount++
			// Minimal logging for uninterrupted streaming - only log significant issues
			if errorCount <= 3 || (errorCount%100 == 0 && connState == webrtc.PeerConnectionStateConnected) {
				log.Printf("❌ Error writing sample (count: %d): %v", errorCount, writeErr)
				log.Printf("   Connection state: PC=%s, ICE=%s", connState.String(), iceState.String())
			}
			// Continue immediately - never block, maintain perfect frame timing
			// WebRTC's internal buffers handle temporary connection issues
			continue
		}

		// Successfully wrote sample
		errorCount = 0 // Reset error count on success
		frameCount++

		if frameCount == 1 {
			log.Printf("✅ First frame written successfully! Size: %d bytes", len(sample.Data))
			log.Printf("   Connection state: PC=%s, ICE=%s", connState.String(), iceState.String())

			// Verify H264 format
			if len(sample.Data) >= 4 {
				if sample.Data[0] == 0x00 && sample.Data[1] == 0x00 && sample.Data[2] == 0x00 && sample.Data[3] == 0x01 {
					log.Printf("   ✅ Valid 4-byte H264 Annex-B start code")
				} else if sample.Data[0] == 0x00 && sample.Data[1] == 0x00 && sample.Data[2] == 0x01 {
					log.Printf("   ✅ Valid 3-byte H264 Annex-B start code")
				}
			}
		}

		if frameCount%30 == 0 {
			log.Printf("✅ Streamed %d frames successfully (PC: %s, ICE: %s, last size: %d bytes)",
				frameCount, connState.String(), iceState.String(), len(sample.Data))
		}

		// Log connection state changes during streaming
		if frameCount > 1 && (connState == webrtc.PeerConnectionStateConnected && iceState == webrtc.ICEConnectionStateConnected) {
			if frameCount == 2 {
				log.Printf("🎉 Stream is active! Frames are being transmitted.")
				log.Printf("   If video doesn't display in browser, check:")
				log.Printf("   1) Browser console for '✅ Received track'")
				log.Printf("   2) chrome://webrtc-internals/ for packet transmission")
				log.Printf("   3) Browser codec support (Chrome/Edge recommended for H264)")
			}
		}
	}

	return nil
}

func (p *Publisher) Close() {
	if p.capturer != nil {
		p.capturer.Close()
	}
	if p.pc != nil {
		p.pc.Close()
	}
	if p.wsConn != nil {
		// Send proper close message before closing
		p.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		p.wsConn.Close()
	}
}

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	publisher, err := NewPublisher()
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	if err := publisher.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Start streaming
	if err := publisher.StartStreaming(); err != nil {
		log.Fatalf("Failed to start streaming: %v", err)
	}
}
