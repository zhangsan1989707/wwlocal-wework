package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type DashboardV2Handler struct {
	svc *service.DashboardV2Service
}

func NewDashboardV2Handler(svc *service.DashboardV2Service) *DashboardV2Handler {
	return &DashboardV2Handler{svc: svc}
}

// GetOverview GET /api/v1/dashboard/v2/overview
func (h *DashboardV2Handler) GetOverview(c echo.Context) error {
	date := c.QueryParam("date")
	result, err := h.svc.GetOverview(date)
	if err != nil {
		return response.Error(c, 500, "查询失败")
	}
	return response.Success(c, result)
}

// GetTrend GET /api/v1/dashboard/v2/trend
func (h *DashboardV2Handler) GetTrend(c echo.Context) error {
	metricType := c.QueryParam("metric_type")
	if metricType == "" {
		return response.Error(c, 400, "metric_type 不能为空")
	}
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	granularity := c.QueryParam("granularity")
	dimensionKey := c.QueryParam("dimension_key")

	result, err := h.svc.GetTrend(metricType, startDate, endDate, granularity, dimensionKey)
	if err != nil {
		return response.Error(c, 500, "查询趋势数据失败")
	}
	return response.Success(c, result)
}

// GetMultiTrend GET /api/v1/dashboard/v2/multi-trend
func (h *DashboardV2Handler) GetMultiTrend(c echo.Context) error {
	metricTypesStr := c.QueryParam("metric_types")
	if metricTypesStr == "" {
		return response.Error(c, 400, "metric_types 不能为空")
	}
	metricTypes := strings.Split(metricTypesStr, ",")
	for i, mt := range metricTypes {
		metricTypes[i] = strings.TrimSpace(mt)
	}

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	granularity := c.QueryParam("granularity")

	result, err := h.svc.GetMultiTrend(metricTypes, startDate, endDate, granularity)
	if err != nil {
		return response.Error(c, 500, "查询趋势数据失败")
	}
	return response.Success(c, result)
}

// GetDepartmentStats GET /api/v1/dashboard/v2/departments
func (h *DashboardV2Handler) GetDepartmentStats(c echo.Context) error {
	date := c.QueryParam("date")
	result, err := h.svc.GetDepartmentStats(date)
	if err != nil {
		return response.Error(c, 500, "查询部门统计失败")
	}
	return response.Success(c, result)
}

// GetDeviceStats GET /api/v1/dashboard/v2/devices
func (h *DashboardV2Handler) GetDeviceStats(c echo.Context) error {
	date := c.QueryParam("date")
	result, err := h.svc.GetDeviceStats(date)
	if err != nil {
		return response.Error(c, 500, "查询设备统计失败")
	}
	return response.Success(c, result)
}

// GetUserList GET /api/v1/dashboard/v2/users
func (h *DashboardV2Handler) GetUserList(c echo.Context) error {
	date := c.QueryParam("date")
	listType := c.QueryParam("list_type")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))

	result, err := h.svc.GetUserList(date, listType, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "查询用户列表失败")
	}
	return response.Success(c, result)
}

// ExportOverviewCSV GET /api/v1/dashboard/v2/export/overview
func (h *DashboardV2Handler) ExportOverviewCSV(c echo.Context) error {
	date := c.QueryParam("date")

	rows, err := h.svc.ExportOverviewCSV(date)
	if err != nil {
		return response.Error(c, 500, "导出失败")
	}

	filename := "overview_" + time.Now().Format("20060102_150405") + ".csv"
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

// ExportUserListCSV GET /api/v1/dashboard/v2/export/users
func (h *DashboardV2Handler) ExportUserListCSV(c echo.Context) error {
	date := c.QueryParam("date")
	listType := c.QueryParam("list_type")

	rows, err := h.svc.ExportUserListCSV(date, listType)
	if err != nil {
		return response.Error(c, 500, "导出失败")
	}

	filename := "users_" + listType + "_" + time.Now().Format("20060102_150405") + ".csv"
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
