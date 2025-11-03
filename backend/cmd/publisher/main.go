package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"webrtc-streaming/internal/config"
	iceutils "webrtc-streaming/internal/ice"
	"webrtc-streaming/internal/video"

	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type ViewerConnection struct {
	clientID string
	pc       *webrtc.PeerConnection
}

type Publisher struct {
	viewers      map[string]*ViewerConnection // Track connections by client ID
	viewersMu    sync.RWMutex                 // Mutex for concurrent access to viewers map
	wsConn       *websocket.Conn
	wsConnMu     sync.RWMutex // Mutex for WebSocket connection
	signalingURL string
	track        *webrtc.TrackLocalStaticSample
	capturer     *video.VideoCapturer
	api          *webrtc.API
	webrtcConfig webrtc.Configuration
	shouldStop   bool       // Flag to stop reconnection attempts
	stopMu       sync.Mutex // Mutex for shouldStop flag
}

func NewPublisher() (*Publisher, error) {
	// Get centralized WebRTC configuration (ICE/STUN/TURN)
	webrtcConfig := iceutils.GetWebRTCConfiguration()

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

	capturer, err := video.NewVideoCapturer()
	if err != nil {
		return nil, fmt.Errorf("failed to create video capturer: %w", err)
	}

	publisher := &Publisher{
		viewers:      make(map[string]*ViewerConnection),
		signalingURL: fmt.Sprintf("ws://%s:%d/ws", config.AppConfig.SignalingServer.Host, config.AppConfig.SignalingServer.Port),
		capturer:     capturer,
		api:          api,
		webrtcConfig: webrtcConfig,
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
	log.Printf("   Track will be added to each viewer's peer connection")

	return publisher, nil
}

// createViewerConnection creates a new peer connection for a viewer
func (p *Publisher) createViewerConnection(clientID string) (*ViewerConnection, error) {
	log.Printf("Creating new peer connection for viewer: %s", clientID)

	// Create new peer connection
	pc, err := p.api.NewPeerConnection(p.webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create a new track instance for this viewer (can reuse the same track data source)
	// Actually, we can use the same track instance - TrackLocalStaticSample can be added to multiple PCs
	sender, err := pc.AddTrack(p.track)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("failed to add track: %w", err)
	}

	// Handle RTCP packets from the receiver
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
				if rtcpErr != io.EOF {
					log.Printf("RTCP read error for viewer %s: %v", clientID, rtcpErr)
				}
				return
			}
		}
	}()

	// Set up ICE candidate handling
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			p.sendICECandidate(candidate, clientID)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("📡 [%s] Peer connection state: %s", clientID, state.String())
		if state == webrtc.PeerConnectionStateClosed {
			// Only clean up when connection is explicitly closed
			p.removeViewer(clientID)
		} else if state == webrtc.PeerConnectionStateFailed {
			// For failed state, wait a bit and try to recover
			go func() {
				time.Sleep(5 * time.Second)
				// Check if connection recovered
				p.viewersMu.RLock()
				viewer, exists := p.viewers[clientID]
				p.viewersMu.RUnlock()

				if exists && viewer.pc != nil {
					currentState := viewer.pc.ConnectionState()
					currentICE := viewer.pc.ICEConnectionState()
					if currentState == webrtc.PeerConnectionStateFailed &&
						(currentICE == webrtc.ICEConnectionStateFailed || currentICE == webrtc.ICEConnectionStateClosed) {
						log.Printf("🔄 [%s] Connection still failed after 5s, attempting ICE restart...", clientID)
						// Try to restart ICE by creating a new offer
						if err := p.restartICEForViewer(clientID); err != nil {
							log.Printf("❌ [%s] Failed to restart ICE: %v, removing connection", clientID, err)
							p.removeViewer(clientID)
						}
					} else if currentState == webrtc.PeerConnectionStateConnected ||
						currentState == webrtc.PeerConnectionStateConnecting {
						log.Printf("✅ [%s] Connection recovered!", clientID)
					}
				}
			}()
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("🧊 [%s] ICE connection state: %s", clientID, state.String())
		if state == webrtc.ICEConnectionStateConnected {
			log.Printf("✅ [%s] ICE connected - media flowing!", clientID)
		} else if state == webrtc.ICEConnectionStateDisconnected {
			log.Printf("⚠️ [%s] ICE disconnected - will wait for recovery (up to 10s)...", clientID)
			// Give it time to recover - disconnected state might be temporary
			go func() {
				time.Sleep(10 * time.Second)
				p.viewersMu.RLock()
				viewer, exists := p.viewers[clientID]
				p.viewersMu.RUnlock()

				if exists && viewer.pc != nil {
					iceState := viewer.pc.ICEConnectionState()
					if iceState == webrtc.ICEConnectionStateDisconnected ||
						iceState == webrtc.ICEConnectionStateFailed {
						log.Printf("🔄 [%s] ICE still disconnected after 10s, attempting restart...", clientID)
						if err := p.restartICEForViewer(clientID); err != nil {
							log.Printf("❌ [%s] Failed to restart ICE: %v", clientID, err)
						}
					} else if iceState == webrtc.ICEConnectionStateConnected {
						log.Printf("✅ [%s] ICE connection recovered!", clientID)
					}
				}
			}()
		} else if state == webrtc.ICEConnectionStateFailed {
			log.Printf("❌ [%s] ICE connection failed - will attempt recovery...", clientID)
			// Don't immediately remove - try to restart ICE first
			go func() {
				time.Sleep(2 * time.Second)
				p.viewersMu.RLock()
				viewer, exists := p.viewers[clientID]
				p.viewersMu.RUnlock()

				if exists && viewer.pc != nil {
					iceState := viewer.pc.ICEConnectionState()
					if iceState == webrtc.ICEConnectionStateFailed {
						log.Printf("🔄 [%s] Attempting ICE restart after failure...", clientID)
						if err := p.restartICEForViewer(clientID); err != nil {
							log.Printf("❌ [%s] Failed to restart ICE: %v, will remove connection", clientID, err)
							// Only remove if restart fails
							time.Sleep(3 * time.Second)
							p.viewersMu.RLock()
							viewer, stillExists := p.viewers[clientID]
							p.viewersMu.RUnlock()
							if stillExists && viewer.pc != nil &&
								viewer.pc.ICEConnectionState() == webrtc.ICEConnectionStateFailed {
								p.removeViewer(clientID)
							}
						}
					}
				}
			}()
		}
	})

	viewerConn := &ViewerConnection{
		clientID: clientID,
		pc:       pc,
	}

	return viewerConn, nil
}

