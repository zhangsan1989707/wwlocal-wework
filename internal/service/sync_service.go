package service

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type SyncService struct {
	weworkSvc       *WeWorkService
	decryptSvc      *DecryptService
	logRepo         *repository.LogRepository
	keyRepo         *repository.KeyRepository
	syncStateRepo   *repository.SyncStateRepository
	syncHistoryRepo *repository.SyncHistoryRepository
	syncFeatureRepo *repository.SyncFeatureRepository
	cfg             *config.Config
	mu              sync.Mutex
	status          *SyncStatus
	cancelCh        chan struct{}
}

func NewSyncService(weworkSvc *WeWorkService, decryptSvc *DecryptService, logRepo *repository.LogRepository, keyRepo *repository.KeyRepository, syncStateRepo *repository.SyncStateRepository, syncHistoryRepo *repository.SyncHistoryRepository, syncFeatureRepo *repository.SyncFeatureRepository, cfg *config.Config) *SyncService {
	logRepo.CreateUserDailyStatsTable()
	return &SyncService{
		weworkSvc:       weworkSvc,
		decryptSvc:      decryptSvc,
		logRepo:         logRepo,
		keyRepo:         keyRepo,
		syncStateRepo:   syncStateRepo,
		syncHistoryRepo: syncHistoryRepo,
		syncFeatureRepo: syncFeatureRepo,
		cfg:             cfg,
		status: &SyncStatus{
			Running: false,
			Results: make(map[int]int),
			Errors:  make(map[int]string),
		},
	}
}

func (s *SyncService) SyncFeature(featureID int, startTime, endTime int64) (int, int, int64, error) {
	totalFetched := 0
	totalFailed := 0
	var maxLogTime int64
	limit := s.cfg.WeWork.SyncLimit

	if limit > 1000 {
		limit = 1000
	}
	if limit < 1 {
		limit = 1
	}

	// 按天拆分，政务微信 API 要求 start_time 和 end_time 在同一天
	days := s.splitByDay(startTime, endTime)
	log.Printf("SyncFeature: feature=%d, %d days to process", featureID, len(days))
	for di, day := range days {
		if s.isCancelled() {
			return totalFetched, totalFailed, maxLogTime, nil
		}
		if di%5 == 0 {
			log.Printf("SyncFeature: feature=%d, day %d/%d (ts=%d)", featureID, di+1, len(days), day.start)
		}
		fetched, failed, dayMax, err := s.syncFeatureDay(featureID, day.start, day.end, limit)
		totalFetched += fetched
		totalFailed += failed
		if dayMax > maxLogTime {
			maxLogTime = dayMax
		}
		if err != nil {
			return totalFetched, totalFailed, maxLogTime, err
		}
	}

	return totalFetched, totalFailed, maxLogTime, nil
}

type dayRange struct {
	start int64
	end   int64
}

func (s *SyncService) splitByDay(startTime, endTime int64) []dayRange {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	var days []dayRange
	cur := time.Unix(startTime, 0).In(loc)
	end := time.Unix(endTime, 0).In(loc)

	for !cur.After(end) {
		dayStart := time.Date(cur.Year(), cur.Month(), cur.Day(), 0, 0, 0, 0, loc)
		dayEnd := dayStart.Add(24*time.Hour - time.Second)
		if dayEnd.After(end) {
			dayEnd = end
		}
		days = append(days, dayRange{start: dayStart.Unix(), end: dayEnd.Unix()})
		cur = dayStart.AddDate(0, 0, 1)
	}
	return days
}

