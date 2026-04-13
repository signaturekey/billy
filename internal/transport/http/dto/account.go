package dto

import "github.com/signaturekey/billy/internal/domain/entity"

type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

type AccountResponse struct {
	ID             int64                `json:"id"`
	Currency       string               `json:"currency"`
	Balance        int64                `json:"balance"`
	ReservedAmount int64                `json:"reserved_amount"`
	Status         entity.AccountStatus `json:"status"`
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
		Currency:       account.Currency,
		Balance:        account.Balance,
		ReservedAmount: account.ReservedAmount,
		Status:         account.Status,
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
