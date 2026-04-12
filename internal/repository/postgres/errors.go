package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNullViolation       = errors.New("null violation")
	ErrCheckViolation      = errors.New("check violation")
	ErrInvalidUUID         = errors.New("invalid uuid")
	ErrInvalidDateFormat   = errors.New("invalid date format")
	ErrDuplicate           = errors.New("duplicate")
	ErrForeignKeyViolation = errors.New("foreign key violation")
	ErrInternal            = errors.New("internal error")
)

var pgErrorMap = map[string]error{
	"23502": ErrNullViolation,
	"23514": ErrCheckViolation,
	"22P02": ErrInvalidUUID,
	"22008": ErrInvalidDateFormat,
	"23505": ErrDuplicate,
	"23503": ErrForeignKeyViolation,
}

func mapPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if mapped, ok := pgErrorMap[pgErr.Code]; ok {
			return mapped
		}
		return ErrInternal
	}
	return err
}
