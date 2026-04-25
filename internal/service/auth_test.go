package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/pkg/auth"
)

func newAuthTestService(t *testing.T) (*authService, *userTestRepository, *refreshTokenTestRepository, *auth.Manager) {
	t.Helper()

	users := newUserTestRepository()
	refreshTokens := newRefreshTokenTestRepository()
	manager := auth.NewManager("test-secret", time.Hour)
	hasher := auth.NewPasswordHasher()
	service := NewAuthService(accountTestTxManager{}, users, refreshTokens, manager, hasher, time.Hour)

	return service, users, refreshTokens, manager
}

func TestAuthServiceRegister(t *testing.T) {
	t.Parallel()

	service, _, _, manager := newAuthTestService(t)

	user, pair, err := service.Register(context.Background(), "User@Example.com ", "password123")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	userID, err := manager.ParseAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
}

func TestAuthServiceRegisterRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, _, err := service.Register(context.Background(), "not-an-email", "password123")
	require.ErrorIs(t, err, domainerrors.ErrInvalidEmail)

	_, _, err = service.Register(context.Background(), "user@example.com", "short")
	require.ErrorIs(t, err, domainerrors.ErrWeakPassword)
}

func TestAuthServiceRegisterRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, _, err := service.Register(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)

	_, _, err = service.Register(context.Background(), "user@example.com", "password123")
	require.ErrorIs(t, err, domainerrors.ErrUserAlreadyExists)
}

func TestAuthServiceLogin(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, _, err := service.Register(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)

	pair, err := service.Login(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestAuthServiceLoginRejectsBadCredentials(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, _, err := service.Register(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)

	_, err = service.Login(context.Background(), "user@example.com", "wrong-password")
	require.ErrorIs(t, err, domainerrors.ErrInvalidCredentials)

	_, err = service.Login(context.Background(), "missing@example.com", "password123")
	require.ErrorIs(t, err, domainerrors.ErrInvalidCredentials)
}

func TestAuthServiceRefreshRotatesToken(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, pair, err := service.Register(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)

	rotated, err := service.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.AccessToken)
	assert.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)

	_, err = service.Refresh(context.Background(), pair.RefreshToken)
	require.ErrorIs(t, err, domainerrors.ErrInvalidToken)
}

func TestAuthServiceRefreshRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	service, _, refreshTokens, manager := newAuthTestService(t)

	raw, hash, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	require.Equal(t, hash, manager.HashRefreshToken(raw))

	refreshTokens.put(entity.RefreshToken{
		ID:        99,
		UserID:    1,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(-time.Hour),
	})

	_, err = service.Refresh(context.Background(), raw)
	require.ErrorIs(t, err, domainerrors.ErrTokenExpired)
}

func TestAuthServiceLogoutRevokesToken(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newAuthTestService(t)

	_, pair, err := service.Register(context.Background(), "user@example.com", "password123")
	require.NoError(t, err)

	require.NoError(t, service.Logout(context.Background(), pair.RefreshToken))

	_, err = service.Refresh(context.Background(), pair.RefreshToken)
	require.ErrorIs(t, err, domainerrors.ErrInvalidToken)

	require.NoError(t, service.Logout(context.Background(), "unknown-token"))
}

type userTestRepository struct {
	nextID  int64
	byID    map[int64]entity.User
	byEmail map[string]entity.User
}

func newUserTestRepository() *userTestRepository {
	return &userTestRepository{
		nextID:  1,
		byID:    make(map[int64]entity.User),
		byEmail: make(map[string]entity.User),
	}
}

func (repo *userTestRepository) Create(_ context.Context, user entity.User) (entity.User, error) {
	if _, ok := repo.byEmail[user.Email]; ok {
		return entity.User{}, domainerrors.ErrUserAlreadyExists
	}

	user.ID = repo.nextID
	repo.nextID++
	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt

	repo.byID[user.ID] = user
	repo.byEmail[user.Email] = user
	return user, nil
}

func (repo *userTestRepository) GetByEmail(_ context.Context, email string) (entity.User, error) {
	user, ok := repo.byEmail[email]
	if !ok {
		return entity.User{}, domainerrors.ErrUserNotFound
	}
	return user, nil
}

func (repo *userTestRepository) GetByID(_ context.Context, id int64) (entity.User, error) {
	user, ok := repo.byID[id]
	if !ok {
		return entity.User{}, domainerrors.ErrUserNotFound
	}
	return user, nil
}

type refreshTokenTestRepository struct {
	nextID int64
	byHash map[string]entity.RefreshToken
	byID   map[int64]entity.RefreshToken
}

func newRefreshTokenTestRepository() *refreshTokenTestRepository {
	return &refreshTokenTestRepository{
		nextID: 1,
		byHash: make(map[string]entity.RefreshToken),
		byID:   make(map[int64]entity.RefreshToken),
	}
}

func (repo *refreshTokenTestRepository) put(token entity.RefreshToken) {
	repo.byHash[token.TokenHash] = token
	repo.byID[token.ID] = token
}

func (repo *refreshTokenTestRepository) Create(
	_ context.Context,
	_ pgx.Tx,
	token entity.RefreshToken,
) (entity.RefreshToken, error) {
	token.ID = repo.nextID
	repo.nextID++
	token.CreatedAt = time.Now()
	repo.put(token)
	return token, nil
}

func (repo *refreshTokenTestRepository) GetByHash(_ context.Context, hash string) (entity.RefreshToken, error) {
	token, ok := repo.byHash[hash]
	if !ok {
		return entity.RefreshToken{}, domainerrors.ErrInvalidToken
	}
	return token, nil
}

func (repo *refreshTokenTestRepository) Revoke(_ context.Context, _ pgx.Tx, id int64) error {
	token, ok := repo.byID[id]
	if !ok {
		return nil
	}

	now := time.Now()
	token.RevokedAt = &now
	repo.put(token)
	return nil
}
