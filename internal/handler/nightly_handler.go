package handler

import (
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

var dateRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type NightlyHandler struct {
	nightlySvc *service.NightlyJobService
}

func NewNightlyHandler(nightlySvc *service.NightlyJobService) *NightlyHandler {
	return &NightlyHandler{nightlySvc: nightlySvc}
}

func (h *NightlyHandler) Run(c echo.Context) error {
	statDate := c.QueryParam("date")
	if statDate == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if loc == nil {
			loc = time.FixedZone("CST", 8*3600)
		}
		statDate = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}
	if !dateRegexp.MatchString(statDate) {
		return response.Error(c, 400, "日期格式错误，应为 YYYY-MM-DD")
	}
	if h.nightlySvc.IsJobRunning() {
		return response.Error(c, 409, "nightly job 正在运行中，请稍后再试")
	}
	h.nightlySvc.RunOnce(statDate)
	return response.Success(c, map[string]interface{}{
		"message": "nightly job triggered",
		"date":    statDate,
	})
}

func (h *NightlyHandler) Status(c echo.Context) error {
	return response.Success(c, h.nightlySvc.Status())
}
