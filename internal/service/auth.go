package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

const minPasswordLength = 8

type UserRepository interface {
	Create(ctx context.Context, user entity.User) (entity.User, error)
	GetByEmail(ctx context.Context, email string) (entity.User, error)
	GetByID(ctx context.Context, id int64) (entity.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, tx pgx.Tx, token entity.RefreshToken) (entity.RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (entity.RefreshToken, error)
	Revoke(ctx context.Context, tx pgx.Tx, id int64) error
}

type TokenManager interface {
	GenerateAccessToken(userID int64) (string, time.Time, error)
	ParseAccessToken(token string) (int64, error)
	GenerateRefreshToken() (raw string, hash string, err error)
	HashRefreshToken(raw string) string
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Check(hash string, plain string) error
}

type authService struct {
	txManager     TxManager
	users         UserRepository
	refreshTokens RefreshTokenRepository
	tokens        TokenManager
	hasher        PasswordHasher
	refreshTTL    time.Duration
}

func NewAuthService(
	txManager TxManager,
	users UserRepository,
	refreshTokens RefreshTokenRepository,
	tokens TokenManager,
	hasher PasswordHasher,
	refreshTTL time.Duration,
) *authService {
	return &authService{
		txManager:     txManager,
		users:         users,
		refreshTokens: refreshTokens,
		tokens:        tokens,
		hasher:        hasher,
		refreshTTL:    refreshTTL,
	}
}

func (service *authService) Register(
	ctx context.Context,
	email string,
	password string,
) (entity.User, entity.TokenPair, error) {
	normalizedEmail := normalizeEmail(email)
	if !isValidEmail(normalizedEmail) {
		return entity.User{}, entity.TokenPair{}, domainerrors.ErrInvalidEmail
	}

	if len(password) < minPasswordLength {
		return entity.User{}, entity.TokenPair{}, domainerrors.ErrWeakPassword
	}

	hash, err := service.hasher.Hash(password)
	if err != nil {
		return entity.User{}, entity.TokenPair{}, err
	}

	user, err := service.users.Create(ctx, entity.User{
		Email:        normalizedEmail,
		PasswordHash: hash,
	})
	if err != nil {
		return entity.User{}, entity.TokenPair{}, err
	}

	pair, err := service.issuePair(ctx, user.ID)
	if err != nil {
		return entity.User{}, entity.TokenPair{}, err
	}

	return user, pair, nil
}

func (service *authService) Login(
	ctx context.Context,
	email string,
	password string,
) (entity.TokenPair, error) {
	user, err := service.users.GetByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, domainerrors.ErrUserNotFound) {
			return entity.TokenPair{}, domainerrors.ErrInvalidCredentials
		}
		return entity.TokenPair{}, err
	}

	if err := service.hasher.Check(user.PasswordHash, password); err != nil {
		return entity.TokenPair{}, domainerrors.ErrInvalidCredentials
	}

	return service.issuePair(ctx, user.ID)
}

func (service *authService) Refresh(ctx context.Context, rawRefreshToken string) (entity.TokenPair, error) {
	stored, err := service.refreshTokens.GetByHash(ctx, service.tokens.HashRefreshToken(rawRefreshToken))
	if err != nil {
		return entity.TokenPair{}, err
	}

	if stored.RevokedAt != nil {
		return entity.TokenPair{}, domainerrors.ErrInvalidToken
	}

	if time.Now().After(stored.ExpiresAt) {
		return entity.TokenPair{}, domainerrors.ErrTokenExpired
	}

	var pair entity.TokenPair
	err = service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := service.refreshTokens.Revoke(ctx, tx, stored.ID); err != nil {
			return err
		}

		var err error
		pair, err = service.issuePairInTx(ctx, tx, stored.UserID)
		return err
	})
	if err != nil {
		return entity.TokenPair{}, err
	}

	return pair, nil
}

func (service *authService) Logout(ctx context.Context, rawRefreshToken string) error {
	stored, err := service.refreshTokens.GetByHash(ctx, service.tokens.HashRefreshToken(rawRefreshToken))
	if err != nil {
		if errors.Is(err, domainerrors.ErrInvalidToken) {
			return nil
		}
		return err
	}

	return service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return service.refreshTokens.Revoke(ctx, tx, stored.ID)
	})
}

func (service *authService) issuePair(ctx context.Context, userID int64) (entity.TokenPair, error) {
	var pair entity.TokenPair
	err := service.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		pair, err = service.issuePairInTx(ctx, tx, userID)
		return err
	})
	if err != nil {
		return entity.TokenPair{}, err
	}

	return pair, nil
}

func (service *authService) issuePairInTx(ctx context.Context, tx pgx.Tx, userID int64) (entity.TokenPair, error) {
	accessToken, accessExpiresAt, err := service.tokens.GenerateAccessToken(userID)
	if err != nil {
		return entity.TokenPair{}, err
	}

	rawRefresh, refreshHash, err := service.tokens.GenerateRefreshToken()
	if err != nil {
		return entity.TokenPair{}, err
	}

	if _, err := service.refreshTokens.Create(ctx, tx, entity.RefreshToken{
		UserID:    userID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(service.refreshTTL),
	}); err != nil {
		return entity.TokenPair{}, err
	}

	return entity.TokenPair{
		AccessToken:     accessToken,
		RefreshToken:    rawRefresh,
		AccessExpiresAt: accessExpiresAt,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}

	return !strings.ContainsAny(email, " \t")
}
