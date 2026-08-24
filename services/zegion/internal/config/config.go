package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	GithubWebhookSecret string
	AWSRegion           string
}

func LoadConfig() *Config {
	_ = godotenv.Load("../../.env") // Load from root if exists

	return &Config{
		Port:                getEnv("ZEGION_PORT", "8083"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://velzion_user:velzion_password@localhost:5432/velzion_dev?sslmode=disable"),
		JWTSecret:           getEnv("JWT_SECRET", "super-secret-local-dev-key"),
		GithubWebhookSecret: getEnv("GITHUB_WEBHOOK_SECRET", "my-github-secret"),
		AWSRegion:           getEnv("AWS_REGION", "us-east-1"),
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
