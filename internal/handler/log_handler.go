package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"
)

type LogHandler struct {
	querySvc *service.QueryService
}

func NewLogHandler(querySvc *service.QueryService) *LogHandler {
	return &LogHandler{querySvc: querySvc}
}

func (h *LogHandler) Query(c echo.Context) error {
	var req model.QueryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if len(req.FeatureIDs) == 0 {
		return response.Error(c, 400, "feature_ids is required")
	}

	if req.StartTime <= 0 || req.EndTime <= 0 {
		return response.Error(c, 400, "start_time and end_time are required")
	}

	result, err := h.querySvc.Query(&req)
	if err != nil {
		return response.Error(c, 500, "查询失败")
	}

	return response.Success(c, result)
}

func (h *LogHandler) GetFeatures(c echo.Context) error {
	features := h.querySvc.GetFeatureIDs()
	var result []map[string]interface{}
	for _, id := range features {
		result = append(result, map[string]interface{}{
			"id":   id,
			"name": h.querySvc.GetFeatureName(id),
		})
	}
	return response.Success(c, result)
}

func (h *LogHandler) GetTimeRange(c echo.Context) error {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	return response.Success(c, map[string]interface{}{
		"start_time": startOfDay.AddDate(0, 0, -7).Unix(),
		"end_time":   startOfDay.Add(24*time.Hour - time.Second).Unix(),
		"now":        now.Unix(),
	})
}

func (h *LogHandler) GetFieldPaths(c echo.Context) error {
	paths := h.querySvc.GetFieldPaths()
	return response.Success(c, paths)
}

func (h *LogHandler) QueryByCursor(c echo.Context) error {
	var req model.QueryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if len(req.FeatureIDs) == 0 {
		return response.Error(c, 400, "feature_ids is required")
	}

	if req.StartTime <= 0 || req.EndTime <= 0 {
		return response.Error(c, 400, "start_time and end_time are required")
	}

	result, err := h.querySvc.QueryByCursor(&req)
	if err != nil {
		return response.Error(c, 500, "查询失败")
	}

	return response.Success(c, result)
}

func (h *LogHandler) ExportCSV(c echo.Context) error {
	var req model.QueryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if len(req.FeatureIDs) == 0 {
		return response.Error(c, 400, "feature_ids is required")
	}

	if req.StartTime <= 0 || req.EndTime <= 0 {
		return response.Error(c, 400, "start_time and end_time are required")
	}

	data, err := h.querySvc.ExportCSV(&req)
	if err != nil {
		return response.Error(c, 500, "导出失败")
	}

	filename := fmt.Sprintf("log_query_%s.csv", time.Now().Format("20060102_150405"))
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	c.Response().Write([]byte("\xef\xbb\xbf"))

	writer := csv.NewWriter(c.Response())
	writer.Write([]string{"日志类型编号", "日志类型名称", "时间", "openid", "数据内容"})

	for _, row := range data {
		featureID := fmt.Sprintf("%d", getInt64(row, "feature_id"))
		featureName := h.querySvc.GetFeatureName(int(getInt64(row, "feature_id")))
		logDate := fmt.Sprintf("%v", row["log_date"])
		openid := fmt.Sprintf("%v", getString(row, "openid"))

		dataContent := ""
		if _, failed := row["_decrypt_failed"]; !failed {
			content := make(map[string]interface{})
			for k, v := range row {
				if k != "id" && k != "feature_id" && k != "log_time" && k != "log_date" && k != "idc" && k != "_decrypt_failed" {
					content[k] = v
				}
			}
			if b, err := jsonMarshal(content); err == nil {
				dataContent = string(b)
			}
		} else {
			dataContent = "[解密失败]"
		}

		writer.Write([]string{featureID, featureName, logDate, openid, dataContent})
	}

	writer.Flush()
	return nil
}

func getInt64(row map[string]interface{}, key string) int64 {
	if v, ok := row[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case float64:
			return int64(val)
		}
	}
	return 0
}

func getString(row map[string]interface{}, key string) string {
	if v, ok := row[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
