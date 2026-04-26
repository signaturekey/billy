package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

type Params struct {
	Page   int
	Limit  int
	Offset int
}

func FromContext(ctx *gin.Context) Params {
	page := parseInt(ctx.Query("page"), DefaultPage)
	limit := parseInt(ctx.Query("limit"), DefaultLimit)

	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}

	return Params{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}
