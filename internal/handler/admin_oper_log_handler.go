package handler

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type AdminOperLogHandler struct {
	svc adminOperLogService
}

type adminOperLogService interface {
	Query(operType, operUserID string, startTime, endTime int64, page, pageSize int) ([]model.AdminOperLog, int64, error)
	StartSync(startTime, endTime int64) bool
	GetStats(startTime, endTime int64) (map[string]interface{}, error)
	GetOperTypes() ([]string, error)
	GetOperUsers() ([]string, error)
	Cleanup(beforeDays int) (int64, error)
	GetStatus() (service.AdminOperLogSyncStatus, error)
}

func NewAdminOperLogHandler(svc *service.AdminOperLogService) *AdminOperLogHandler {
	return &AdminOperLogHandler{svc: svc}
}

func (h *AdminOperLogHandler) List(c echo.Context) error {
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	operType := c.QueryParam("oper_type")
	operUserID := c.QueryParam("oper_userid")
	startTime, err := parseOptionalNonNegativeInt64Query(c, "start_time")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	endTime, err := parseOptionalNonNegativeInt64Query(c, "end_time")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	if startTime > 0 && endTime > 0 && startTime > endTime {
		return response.Error(c, 400, "start_time must be less than end_time")
	}

	logs, total, err := h.svc.Query(operType, operUserID, startTime, endTime, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "查询操作日志失败")
	}

	return response.Success(c, map[string]interface{}{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminOperLogHandler) Sync(c echo.Context) error {
	var req struct {
		StartTime int64 `json:"start_time"`
		EndTime   int64 `json:"end_time"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	startTime := req.StartTime
	endTime := req.EndTime
	if startTime < 0 || endTime < 0 {
		return response.Error(c, 400, "start_time and end_time must be >= 0")
	}

	if startTime > 0 && endTime > 0 && startTime > endTime {
		return response.Error(c, 400, "start_time must be less than end_time")
	}

	if !h.svc.StartSync(startTime, endTime) {
		return response.Error(c, 409, "企微操作日志同步正在进行中")
	}

	return response.Success(c, map[string]interface{}{
		"message": "sync started",
		"running": true,
	})
}

func (h *AdminOperLogHandler) GetStats(c echo.Context) error {
	startTime, err := parseOptionalNonNegativeInt64Query(c, "start_time")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	endTime, err := parseOptionalNonNegativeInt64Query(c, "end_time")
	if err != nil {
		return response.Error(c, 400, err.Error())
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	if endTime == 0 {
		endTime = now.Unix()
	}
	if startTime == 0 {
		startTime = now.AddDate(0, 0, -30).Unix()
	}
	if startTime > endTime {
		return response.Error(c, 400, "start_time must be less than end_time")
	}

	stats, err := h.svc.GetStats(startTime, endTime)
	if err != nil {
		return response.Error(c, 500, "获取统计数据失败")
	}

	return response.Success(c, stats)
}

func (h *AdminOperLogHandler) GetOperTypes(c echo.Context) error {
	types, err := h.svc.GetOperTypes()
	if err != nil {
		return response.Error(c, 500, "获取操作类型失败")
	}
	return response.Success(c, types)
}

func (h *AdminOperLogHandler) GetOperUsers(c echo.Context) error {
	users, err := h.svc.GetOperUsers()
	if err != nil {
		return response.Error(c, 500, "获取操作用户失败")
	}
	return response.Success(c, users)
}

func (h *AdminOperLogHandler) Cleanup(c echo.Context) error {
	daysStr := c.QueryParam("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 90
	}

	deleted, err := h.svc.Cleanup(days)
	if err != nil {
		return response.Error(c, 500, "清理失败")
	}

	return response.Success(c, map[string]interface{}{
		"deleted": deleted,
		"message": "cleanup completed",
	})
}

func (h *AdminOperLogHandler) Status(c echo.Context) error {
	status, err := h.svc.GetStatus()
	if err != nil {
		return response.Error(c, 500, "获取状态失败")
	}

	return response.Success(c, status)
}
