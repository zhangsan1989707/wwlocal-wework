package handler

import (
	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type OperationLogHandler struct {
	svc operationLogLister
}

type operationLogLister interface {
	List(page, pageSize int, action string, statusCode int) ([]model.OperationLog, int64, error)
	GetDistinctActions() ([]string, error)
}

func NewOperationLogHandler(svc *service.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

func (h *OperationLogHandler) List(c echo.Context) error {
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	action := c.QueryParam("action")
	statusCode, err := parseOptionalIntQuery(c, "status_code")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	logs, total, err := h.svc.List(page, pageSize, action, statusCode)
	if err != nil {
		return response.Error(c, 500, "query operation logs failed")
	}
	return response.Success(c, map[string]interface{}{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *OperationLogHandler) GetActions(c echo.Context) error {
	actions, err := h.svc.GetDistinctActions()
	if err != nil {
		return response.Error(c, 500, "query actions failed")
	}
	return response.Success(c, actions)
}
