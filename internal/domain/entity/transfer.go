package entity

import "time"

type TransferStatus string

const (
	TransferStatusCompleted TransferStatus = "completed"
)

type Transfer struct {
	ID            int64
	FromAccountID int64
	ToAccountID   int64
	Amount        int64
	Status        TransferStatus
	CreatedAt     time.Time
}
