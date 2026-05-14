package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/pkg/response"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c echo.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
		})
	}
	return response.Success(c, map[string]string{
		"status": "ok",
	})
}
