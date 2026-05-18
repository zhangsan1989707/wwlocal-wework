package handler

import (
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
		return response.Error(c, 500, err.Error())
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
		return response.Error(c, 500, err.Error())
	}

	return response.Success(c, result)
}
