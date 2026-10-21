package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	MongoURI            string
	DatabaseName        string
	JWTSecret           string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURI  string
	FrontendURL         string
	AllowMockAuth       bool
}

func LoadConfig() *Config {
	// Attempt to load .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found or error reading, using environment defaults")
	}

	port := getEnv("PORT", "8080")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("DB_NAME", "skribbl_dare")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-skribbl-dare-jwt-key-2026")
	discordClientID := getEnv("DISCORD_CLIENT_ID", "")
	discordClientSecret := getEnv("DISCORD_CLIENT_SECRET", "")
	discordRedirectURI := getEnv("DISCORD_REDIRECT_URI", "http://localhost:8080/api/auth/discord/callback")
	frontendURL := getEnv("FRONTEND_URL", "https://skribbl.io")
	allowMockAuthStr := getEnv("ALLOW_MOCK_AUTH", "true")
	allowMockAuth, _ := strconv.ParseBool(allowMockAuthStr)

	return &Config{
		Port:                port,
		MongoURI:            mongoURI,
		DatabaseName:        dbName,
		JWTSecret:           jwtSecret,
		DiscordClientID:     discordClientID,
		DiscordClientSecret: discordClientSecret,
		DiscordRedirectURI:  discordRedirectURI,
		FrontendURL:         frontendURL,
		AllowMockAuth:       allowMockAuth,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
