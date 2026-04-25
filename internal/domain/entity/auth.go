package entity

import "time"

type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type TokenPair struct {
	AccessToken     string
	RefreshToken    string
	AccessExpiresAt time.Time
}
