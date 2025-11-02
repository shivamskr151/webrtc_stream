package signaling

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"webrtc-streaming/internal/config"

	"github.com/gorilla/websocket"
)

type SignalingServer struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	config     *config.Config
}

type Client struct {
	conn     *websocket.Conn
	server   *SignalingServer
	send     chan []byte
	clientID string
}

type Message struct {
	Type      string      `json:"type"`
	ClientID  string      `json:"clientId,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Offer     interface{} `json:"offer,omitempty"`
	Answer    interface{} `json:"answer,omitempty"`
	Candidate interface{} `json:"candidate,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Allow connections without Origin header (e.g., from Go publisher client)
		if origin == "" {
			log.Printf("WebSocket connection from client without Origin header (allowed)")
			return true
		}

		// Check against allowed origins
		for _, allowedOrigin := range config.AppConfig.CORS.AllowedOrigins {
			if origin == allowedOrigin {
				log.Printf("WebSocket connection from allowed origin: %s", origin)
				return true
			}
		}

		// Log rejected origin for debugging
		log.Printf("WebSocket connection rejected - origin '%s' not in allowed list", origin)
		log.Printf("Allowed origins: %v", config.AppConfig.CORS.AllowedOrigins)

		// For development: allow localhost origins even if not in config
		// This helps when frontend runs on different ports
		if origin != "" {
			// Allow any localhost origin during development
			if (strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1")) &&
				(strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://")) {
				log.Printf("⚠️ Allowing localhost origin for development: %s", origin)
				return true
			}
		}

		return false
	},
}

func NewSignalingServer() *SignalingServer {
	return &SignalingServer{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		config:     config.AppConfig,
	}
}

func (s *SignalingServer) Run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			// Check if there are existing clients before adding this one
			hasExistingClients := len(s.clients) > 0
			s.clients[client] = true
			clientCount := len(s.clients)
			s.mu.Unlock()
			log.Printf("Client connected: %s (total clients: %d)", client.clientID, clientCount)

			// If there were existing clients (likely a publisher), notify them about the new viewer
			if hasExistingClients {
				notifyMsg := map[string]interface{}{
					"type":     "viewer_connected",
					"clientId": client.clientID,
				}
				notifyBytes, _ := json.Marshal(notifyMsg)
				s.mu.RLock()
				for otherClient := range s.clients {
					if otherClient != client {
						select {
						case otherClient.send <- notifyBytes:
							log.Printf("Notified client %s about new viewer %s", otherClient.clientID, client.clientID)
						default:
							log.Printf("Warning: Could not notify client %s (channel full)", otherClient.clientID)
						}
					}
				}
				s.mu.RUnlock()
			}

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				log.Printf("Client disconnected: %s", client.clientID)
			}
			s.mu.Unlock()

		// Broadcast channel is no longer needed, but kept for compatibility
		case <-s.broadcast:
		}
	}
}

func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client-%d", len(s.clients)+1)
	client := &Client{
		conn:     conn,
		server:   s,
		send:     make(chan []byte, 256),
		clientID: clientID,
	}

	// Register client (notification will be sent in Run() goroutine after registration)
	client.server.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		// Use proper close code
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}()

	// Set read deadline to detect dead connections
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Reset read deadline on successful read
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Parse as generic map first to preserve structure
		var rawMsg map[string]interface{}
		if err := json.Unmarshal(messageBytes, &rawMsg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Add sender's client ID as "fromClientId" to preserve target "clientId" if present
		// If clientId is not already in the message (from sender), add it as the sender's ID
		if _, exists := rawMsg["clientId"]; !exists {
			rawMsg["clientId"] = c.clientID
		}
		// Always include sender ID for routing
		rawMsg["fromClientId"] = c.clientID

		// Convert back to JSON and create Message
		messageBytes, err = json.Marshal(rawMsg)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			continue
		}

		// Broadcast to all other clients
		c.server.mu.RLock()
		for client := range c.server.clients {
			if client != c { // Don't send to sender
				select {
				case client.send <- messageBytes:
				default:
					close(client.send)
					delete(c.server.clients, client)
				}
			}
		}
		c.server.mu.RUnlock()

		// Note: viewer_connected notification is now sent in HandleWebSocket when client registers
		// This ensures publisher is notified immediately when viewer connects, not waiting for a message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed, send close message
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Error writing message: %v", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
