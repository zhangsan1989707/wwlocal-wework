package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func (h *BehaviorQueryHandler) ExportCSV(c echo.Context) error {
	var req model.BehaviorQueryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	result, err := h.behaviorSvc.Export(&req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "range") || strings.Contains(msg, "time") {
			return response.Error(c, 400, msg)
		}
		return response.Error(c, 500, "行为导出失败")
	}

	filename := fmt.Sprintf("behavior_timeline_%s.csv", time.Now().Format("20060102_150405"))
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)
	c.Response().Write([]byte("\xef\xbb\xbf"))

	writer := csv.NewWriter(c.Response())
	writer.Write([]string{"时间", "日志类型编号", "日志类型名称", "命中字段", "摘要", "数据详情"})
	for _, row := range result.Data {
		writer.Write([]string{
			row.LogDate,
			fmt.Sprintf("%d", row.FeatureID),
			row.FeatureName,
			formatMatchedFields(row.MatchedFields),
			formatBehaviorSummary(row.Data),
			formatBehaviorData(row.Data),
		})
	}
	writer.Flush()
	return nil
}

func formatMatchedFields(fields []model.MatchedField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("%s:%s", field.Label, field.Field))
	}
	return strings.Join(parts, " | ")
}

func formatBehaviorSummary(data map[string]interface{}) string {
	keys := []string{"msgid", "msg_type", "chatid", "name", "deviceid", "cli_ip", "access_ip"}
	var parts []string
	for _, key := range keys {
		value, ok := data[key]
		if !ok || value == nil || fmt.Sprintf("%v", value) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %v", key, value))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}
	text := formatBehaviorData(data)
	runes := []rune(text)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return text
}

func formatBehaviorData(data map[string]interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(bytes)
}