func (s *SyncService) syncFeatureDay(featureID int, startTime, endTime int64, limit int) (int, int, int64, error) {
	totalFetched := 0
	totalFailed := 0
	var maxLogTime int64
	startIndex := 0
	savedMobiles := make(map[string]bool)

	for {
		if s.isCancelled() {
			return totalFetched, totalFailed, maxLogTime, nil
		}
		logItems, err := s.weworkSvc.GetLogList(featureID, startTime, endTime, startIndex, limit)
		if err != nil {
			return totalFetched, totalFailed, maxLogTime, fmt.Errorf("fetch log list failed: %w", err)
		}

		if len(logItems) == 0 {
			break
		}

		var entries []model.LogEntry
		pageFailed := 0
		for _, item := range logItems {
			if item.LogTime > maxLogTime {
				maxLogTime = item.LogTime
			}
			entry, err := s.decryptSvc.DecryptWithKey(&model.WeWorkLogItem{
				FeatureID: item.FeatureID,
				LogTime:   item.LogTime,
				IDC:       item.IDC,
				EncKey:    item.EncKey,
				EncData:   item.EncData,
			}, "")
			if err != nil {
				pageFailed++
				totalFailed++
				continue
			}
			entries = append(entries, *entry)
		}

		// 当前页全部解密失败，密钥不匹配，跳过该 feature
		if pageFailed == len(logItems) {
			log.Printf("feature %d: all %d items failed to decrypt (likely missing key), skipping", featureID, len(logItems))
			break
		}

		if len(entries) > 0 {
			if err := s.logRepo.BatchSave(featureID, entries); err != nil {
				log.Printf("batch save failed for feature %d: %v", featureID, err)
				totalFailed += len(entries)
			} else {
				totalFetched += len(entries)
				// 提取已保存条目中的手机号，用于日活跃汇总
				for _, e := range entries {
					if e.ParsedJSON != "" {
						var parsed map[string]interface{}
						if json.Unmarshal([]byte(e.ParsedJSON), &parsed) == nil {
							if lu, ok := parsed["login_user"].(map[string]interface{}); ok {
								if openid, ok := lu["openid"].(string); ok && openid != "" {
									savedMobiles[openid] = true
								}
							}
						}
					}
				}
			}
		}

		if len(logItems) < limit {
			break
		}

		startIndex += len(logItems)
	}

	// 写入日活跃汇总表
	if len(savedMobiles) > 0 {
		s.logRepo.BatchUpsertDailyStats(featureID, savedMobiles, startTime)
	}

	return totalFetched, totalFailed, maxLogTime, nil
}

// SyncFeatureIncremental 增量同步单个 feature，从 sync_state.last_log_time + 1 开始
func (s *SyncService) SyncFeatureIncremental(featureID int) (int, int, error) {
	lastLogTime := s.syncStateRepo.GetLastLogTime(featureID)
	startTime := lastLogTime + 1
	endTime := time.Now().Unix()

	// 首次同步（无历史记录）时拉取最近 30 天，政务微信只保留一个月数据
	if lastLogTime == 0 {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if loc == nil {
			loc = time.FixedZone("CST", 8*3600)
		}
		now := time.Now().In(loc)
		startTime = now.AddDate(0, 0, -30).Unix()
		log.Printf("first sync for feature %d, pulling last 30 days", featureID)
	}

	log.Printf("SyncFeatureIncremental: feature=%d, lastLogTime=%d, startTime=%d, endTime=%d", featureID, lastLogTime, startTime, endTime)

	if startTime > endTime {
		return 0, 0, nil
	}

	fetched, failed, maxLogTime, err := s.SyncFeature(featureID, startTime, endTime)
	if err != nil {
		log.Printf("SyncFeatureIncremental: feature=%d returned error: %v", featureID, err)
	} else if maxLogTime > 0 {
		if updateErr := s.syncStateRepo.UpdateState(featureID, maxLogTime, fetched); updateErr != nil {
			log.Printf("SyncFeatureIncremental: UpdateState failed for feature %d: %v", featureID, updateErr)
		}
	}
	return fetched, failed, err
}

// SyncAllFeaturesIncremental 增量同步所有启用的 feature
func (s *SyncService) SyncAllFeaturesIncremental() map[int]int {
	ids, err := s.syncFeatureRepo.GetEnabledIDs()
	if err != nil {
		log.Printf("get enabled features failed: %v", err)
		return nil
	}
	return s.syncFeaturesIncremental(ids)
}

