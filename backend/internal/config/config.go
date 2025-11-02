package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	SignalingServer SignalingServerConfig
	PublisherServer PublisherServerConfig
	WebRTC          WebRTCConfig
	Video           VideoConfig
	CORS            CORSConfig
	StaticFiles     StaticFilesConfig
}

type SignalingServerConfig struct {
	Host string
	Port int
}

type PublisherServerConfig struct {
	Host string
	Port int
}

type WebRTCConfig struct {
	ICEServerURLs       []string
	ICEServerUsername   string
	ICEServerCredential string
}

type VideoConfig struct {
	DeviceIndex int
	Width       int
	Height      int
	FPS         int
	RTSPURL     string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type StaticFilesConfig struct {
	Path string
}

var AppConfig *Config

func LoadConfig() error {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		// .env file is optional, continue with environment variables
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	AppConfig = &Config{
		SignalingServer: SignalingServerConfig{
			Host: getEnv("SIGNALING_SERVER_HOST", "localhost"),
			Port: getEnvAsInt("SIGNALING_SERVER_PORT", 8080),
		},
		PublisherServer: PublisherServerConfig{
			Host: getEnv("PUBLISHER_SERVER_HOST", "localhost"),
			Port: getEnvAsInt("PUBLISHER_SERVER_PORT", 8081),
		},
		WebRTC: WebRTCConfig{
			ICEServerURLs:       parseStringSlice(getEnv("ICE_SERVER_URLS", "stun:stun.l.google.com:19302"), ","),
			ICEServerUsername:   getEnv("ICE_SERVER_USERNAME", ""),
			ICEServerCredential: getEnv("ICE_SERVER_CREDENTIAL", ""),
		},
		Video: VideoConfig{
			DeviceIndex: getEnvAsInt("VIDEO_DEVICE_INDEX", 0),
			Width:       getEnvAsInt("VIDEO_WIDTH", 1280),
			Height:      getEnvAsInt("VIDEO_HEIGHT", 720),
			FPS:         getEnvAsInt("VIDEO_FPS", 30),
			RTSPURL:     getEnv("RTSP_URL", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: parseStringSlice(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"), ","),
		},
		StaticFiles: StaticFilesConfig{
			Path: getEnv("STATIC_FILES_PATH", "../frontend/dist"),
		},
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func parseStringSlice(value string, separator string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, separator)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
