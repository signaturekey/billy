package dto

import "github.com/signaturekey/billy/internal/domain/entity"

type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

type AccountResponse struct {
	ID             int64                `json:"id"`
	UserID         int64                `json:"user_id"`
	Currency       string               `json:"currency"`
	Balance        int64                `json:"balance"`
	ReservedAmount int64                `json:"reserved_amount"`
	Status         entity.AccountStatus `json:"status"`
	CreatedAt      string               `json:"created_at"`
	UpdatedAt      string               `json:"updated_at"`
}

type BalanceResponse struct {
	AccountID       int64  `json:"account_id"`
	Balance         int64  `json:"balance"`
	ReservedAmount  int64  `json:"reserved_amount"`
	AvailableAmount int64  `json:"available_amount"`
	Currency        string `json:"currency"`
}

func NewAccountResponse(account entity.Account) AccountResponse {
	return AccountResponse{
		ID:             account.ID,
		UserID:         account.UserID,
		Currency:       account.Currency,
		Balance:        account.Balance,
		ReservedAmount: account.ReservedAmount,
		Status:         account.Status,
		CreatedAt:      account.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      account.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func NewBalanceResponse(balance entity.AccountBalance) BalanceResponse {
	return BalanceResponse{
		AccountID:       balance.AccountID,
		Balance:         balance.Balance,
		ReservedAmount:  balance.ReservedAmount,
		AvailableAmount: balance.AvailableAmount,
		Currency:        balance.Currency,
	}
}
