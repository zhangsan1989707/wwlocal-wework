package handler

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type AdminOperLogHandler struct {
	svc *service.AdminOperLogService
}

func NewAdminOperLogHandler(svc *service.AdminOperLogService) *AdminOperLogHandler {
	return &AdminOperLogHandler{svc: svc}
}

type ListAdminOperLogsRequest struct {
	OperType   string `json:"oper_type"`
	OperUserID string `json:"oper_userid"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

func (h *AdminOperLogHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	operType := c.QueryParam("oper_type")
	operUserID := c.QueryParam("oper_userid")
	startTime, _ := strconv.ParseInt(c.QueryParam("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.QueryParam("end_time"), 10, 64)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	logs, total, err := h.svc.Query(operType, operUserID, startTime, endTime, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "query admin oper logs failed: "+err.Error())
	}

	return response.Success(c, map[string]interface{}{
		"data":  logs,
		"total": total,
		"page":  page,
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

	if startTime > 0 && endTime > 0 && startTime > endTime {
		return response.Error(c, 400, "start_time must be less than end_time")
	}

	var count int
	var err error
	if startTime > 0 && endTime > 0 {
		count, err = h.svc.SyncLogs(startTime, endTime)
	} else {
		count, err = h.svc.SyncIncremental()
	}
	if err != nil {
		return response.Error(c, 500, "sync failed: "+err.Error())
	}

	return response.Success(c, map[string]interface{}{
		"synced": count,
		"message": "sync completed",
	})
}

func (h *AdminOperLogHandler) GetStats(c echo.Context) error {
	startTimeStr := c.QueryParam("start_time")
	endTimeStr := c.QueryParam("end_time")

	var startTime, endTime int64
	var err error

	if startTimeStr != "" {
		startTime, err = strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return response.Error(c, 400, "invalid start_time")
		}
	}

	if endTimeStr != "" {
		endTime, err = strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			return response.Error(c, 400, "invalid end_time")
		}
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

	stats, err := h.svc.GetStats(startTime, endTime)
	if err != nil {
		return response.Error(c, 500, "get stats failed: "+err.Error())
	}

	return response.Success(c, stats)
}

func (h *AdminOperLogHandler) GetOperTypes(c echo.Context) error {
	types, err := h.svc.GetOperTypes()
	if err != nil {
		return response.Error(c, 500, "get oper types failed: "+err.Error())
	}
	return response.Success(c, types)
}

func (h *AdminOperLogHandler) GetOperUsers(c echo.Context) error {
	users, err := h.svc.GetOperUsers()
	if err != nil {
		return response.Error(c, 500, "get oper users failed: "+err.Error())
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
		return response.Error(c, 500, "cleanup failed: "+err.Error())
	}

	return response.Success(c, map[string]interface{}{
		"deleted": deleted,
		"message": "cleanup completed",
	})
}

func (h *AdminOperLogHandler) Status(c echo.Context) error {
	running, total, lastTime, err := h.svc.GetStatus()
	if err != nil {
		return response.Error(c, 500, "get status failed: "+err.Error())
	}

	return response.Success(c, map[string]interface{}{
		"running":   running,
		"total":     total,
		"last_time": lastTime,
	})
}
