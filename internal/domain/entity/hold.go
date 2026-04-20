package entity

import "time"

type HoldStatus string

const (
	HoldStatusPending   HoldStatus = "pending"
	HoldStatusConfirmed HoldStatus = "confirmed"
	HoldStatusCancelled HoldStatus = "cancelled"
	HoldStatusExpired   HoldStatus = "expired"
)

type Hold struct {
	ID        int64
	AccountID int64
	Amount    int64
	Status    HoldStatus
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
