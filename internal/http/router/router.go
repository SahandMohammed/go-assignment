package router

import (
	"github.com/SahandMohammed/wallet-service/internal/config"
	"github.com/SahandMohammed/wallet-service/internal/http/handler"
	"github.com/SahandMohammed/wallet-service/internal/http/middleware"
	"github.com/SahandMohammed/wallet-service/internal/repository"
	"github.com/SahandMohammed/wallet-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *gin.Engine {
	r := gin.New()

	// Middleware
	r.Use(gin.Recovery())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, cfg, redisClient)
	walletService := service.NewWalletService(walletRepo, transactionRepo, userRepo, redisClient, db)
	adminService := service.NewAdminService(userRepo, transactionRepo)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler(db, redisClient)
	authHandler := handler.NewAuthHandler(authService)
	walletHandler := handler.NewWalletHandler(walletService)
	adminHandler := handler.NewAdminHandler(adminService)

	// Health check endpoints
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)
	r.GET("/live", healthHandler.Live)

	// Auth routes
	auth := r.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	// Protected routes
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		// Wallet routes
		wallets := protected.Group("/wallets")
		{
			wallets.POST("", walletHandler.CreateWallet)
			wallets.GET("", walletHandler.GetUserWallets)
			wallets.GET("/:id", walletHandler.GetWallet)
			wallets.POST("/deposit", walletHandler.Deposit)
			wallets.POST("/transfer", walletHandler.Transfer)
			wallets.GET("/:id/transactions", walletHandler.GetTransactions)
		}

		// Admin routes
		admin := protected.Group("/admin")
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.GET("/transactions", adminHandler.ListTransactions)
		}
	}

	return r
}
