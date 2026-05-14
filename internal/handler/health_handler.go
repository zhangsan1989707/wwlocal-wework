package handler

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/config"
	"wwlocal-wework/pkg/response"
)

type HealthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewHealthHandler(db *gorm.DB, cfg *config.Config) *HealthHandler {
	return &HealthHandler{db: db, cfg: cfg}
}

func (h *HealthHandler) Check(c echo.Context) error {
	checks := make(map[string]string)
	healthy := true

	// DB 连通性
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		checks["db"] = "error"
		healthy = false
	} else {
		checks["db"] = "ok"
	}

	// 密钥目录可读
	if _, err := os.Stat(h.cfg.Keys.StoragePath); err != nil {
		checks["keys"] = "error: " + err.Error()
		healthy = false
	} else {
		checks["keys"] = "ok"
	}

	if !healthy {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unhealthy",
			"checks": checks,
		})
	}

	return response.Success(c, map[string]interface{}{
		"status": "ok",
		"checks": checks,
	})
}
