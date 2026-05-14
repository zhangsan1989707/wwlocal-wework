package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"
)

type SyncHistoryHandler struct {
	repo *repository.SyncHistoryRepository
}

func NewSyncHistoryHandler(repo *repository.SyncHistoryRepository) *SyncHistoryHandler {
	return &SyncHistoryHandler{repo: repo}
}

func (h *SyncHistoryHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
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
