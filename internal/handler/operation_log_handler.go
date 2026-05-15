package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type OperationLogHandler struct {
	svc *service.OperationLogService
}

func NewOperationLogHandler(svc *service.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

func (h *OperationLogHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	action := c.QueryParam("action")
	statusCode, _ := strconv.Atoi(c.QueryParam("status_code"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	logs, total, err := h.svc.List(page, pageSize, action, statusCode)
	if err != nil {
		return response.Error(c, 500, "query operation logs failed")
	}
	return response.Success(c, map[string]interface{}{
		"data":       logs,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

func (h *OperationLogHandler) GetActions(c echo.Context) error {
	actions, err := h.svc.GetDistinctActions()
	if err != nil {
		return response.Error(c, 500, "query actions failed")
	}
	return response.Success(c, actions)
}
