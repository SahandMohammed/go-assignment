package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv       string
	AppPort      string
	AppJWTSecret string

	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDB       string

	RedisAddr     string
	RedisDB       string
	RedisPassword string

	LogLevel string
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	return &Config{
		AppEnv:       getEnv("APP_ENV", "development"),
		AppPort:      getEnv("APP_PORT", "8080"),
		AppJWTSecret: getEnv("APP_JWT_SECRET", "supersecret"),

		MySQLHost:     getEnv("MYSQL_HOST", "127.0.0.1"),
		MySQLPort:     getEnv("MYSQL_PORT", "3306"),
		MySQLUser:     getEnv("MYSQL_USER", "wallet"),
		MySQLPassword: getEnv("MYSQL_PASSWORD", "walletpw"),
		MySQLDB:       getEnv("MYSQL_DB", "walletdb"),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisDB:       getEnv("REDIS_DB", "0"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		LogLevel: getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