func (p *Publisher) removeViewer(clientID string) {
	p.viewersMu.Lock()
	defer p.viewersMu.Unlock()

	if viewer, exists := p.viewers[clientID]; exists {
		if viewer.pc != nil {
			viewer.pc.Close()
		}
		delete(p.viewers, clientID)
		log.Printf("Removed viewer connection: %s", clientID)
	}
}

func (p *Publisher) Connect() error {
	// Close existing connection if any
	p.wsConnMu.Lock()
	if p.wsConn != nil {
		p.wsConn.Close()
		p.wsConn = nil
	}
	p.wsConnMu.Unlock()

	log.Println("Connecting to signaling server...")
	// Connect to signaling server
	conn, _, err := websocket.DefaultDialer.Dial(p.signalingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to signaling server: %w", err)
	}

	p.wsConnMu.Lock()
	p.wsConn = conn
	p.wsConnMu.Unlock()

	log.Println("Connected to signaling server")

	// Start reading messages
	go p.readMessages()

	// Send initial offer after connection is established
	// Note: We'll wait for "viewer_connected" message instead
	log.Println("   Waiting for viewer to connect...")
	log.Println("   (Offer will be sent when viewer connects)")

	return nil
}

// restartICEForViewer attempts to restart ICE by creating a new offer
func (p *Publisher) restartICEForViewer(clientID string) error {
	p.viewersMu.RLock()
	viewer, exists := p.viewers[clientID]
	p.viewersMu.RUnlock()

	if !exists || viewer.pc == nil {
		return fmt.Errorf("viewer connection not found: %s", clientID)
	}

	// Create a new offer to restart ICE
	log.Printf("🔄 [%s] Creating new offer to restart ICE...", clientID)
	offer, err := viewer.pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create restart offer: %w", err)
	}

	// Set local description with ICE restart flag
	if err := viewer.pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description for restart: %w", err)
	}

	// Send the offer to restart ICE negotiation
	offerMsg := map[string]interface{}{
		"type":     "offer",
		"clientId": clientID,
		"offer": map[string]interface{}{
			"type": offer.Type.String(),
			"sdp":  offer.SDP,
		},
	}
	if err := p.sendMessage(offerMsg); err != nil {
		return fmt.Errorf("failed to send restart offer: %w", err)
	}

	log.Printf("✅ [%s] Restart offer sent successfully", clientID)
	return nil
}

