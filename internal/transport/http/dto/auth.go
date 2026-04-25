package dto

import (
	"time"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RegisterResponse struct {
	User   UserResponse `json:"user"`
	Tokens AuthResponse `json:"tokens"`
}

func NewUserResponse(user entity.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

func NewAuthResponse(pair entity.TokenPair) AuthResponse {
	return AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.AccessExpiresAt,
	}
}

func NewRegisterResponse(user entity.User, pair entity.TokenPair) RegisterResponse {
	return RegisterResponse{
		User:   NewUserResponse(user),
		Tokens: NewAuthResponse(pair),
	}
}
