package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestRefreshTokenRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	users := NewUserRepository(pool)
	tokens := NewRefreshTokenRepository(pool)

	user, err := users.Create(ctx, entity.User{Email: "user@example.com", PasswordHash: "hash"})
	require.NoError(t, err)

	tx := beginIntegrationTx(t, pool)
	created, err := tokens.Create(ctx, tx, entity.RefreshToken{
		UserID:    user.ID,
		TokenHash: "hash-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	assert.NotZero(t, created.ID)
	assert.Equal(t, user.ID, created.UserID)
	assert.Nil(t, created.RevokedAt)

	found, err := tokens.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	revokeTx := beginIntegrationTx(t, pool)
	require.NoError(t, tokens.Revoke(ctx, revokeTx, created.ID))
	require.NoError(t, revokeTx.Commit(ctx))

	revoked, err := tokens.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)

	_, err = tokens.GetByHash(ctx, "missing")
	require.ErrorIs(t, err, domainerrors.ErrInvalidToken)
}
