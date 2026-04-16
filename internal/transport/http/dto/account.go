package dto

import (
	"time"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

type TopUpRequest struct {
	Amount int64 `json:"amount"`
}

type WithdrawalRequest struct {
	Amount int64 `json:"amount"`
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

type LedgerEntryResponse struct {
	ID            int64                  `json:"id"`
	AccountID     int64                  `json:"account_id"`
	Type          entity.LedgerEntryType `json:"type"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	BalanceBefore int64                  `json:"balance_before"`
	BalanceAfter  int64                  `json:"balance_after"`
	CreatedAt     time.Time              `json:"created_at"`
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

func NewLedgerEntryResponse(entry entity.LedgerEntry) LedgerEntryResponse {
	return LedgerEntryResponse{
		ID:            entry.ID,
		AccountID:     entry.AccountID,
		Type:          entry.Type,
		Amount:        entry.Amount,
		Currency:      entry.Currency,
		BalanceBefore: entry.BalanceBefore,
		BalanceAfter:  entry.BalanceAfter,
		CreatedAt:     entry.CreatedAt,
	}
}
