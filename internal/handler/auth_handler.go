package handler

import (
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/pkg/response"
)

type AuthHandler struct {
	cfg          *config.AuthConfig
	passwordHash []byte
}

func NewAuthHandler(cfg *config.AuthConfig) *AuthHandler {
	hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	return &AuthHandler{cfg: cfg, passwordHash: hash}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if req.Username != h.cfg.Username || bcrypt.CompareHashAndPassword(h.passwordHash, []byte(req.Password)) != nil {
		return response.Error(c, 401, "invalid username or password")
	}

	c.Set("username", req.Username)
	token, err := middleware.GenerateToken(req.Username, h.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		return response.Error(c, 500, "generate token failed")
	}

	return response.Success(c, map[string]interface{}{
		"token":    token,
		"username": req.Username,
	})
}
