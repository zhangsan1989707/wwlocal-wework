package handler

import (
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type SyncFeatureHandler struct {
	repo *repository.SyncFeatureRepository
}

func NewSyncFeatureHandler(repo *repository.SyncFeatureRepository) *SyncFeatureHandler {
	return &SyncFeatureHandler{repo: repo}
}

func (h *SyncFeatureHandler) List(c echo.Context) error {
	features, err := h.repo.GetAll()
	if err != nil {
		return response.Error(c, 500, "query sync features failed")
	}
	return response.Success(c, features)
}

type UpdateSyncFeaturesRequest struct {
	Features []struct {
		FeatureID int  `json:"feature_id"`
		Enabled   bool `json:"enabled"`
	} `json:"features"`
}

func (h *SyncFeatureHandler) Update(c echo.Context) error {
	var req UpdateSyncFeaturesRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}

	updates := make(map[int]bool)
	for _, f := range req.Features {
		updates[f.FeatureID] = f.Enabled
	}

	if err := h.repo.BatchSetEnabled(updates); err != nil {
		return response.Error(c, 500, "update sync features failed")
	}

	features, err := h.repo.GetAll()
	if err != nil {
		return response.Error(c, 500, "query sync features failed")
	}
	return response.Success(c, features)
}
