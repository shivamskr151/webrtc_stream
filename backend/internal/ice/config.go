package ice

import (
	"log"

	"webrtc-streaming/internal/config"

	"github.com/pion/webrtc/v4"
)

// DefaultSTUNServers provides fallback STUN servers for redundancy
var DefaultSTUNServers = []string{
	"stun:stun1.l.google.com:19302",
	"stun:stun2.l.google.com:19302",
	"stun:stun3.l.google.com:19302",
}

// GetWebRTCConfiguration creates and returns a WebRTC configuration with optimized ICE/STUN/TURN settings
// This centralizes all ICE server configuration logic for reuse across the application
func GetWebRTCConfiguration() webrtc.Configuration {
	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
		// Allow all ICE transport types (UDP, TCP, relay)
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
		// Enable ICE candidate gathering for all types
		ICECandidatePoolSize: 0, // Let Pion manage this
	}

	// Add ICE servers from config (STUN/TURN servers)
	for _, iceURL := range config.AppConfig.WebRTC.ICEServerURLs {
		iceServer := webrtc.ICEServer{
			URLs: []string{iceURL},
		}
		// Add credentials if provided (required for TURN servers)
		if config.AppConfig.WebRTC.ICEServerUsername != "" {
			iceServer.Username = config.AppConfig.WebRTC.ICEServerUsername
			iceServer.Credential = config.AppConfig.WebRTC.ICEServerCredential
		}
		webrtcConfig.ICEServers = append(webrtcConfig.ICEServers, iceServer)
	}

	// Add multiple STUN servers for redundancy (if only one configured)
	// This ensures better connectivity even if one STUN server is down
	if len(webrtcConfig.ICEServers) <= 1 {
		log.Println("Adding additional STUN servers for better connectivity...")
		for _, stunURL := range DefaultSTUNServers {
			webrtcConfig.ICEServers = append(webrtcConfig.ICEServers, webrtc.ICEServer{
				URLs: []string{stunURL},
			})
		}
	}

	log.Printf("✅ Configured %d ICE server(s) for connectivity", len(webrtcConfig.ICEServers))

	return webrtcConfig
}
