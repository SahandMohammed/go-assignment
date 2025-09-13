package handler

import (
	"net/http"
	"strconv"

	"github.com/SahandMohammed/wallet-service/internal/domain"
	"github.com/SahandMohammed/wallet-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type WalletHandler struct {
	walletService service.WalletService
	validator     *validator.Validate
}

func NewWalletHandler(walletService service.WalletService) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
		validator:     validator.New(),
	}
}

type CreateWalletRequest struct {
	UserID uint `json:"user_id" validate:"required"`
}

type DepositRequest struct {
	WalletID    uint    `json:"wallet_id" validate:"required"`
	Amount      float64 `json:"amount" validate:"required,gt=0"`
	Description string  `json:"description"`
}

type TransferRequest struct {
	FromWalletID uint    `json:"from_wallet_id" validate:"required"`
	ToWalletID   uint    `json:"to_wallet_id" validate:"required"`
	Amount       float64 `json:"amount" validate:"required,gt=0"`
	Description  string  `json:"description"`
}

type WalletResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (h *WalletHandler) CreateWallet(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, WalletResponse{Error: "User not authenticated"})
		return
	}

	wallet, err := h.walletService.CreateWallet(c.Request.Context(), userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: err.Error()})
		return
	}

	response := map[string]interface{}{
		"id":         wallet.ID,
		"user_id":    wallet.UserID,
		"balance":    domain.MinorUnitsToDollars(wallet.Balance),
		"created_at": wallet.CreatedAt,
	}

	c.JSON(http.StatusCreated, WalletResponse{Data: response})
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	walletIDStr := c.Param("id")
	walletID, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Invalid wallet ID"})
		return
	}

	wallet, err := h.walletService.GetWallet(c.Request.Context(), uint(walletID))
	if err != nil {
		c.JSON(http.StatusNotFound, WalletResponse{Error: "Wallet not found"})
		return
	}

	// Check if user owns this wallet
	userID, _ := c.Get("user_id")
	if wallet.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, WalletResponse{Error: "Access denied"})
		return
	}

	response := map[string]interface{}{
		"id":         wallet.ID,
		"user_id":    wallet.UserID,
		"balance":    domain.MinorUnitsToDollars(wallet.Balance),
		"created_at": wallet.CreatedAt,
	}

	c.JSON(http.StatusOK, WalletResponse{Data: response})
}

func (h *WalletHandler) GetUserWallets(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, WalletResponse{Error: "User not authenticated"})
		return
	}

	wallets, err := h.walletService.GetUserWallets(c.Request.Context(), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, WalletResponse{Error: err.Error()})
		return
	}

	var response []map[string]interface{}
	for _, wallet := range wallets {
		response = append(response, map[string]interface{}{
			"id":         wallet.ID,
			"user_id":    wallet.UserID,
			"balance":    domain.MinorUnitsToDollars(wallet.Balance),
			"created_at": wallet.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, WalletResponse{Data: response})
}

func (h *WalletHandler) Deposit(c *gin.Context) {
	var req DepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Invalid request format"})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// Check if user owns this wallet
	userID, _ := c.Get("user_id")
	wallet, err := h.walletService.GetWallet(c.Request.Context(), req.WalletID)
	if err != nil {
		c.JSON(http.StatusNotFound, WalletResponse{Error: "Wallet not found"})
		return
	}

	if wallet.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, WalletResponse{Error: "Access denied"})
		return
	}

	transaction, err := h.walletService.Deposit(c.Request.Context(), req.WalletID, req.Amount, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: err.Error()})
		return
	}

	response := map[string]interface{}{
		"transaction_id":   transaction.ID,
		"wallet_id":        transaction.WalletID,
		"type":             transaction.Type,
		"amount":           domain.MinorUnitsToDollars(transaction.Amount),
		"balance_before":   domain.MinorUnitsToDollars(transaction.BalanceBefore),
		"balance_after":    domain.MinorUnitsToDollars(transaction.BalanceAfter),
		"transaction_uuid": transaction.TransactionUUID,
		"description":      transaction.Description,
		"created_at":       transaction.CreatedAt,
	}

	c.JSON(http.StatusOK, WalletResponse{Data: response})
}

func (h *WalletHandler) Transfer(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Invalid request format"})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// Check if user owns the source wallet
	userID, _ := c.Get("user_id")
	fromWallet, err := h.walletService.GetWallet(c.Request.Context(), req.FromWalletID)
	if err != nil {
		c.JSON(http.StatusNotFound, WalletResponse{Error: "Source wallet not found"})
		return
	}

	if fromWallet.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, WalletResponse{Error: "Access denied to source wallet"})
		return
	}

	transaction, err := h.walletService.Transfer(c.Request.Context(), req.FromWalletID, req.ToWalletID, req.Amount, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: err.Error()})
		return
	}

	response := map[string]interface{}{
		"transaction_id":   transaction.ID,
		"wallet_id":        transaction.WalletID,
		"type":             transaction.Type,
		"amount":           domain.MinorUnitsToDollars(transaction.Amount),
		"balance_before":   domain.MinorUnitsToDollars(transaction.BalanceBefore),
		"balance_after":    domain.MinorUnitsToDollars(transaction.BalanceAfter),
		"from_wallet_id":   transaction.FromWalletID,
		"to_wallet_id":     transaction.ToWalletID,
		"transaction_uuid": transaction.TransactionUUID,
		"description":      transaction.Description,
		"created_at":       transaction.CreatedAt,
	}

	c.JSON(http.StatusOK, WalletResponse{Data: response})
}

func (h *WalletHandler) GetTransactions(c *gin.Context) {
	walletIDStr := c.Param("id")
	walletID, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, WalletResponse{Error: "Invalid wallet ID"})
		return
	}

	// Check if user owns this wallet
	userID, _ := c.Get("user_id")
	wallet, err := h.walletService.GetWallet(c.Request.Context(), uint(walletID))
	if err != nil {
		c.JSON(http.StatusNotFound, WalletResponse{Error: "Wallet not found"})
		return
	}

	if wallet.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, WalletResponse{Error: "Access denied"})
		return
	}

	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	transactions, err := h.walletService.GetTransactions(c.Request.Context(), uint(walletID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, WalletResponse{Error: err.Error()})
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

		response = append(response, txData)
	}

	c.JSON(http.StatusOK, WalletResponse{Data: response})
}
