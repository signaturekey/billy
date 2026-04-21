package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type currentUserIDKey struct{}

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("X-User-ID")
		if header == "" {
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

		ctx.Set(currentUserIDKey{}, id)
		ctx.Next()
	}
}

func CurrentUserID(ctx *gin.Context) int64 {
	return ctx.MustGet(currentUserIDKey{}).(int64)
}

func LookupCurrentUserID(ctx *gin.Context) (int64, bool) {
	value, ok := ctx.Get(currentUserIDKey{})
	if !ok {
		return 0, false
	}

	id, ok := value.(int64)
	return id, ok
}
