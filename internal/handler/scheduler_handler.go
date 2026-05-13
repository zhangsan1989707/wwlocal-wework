package handler

import (
	"log"
	"runtime/debug"
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

func (h *SchedulerHandler) Start(c echo.Context) error {
	h.schedulerSvc.Start()
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

	if h.syncSvc.IsRunning() {
		return response.Error(c, 409, "sync already in progress")
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("incremental sync goroutine panic: %v\n%s", r, debug.Stack())
			}
		}()
		if req.SyncAll {
			h.syncSvc.SyncAllFeaturesIncremental()
		} else if len(req.FeatureIDs) > 0 {
			h.syncSvc.SyncMultipleFeaturesIncremental(req.FeatureIDs)
		} else {
			h.syncSvc.SyncAllFeaturesIncremental()
		}
	}()

	return response.Success(c, map[string]interface{}{
		"message": "incremental sync started",
		"running": true,
	})
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
