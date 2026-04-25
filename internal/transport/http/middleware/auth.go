package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

const bearerPrefix = "Bearer "

type currentUserIDKey struct{}

type TokenVerifier interface {
	ParseAccessToken(token string) (int64, error)
}

func AuthMiddleware(verifier TokenVerifier) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if len(header) <= len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
			response.Unauthorized(ctx, "unauthorized")
			ctx.Abort()

			return
		}

		raw := strings.TrimSpace(header[len(bearerPrefix):])
		userID, err := verifier.ParseAccessToken(raw)
		if err != nil {
			response.Unauthorized(ctx, "unauthorized")
			ctx.Abort()

			return
		}

		ctx.Set(currentUserIDKey{}, userID)
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
