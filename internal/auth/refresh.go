package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const refreshTokenBytes = 32

func GenerateRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
