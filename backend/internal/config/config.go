package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	FrontendURL string

	SteamAPIKey   string
	AdminSteamIDs map[string]bool

	JWTSecret string
	JWTExpiry time.Duration

	BotPort        string
	GameServerAddr string
	GameServers    []string

	LogPort    string // UDP port for CS2 remote log listener
	BackendAddr string // IP of this backend (sent to CS2 via logaddress_add_ss)

	FaceitAPIKey string
}

var C Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	expiry, err := time.ParseDuration(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		log.Fatalf("Invalid JWT_EXPIRY: %v", err)
	}

	adminIDs := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("ADMIN_STEAM_IDS"), ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			adminIDs[id] = true
		}
	}

	C = Config{
		Port:          getEnv("PORT", "8080"),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:5173"),
		SteamAPIKey:   mustGetEnv("STEAM_API_KEY"),
		AdminSteamIDs: adminIDs,
		JWTSecret:     mustGetEnv("JWT_SECRET"),
		JWTExpiry:     expiry,
		BotPort:        getEnv("BOT_PORT", "3001"),
		GameServerAddr: getEnv("GAME_SERVER_ADDR", ""),
		GameServers:    splitTrimmed(os.Getenv("GAME_SERVERS")),
		LogPort:        getEnv("LOG_PORT", "27016"),
		BackendAddr:    getEnv("BACKEND_ADDR", ""),
		FaceitAPIKey:   getEnv("FACEIT_API_KEY", ""),
	}
}

func splitTrimmed(s string) []string {
	var result []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return v
}
