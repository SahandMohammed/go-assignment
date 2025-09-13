package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
	Version   string            `json:"version"`
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// Health check endpoint
func (h *HealthHandler) Health(c *gin.Context) {
	health := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  make(map[string]string),
		Version:   "1.0.0",
	}

	// Check database connection
	sqlDB, err := h.db.DB()
	if err != nil {
		health.Status = "unhealthy"
		health.Services["database"] = "error: " + err.Error()
	} else {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			health.Status = "unhealthy"
			health.Services["database"] = "error: " + err.Error()
		} else {
			health.Services["database"] = "healthy"
		}
	}

	// Check Redis connection
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		_, err := h.redis.Ping(ctx).Result()
		if err != nil {
			health.Status = "unhealthy"
			health.Services["redis"] = "error: " + err.Error()
		} else {
			health.Services["redis"] = "healthy"
		}
	} else {
		health.Services["redis"] = "not configured"
	}

	// Set appropriate HTTP status
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, health)
}

// Readiness check endpoint
func (h *HealthHandler) Ready(c *gin.Context) {
	// Check if all critical services are ready
	ready := true
	services := make(map[string]string)

	// Check database
	sqlDB, err := h.db.DB()
	if err != nil {
		ready = false
		services["database"] = "not ready: " + err.Error()
	} else {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			ready = false
			services["database"] = "not ready: " + err.Error()
		} else {
			services["database"] = "ready"
		}
	}

	// Check Redis
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		_, err := h.redis.Ping(ctx).Result()
		if err != nil {
			ready = false
			services["redis"] = "not ready: " + err.Error()
		} else {
			services["redis"] = "ready"
		}
	}

	response := gin.H{
		"ready":     ready,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"services":  services,
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// Liveness check endpoint
func (h *HealthHandler) Live(c *gin.Context) {
	// Simple liveness check - just verify the service is running
	c.JSON(http.StatusOK, gin.H{
		"alive":     true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}