func (p *Publisher) sendOffer(clientID string) error {
	p.viewersMu.RLock()
	viewer, exists := p.viewers[clientID]
	p.viewersMu.RUnlock()

	if !exists {
		return fmt.Errorf("viewer connection not found: %s", clientID)
	}

	// Create and send offer
	log.Printf("[%s] Creating WebRTC offer...", clientID)
	offer, err := viewer.pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := viewer.pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	// Send offer through signaling server
	log.Printf("[%s] Sending offer to viewer...", clientID)
	// Serialize offer to match browser's RTCSessionDescription format
	offerMsg := map[string]interface{}{
		"type":     "offer",
		"clientId": clientID,
		"offer": map[string]interface{}{
			"type": offer.Type.String(),
			"sdp":  offer.SDP,
		},
	}
	if err := p.sendMessage(offerMsg); err != nil {
		return fmt.Errorf("failed to send offer: %w", err)
	}
	log.Printf("✅ [%s] Offer sent successfully (SDP length: %d bytes)", clientID, len(offer.SDP))
	return nil
}

func (p *Publisher) readMessages() {
	// Set read deadline and pong handler
	p.wsConnMu.RLock()
	conn := p.wsConn
	p.wsConnMu.RUnlock()

	if conn == nil {
		log.Printf("❌ WebSocket connection is nil, cannot read messages")
		return
	}

	// Set longer read deadline to handle ping intervals (server pings every 54s)
	readDeadline := 90 * time.Second
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	conn.SetPongHandler(func(string) error {
		p.wsConnMu.RLock()
		if p.wsConn != nil {
			p.wsConn.SetReadDeadline(time.Now().Add(readDeadline))
		}
		p.wsConnMu.RUnlock()
		return nil
	})

	// Set ping handler to send pong response
	conn.SetPingHandler(func(message string) error {
		p.wsConnMu.RLock()
		if p.wsConn != nil {
			p.wsConn.SetReadDeadline(time.Now().Add(readDeadline))
			// Send pong in response to ping
			if err := p.wsConn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(10*time.Second)); err != nil {
				log.Printf("⚠️ Error sending pong: %v", err)
			}
		}
		p.wsConnMu.RUnlock()
		return nil
	})

	for {
		p.wsConnMu.RLock()
		conn = p.wsConn
		p.wsConnMu.RUnlock()

		if conn == nil {
			log.Printf("❌ WebSocket connection lost, will attempt reconnection")
			break
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Check if it's a close error
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket read error: %v", err)
			} else {
				log.Printf("WebSocket closed: %v", err)
			}
			// Mark connection as lost
			p.wsConnMu.Lock()
			p.wsConn = nil
			p.wsConnMu.Unlock()
			break
		}

		// Reset read deadline on successful read
		p.wsConnMu.RLock()
		if p.wsConn != nil {
			p.wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
		p.wsConnMu.RUnlock()

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		msgType, _ := msg["type"].(string)
		log.Printf("📥 Received message type: %s (full message keys: %v)", msgType, getKeys(msg))

		switch msgType {
		case "viewer_connected":
			// Extract client ID from message
			clientID, ok := msg["clientId"].(string)
			if !ok {
				log.Printf("⚠️ viewer_connected message missing clientId")
				continue
			}

			log.Printf("New viewer connected: %s, creating peer connection...", clientID)

			// Check if a connection already exists for this client ID and clean it up
			p.viewersMu.Lock()
			if existingViewer, exists := p.viewers[clientID]; exists {
				log.Printf("⚠️ Viewer %s already exists, cleaning up old connection first", clientID)
				if existingViewer.pc != nil {
					existingViewer.pc.Close()
				}
				delete(p.viewers, clientID)
			}
			p.viewersMu.Unlock()

			// Create new peer connection for this viewer
			viewerConn, err := p.createViewerConnection(clientID)
			if err != nil {
				log.Printf("❌ Failed to create peer connection for %s: %v", clientID, err)
				continue
			}

			// Store viewer connection
			p.viewersMu.Lock()
			p.viewers[clientID] = viewerConn
			p.viewersMu.Unlock()

			log.Printf("✅ Created peer connection for viewer: %s", clientID)
			log.Printf("   Active viewers: %d", len(p.viewers))

			// Send offer to the new viewer
			if err := p.sendOffer(clientID); err != nil {
				log.Printf("❌ Failed to send offer to %s: %v", clientID, err)
				p.removeViewer(clientID)
			}

		case "answer":
			log.Printf("📥 Received answer message, checking clientId...")
			// Get client ID to route to correct peer connection
			// Try both clientId and fromClientId (signaling server might add fromClientId)
			clientID, ok := msg["clientId"].(string)
			if !ok {
				// Try fromClientId as fallback
				if fromClientID, ok2 := msg["fromClientId"].(string); ok2 {
					clientID = fromClientID
					ok = true
					log.Printf("⚠️ Answer message missing clientId, using fromClientId: %s", clientID)
				} else {
					log.Printf("⚠️ Answer message missing both clientId and fromClientId, cannot route")
					log.Printf("   Message keys: %v", getKeys(msg))
					continue
				}
			}

			p.viewersMu.RLock()
			viewer, exists := p.viewers[clientID]
			p.viewersMu.RUnlock()

			if !exists {
				log.Printf("⚠️ Received answer from unknown viewer: %s", clientID)
				continue
			}

			log.Printf("📥 [%s] Received answer from viewer!", clientID)
			answerSDP := msg["answer"].(map[string]interface{})
			sdpStr, ok := answerSDP["sdp"].(string)
			if !ok {
				log.Printf("❌ [%s] Answer SDP is not a string: %T", clientID, answerSDP["sdp"])
				continue
			}

			answer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeAnswer,
				SDP:  sdpStr,
			}

			log.Printf("   [%s] Answer SDP length: %d bytes", clientID, len(sdpStr))

			// CRITICAL: Set remote description BEFORE adding ICE candidates
			if err := viewer.pc.SetRemoteDescription(answer); err != nil {
				log.Printf("❌ [%s] Error setting remote description: %v", clientID, err)
				continue
			}

			log.Printf("✅ [%s] Remote description (answer) set successfully", clientID)

			// Check if video codec is negotiated
			if strings.Contains(answer.SDP, "H264") || strings.Contains(answer.SDP, "h264") {
				log.Printf("✅ [%s] H264 codec detected in answer SDP", clientID)
			} else {
				log.Printf("⚠️ [%s] H264 codec not found in answer SDP", clientID)
			}

			log.Printf("✅ [%s] WebRTC connection should establish now - ICE will negotiate", clientID)
			log.Printf("   [%s] Current state: PC=%s, ICE=%s",
				clientID, viewer.pc.ConnectionState().String(), viewer.pc.ICEConnectionState().String())

		case "candidate":
			// Get client ID to route to correct peer connection
			// Try both clientId and fromClientId (signaling server adds fromClientId)
			clientID, ok := msg["clientId"].(string)
			if !ok {
				// Try fromClientId as fallback
				if fromClientID, ok2 := msg["fromClientId"].(string); ok2 {
					clientID = fromClientID
					ok = true
					log.Printf("⚠️ Candidate message missing clientId, using fromClientId: %s", clientID)
				} else {
					log.Printf("⚠️ Candidate message missing both clientId and fromClientId, cannot route")
					log.Printf("   Message keys: %v", getKeys(msg))
					continue
				}
			}

			p.viewersMu.RLock()
			viewer, exists := p.viewers[clientID]
			p.viewersMu.RUnlock()

			if !exists {
				log.Printf("⚠️ Received candidate from unknown viewer: %s", clientID)
				p.viewersMu.RLock()
				ids := make([]string, 0, len(p.viewers))
				for id := range p.viewers {
					ids = append(ids, id)
				}
				p.viewersMu.RUnlock()
				log.Printf("   Available viewers: %v", ids)
				continue
			}

			log.Printf("🧊 [%s] Received ICE candidate from viewer", clientID)
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

			if err := viewer.pc.AddICECandidate(candidate); err != nil {
				candidatePreview := candidateStr
				if len(candidatePreview) > 80 {
					candidatePreview = candidatePreview[:80]
				}
				log.Printf("❌ [%s] Error adding ICE candidate (%s): %v - %s", clientID, candidateType, err, candidatePreview)
			} else {
				log.Printf("✅ [%s] Added remote ICE candidate (%s)", clientID, candidateType)
			}
		}
	}

	// After loop exits, try to reconnect if not stopped
	p.stopMu.Lock()
	shouldReconnect := !p.shouldStop
	p.stopMu.Unlock()

	if shouldReconnect {
		log.Printf("🔄 Attempting to reconnect to signaling server in 2 seconds...")
		time.Sleep(2 * time.Second)
		if err := p.Connect(); err != nil {
			log.Printf("❌ Reconnection failed: %v, will retry in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			// Retry once more, then let it fail silently (could add exponential backoff)
			if err := p.Connect(); err != nil {
				log.Printf("❌ Reconnection failed again: %v", err)
			}
		} else {
			log.Printf("✅ Successfully reconnected to signaling server")
		}
	}
}

