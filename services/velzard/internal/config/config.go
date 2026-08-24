package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	TelemetrySecret   string
	AWSRegion         string
}

func LoadConfig() *Config {
	_ = godotenv.Load("../../.env") // Load from root if exists

	return &Config{
		Port:            getEnv("VELZARD_PORT", "8082"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://velzion_user:velzion_password@localhost:5432/velzion_dev?sslmode=disable"),
		JWTSecret:       getEnv("JWT_SECRET", "super-secret-local-dev-key"),
		TelemetrySecret: getEnv("TELEMETRY_WEBHOOK_SECRET", "L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA"),
		AWSRegion:       getEnv("AWS_REGION", "us-east-1"),
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
