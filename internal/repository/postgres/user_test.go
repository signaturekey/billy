package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestUserRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	users := NewUserRepository(pool)

	created, err := users.Create(ctx, entity.User{
		Email:        "user@example.com",
		PasswordHash: "hash",
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "user@example.com", created.Email)
	assert.Equal(t, "hash", created.PasswordHash)
	assert.NotZero(t, created.CreatedAt)

	byEmail, err := users.GetByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byEmail.ID)

	byID, err := users.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Email, byID.Email)

	_, err = users.Create(ctx, entity.User{Email: "user@example.com", PasswordHash: "hash2"})
	require.ErrorIs(t, err, domainerrors.ErrUserAlreadyExists)

	_, err = users.GetByEmail(ctx, "missing@example.com")
	require.ErrorIs(t, err, domainerrors.ErrUserNotFound)

	_, err = users.GetByID(ctx, created.ID+1000)
	require.ErrorIs(t, err, domainerrors.ErrUserNotFound)
}
