package dto

import (
	"time"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type CreateHoldRequest struct {
	AccountID int64 `json:"account_id"`
	Amount    int64 `json:"amount"`
}

type HoldResponse struct {
	ID        int64             `json:"id"`
	AccountID int64             `json:"account_id"`
	Amount    int64             `json:"amount"`
	Status    entity.HoldStatus `json:"status"`
	ExpiresAt time.Time         `json:"expires_at"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func NewHoldResponse(hold entity.Hold) HoldResponse {
	return HoldResponse{
		ID:        hold.ID,
		AccountID: hold.AccountID,
		Amount:    hold.Amount,
		Status:    hold.Status,
		ExpiresAt: hold.ExpiresAt,
		CreatedAt: hold.CreatedAt,
		UpdatedAt: hold.UpdatedAt,
	}
}
