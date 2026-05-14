package handler

import (
	"log"
	"runtime/debug"

	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type SyncHandler struct {
	syncSvc *service.SyncService
}

func NewSyncHandler(syncSvc *service.SyncService) *SyncHandler {
	return &SyncHandler{syncSvc: syncSvc}
}

type SyncRequest struct {
	FeatureIDs []int  `json:"feature_ids"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	SyncAll    bool   `json:"sync_all"`
}

func (h *SyncHandler) Sync(c echo.Context) error {
	var req SyncRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	if h.syncSvc.IsRunning() {
		return response.Error(c, 409, "sync already in progress")
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("sync goroutine panic: %v\n%s", r, debug.Stack())
			}
			h.syncSvc.ResetRunning()
		}()
		if !h.syncSvc.TryStartRunning() {
			return
		}
		if req.SyncAll {
			h.syncSvc.SyncAllFeatures(req.StartTime, req.EndTime)
		} else if len(req.FeatureIDs) > 0 {
			h.syncSvc.SyncMultipleFeatures(req.FeatureIDs, req.StartTime, req.EndTime)
		}
	}()

	return response.Success(c, map[string]interface{}{
		"message": "sync started",
		"running": true,
	})
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