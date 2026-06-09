package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type DashboardV2Handler struct {
	svc     *service.DashboardV2Service
	userSvc *service.UserService
}

func NewDashboardV2Handler(svc *service.DashboardV2Service, userSvc *service.UserService) *DashboardV2Handler {
	return &DashboardV2Handler{svc: svc, userSvc: userSvc}
}

func (h *DashboardV2Handler) scope(c echo.Context) (*service.DataScope, error) {
	userID := middleware.CurrentUserID(c)
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user")
	}
	return h.userSvc.ResolveDataScope(userID)
}

func dashboardV2Error(c echo.Context, err error, fallback string) error {
	if errors.Is(err, service.ErrDashboardInvalidParam) {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}
	return response.Error(c, http.StatusInternalServerError, fallback)
}

// GetOverview GET /api/v1/dashboard/v2/overview
func (h *DashboardV2Handler) GetOverview(c echo.Context) error {
	date := c.QueryParam("date")
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetOverview(date, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询失败")
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

	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetTrend(metricType, startDate, endDate, granularity, dimensionKey, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询趋势数据失败")
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
	cleaned := make([]string, 0, len(metricTypes))
	for _, mt := range metricTypes {
		if mt = strings.TrimSpace(mt); mt != "" {
			cleaned = append(cleaned, mt)
		}
	}
	metricTypes = cleaned
	if len(metricTypes) == 0 {
		return response.Error(c, 400, "metric_types 不能为空")
	}

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	granularity := c.QueryParam("granularity")

	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetMultiTrend(metricTypes, startDate, endDate, granularity, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询趋势数据失败")
	}
	return response.Success(c, result)
}

// GetDepartmentStats GET /api/v1/dashboard/v2/departments
func (h *DashboardV2Handler) GetDepartmentStats(c echo.Context) error {
	date := c.QueryParam("date")
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetDepartmentStats(date, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询部门统计失败")
	}
	return response.Success(c, result)
}

// GetDeviceStats GET /api/v1/dashboard/v2/devices
func (h *DashboardV2Handler) GetDeviceStats(c echo.Context) error {
	date := c.QueryParam("date")
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetDeviceStats(date, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询设备统计失败")
	}
	return response.Success(c, result)
}

// GetUserList GET /api/v1/dashboard/v2/users
func (h *DashboardV2Handler) GetUserList(c echo.Context) error {
	date := c.QueryParam("date")
	listType := c.QueryParam("list_type")
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}

	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetUserList(date, listType, page, pageSize, scope)
	if err != nil {
		return dashboardV2Error(c, err, "查询用户列表失败")
	}
	return response.Success(c, result)
}

// ExportOverviewCSV GET /api/v1/dashboard/v2/export/overview
func (h *DashboardV2Handler) ExportOverviewCSV(c echo.Context) error {
	date := c.QueryParam("date")

	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	rows, err := h.svc.ExportOverviewCSV(date, scope)
	if err != nil {
		return dashboardV2Error(c, err, "导出失败")
	}

	return writeCSV(c, "overview_"+time.Now().Format("20060102_150405")+".csv", rows)
}

// ExportUserListCSV GET /api/v1/dashboard/v2/export/users
func (h *DashboardV2Handler) ExportUserListCSV(c echo.Context) error {
	date := c.QueryParam("date")
	listType := c.QueryParam("list_type")

	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	rows, err := h.svc.ExportUserListCSV(date, listType, scope)
	if err != nil {
		return dashboardV2Error(c, err, "导出失败")
	}

	return writeCSV(c, "users_"+listType+"_"+time.Now().Format("20060102_150405")+".csv", rows)
}

func (h *DashboardV2Handler) ExportTrendCSV(c echo.Context) error {
	metricTypesStr := c.QueryParam("metric_types")
	if metricTypesStr == "" {
		metricTypesStr = c.QueryParam("metric_type")
	}
	if metricTypesStr == "" {
		metricTypesStr = "login_users,usage_users,msg_count,app_access_count"
	}
	metricTypes := strings.Split(metricTypesStr, ",")
	cleaned := make([]string, 0, len(metricTypes))
	for _, mt := range metricTypes {
		if mt = strings.TrimSpace(mt); mt != "" {
			cleaned = append(cleaned, mt)
		}
	}
	metricTypes = cleaned
	if len(metricTypes) == 0 {
		return response.Error(c, 400, "metric_types 不能为空")
	}
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	result, err := h.svc.GetMultiTrend(metricTypes, c.QueryParam("start_date"), c.QueryParam("end_date"), c.QueryParam("granularity"), scope)
	if err != nil {
		return dashboardV2Error(c, err, "导出失败")
	}
	rows := [][]string{{"周期"}}
	for _, mt := range metricTypes {
		rows[0] = append(rows[0], mt)
	}
	periods, _ := result["periods"].([]string)
	series, _ := result["series"].(map[string][]int64)
	for i, p := range periods {
		row := []string{p}
		for _, mt := range metricTypes {
			var v int64
			if i < len(series[mt]) {
				v = series[mt][i]
			}
			row = append(row, strconv.FormatInt(v, 10))
		}
		rows = append(rows, row)
	}
	return writeCSV(c, "trend_"+time.Now().Format("20060102_150405")+".csv", rows)
}

func (h *DashboardV2Handler) ExportDepartmentsCSV(c echo.Context) error {
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	depts, err := h.svc.GetDepartmentStats(c.QueryParam("date"), scope)
	if err != nil {
		return dashboardV2Error(c, err, "导出失败")
	}
	rows := [][]string{{"部门ID", "部门名称", "总人数", "活跃人数", "未活跃人数", "活跃率"}}
	for _, d := range depts {
		rows = append(rows, []string{
			fmt.Sprintf("%v", d["dept_id"]),
			fmt.Sprintf("%v", d["dept_name"]),
			fmt.Sprintf("%v", d["total_contacts"]),
			fmt.Sprintf("%v", d["active"]),
			fmt.Sprintf("%v", d["inactive"]),
			fmt.Sprintf("%.2f", d["active_rate"]),
		})
	}
	return writeCSV(c, "departments_"+time.Now().Format("20060102_150405")+".csv", rows)
}

func (h *DashboardV2Handler) ExportDevicesCSV(c echo.Context) error {
	scope, err := h.scope(c)
	if err != nil {
		return response.Error(c, 401, "用户无效")
	}
	devices, err := h.svc.GetDeviceStats(c.QueryParam("date"), scope)
	if err != nil {
		return dashboardV2Error(c, err, "导出失败")
	}
	rows := [][]string{{"设备类型", "名称", "数量", "占比"}}
	if items, ok := devices["types"].([]map[string]interface{}); ok {
		for _, item := range items {
			rows = append(rows, []string{
				fmt.Sprintf("%v", item["type"]),
				fmt.Sprintf("%v", item["name"]),
				fmt.Sprintf("%v", item["count"]),
				fmt.Sprintf("%.2f", item["percentage"]),
			})
		}
	} else {
		rows = append(rows, []string{"total", "总计", fmt.Sprintf("%v", devices["total"]), ""})
	}
	return writeCSV(c, "devices_"+time.Now().Format("20060102_150405")+".csv", rows)
}

func writeCSV(c echo.Context, filename string, rows [][]string) error {
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
