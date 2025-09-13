package main

import (
	"log"

	"github.com/SahandMohammed/wallet-service/internal/config"
	"github.com/SahandMohammed/wallet-service/internal/db"
	"github.com/SahandMohammed/wallet-service/internal/http/router"
	"github.com/SahandMohammed/wallet-service/internal/migration"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Setup logger
	logrus.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// Initialize database connections
	mysqlDB, err := db.NewMySQLConnection(cfg)
	if err != nil {
		logrus.Fatal("Failed to connect to MySQL:", err)
	}

	redisClient, err := db.NewRedisConnection(cfg)
	if err != nil {
		logrus.Fatal("Failed to connect to Redis:", err)
	}

	// Run migrations
	if err := migration.AutoMigrate(mysqlDB); err != nil {
		logrus.Fatal("Failed to run migrations:", err)
	}

	// Set Gin mode
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Setup router
	r := router.SetupRouter(mysqlDB, redisClient, cfg)

	// Start server
	port := cfg.AppPort
	if port == "" {
		port = "8080"
	}

	logrus.WithField("port", port).Info("Starting server")
	if err := r.Run(":" + port); err != nil {
		logrus.Fatal("Failed to start server:", err)
	}
}
