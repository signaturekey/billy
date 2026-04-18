package dto

import (
	"time"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type CreateTransferRequest struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type TransferResponse struct {
	ID            int64                 `json:"id"`
	FromAccountID int64                 `json:"from_account_id"`
	ToAccountID   int64                 `json:"to_account_id"`
	Amount        int64                 `json:"amount"`
	Status        entity.TransferStatus `json:"status"`
	CreatedAt     time.Time             `json:"created_at"`
}

func NewTransferResponse(transfer entity.Transfer) TransferResponse {
	return TransferResponse{
		ID:            transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		Status:        transfer.Status,
		CreatedAt:     transfer.CreatedAt,
	}
}
