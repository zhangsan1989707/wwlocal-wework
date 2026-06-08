package handler

import (
	"strings"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type BehaviorQueryHandler struct {
	behaviorSvc *service.BehaviorQueryService
}

func NewBehaviorQueryHandler(behaviorSvc *service.BehaviorQueryService) *BehaviorQueryHandler {
	return &BehaviorQueryHandler{behaviorSvc: behaviorSvc}
}

func (h *BehaviorQueryHandler) Query(c echo.Context) error {
	var req model.BehaviorQueryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	result, err := h.behaviorSvc.Query(&req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "range") || strings.Contains(msg, "time") {
			return response.Error(c, 400, msg)
		}
		return response.Error(c, 500, "行为查询失败")
	}

	return response.Success(c, result)
}
