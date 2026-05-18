package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type DashboardHandler struct {
	dashboardSvc *service.DashboardService
}

func NewDashboardHandler(dashboardSvc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardSvc: dashboardSvc}
}

func (h *DashboardHandler) GetOverview(c echo.Context) error {
	result, err := h.dashboardSvc.GetOverview()
	if err != nil {
		return response.Error(c, 500, "查询失败: "+err.Error())
	}
	return response.Success(c, result)
}

func (h *DashboardHandler) GetInactiveUsers(c echo.Context) error {
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, _ := strconv.Atoi(c.QueryParam("dept_id"))
	minInactiveDays, _ := strconv.Atoi(c.QueryParam("min_inactive_days"))

	result, err := h.dashboardSvc.GetInactiveUsers(rangeParam, deptID, minInactiveDays)
	if err != nil {
		return response.Error(c, 500, "查询失败: "+err.Error())
	}
	return response.Success(c, result)
}
