package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"github.com/SahandMohammed/wallet-service/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletService interface {
	CreateWallet(ctx context.Context, userID uint) (*domain.Wallet, error)
	GetWallet(ctx context.Context, walletID uint) (*domain.Wallet, error)
	GetUserWallets(ctx context.Context, userID uint) ([]*domain.Wallet, error)
	Deposit(ctx context.Context, walletID uint, amount float64, description string) (*domain.Transaction, error)
	Transfer(ctx context.Context, fromWalletID, toWalletID uint, amount float64, description string) (*domain.Transaction, error)
	GetTransactions(ctx context.Context, walletID uint, limit, offset int) ([]*domain.Transaction, error)
}

type walletService struct {
	walletRepo      repository.WalletRepository
	transactionRepo repository.TransactionRepository
	userRepo        repository.UserRepository
	redisClient     *redis.Client
	db              *gorm.DB
}

func NewWalletService(
	walletRepo repository.WalletRepository,
	transactionRepo repository.TransactionRepository,
	userRepo repository.UserRepository,
	redisClient *redis.Client,
	db *gorm.DB,
) WalletService {
	return &walletService{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		redisClient:     redisClient,
		db:              db,
	}
}

func (s *walletService) CreateWallet(ctx context.Context, userID uint) (*domain.Wallet, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	wallet := &domain.Wallet{
		UserID:  userID,
		Balance: 0,
	}

	if err := s.walletRepo.Create(ctx, wallet); err != nil {
		return nil, err
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"wallet_id": wallet.ID,
		"action":    "wallet_created",
	}).Info("Wallet created successfully")

	return wallet, nil
}

func (s *walletService) GetWallet(ctx context.Context, walletID uint) (*domain.Wallet, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("wallet:%d", walletID)
	cachedWallet, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var wallet domain.Wallet
		if json.Unmarshal([]byte(cachedWallet), &wallet) == nil {
			return &wallet, nil
		}
	}

	// Get from database
	wallet, err := s.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		return nil, err
	}

	// Cache the wallet
	walletJSON, _ := json.Marshal(wallet)
	s.redisClient.Set(ctx, cacheKey, walletJSON, 5*time.Minute)

	return wallet, nil
}

func (s *walletService) GetUserWallets(ctx context.Context, userID uint) ([]*domain.Wallet, error) {
	return s.walletRepo.GetByUserID(ctx, userID)
}

func (s *walletService) Deposit(ctx context.Context, walletID uint, amount float64, description string) (*domain.Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	amountInMinorUnits := domain.DollarsToMinorUnits(amount)

	var transaction *domain.Transaction
	var userID uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get wallet with row lock
		var wallet domain.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&wallet, walletID).Error; err != nil {
			return err
		}

		userID = wallet.UserID

		// Calculate new balance
		oldBalance := wallet.Balance
		newBalance := oldBalance + amountInMinorUnits

		// Update wallet balance
		if err := tx.Model(&wallet).Update("balance", newBalance).Error; err != nil {
			return err
		}

		// Create transaction record
		transaction = &domain.Transaction{
			WalletID:        walletID,
			Type:            domain.TransactionTypeDeposit,
			Amount:          amountInMinorUnits,
			BalanceBefore:   oldBalance,
			BalanceAfter:    newBalance,
			TransactionUUID: uuid.New().String(),
			Description:     description,
		}

		return tx.Create(transaction).Error
	})

	if err != nil {
		return nil, err
	}

	// Invalidate caches
	s.invalidateWalletCache(ctx, walletID)
	s.invalidateTransactionCache(ctx, walletID)

	logrus.WithFields(logrus.Fields{
		"user_id":          userID,
		"wallet_id":        walletID,
		"amount":           amount,
		"transaction_uuid": transaction.TransactionUUID,
		"description":      description,
		"action":           "deposit",
		"transaction_type": "financial",
	}).Info("Financial transaction completed")

	return transaction, nil
}

