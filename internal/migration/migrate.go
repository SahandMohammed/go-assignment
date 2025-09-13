package migration

import (
	"github.com/SahandMohammed/wallet-service/internal/domain"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.User{},
		&domain.Wallet{},
		&domain.Transaction{},
	)
}
