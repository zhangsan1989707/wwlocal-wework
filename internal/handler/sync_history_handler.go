package handler

import (
	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"
)

type SyncHistoryHandler struct {
	repo syncHistoryLister
}

type syncHistoryLister interface {
	List(syncType string, page, pageSize int) ([]model.SyncHistory, int64, error)
}

func NewSyncHistoryHandler(repo *repository.SyncHistoryRepository) *SyncHistoryHandler {
	return &SyncHistoryHandler{repo: repo}
}

func (h *SyncHistoryHandler) List(c echo.Context) error {
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return response.Error(c, 400, err.Error())
	}
	syncType := c.QueryParam("sync_type")

	items, total, err := h.repo.List(syncType, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "query sync history failed")
	}

	return response.Success(c, map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      items,
	})
}