// SyncMultipleFeaturesIncremental 增量同步指定的 feature 列表
func (s *SyncService) SyncMultipleFeaturesIncremental(featureIDs []int) map[int]int {
	return s.syncFeaturesIncremental(featureIDs)
}

func (s *SyncService) syncFeaturesIncremental(featureIDs []int) map[int]int {
	s.mu.Lock()
	s.status.Total = len(featureIDs)
	s.mu.Unlock()

	startTime := time.Now()
	log.Printf("incremental sync started, %d features to sync", len(featureIDs))

	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.LastSync = time.Now()
		errCount := len(s.status.Errors)
		s.mu.Unlock()
		log.Printf("incremental sync finished, errors=%d", errCount)
		s.saveSyncHistory("log", "scheduler", startTime)
	}()

	results := make(map[int]int)
	for i, featureID := range featureIDs {
		select {
		case <-s.cancelCh:
			log.Printf("incremental sync cancelled at feature %d", featureID)
			return results
		default:
		}

		log.Printf("syncing feature %d (%d/%d)...", featureID, i+1, len(featureIDs))
		s.mu.Lock()
		s.status.CurrentFeature = featureID
		s.mu.Unlock()
		count, failed, err := s.SyncFeatureIncremental(featureID)
		if err != nil {
			log.Printf("incremental sync feature %d failed: %v", featureID, err)
			results[featureID] = -1
			s.mu.Lock()
			s.status.Errors[featureID] = err.Error()
			log.Printf("stored error for feature %d: %s", featureID, s.status.Errors[featureID])
			s.mu.Unlock()
		} else {
			results[featureID] = count
			log.Printf("feature %d synced: %d records", featureID, count)
		}

		s.mu.Lock()
		s.status.Progress = i + 1
		s.status.Results = results
		s.status.Failed += failed
		s.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (s *SyncService) SyncAllFeatures(startTime, endTime int64) map[int]int {
	ids, err := s.syncFeatureRepo.GetEnabledIDs()
	if err != nil {
		log.Printf("get enabled features failed: %v", err)
		return nil
	}
	return s.syncFeatures(ids, startTime, endTime)
}

func (s *SyncService) SyncMultipleFeatures(featureIDs []int, startTime, endTime int64) map[int]int {
	return s.syncFeatures(featureIDs, startTime, endTime)
}

func (s *SyncService) syncFeatures(featureIDs []int, startTime, endTime int64) map[int]int {
	s.mu.Lock()
	s.status.Total = len(featureIDs)
	s.mu.Unlock()

	syncStart := time.Now()
	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.LastSync = time.Now()
		s.mu.Unlock()
		s.saveSyncHistory("log", "manual", syncStart)
	}()

	results := make(map[int]int)
	for i, featureID := range featureIDs {
		select {
		case <-s.cancelCh:
			log.Printf("sync cancelled at feature %d", featureID)
			return results
		default:
		}

		count, failed, maxLogTime, err := s.SyncFeature(featureID, startTime, endTime)
		if err != nil {
			log.Printf("sync feature %d failed: %v", featureID, err)
			results[featureID] = -1
			s.mu.Lock()
			s.status.Errors[featureID] = err.Error()
			s.mu.Unlock()
		} else {
			results[featureID] = count
			if maxLogTime > 0 {
				if updateErr := s.syncStateRepo.UpdateState(featureID, maxLogTime, count); updateErr != nil {
					log.Printf("sync feature %d: UpdateState failed: %v", featureID, updateErr)
				}
			}
		}

		s.mu.Lock()
		s.status.Progress = i + 1
		s.status.Results = results
		s.status.Failed += failed
		s.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (s *SyncService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelCh != nil {
		close(s.cancelCh)
		s.cancelCh = nil // 下次 sync 会重新创建，重复 Cancel 不会 panic
	}
}

func (s *SyncService) isCancelled() bool {
	s.mu.Lock()
	ch := s.cancelCh
	s.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func (s *SyncService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status.Running
}

// TryStartRunning 原子性地检查并设置运行状态。
// 如果当前没有同步在运行，初始化状态并返回 true；否则返回 false。
// 用于防止定时调度和手动同步的竞争条件。
func (s *SyncService) TryStartRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Running {
		return false
	}
	s.status.Running = true
	s.status.Progress = 0
	s.status.Total = 0
	s.status.Results = make(map[int]int)
	s.status.Errors = make(map[int]string)
	s.status.Failed = 0
	s.cancelCh = make(chan struct{})
	return true
}

// ResetRunning 重置运行状态。用于 handler 层 defer 确保即使 panic 后状态也能恢复。
func (s *SyncService) ResetRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Running = false
}

func (s *SyncService) GetStatus() *SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	statusCopy := *s.status
	if s.status.Results != nil {
		statusCopy.Results = make(map[int]int)
		for k, v := range s.status.Results {
			statusCopy.Results[k] = v
		}
	}
	if s.status.Errors != nil {
		statusCopy.Errors = make(map[int]string)
		for k, v := range s.status.Errors {
			statusCopy.Errors[k] = v
		}
	}
	return &statusCopy
}

