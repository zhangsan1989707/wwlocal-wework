package handler

import (
	"fmt"

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

type SyncFeatureUpdate struct {
	FeatureID int  `json:"feature_id"`
	Enabled   bool `json:"enabled"`
}

type UpdateSyncFeaturesRequest struct {
	Features []SyncFeatureUpdate `json:"features"`
}

func (h *SyncFeatureHandler) Update(c echo.Context) error {
	var req UpdateSyncFeaturesRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, 400, "invalid request body")
	}
	if err := validateUpdateSyncFeaturesRequest(&req); err != nil {
		return response.Error(c, 400, err.Error())
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

func validateUpdateSyncFeaturesRequest(req *UpdateSyncFeaturesRequest) error {
	if len(req.Features) == 0 {
		return fmt.Errorf("features is required")
	}
	seen := make(map[int]bool, len(req.Features))
	for _, feature := range req.Features {
		if feature.FeatureID <= 0 {
			return fmt.Errorf("feature_id must be positive")
		}
		if seen[feature.FeatureID] {
			return fmt.Errorf("duplicate feature_id %d", feature.FeatureID)
		}
		seen[feature.FeatureID] = true
	}
	return nil
}
