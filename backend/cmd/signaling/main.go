package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"webrtc-streaming/internal/config"
	"webrtc-streaming/internal/signaling"
)

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create signaling server
	signalServer := signaling.NewSignalingServer()
	go signalServer.Run()

	// Create HTTP mux
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", signalServer.HandleWebSocket)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serve static files
	staticPath := config.AppConfig.StaticFiles.Path
	if absPath, err := filepath.Abs(staticPath); err == nil {
		staticPath = absPath
	}

	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		log.Printf("Warning: Static files directory not found: %s", staticPath)
		log.Printf("Frontend will not be served. Build the frontend first with: cd frontend && npm run build")
	} else {
		log.Printf("Serving static files from: %s", staticPath)

		// File server for static files
		fileServer := http.FileServer(http.Dir(staticPath))

		// Serve static files, but exclude /ws and /health
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't serve static files for WebSocket and health endpoints
			if strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/health") {
				http.NotFound(w, r)
				return
			}

			// Check if the requested file exists
			requestedPath := filepath.Join(staticPath, r.URL.Path)

			// If it's a file (not a directory), serve it
			if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}

			// For SPA routing: if the file doesn't exist or it's a directory,
			// serve index.html to let the frontend router handle it
			indexPath := filepath.Join(staticPath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}

			// Fallback: try to serve from file server (for assets, etc.)
			fileServer.ServeHTTP(w, r)
		})
	}

	addr := fmt.Sprintf("%s:%d", config.AppConfig.SignalingServer.Host, config.AppConfig.SignalingServer.Port)
	log.Printf("Server starting on %s", addr)
	log.Printf("WebSocket endpoint: ws://%s/ws", addr)
	log.Printf("Frontend will be served at: http://%s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
