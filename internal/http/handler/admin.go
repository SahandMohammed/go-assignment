package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"github.com/SahandMohammed/wallet-service/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	adminService service.AdminService
}

func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
	}
}

type AdminResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	users, err := h.adminService.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AdminResponse{Error: err.Error()})
		return
	}

	var response []map[string]interface{}
	for _, user := range users {
		userData := map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"created_at": user.CreatedAt,
		}

		// Include wallets if loaded
		if len(user.Wallets) > 0 {
			var wallets []map[string]interface{}
			for _, wallet := range user.Wallets {
				wallets = append(wallets, map[string]interface{}{
					"id":         wallet.ID,
					"balance":    domain.MinorUnitsToDollars(wallet.Balance),
					"created_at": wallet.CreatedAt,
				})
			}
			userData["wallets"] = wallets
		}

		response = append(response, userData)
	}

	c.JSON(http.StatusOK, AdminResponse{Data: response})
}

func (h *AdminHandler) ListTransactions(c *gin.Context) {
	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// Parse filters
	filters := service.AdminTransactionFilters{
		Limit:  limit,
		Offset: offset,
	}

	// User ID filter
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uid := uint(userID)
			filters.UserID = &uid
		}
	}

	// Transaction type filter
	if typeStr := c.Query("type"); typeStr != "" {
		txType := domain.TransactionType(typeStr)
		if txType == domain.TransactionTypeDeposit ||
			txType == domain.TransactionTypeTransfer ||
			txType == domain.TransactionTypeWithdraw {
			filters.Type = &txType
		}
	}

	// Date range filters
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filters.StartDate = &startDate
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.Parse("2006-01-02", endDateStr); err == nil {
			// Set to end of day
			endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.EndDate = &endDate
		}
	}

	transactions, err := h.adminService.ListTransactions(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AdminResponse{Error: err.Error()})
		return
	}

	var response []map[string]interface{}
	for _, tx := range transactions {
		txData := map[string]interface{}{
			"transaction_id":   tx.ID,
			"wallet_id":        tx.WalletID,
			"type":             tx.Type,
			"amount":           domain.MinorUnitsToDollars(tx.Amount),
			"balance_before":   domain.MinorUnitsToDollars(tx.BalanceBefore),
			"balance_after":    domain.MinorUnitsToDollars(tx.BalanceAfter),
			"transaction_uuid": tx.TransactionUUID,
			"description":      tx.Description,
			"created_at":       tx.CreatedAt,
		}

		if tx.FromWalletID != nil {
			txData["from_wallet_id"] = *tx.FromWalletID
		}
		if tx.ToWalletID != nil {
			txData["to_wallet_id"] = *tx.ToWalletID
		}

		// Include wallet and user information if loaded
		if tx.Wallet.ID != 0 {
			txData["wallet"] = map[string]interface{}{
				"id":      tx.Wallet.ID,
				"user_id": tx.Wallet.UserID,
			}

			if tx.Wallet.User.ID != 0 {
				txData["user"] = map[string]interface{}{
					"id":       tx.Wallet.User.ID,
					"username": tx.Wallet.User.Username,
				}
			}
		}

		response = append(response, txData)
	}

	c.JSON(http.StatusOK, AdminResponse{Data: response})
}
