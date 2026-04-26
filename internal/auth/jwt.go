package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type Manager struct {
	secret         []byte
	accessTokenTTL time.Duration
}

func NewManager(secret string, accessTokenTTL time.Duration) *Manager {
	return &Manager{
		secret:         []byte(secret),
		accessTokenTTL: accessTokenTTL,
	}
}

func (m *Manager) GenerateAccessToken(userID int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTokenTTL)

	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

func (m *Manager) ParseAccessToken(raw string) (int64, error) {
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, domainerrors.ErrTokenExpired
		}
		return 0, domainerrors.ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, domainerrors.ErrInvalidToken
	}

	return userID, nil
}

func (m *Manager) GenerateRefreshToken() (string, string, error) {
	return GenerateRefreshToken()
}

func (m *Manager) HashRefreshToken(raw string) string {
	return HashRefreshToken(raw)
}
