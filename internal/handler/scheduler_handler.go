package handler

import (
	"fmt"
	"time"

	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type SchedulerHandler struct {
	schedulerSvc *service.SchedulerService
	syncSvc      *service.SyncService
}

func NewSchedulerHandler(schedulerSvc *service.SchedulerService, syncSvc *service.SyncService) *SchedulerHandler {
	return &SchedulerHandler{schedulerSvc: schedulerSvc, syncSvc: syncSvc}
}

type StartSchedulerRequest struct {
	StartDelay string `json:"start_delay"` // "10m", "30m", "1h"
}

func (h *SchedulerHandler) Start(c echo.Context) error {
	var req StartSchedulerRequest
	if err := c.Bind(&req); err == nil && req.StartDelay != "" {
		d, err := time.ParseDuration(req.StartDelay)
		if err == nil && d >= time.Minute {
			h.schedulerSvc.Start(d)
			return response.Success(c, h.schedulerSvc.GetStatus())
		}
	}
	h.schedulerSvc.Start(h.schedulerSvc.GetInterval())
	return response.Success(c, h.schedulerSvc.GetStatus())
}

func (h *SchedulerHandler) Stop(c echo.Context) error {
	h.schedulerSvc.Stop()
	return response.Success(c, h.schedulerSvc.GetStatus())
}

func (h *SchedulerHandler) Status(c echo.Context) error {
	return response.Success(c, h.schedulerSvc.GetStatus())
}

type IncrementalSyncRequest struct {
	FeatureIDs []int `json:"feature_ids"`
	SyncAll    bool  `json:"sync_all"`
}

func (h *SchedulerHandler) IncrementalSync(c echo.Context) error {
	var req IncrementalSyncRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	if err := validateIncrementalSyncRequest(&req); err != nil {
		return response.Error(c, 400, err.Error())
	}

	if h.syncSvc.IsRunning() {
		return response.Error(c, 409, "sync already in progress")
	}

	h.syncSvc.StartSync(func() {
		if req.SyncAll {
			h.syncSvc.SyncAllFeaturesIncremental()
		} else {
			h.syncSvc.SyncMultipleFeaturesIncremental(req.FeatureIDs)
		}
	})

	return response.Success(c, map[string]interface{}{
		"message": "incremental sync started",
		"running": true,
	})
}

func validateIncrementalSyncRequest(req *IncrementalSyncRequest) error {
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

type SetIntervalRequest struct {
	Interval string `json:"interval"` // "1h", "30m", "24h"
}

func (h *SchedulerHandler) SetInterval(c echo.Context) error {
	var req SetIntervalRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	d, err := time.ParseDuration(req.Interval)
	if err != nil {
		return response.Error(c, 400, "invalid interval format, use e.g. '1h', '30m', '24h'")
	}

	if d < time.Minute {
		return response.Error(c, 400, "interval must be at least 1m")
	}

	h.schedulerSvc.SetInterval(d)
	return response.Success(c, h.schedulerSvc.GetStatus())
}
