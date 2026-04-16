package entity

import "time"

type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "active"
	AccountStatusBlocked AccountStatus = "blocked"
)

type Account struct {
	ID             int64
	UserID         int64
	Currency       string
	Balance        int64
	ReservedAmount int64
	Status         AccountStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AccountBalance struct {
	AccountID       int64
	Balance         int64
	ReservedAmount  int64
	AvailableAmount int64
	Currency        string
}

type LedgerEntryType string

const (
	LedgerEntryTypeTopup      LedgerEntryType = "topup"
	LedgerEntryTypeWithdrawal LedgerEntryType = "withdrawal"
)

type LedgerEntry struct {
	ID            int64
	AccountID     int64
	Type          LedgerEntryType
	Amount        int64
	Currency      string
	BalanceBefore int64
	BalanceAfter  int64
	CreatedAt     time.Time
}
