package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;not null;size:50"`
	Password  string         `json:"-" gorm:"not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Wallets []Wallet `json:"wallets,omitempty" gorm:"foreignKey:UserID"`
}

type Wallet struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    uint           `json:"user_id" gorm:"not null;index"`
	Balance   int64          `json:"balance" gorm:"not null;default:0"` // Store in minor units (cents)
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	User         User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Transactions []Transaction `json:"transactions,omitempty" gorm:"foreignKey:WalletID"`
}

type TransactionType string

const (
	TransactionTypeDeposit  TransactionType = "deposit"
	TransactionTypeTransfer TransactionType = "transfer"
	TransactionTypeWithdraw TransactionType = "withdraw"
)

type Transaction struct {
	ID              uint            `json:"id" gorm:"primaryKey"`
	WalletID        uint            `json:"wallet_id" gorm:"not null;index"`
	Type            TransactionType `json:"type" gorm:"not null;size:20"`
	Amount          int64           `json:"amount" gorm:"not null"` // Store in minor units (cents)
	BalanceBefore   int64           `json:"balance_before" gorm:"not null"`
	BalanceAfter    int64           `json:"balance_after" gorm:"not null"`
	FromWalletID    *uint           `json:"from_wallet_id,omitempty" gorm:"index"` // For transfers
	ToWalletID      *uint           `json:"to_wallet_id,omitempty" gorm:"index"`   // For transfers
	TransactionUUID string          `json:"transaction_uuid" gorm:"uniqueIndex;not null;size:36"`
	Description     string          `json:"description" gorm:"size:255"`
	CreatedAt       time.Time       `json:"created_at"`

	Wallet     Wallet  `json:"wallet,omitempty" gorm:"foreignKey:WalletID"`
	FromWallet *Wallet `json:"from_wallet,omitempty" gorm:"foreignKey:FromWalletID"`
	ToWallet   *Wallet `json:"to_wallet,omitempty" gorm:"foreignKey:ToWalletID"`
}

// Helper methods to convert between cents and dollars
func (w *Wallet) GetBalanceInDollars() float64 {
	return float64(w.Balance) / 100.0
}

func (w *Wallet) SetBalanceFromDollars(dollars float64) {
	w.Balance = int64(dollars * 100)
}

func (t *Transaction) GetAmountInDollars() float64 {
	return float64(t.Amount) / 100.0
}

func (t *Transaction) SetAmountFromDollars(dollars float64) {
	t.Amount = int64(dollars * 100)
}

func DollarsToMinorUnits(dollars float64) int64 {
	return int64(dollars * 100)
}

func MinorUnitsToDollars(minorUnits int64) float64 {
	return float64(minorUnits) / 100.0
}
