package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher struct{}

func NewPasswordHasher() PasswordHasher {
	return PasswordHasher{}
}

func (PasswordHasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func (PasswordHasher) Check(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
