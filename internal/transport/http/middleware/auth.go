package middleware

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type ctxKey int

const currentUserIDKey ctxKey = iota

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("X-User-ID")
		if strings.TrimSpace(header) == "" {
			response.Unauthorized(ctx, "unauthorized")
			ctx.Abort()

			return
		}

		id, err := strconv.ParseInt(header, 10, 64)
		if err != nil {
			response.Unauthorized(ctx, "unauthorized")
			ctx.Abort()

			return
		}

		ctx.Set(currentUserIDKey, id)
		ctx.Next()
	}
}

func CurrentUserID(ctx *gin.Context) (int64, bool) {
	value, exists := ctx.Get(currentUserIDKey)
	if !exists {
		return 0, false
	}

	id, ok := value.(int64)
	if !ok {
		return 0, false
	}

	return id, true
}
