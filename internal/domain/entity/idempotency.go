package entity

import (
	"encoding/json"
	"time"
)

type IdempotencyStatus string

const (
	IdempotencyStatusProcessing IdempotencyStatus = "processing"
	IdempotencyStatusCompleted  IdempotencyStatus = "completed"
)

type IdempotencyKey struct {
	Key           string
	OperationType string
	RequestHash   string
	Status        IdempotencyStatus
	ResponseCode  int
	ResponseBody  json.RawMessage
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     time.Time
}