func (s *walletService) Transfer(ctx context.Context, fromWalletID, toWalletID uint, amount float64, description string) (*domain.Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	if fromWalletID == toWalletID {
		return nil, errors.New("cannot transfer to the same wallet")
	}

	amountInMinorUnits := domain.DollarsToMinorUnits(amount)

	var fromTransaction *domain.Transaction
	var fromUserID, toUserID uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get both wallets with row locks
		var fromWallet, toWallet domain.Wallet

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&fromWallet, fromWalletID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("source wallet not found")
			}
			return err
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&toWallet, toWalletID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("destination wallet not found")
			}
			return err
		}

		fromUserID = fromWallet.UserID
		toUserID = toWallet.UserID

		// Check sufficient balance
		if fromWallet.Balance < amountInMinorUnits {
			return errors.New("insufficient balance")
		}

		// Calculate new balances
		fromOldBalance := fromWallet.Balance
		fromNewBalance := fromOldBalance - amountInMinorUnits
		toOldBalance := toWallet.Balance
		toNewBalance := toOldBalance + amountInMinorUnits

		// Update wallet balances
		if err := tx.Model(&fromWallet).Update("balance", fromNewBalance).Error; err != nil {
			return err
		}
		if err := tx.Model(&toWallet).Update("balance", toNewBalance).Error; err != nil {
			return err
		}

		// Create transaction records for both wallets with unique UUIDs
		fromTransaction = &domain.Transaction{
			WalletID:        fromWalletID,
			Type:            domain.TransactionTypeTransfer,
			Amount:          -amountInMinorUnits, // Negative for outgoing transfer
			BalanceBefore:   fromOldBalance,
			BalanceAfter:    fromNewBalance,
			FromWalletID:    &fromWalletID,
			ToWalletID:      &toWalletID,
			TransactionUUID: uuid.New().String(), // Unique UUID for this transaction
			Description:     description,
		}

		toTransaction := &domain.Transaction{
			WalletID:        toWalletID,
			Type:            domain.TransactionTypeTransfer,
			Amount:          amountInMinorUnits, // Positive for incoming transfer
			BalanceBefore:   toOldBalance,
			BalanceAfter:    toNewBalance,
			FromWalletID:    &fromWalletID,
			ToWalletID:      &toWalletID,
			TransactionUUID: uuid.New().String(), // Unique UUID for this transaction
			Description:     description,
		}

		if err := tx.Create(fromTransaction).Error; err != nil {
			return err
		}
		return tx.Create(toTransaction).Error
	})

	if err != nil {
		return nil, err
	}

	// Invalidate wallet caches
	s.invalidateWalletCache(ctx, fromWalletID)
	s.invalidateWalletCache(ctx, toWalletID)
	s.invalidateTransactionCache(ctx, fromWalletID)
	s.invalidateTransactionCache(ctx, toWalletID)

	logrus.WithFields(logrus.Fields{
		"from_user_id":     fromUserID,
		"to_user_id":       toUserID,
		"from_wallet_id":   fromWalletID,
		"to_wallet_id":     toWalletID,
		"amount":           amount,
		"transaction_uuid": fromTransaction.TransactionUUID,
		"description":      description,
		"action":           "transfer",
		"transaction_type": "financial",
	}).Info("Financial transaction completed")

	return fromTransaction, nil
}

func (s *walletService) GetTransactions(ctx context.Context, walletID uint, limit, offset int) ([]*domain.Transaction, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("wallet:%d:transactions:%d:%d", walletID, limit, offset)
	if cached, err := s.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var transactions []*domain.Transaction
		if json.Unmarshal([]byte(cached), &transactions) == nil {
			return transactions, nil
		}
	}

	// Get from database
	transactions, err := s.transactionRepo.GetByWalletID(ctx, walletID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Cache the transactions with 2 minute TTL
	if transactionsJSON, err := json.Marshal(transactions); err == nil {
		s.redisClient.Set(ctx, cacheKey, transactionsJSON, 2*time.Minute)
	}

	return transactions, nil
}

func (s *walletService) invalidateWalletCache(ctx context.Context, walletID uint) {
	cacheKey := fmt.Sprintf("wallet:%d", walletID)
	s.redisClient.Del(ctx, cacheKey)
}

func (s *walletService) invalidateUserCache(ctx context.Context, userID uint) {
	cacheKey := fmt.Sprintf("user:%d", userID)
	s.redisClient.Del(ctx, cacheKey)
}

func (s *walletService) invalidateTransactionCache(ctx context.Context, walletID uint) {
	// Find and delete all transaction cache keys for this wallet
	pattern := fmt.Sprintf("wallet:%d:transactions:*", walletID)
	iter := s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		s.redisClient.Del(ctx, iter.Val())
	}
	if err := iter.Err(); err != nil {
		// Log error but don't fail the operation
		logrus.WithError(err).Warn("Failed to invalidate transaction cache")
	}
}
