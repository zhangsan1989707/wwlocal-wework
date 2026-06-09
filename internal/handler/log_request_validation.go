package handler

import (
	"fmt"

	"wwlocal-wework/internal/model"
)

const (
	defaultLogPageSize = 100
	maxLogPageSize     = 1000
	maxExportPageSize  = 50000
	maxLogTimeRange    = 90 * 24 * 3600
	maxLogFeatureCount = 10
)

func validateLogQueryRequest(req *model.QueryRequest, maxPageSize int) error {
	if len(req.FeatureIDs) == 0 {
		return fmt.Errorf("feature_ids is required")
	}
	if len(req.FeatureIDs) > maxLogFeatureCount {
		return fmt.Errorf("cannot query more than %d feature types at the same time", maxLogFeatureCount)
	}
	for _, featureID := range req.FeatureIDs {
		if featureID <= 0 {
			return fmt.Errorf("feature_ids contains invalid id")
		}
	}
	if req.StartTime <= 0 || req.EndTime <= 0 {
		return fmt.Errorf("start_time and end_time are required")
	}
	if req.StartTime > req.EndTime {
		return fmt.Errorf("start_time must be less than or equal to end_time")
	}
	if req.EndTime-req.StartTime > maxLogTimeRange {
		return fmt.Errorf("time range cannot exceed 90 days")
	}
	if req.Page < 0 {
		return fmt.Errorf("page must be >= 0")
	}
	if req.Page == 0 {
		req.Page = defaultPage
	}
	if req.PageSize < 0 {
		return fmt.Errorf("page_size must be >= 0")
	}
	if req.PageSize == 0 {
		req.PageSize = defaultLogPageSize
	}
	if req.PageSize > maxPageSize {
		return fmt.Errorf("page_size must be <= %d", maxPageSize)
	}
	if req.Cursor < 0 {
		return fmt.Errorf("cursor must be >= 0")
	}
	return nil
}
