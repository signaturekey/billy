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

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *userRepository {
	return &userRepository{pool: pool}
}

func (repo *userRepository) Create(ctx context.Context, user entity.User) (entity.User, error) {
	const query = `
		INSERT INTO users (
			email,
			password_hash
		)
		VALUES ($1, $2)
		RETURNING
			id,
			email,
			password_hash,
			created_at,
			updated_at
	`

	rows, err := repo.pool.Query(ctx, query, user.Email, user.PasswordHash)
	if err != nil {
		if errors.Is(mapPgError(err), ErrDuplicate) {
			return entity.User{}, domainerrors.ErrUserAlreadyExists
		}
		return entity.User{}, fmt.Errorf("insert user: %w", err)
	}
	defer rows.Close()

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.User])
	if err != nil {
		if errors.Is(mapPgError(err), ErrDuplicate) {
			return entity.User{}, domainerrors.ErrUserAlreadyExists
		}
		return entity.User{}, fmt.Errorf("collect inserted user: %w", err)
	}

	return created, nil
}

func (repo *userRepository) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	const query = `
		SELECT
			id,
			email,
			password_hash,
			created_at,
			updated_at
		FROM users
		WHERE email = $1
	`

	rows, err := repo.pool.Query(ctx, query, email)
	if err != nil {
		return entity.User{}, fmt.Errorf("query user by email: %w", err)
	}
	defer rows.Close()

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, domainerrors.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("collect user by email: %w", err)
	}

	return user, nil
}

func (repo *userRepository) GetByID(ctx context.Context, id int64) (entity.User, error) {
	const query = `
		SELECT
			id,
			email,
			password_hash,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`

	rows, err := repo.pool.Query(ctx, query, id)
	if err != nil {
		return entity.User{}, fmt.Errorf("query user by id: %w", err)
	}
	defer rows.Close()

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, domainerrors.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("collect user by id: %w", err)
	}

	return user, nil
}
