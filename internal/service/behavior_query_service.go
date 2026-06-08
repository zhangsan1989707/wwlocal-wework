package service

import (
	"fmt"
	"strings"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type BehaviorQueryService struct {
	logRepo         *repository.LogRepository
	syncFeatureRepo *repository.SyncFeatureRepository
	cfg             *config.Config
}

func NewBehaviorQueryService(logRepo *repository.LogRepository, syncFeatureRepo *repository.SyncFeatureRepository, cfg *config.Config) *BehaviorQueryService {
	return &BehaviorQueryService{logRepo: logRepo, syncFeatureRepo: syncFeatureRepo, cfg: cfg}
}

func (s *BehaviorQueryService) Query(req *model.BehaviorQueryRequest) (*model.BehaviorQueryResult, error) {
	featureIDs, err := s.prepareRequest(req, 50, 200)
	if err != nil {
		return nil, err
	}
	return s.query(req, featureIDs)
}

func (s *BehaviorQueryService) Export(req *model.BehaviorQueryRequest) (*model.BehaviorQueryResult, error) {
	req.Page = 1
	req.PageSize = 50000
	featureIDs, err := s.prepareRequest(req, 50000, 50000)
	if err != nil {
		return nil, err
	}
	return s.query(req, featureIDs)
}

func (s *BehaviorQueryService) prepareRequest(req *model.BehaviorQueryRequest, defaultPageSize, maxPageSize int) ([]int, error) {
	req.OpenID = strings.TrimSpace(req.OpenID)
	if req.OpenID == "" {
		return nil, fmt.Errorf("openid is required")
	}
	if req.StartTime <= 0 || req.EndTime <= 0 {
		return nil, fmt.Errorf("start_time and end_time are required")
	}
	if req.EndTime < req.StartTime {
		return nil, fmt.Errorf("end_time must be greater than or equal to start_time")
	}
	if req.EndTime-req.StartTime > 31*24*3600 {
		return nil, fmt.Errorf("time range cannot exceed 31 days")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	if req.PageSize > maxPageSize {
		req.PageSize = maxPageSize
	}
	featureIDs := req.FeatureIDs
	if len(featureIDs) == 0 {
		ids, err := s.syncFeatureRepo.GetEnabledIDs()
		if err == nil && len(ids) > 0 {
			featureIDs = ids
		} else {
			featureIDs = s.cfg.Features.IDs
		}
	}
	return featureIDs, nil
}

func (s *BehaviorQueryService) query(req *model.BehaviorQueryRequest, featureIDs []int) (*model.BehaviorQueryResult, error) {
	rows, summaries, total, err := s.logRepo.QueryBehavior(featureIDs, req.OpenID, req.StartTime, req.EndTime, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].FeatureName = s.GetFeatureName(rows[i].FeatureID)
	}
	for i := range summaries {
		summaries[i].FeatureName = s.GetFeatureName(summaries[i].FeatureID)
	}
	return &model.BehaviorQueryResult{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Features: summaries,
		Data:     rows,
	}, nil
}

func (s *BehaviorQueryService) GetFeatureName(featureID int) string {
	if name, ok := s.cfg.Features.Names[featureID]; ok {
		return name
	}
	return fmt.Sprintf("未知(%d)", featureID)
}
