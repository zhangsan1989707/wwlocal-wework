package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/metrics"
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
	slog.Info(fmt.Sprintf("SyncFeature: feature=%d, %d days to process", featureID, len(days)))
	for di, day := range days {
		if s.isCancelled() {
			return totalFetched, totalFailed, maxLogTime, nil
		}
		if di%5 == 0 {
			slog.Info(fmt.Sprintf("SyncFeature: feature=%d, day %d/%d (ts=%d)", featureID, di+1, len(days), day.start))
		}
		fetched, failed, dayMax, err := s.syncFeatureDay(nil, featureID, day.start, day.end, limit)
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

func (s *SyncService) syncFeatureDay(tx *gorm.DB, featureID int, startTime, endTime int64, limit int) (int, int, int64, error) {
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

		if pageFailed == len(logItems) {
			slog.Warn(fmt.Sprintf("feature %d: all %d items failed to decrypt (likely missing key), skipping", featureID, len(logItems)))
			break
		}

		if len(entries) > 0 {
			if tx != nil {
				// 事务模式：使用事务写入，失败则回滚当天
				ms, count, err := s.logRepo.BatchSaveWithTx(tx, featureID, entries)
				if err != nil {
					slog.Error(fmt.Sprintf("batch save failed for feature %d: %v", featureID, err))
					totalFailed += len(entries)
					return totalFetched, totalFailed, maxLogTime, err
				}
				totalFetched += count
				if len(ms) > 0 {
					if err := s.logRepo.BatchUpsertDailyStatsWithTx(tx, featureID, ms, startTime); err != nil {
						slog.Error(fmt.Sprintf("upsert daily stats failed for feature %d: %v", featureID, err))
						return totalFetched, totalFailed, maxLogTime, err
					}
				}
			} else {
				// 非事务模式：直接写入，失败仅计数
				if err := s.logRepo.BatchSave(featureID, entries); err != nil {
					slog.Error(fmt.Sprintf("batch save failed for feature %d: %v", featureID, err))
					totalFailed += len(entries)
				} else {
					totalFetched += len(entries)
					for _, e := range entries {
						if e.ParsedJSON != "" {
							var parsed map[string]interface{}
							if json.Unmarshal([]byte(e.ParsedJSON), &parsed) == nil {
								if openid := extractMobile(parsed); openid != "" {
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

	// 非事务模式：最后统一写入日活跃汇总
	if tx == nil && len(savedMobiles) > 0 {
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
		slog.Info(fmt.Sprintf("first sync for feature %d, pulling last 30 days", featureID))
	}

	slog.Info(fmt.Sprintf("SyncFeatureIncremental: feature=%d, lastLogTime=%d, startTime=%d, endTime=%d", featureID, lastLogTime, startTime, endTime))

	if startTime > endTime {
		return 0, 0, nil
	}

	// 按天拆分，政务微信 API 要求 start_time 和 end_time 在同一天
	days := s.splitByDay(startTime, endTime)
	slog.Info(fmt.Sprintf("SyncFeature: feature=%d, %d days to process", featureID, len(days)))

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

	for di, day := range days {
		if s.isCancelled() {
			return totalFetched, totalFailed, nil
		}
		if di%5 == 0 {
			slog.Info(fmt.Sprintf("SyncFeature: feature=%d, day %d/%d (ts=%d)", featureID, di+1, len(days), day.start))
		}

		// 启动事务
		tx := s.syncStateRepo.DB.Begin()
		if tx.Error != nil {
			slog.Error(fmt.Sprintf("SyncFeatureIncremental: failed to begin transaction for feature %d: %v", featureID, tx.Error))
			continue
		}

		// 同步当天数据
		fetched, failed, dayMax, err := s.syncFeatureDay(tx, featureID, day.start, day.end, limit)
		if err != nil {
			tx.Rollback()
			slog.Error(fmt.Sprintf("SyncFeatureIncremental: failed to sync day for feature %d: %v", featureID, err))
			totalFailed += failed
			continue
		}

		// 更新 sync_state
		if dayMax > maxLogTime {
			maxLogTime = dayMax
			if updateErr := s.syncStateRepo.UpdateStateWithTx(tx, featureID, maxLogTime, fetched); updateErr != nil {
				tx.Rollback()
				slog.Error(fmt.Sprintf("SyncFeatureIncremental: UpdateState failed for feature %d: %v", featureID, updateErr))
				continue
			}
		}

		// 提交事务
		if commitErr := tx.Commit().Error; commitErr != nil {
			slog.Error(fmt.Sprintf("SyncFeatureIncremental: commit failed for feature %d: %v", featureID, commitErr))
			continue
		}

		totalFetched += fetched
		totalFailed += failed
	}

	return totalFetched, totalFailed, nil
}

// SyncAllFeaturesIncremental 增量同步所有启用的 feature
func (s *SyncService) SyncAllFeaturesIncremental() map[int]int {
	ids, err := s.syncFeatureRepo.GetEnabledIDs()
	if err != nil {
		slog.Info(fmt.Sprintf("get enabled features failed: %v", err))
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
	slog.Info(fmt.Sprintf("incremental sync started, %d features to sync", len(featureIDs)))

	defer func() {
		s.mu.Lock()
		s.status.LastSync = time.Now()
		errCount := len(s.status.Errors)
		s.mu.Unlock()
		slog.Info(fmt.Sprintf("incremental sync finished, errors=%d", errCount))
		s.saveSyncHistory("log", "scheduler", startTime)
	}()

	results := make(map[int]int)
	var resultsMu sync.Mutex
	var progress int

	sem := make(chan struct{}, 3) // 并发度 3
	var wg sync.WaitGroup

	for _, featureID := range featureIDs {
		// 检查取消
		select {
		case <-s.cancelCh:
			slog.Info(fmt.Sprintf("incremental sync cancelled"))
			wg.Wait()
			return results
		default:
		}

		sem <- struct{}{} // 获取信号量
		wg.Add(1)
		go func(fid int) {
			defer wg.Done()
			defer func() { <-sem }()

			slog.Info(fmt.Sprintf("syncing feature %d...", fid))
			s.mu.Lock()
			s.status.CurrentFeature = fid
			s.mu.Unlock()

			featureStart := time.Now()
			count, failed, err := s.SyncFeatureIncremental(fid)
			fidStr := strconv.Itoa(fid)

			if err != nil {
				slog.Error(fmt.Sprintf("incremental sync feature %d failed: %v", fid, err))
				metrics.RecordSyncOperation(fidStr, "failure", time.Since(featureStart), 0)
				s.mu.Lock()
				s.status.Errors[fid] = err.Error()
				s.mu.Unlock()
			} else {
				metrics.RecordSyncOperation(fidStr, "success", time.Since(featureStart), count)
				slog.Info(fmt.Sprintf("feature %d synced: %d records", fid, count))
			}

			resultsMu.Lock()
			if err != nil {
				results[fid] = -1
			} else {
				results[fid] = count
			}
			resultsMu.Unlock()

			s.mu.Lock()
			progress++
			s.status.Progress = progress
			s.status.Results = results
			s.status.Failed += failed
			s.mu.Unlock()
		}(featureID)
	}

	wg.Wait()
	return results
}

func (s *SyncService) SyncAllFeatures(startTime, endTime int64) map[int]int {
	ids, err := s.syncFeatureRepo.GetEnabledIDs()
	if err != nil {
		slog.Info(fmt.Sprintf("get enabled features failed: %v", err))
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
		s.status.LastSync = time.Now()
		s.mu.Unlock()
		s.saveSyncHistory("log", "manual", syncStart)
	}()

	results := make(map[int]int)
	var resultsMu sync.Mutex
	var progress int

	sem := make(chan struct{}, 3) // 并发度 3
	var wg sync.WaitGroup

	for _, featureID := range featureIDs {
		select {
		case <-s.cancelCh:
			slog.Info(fmt.Sprintf("sync cancelled"))
			wg.Wait()
			return results
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(fid int) {
			defer wg.Done()
			defer func() { <-sem }()

			featureStart := time.Now()
			count, failed, maxLogTime, err := s.SyncFeature(fid, startTime, endTime)
			fidStr := strconv.Itoa(fid)

			if err != nil {
				slog.Error(fmt.Sprintf("sync feature %d failed: %v", fid, err))
				metrics.RecordSyncOperation(fidStr, "failure", time.Since(featureStart), 0)
				s.mu.Lock()
				s.status.Errors[fid] = err.Error()
				s.mu.Unlock()
			} else {
				metrics.RecordSyncOperation(fidStr, "success", time.Since(featureStart), count)
				if maxLogTime > 0 {
					if updateErr := s.syncStateRepo.UpdateState(fid, maxLogTime, count); updateErr != nil {
						slog.Error(fmt.Sprintf("sync feature %d: UpdateState failed: %v", fid, updateErr))
					}
				}
			}

			resultsMu.Lock()
			if err != nil {
				results[fid] = -1
			} else {
				results[fid] = count
			}
			resultsMu.Unlock()

			s.mu.Lock()
			progress++
			s.status.Progress = progress
			s.status.Results = results
			s.status.Failed += failed
			s.mu.Unlock()
		}(featureID)
	}

	wg.Wait()
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

// StartSync 在后台 goroutine 中启动同步，自动处理 TryStartRunning、panic 恢复和 ResetRunning。
func (s *SyncService) StartSync(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(fmt.Sprintf("sync goroutine panic: %v\n%s", r, debug.Stack()))
			}
			s.ResetRunning()
		}()
		if !s.TryStartRunning() {
			return
		}
		fn()
	}()
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
		slog.Error(fmt.Sprintf("save sync history failed: %v", err))
	}
}

// VerifySyncState 校验 sync_state 与数据库实际数据是否一致，修复不一致的状态
func (s *SyncService) VerifySyncState() {
	states, err := s.syncStateRepo.GetAll()
	if err != nil {
		slog.Error(fmt.Sprintf("verify sync state: failed to get states: %v", err))
		return
	}

	for _, state := range states {
		actualMax := s.logRepo.GetActualMaxLogTime(state.FeatureID)
		if actualMax > state.LastLogTime {
			slog.Info(fmt.Sprintf("verify sync state: feature %d last_log_time corrected from %d to %d",
				state.FeatureID, state.LastLogTime, actualMax))
			if updateErr := s.syncStateRepo.UpdateState(state.FeatureID, actualMax, 0); updateErr != nil {
				slog.Error(fmt.Sprintf("verify sync state: UpdateState failed for feature %d: %v", state.FeatureID, updateErr))
			}
		} else if actualMax < state.LastLogTime && actualMax > 0 {
			slog.Info(fmt.Sprintf("verify sync state: feature %d last_log_time %d ahead of actual max %d (state is fine, data may have been purged)",
				state.FeatureID, state.LastLogTime, actualMax))
		}
	}
}

// SyncFeaturesTask 处理来自队列的任务
func (s *SyncService) SyncFeaturesTask(task *model.SyncTask) (map[string]interface{}, error) {
	if len(task.FeatureIDs) == 0 {
		// 默认同步所有启用的 feature
		ids, err := s.syncFeatureRepo.GetEnabledIDs()
		if err != nil {
			return nil, err
		}
		task.FeatureIDs = ids
	}

	results := s.SyncMultipleFeatures(task.FeatureIDs, task.StartTime, task.EndTime)
	return map[string]interface{}{
		"results": results,
	}, nil
}

// extractMobile 从 parsed JSON 中提取用户标识（openid），尝试多个常见路径
func extractMobile(parsed map[string]interface{}) string {
	// 路径1: login_user.openid（登录/唤醒/访问应用等）
	if lu, ok := parsed["login_user"].(map[string]interface{}); ok {
		if openid, ok := lu["openid"].(string); ok && openid != "" {
			return openid
		}
	}
	// 路径2: sender.openid（单聊/群聊消息）
	if sender, ok := parsed["sender"].(map[string]interface{}); ok {
		if openid, ok := sender["openid"].(string); ok && openid != "" {
			return openid
		}
	}
	// 路径3: 根级 openid
	if openid, ok := parsed["openid"].(string); ok && openid != "" {
		return openid
	}
	return ""
}