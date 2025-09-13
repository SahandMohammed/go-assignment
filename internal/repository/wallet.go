package repository

import (
	"context"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"gorm.io/gorm"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *domain.Wallet) error
	GetByID(ctx context.Context, id uint) (*domain.Wallet, error)
	GetByUserID(ctx context.Context, userID uint) ([]*domain.Wallet, error)
	Update(ctx context.Context, wallet *domain.Wallet) error
	UpdateBalance(ctx context.Context, walletID uint, newBalance int64) error
}

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) Create(ctx context.Context, wallet *domain.Wallet) error {
	return r.db.WithContext(ctx).Create(wallet).Error
}

func (r *walletRepository) GetByID(ctx context.Context, id uint) (*domain.Wallet, error) {
	var wallet domain.Wallet
	err := r.db.WithContext(ctx).Preload("User").First(&wallet, id).Error
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) GetByUserID(ctx context.Context, userID uint) ([]*domain.Wallet, error) {
	var wallets []*domain.Wallet
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&wallets).Error
	return wallets, err
}

func (r *walletRepository) Update(ctx context.Context, wallet *domain.Wallet) error {
	return r.db.WithContext(ctx).Save(wallet).Error
}

func (r *walletRepository) UpdateBalance(ctx context.Context, walletID uint, newBalance int64) error {
	return r.db.WithContext(ctx).Model(&domain.Wallet{}).
		Where("id = ?", walletID).
		Update("balance", newBalance).Error
}
