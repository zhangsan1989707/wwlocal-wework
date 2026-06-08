package handler

import (
	"fmt"

	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type SyncHandler struct {
	syncSvc logSyncService
}

type logSyncService interface {
	IsRunning() bool
	StartSync(fn func())
	SyncAllFeatures(startTime, endTime int64) map[int]int
	SyncMultipleFeatures(featureIDs []int, startTime, endTime int64) map[int]int
	GetStatus() *service.SyncStatus
	Cancel()
}

func NewSyncHandler(syncSvc *service.SyncService) *SyncHandler {
	return &SyncHandler{syncSvc: syncSvc}
}

type SyncRequest struct {
	FeatureIDs []int `json:"feature_ids"`
	StartTime  int64 `json:"start_time"`
	EndTime    int64 `json:"end_time"`
	SyncAll    bool  `json:"sync_all"`
}

func (h *SyncHandler) Sync(c echo.Context) error {
	var req SyncRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if err := validateSyncRequest(&req); err != nil {
		return response.Error(c, 400, err.Error())
	}

	if h.syncSvc.IsRunning() {
		return response.Error(c, 409, "sync already in progress")
	}

	h.syncSvc.StartSync(func() {
		if req.SyncAll {
			h.syncSvc.SyncAllFeatures(req.StartTime, req.EndTime)
		} else if len(req.FeatureIDs) > 0 {
			h.syncSvc.SyncMultipleFeatures(req.FeatureIDs, req.StartTime, req.EndTime)
		}
	})

	return response.Success(c, map[string]interface{}{
		"message": "sync started",
		"running": true,
	})
}

func validateSyncRequest(req *SyncRequest) error {
	if req.StartTime < 0 || req.EndTime < 0 {
		return fmt.Errorf("start_time and end_time must be >= 0")
	}
	if req.StartTime > 0 && req.EndTime > 0 && req.StartTime > req.EndTime {
		return fmt.Errorf("start_time must be less than or equal to end_time")
	}
	if (req.StartTime > 0 && req.EndTime == 0) || (req.StartTime == 0 && req.EndTime > 0) {
		return fmt.Errorf("start_time and end_time must be provided together")
	}
	if req.SyncAll {
		return nil
	}
	if len(req.FeatureIDs) == 0 {
		return fmt.Errorf("sync_all or feature_ids is required")
	}
	if len(req.FeatureIDs) > maxLogFeatureCount {
		return fmt.Errorf("cannot sync more than %d feature types at the same time", maxLogFeatureCount)
	}
	for _, featureID := range req.FeatureIDs {
		if featureID <= 0 {
			return fmt.Errorf("feature_ids contains invalid id")
		}
	}
	return nil
}

func (h *SyncHandler) Status(c echo.Context) error {
	return response.Success(c, h.syncSvc.GetStatus())
}

func (h *SyncHandler) Cancel(c echo.Context) error {
	if !h.syncSvc.IsRunning() {
		return response.Error(c, 400, "no sync in progress")
	}
	h.syncSvc.Cancel()
	return response.Success(c, map[string]interface{}{
		"message": "sync cancellation requested",
	})
}