type SyncStatus struct {
	Running        bool            `json:"running"`
	Progress       int             `json:"progress"`
	Total          int             `json:"total"`
	Failed         int             `json:"failed"`
	CurrentFeature int             `json:"current_feature,omitempty"`
	LastSync       time.Time       `json:"last_sync,omitempty"`
	Results        map[int]int     `json:"results,omitempty"`
	Errors         map[int]string  `json:"errors,omitempty"`
}

func (s *SyncService) saveSyncHistory(syncType, trigger string, startTime time.Time) {
	s.mu.Lock()
	results := make(map[int]int)
	errors := make(map[int]string)
	for k, v := range s.status.Results {
		results[k] = v
	}
	for k, v := range s.status.Errors {
		errors[k] = v
	}
	total := s.status.Total
	failed := s.status.Failed
	s.mu.Unlock()

	succeeded := 0
	for _, count := range results {
		if count > 0 {
			succeeded += count
		}
	}

	detailsMap := map[string]interface{}{
		"results": results,
		"errors":  errors,
	}
	detailsJSON, _ := json.Marshal(detailsMap)

	history := &model.SyncHistory{
		SyncType:   syncType,
		Trigger:    trigger,
		StartTime:  startTime,
		EndTime:    time.Now(),
		DurationMs: time.Since(startTime).Milliseconds(),
		Total:      total,
		Succeeded:  succeeded,
		Failed:     failed,
		Details:    string(detailsJSON),
	}
	if err := s.syncHistoryRepo.Create(history); err != nil {
		log.Printf("save sync history failed: %v", err)
	}
}

// VerifySyncState 校验 sync_state 与数据库实际数据是否一致，修复不一致的状态
func (s *SyncService) VerifySyncState() {
	states, err := s.syncStateRepo.GetAll()
	if err != nil {
		log.Printf("verify sync state: failed to get states: %v", err)
		return
	}

	for _, state := range states {
		actualMax := s.logRepo.GetActualMaxLogTime(state.FeatureID)
		if actualMax > state.LastLogTime {
			log.Printf("verify sync state: feature %d last_log_time corrected from %d to %d",
				state.FeatureID, state.LastLogTime, actualMax)
			if updateErr := s.syncStateRepo.UpdateState(state.FeatureID, actualMax, 0); updateErr != nil {
				log.Printf("verify sync state: UpdateState failed for feature %d: %v", state.FeatureID, updateErr)
			}
		} else if actualMax < state.LastLogTime && actualMax > 0 {
			log.Printf("verify sync state: feature %d last_log_time %d ahead of actual max %d (state is fine, data may have been purged)",
				state.FeatureID, state.LastLogTime, actualMax)
		}
	}
}