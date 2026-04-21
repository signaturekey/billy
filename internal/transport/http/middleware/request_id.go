package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx.Set(requestIDHeader, requestID)
		ctx.Header(requestIDHeader, requestID)
		requestCtx := context.WithValue(ctx.Request.Context(), requestIDContextKey{}, requestID)
		ctx.Request = ctx.Request.WithContext(requestCtx)

		ctx.Next()
	}
}

func RequestID(ctx *gin.Context) string {
	if value, ok := ctx.Get(requestIDHeader); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}

	if requestID, ok := ctx.Request.Context().Value(requestIDContextKey{}).(string); ok {
		return requestID
	}

	return ""
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}

	return fmt.Sprintf("%d", time.Now().UnixNano())
}
