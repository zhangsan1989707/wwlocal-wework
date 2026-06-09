package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

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
		return response.Error(c, 500, "查询失败")
	}
	return response.Success(c, result)
}

func (h *DashboardHandler) GetInactiveUsers(c echo.Context) error {
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, err := parseOptionalNonNegativeIntQuery(c, "dept_id")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	minInactiveDays, err := parseOptionalNonNegativeIntQuery(c, "min_inactive_days")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}

	result, err := h.dashboardSvc.GetInactiveUsers(rangeParam, deptID, minInactiveDays, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "查询失败")
	}
	return response.Success(c, result)
}

func (h *DashboardHandler) ExportInactiveUsers(c echo.Context) error {
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, err := parseOptionalNonNegativeIntQuery(c, "dept_id")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	minInactiveDays, err := parseOptionalNonNegativeIntQuery(c, "min_inactive_days")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}

	rows, err := h.dashboardSvc.ExportInactiveUsersCSV(rangeParam, deptID, minInactiveDays)
	if err != nil {
		return response.Error(c, 500, "导出失败")
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="inactive_users.csv"`)
	c.Response().WriteHeader(http.StatusOK)

	// UTF-8 BOM for Excel compatibility
	c.Response().Write([]byte("\xEF\xBB\xBF"))

	w := csv.NewWriter(c.Response())
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}

func (h *DashboardHandler) GetTrend(c echo.Context) error {
	granularity := c.QueryParam("granularity")
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, err := parseOptionalNonNegativeIntQuery(c, "dept_id")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	featureIDs := c.QueryParam("feature_ids")

	result, err := h.dashboardSvc.GetTrend(granularity, rangeParam, deptID, featureIDs)
	if err != nil {
		return response.Error(c, 500, "查询趋势数据失败")
	}
	return response.Success(c, result)
}

func (h *DashboardHandler) GetTrendByDept(c echo.Context) error {
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	featureID, err := parseOptionalNonNegativeIntQuery(c, "feature_id")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}

	result, err := h.dashboardSvc.GetTrendByDept(rangeParam, featureID)
	if err != nil {
		return response.Error(c, 500, "查询部门趋势失败")
	}
	return response.Success(c, result)
}

func (h *DashboardHandler) ExportTrend(c echo.Context) error {
	granularity := c.QueryParam("granularity")
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, err := parseOptionalNonNegativeIntQuery(c, "dept_id")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	featureIDs := c.QueryParam("feature_ids")

	rows, err := h.dashboardSvc.ExportTrendCSV(granularity, rangeParam, deptID, featureIDs)
	if err != nil {
		return response.Error(c, 500, "导出失败")
	}

	filename := "trend_" + time.Now().Format("20060102_150405") + ".csv"
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	c.Response().WriteHeader(http.StatusOK)
	c.Response().Write([]byte("\xEF\xBB\xBF"))

	w := csv.NewWriter(c.Response())
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
