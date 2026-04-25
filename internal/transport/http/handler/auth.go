package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/transport/http/dto"
	transporterrors "github.com/signaturekey/billy/internal/transport/http/errors"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type AuthService interface {
	Register(ctx context.Context, email string, password string) (entity.User, entity.TokenPair, error)
	Login(ctx context.Context, email string, password string) (entity.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (entity.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthHandler struct {
	service AuthService
}

func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (handler *AuthHandler) Register(ctx *gin.Context) {
	var request dto.RegisterRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	user, pair, err := handler.service.Register(ctx.Request.Context(), request.Email, request.Password)
	if err != nil {
		transporterrors.WriteAuthError(ctx, err)
		return
	}

	response.Created(ctx, dto.NewRegisterResponse(user, pair))
}

func (handler *AuthHandler) Login(ctx *gin.Context) {
	var request dto.LoginRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	pair, err := handler.service.Login(ctx.Request.Context(), request.Email, request.Password)
	if err != nil {
		transporterrors.WriteAuthError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewAuthResponse(pair))
}

func (handler *AuthHandler) Refresh(ctx *gin.Context) {
	var request dto.RefreshRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	pair, err := handler.service.Refresh(ctx.Request.Context(), request.RefreshToken)
	if err != nil {
		transporterrors.WriteAuthError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewAuthResponse(pair))
}

func (handler *AuthHandler) Logout(ctx *gin.Context) {
	var request dto.LogoutRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	if err := handler.service.Logout(ctx.Request.Context(), request.RefreshToken); err != nil {
		transporterrors.WriteAuthError(ctx, err)
		return
	}

	response.OK(ctx, gin.H{"status": "logged out"})
}
