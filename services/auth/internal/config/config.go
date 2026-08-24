package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	FrontendURL        string
	JWTSecret          string
}

func LoadConfig() *Config {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	return &Config{
		Port:               getEnv("AUTH_PORT", "8081"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://velzion_user:velzion_password@localhost:5432/velzion_dev?sslmode=disable"),
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		JWTSecret:          getEnv("JWT_SECRET", "super-secret-key"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if fallback == "" {
		log.Fatalf("Environment variable %s is required", key)
	}
	return fallback
}
