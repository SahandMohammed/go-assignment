package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/SahandMohammed/wallet-service/internal/config"
	"github.com/SahandMohammed/wallet-service/internal/domain"
	"github.com/SahandMohammed/wallet-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService interface {
	Register(ctx context.Context, username, password string) (*domain.User, error)
	Login(ctx context.Context, username, password string) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type authService struct {
	userRepo    repository.UserRepository
	config      *config.Config
	redisClient *redis.Client
}

func NewAuthService(userRepo repository.UserRepository, config *config.Config, redisClient *redis.Client) AuthService {
	return &authService{
		userRepo:    userRepo,
		config:      config,
		redisClient: redisClient,
	}
}

func (s *authService) Register(ctx context.Context, username, password string) (*domain.User, error) {
	// Validate username length
	if len(username) < 3 || len(username) > 50 {
		return nil, errors.New("username must be between 3 and 50 characters")
	}

	// Validate username contains only alphabetic characters
	if !regexp.MustCompile(`^[A-Za-z]+$`).MatchString(username) {
		return nil, errors.New("username must contain only alphabetic characters")
	}

	// Validate password length
	if len(password) < 8 || len(password) > 15 {
		return nil, errors.New("password must be between 8 and 15 characters")
	}

	// Check if username already exists
	_, err := s.userRepo.GetByUsername(ctx, username)
	if err == nil {
		return nil, errors.New("username already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user
	user := &domain.User{
		Username: username,
		Password: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Cache the user for future access
	s.cacheUser(ctx, user)

	return user, nil
}

func (s *authService) Login(ctx context.Context, username, password string) (string, error) {
	// Get user by username from database
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("invalid credentials")
		}
		return "", err
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	// Cache the user for future access
	s.cacheUser(ctx, user)

	// Generate JWT token
	return s.generateToken(user)
}

func (s *authService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.AppJWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// generateToken creates a JWT token for the given user
func (s *authService) generateToken(user *domain.User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.AppJWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// cacheUser stores user data in Redis with appropriate TTL
func (s *authService) cacheUser(ctx context.Context, user *domain.User) {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return // Silently fail, caching is not critical
	}

	// Cache by username (for login)
	usernameKey := fmt.Sprintf("user:username:%s", user.Username)
	s.redisClient.Set(ctx, usernameKey, userJSON, 10*time.Minute)

	// Cache by ID (for other operations)
	idKey := fmt.Sprintf("user:id:%d", user.ID)
	s.redisClient.Set(ctx, idKey, userJSON, 10*time.Minute)
}
