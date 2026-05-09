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
	cfg        *config.WeWorkConfig
	mu         sync.Mutex
	running    bool
}

func NewSyncService(weworkSvc *WeWorkService, decryptSvc *DecryptService, logRepo *repository.LogRepository, keyRepo *repository.KeyRepository, cfg *config.WeWorkConfig) *SyncService {
	return &SyncService{
		weworkSvc:  weworkSvc,
		decryptSvc: decryptSvc,
		logRepo:    logRepo,
		keyRepo:    keyRepo,
		cfg:        cfg,
	}
}

func (s *SyncService) SyncFeature(featureID int, startTime, endTime int64) (int, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return 0, fmt.Errorf("sync already in progress")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	totalFetched := 0
	startIndex := 0
	limit := s.cfg.SyncLimit

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
	results := make(map[int]int)
	featureIDs := []int{
		90000031, 90000032, 90000033, 90000034, 90000035,
		90000036, 90000037, 90000038, 90000039, 90000040,
		90000041, 90000042, 90000043, 90000044, 90000047,
		90000048, 90000054, 90000055, 90000058, 90000059,
		90000061, 90000062, 90000063, 90000066,
	}

	for _, featureID := range featureIDs {
		count, err := s.SyncFeature(featureID, startTime, endTime)
		if err != nil {
			log.Printf("sync feature %d failed: %v", featureID, err)
			results[featureID] = -1
		} else {
			results[featureID] = count
		}

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (s *SyncService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

type SyncStatus struct {
	Running     bool               `json:"running"`
	LastSync    time.Time          `json:"last_sync,omitempty"`
	Results     map[int]int        `json:"results,omitempty"`
}