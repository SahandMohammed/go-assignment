package repository

import (
	"context"
	"time"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *domain.Transaction) error
	GetByWalletID(ctx context.Context, walletID uint, limit, offset int) ([]*domain.Transaction, error)
	GetByUserID(ctx context.Context, userID uint, limit, offset int) ([]*domain.Transaction, error)
	List(ctx context.Context, filters TransactionFilters) ([]*domain.Transaction, error)
}

type TransactionFilters struct {
	UserID    *uint
	Type      *domain.TransactionType
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, transaction *domain.Transaction) error {
	return r.db.WithContext(ctx).Create(transaction).Error
}

func (r *transactionRepository) GetByWalletID(ctx context.Context, walletID uint, limit, offset int) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	err := r.db.WithContext(ctx).
		Where("wallet_id = ?", walletID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error
	return transactions, err
}

func (r *transactionRepository) GetByUserID(ctx context.Context, userID uint, limit, offset int) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	err := r.db.WithContext(ctx).
		Joins("JOIN wallets ON transactions.wallet_id = wallets.id").
		Where("wallets.user_id = ?", userID).
		Order("transactions.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error
	return transactions, err
}

func (r *transactionRepository) List(ctx context.Context, filters TransactionFilters) ([]*domain.Transaction, error) {
	query := r.db.WithContext(ctx).
		Preload("Wallet").
		Preload("Wallet.User")

	if filters.UserID != nil {
		query = query.Joins("JOIN wallets ON transactions.wallet_id = wallets.id").
			Where("wallets.user_id = ?", *filters.UserID)
	}

	if filters.Type != nil {
		query = query.Where("type = ?", *filters.Type)
	}

	if filters.StartDate != nil {
		query = query.Where("created_at >= ?", *filters.StartDate)
	}

	if filters.EndDate != nil {
		query = query.Where("created_at <= ?", *filters.EndDate)
	}

	query = query.Order("created_at DESC")

	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}

	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	var transactions []*domain.Transaction
	err := query.Find(&transactions).Error
	return transactions, err
}
