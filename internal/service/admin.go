package service

import (
	"context"
	"time"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"github.com/SahandMohammed/wallet-service/internal/repository"
)

type AdminService interface {
	ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, error)
	ListTransactions(ctx context.Context, filters AdminTransactionFilters) ([]*domain.Transaction, error)
}

type AdminTransactionFilters struct {
	UserID    *uint
	Type      *domain.TransactionType
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

type adminService struct {
	userRepo        repository.UserRepository
	transactionRepo repository.TransactionRepository
}

func NewAdminService(
	userRepo repository.UserRepository,
	transactionRepo repository.TransactionRepository,
) AdminService {
	return &adminService{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
	}
}

func (s *adminService) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	return s.userRepo.List(ctx, limit, offset)
}

func (s *adminService) ListTransactions(ctx context.Context, filters AdminTransactionFilters) ([]*domain.Transaction, error) {
	repoFilters := repository.TransactionFilters{
		UserID:    filters.UserID,
		Type:      filters.Type,
		StartDate: filters.StartDate,
		EndDate:   filters.EndDate,
		Limit:     filters.Limit,
		Offset:    filters.Offset,
	}

	return s.transactionRepo.List(ctx, repoFilters)
}
