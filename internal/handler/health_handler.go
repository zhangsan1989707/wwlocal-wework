package handler

import (
	"github.com/labstack/echo/v4"
	"wwlocal-wework/pkg/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(c echo.Context) error {
	return response.Success(c, map[string]string{
		"status": "ok",
	})
}