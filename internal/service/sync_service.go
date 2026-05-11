package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type SyncService struct {
	weworkSvc  *WeWorkService
	decryptSvc *DecryptService
	logRepo    *repository.LogRepository
	keyRepo    *repository.KeyRepository
	cfg        *config.Config
	mu         sync.Mutex
	status     *SyncStatus
}

func NewSyncService(weworkSvc *WeWorkService, decryptSvc *DecryptService, logRepo *repository.LogRepository, keyRepo *repository.KeyRepository, cfg *config.Config) *SyncService {
	return &SyncService{
		weworkSvc:  weworkSvc,
		decryptSvc: decryptSvc,
		logRepo:    logRepo,
		keyRepo:    keyRepo,
		cfg:        cfg,
		status: &SyncStatus{
			Running: false,
			Results: make(map[int]int),
		},
	}
}

func (s *SyncService) SyncFeature(featureID int, startTime, endTime int64) (int, error) {
	totalFetched := 0
	startIndex := 0
	limit := s.cfg.WeWork.SyncLimit

	if limit > 1000 {
		limit = 1000
	}

	for {
		logItems, err := s.weworkSvc.GetLogList(featureID, startTime, endTime, startIndex, limit)
		if err != nil {
			return totalFetched, fmt.Errorf("fetch log list failed: %w", err)
		}

		if len(logItems) == 0 {
			break
		}

		for _, item := range logItems {
			entry, err := s.decryptSvc.DecryptWithKey(&model.WeWorkLogItem{
				FeatureID: item.FeatureID,
				LogTime:   item.LogTime,
				IDC:       item.IDC,
				EncKey:    item.EncKey,
				EncData:   item.EncData,
			}, "")
			if err != nil {
				log.Printf("decrypt failed for feature %d, log_time %d: %v", featureID, item.LogTime, err)
				continue
			}

			if err := s.logRepo.Save(featureID, entry); err != nil {
				log.Printf("save log entry failed: %v", err)
				continue
			}
			totalFetched++
		}

		if len(logItems) < limit {
			break
		}

		startIndex += len(logItems)
	}

	return totalFetched, nil
}

func (s *SyncService) SyncAllFeatures(startTime, endTime int64) map[int]int {
	s.mu.Lock()
	s.status.Running = true
	s.status.Progress = 0
	s.status.Total = len(s.cfg.Features.IDs)
	s.status.Results = make(map[int]int)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.LastSync = time.Now()
		s.mu.Unlock()
	}()

	results := make(map[int]int)
	for i, featureID := range s.cfg.Features.IDs {
		count, err := s.SyncFeature(featureID, startTime, endTime)
		if err != nil {
			log.Printf("sync feature %d failed: %v", featureID, err)
			results[featureID] = -1
		} else {
			results[featureID] = count
		}

		s.mu.Lock()
		s.status.Progress = i + 1
		s.status.Results = results
		s.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (s *SyncService) SyncMultipleFeatures(featureIDs []int, startTime, endTime int64) map[int]int {
	s.mu.Lock()
	s.status.Running = true
	s.status.Progress = 0
	s.status.Total = len(featureIDs)
	s.status.Results = make(map[int]int)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.LastSync = time.Now()
		s.mu.Unlock()
	}()

	results := make(map[int]int)
	for i, featureID := range featureIDs {
		count, err := s.SyncFeature(featureID, startTime, endTime)
		if err != nil {
			log.Printf("sync feature %d failed: %v", featureID, err)
			results[featureID] = -1
		} else {
			results[featureID] = count
		}

		s.mu.Lock()
		s.status.Progress = i + 1
		s.status.Results = results
		s.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (s *SyncService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.Running
}

func (s *SyncService) GetStatus() *SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 返回状态的副本
	statusCopy := *s.status
	if s.status.Results != nil {
		statusCopy.Results = make(map[int]int)
		for k, v := range s.status.Results {
			statusCopy.Results[k] = v
		}
	}
	return &statusCopy
}

type SyncStatus struct {
	Running  bool        `json:"running"`
	Progress int         `json:"progress"`
	Total    int         `json:"total"`
	LastSync time.Time   `json:"last_sync,omitempty"`
	Results  map[int]int `json:"results,omitempty"`
}