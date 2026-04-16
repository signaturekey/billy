package errors

import (
	"errors"

	"github.com/gin-gonic/gin"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

func WriteAccountError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, domainerrors.ErrInvalidCurrency):
		response.BadRequest(ctx, "invalid currency")
	case errors.Is(err, domainerrors.ErrInvalidAmount):
		response.BadRequest(ctx, "invalid amount")
	case errors.Is(err, domainerrors.ErrAccountAlreadyExists):
		response.Conflict(ctx, "account already exists")
	case errors.Is(err, domainerrors.ErrAccountNotFound):
		response.NotFound(ctx, "account not found")
	case errors.Is(err, domainerrors.ErrForbidden):
		response.Forbidden(ctx, "forbidden")
	case errors.Is(err, domainerrors.ErrInsufficientFunds):
		response.UnprocessableEntity(ctx, "insufficient funds")
	default:
		response.InternalError(ctx)
	}
}