func (p *Publisher) sendMessage(msg map[string]interface{}) error {
	p.wsConnMu.RLock()
	conn := p.wsConn
	p.wsConnMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket connection is nil")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (p *Publisher) sendICECandidate(candidate *webrtc.ICECandidate, clientID string) {
	p.wsConnMu.RLock()
	conn := p.wsConn
	p.wsConnMu.RUnlock()

	if conn == nil {
		log.Printf("⚠️ Cannot send ICE candidate - WebSocket not connected")
		return
	}

	candidateJSON := candidate.ToJSON()
	msg := map[string]interface{}{
		"type":     "candidate",
		"clientId": clientID,
		"candidate": map[string]interface{}{
			"candidate":     candidateJSON.Candidate,
			"sdpMLineIndex": candidateJSON.SDPMLineIndex,
			"sdpMid":        candidateJSON.SDPMid,
		},
	}

	if err := p.sendMessage(msg); err != nil {
		log.Printf("❌ [%s] Error sending ICE candidate: %v", clientID, err)
	} else {
		// Log candidate type for debugging (but limit logging to avoid spam)
		candidateStr := candidateJSON.Candidate
		if strings.Contains(candidateStr, " typ host ") {
			log.Printf("📤 [%s] Sent host ICE candidate (localhost)", clientID)
		} else if strings.Contains(candidateStr, " typ srflx ") {
			log.Printf("📤 [%s] Sent srflx ICE candidate (STUN)", clientID)
		}
	}
}

