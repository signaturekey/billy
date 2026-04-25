package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type refreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *refreshTokenRepository {
	return &refreshTokenRepository{pool: pool}
}

func (repo *refreshTokenRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	token entity.RefreshToken,
) (entity.RefreshToken, error) {
	const query = `
		INSERT INTO refresh_tokens (
			user_id,
			token_hash,
			expires_at
		)
		VALUES ($1, $2, $3)
		RETURNING
			id,
			user_id,
			token_hash,
			expires_at,
			revoked_at,
			created_at
	`

	rows, err := tx.Query(ctx, query, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return entity.RefreshToken{}, fmt.Errorf("insert refresh token: %w", err)
	}
	defer rows.Close()

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.RefreshToken])
	if err != nil {
		return entity.RefreshToken{}, fmt.Errorf("collect inserted refresh token: %w", err)
	}

	return created, nil
}

func (repo *refreshTokenRepository) GetByHash(ctx context.Context, hash string) (entity.RefreshToken, error) {
	const query = `
		SELECT
			id,
			user_id,
			token_hash,
			expires_at,
			revoked_at,
			created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	rows, err := repo.pool.Query(ctx, query, hash)
	if err != nil {
		return entity.RefreshToken{}, fmt.Errorf("query refresh token by hash: %w", err)
	}
	defer rows.Close()

	token, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.RefreshToken])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.RefreshToken{}, domainerrors.ErrInvalidToken
		}
		return entity.RefreshToken{}, fmt.Errorf("collect refresh token by hash: %w", err)
	}

	return token, nil
}

func (repo *refreshTokenRepository) Revoke(ctx context.Context, tx pgx.Tx, id int64) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE id = $1
			AND revoked_at IS NULL
	`

	if _, err := tx.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}
