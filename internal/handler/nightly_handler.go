package handler

import (
	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type NightlyHandler struct {
	nightlySvc *service.NightlyJobService
}

func NewNightlyHandler(nightlySvc *service.NightlyJobService) *NightlyHandler {
	return &NightlyHandler{nightlySvc: nightlySvc}
}

func (h *NightlyHandler) Run(c echo.Context) error {
	statDate := c.QueryParam("date")
	h.nightlySvc.RunOnce(statDate)
	return response.Success(c, map[string]interface{}{
		"message": "nightly job triggered",
		"date":    statDate,
	})
}

func (h *NightlyHandler) Status(c echo.Context) error {
	return response.Success(c, map[string]interface{}{
		"running": h.nightlySvc.IsRunning(),
	})
}