func (p *Publisher) StartStreaming() error {
	log.Println("🎬 Starting video stream...")
	log.Println("   Video will be sent to all connected viewers")
	log.Println("   (Streaming will start regardless of connection state - WebRTC handles buffering)")

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

	log.Println("🎥 Starting frame capture loop...")
	log.Println("   IMPORTANT: Transcoding HEVC→H.264 may take 5-15 seconds to produce first frame")
	log.Println("   IMPORTANT: ICE negotiation may take 10-30 seconds to complete")
	log.Println("   Total wait time: 15-45 seconds before video appears")
	log.Println("   Connection will be checked continuously - frames will buffer if not ready")

	lastFrameTime := time.Now()
	maxFrameWait := 15 * time.Second // Max time to wait for first frame

	for range ticker.C {
		// Check active viewers
		p.viewersMu.RLock()
		viewerCount := len(p.viewers)
		p.viewersMu.RUnlock()

		// Log active viewers periodically
		if frameCount%300 == 0 && viewerCount > 0 {
			log.Printf("📊 Active viewers: %d", viewerCount)
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
		// The same track instance is used for all viewers - writing once sends to all
		writeErr := p.track.WriteSample(sample)
		if writeErr != nil {
			errorCount++
			// Minimal logging for uninterrupted streaming - only log significant issues
			if errorCount <= 3 || errorCount%100 == 0 {
				log.Printf("❌ Error writing sample (count: %d): %v", errorCount, writeErr)
				log.Printf("   Active viewers: %d", viewerCount)
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
			log.Printf("   Active viewers: %d", viewerCount)

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
			log.Printf("✅ Streamed %d frames successfully (viewers: %d, last size: %d bytes)",
				frameCount, viewerCount, len(sample.Data))
		}

		// Log when streaming starts
		if frameCount == 2 && viewerCount > 0 {
			log.Printf("🎉 Stream is active! Frames are being transmitted to %d viewer(s).", viewerCount)
			log.Printf("   If video doesn't display in browser, check:")
			log.Printf("   1) Browser console for '✅ Received track'")
			log.Printf("   2) chrome://webrtc-internals/ for packet transmission")
			log.Printf("   3) Browser codec support (Chrome/Edge recommended for H264)")
		}
	}

	return nil
}

func (p *Publisher) Close() {
	// Set stop flag to prevent reconnection
	p.stopMu.Lock()
	p.shouldStop = true
	p.stopMu.Unlock()

	if p.capturer != nil {
		p.capturer.Close()
	}

	// Close all viewer connections
	p.viewersMu.Lock()
	for clientID, viewer := range p.viewers {
		if viewer.pc != nil {
			viewer.pc.Close()
		}
		log.Printf("Closed connection for viewer: %s", clientID)
	}
	p.viewers = make(map[string]*ViewerConnection)
	p.viewersMu.Unlock()

	p.wsConnMu.Lock()
	if p.wsConn != nil {
		// Send proper close message before closing
		p.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		p.wsConn.Close()
		p.wsConn = nil
	}
	p.wsConnMu.Unlock()
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
