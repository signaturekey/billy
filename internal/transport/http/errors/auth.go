package errors

import (
	"errors"

	"github.com/gin-gonic/gin"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

func WriteAuthError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, domainerrors.ErrInvalidEmail):
		response.BadRequest(ctx, "invalid email")
	case errors.Is(err, domainerrors.ErrWeakPassword):
		response.BadRequest(ctx, "password too weak")
	case errors.Is(err, domainerrors.ErrUserAlreadyExists):
		response.Conflict(ctx, "user already exists")
	case errors.Is(err, domainerrors.ErrInvalidCredentials):
		response.Unauthorized(ctx, "invalid credentials")
	case errors.Is(err, domainerrors.ErrInvalidToken):
		response.Unauthorized(ctx, "invalid token")
	case errors.Is(err, domainerrors.ErrTokenExpired):
		response.Unauthorized(ctx, "token expired")
	case errors.Is(err, domainerrors.ErrUserNotFound):
		response.NotFound(ctx, "user not found")
	default:
		response.InternalError(ctx)
	}
}
