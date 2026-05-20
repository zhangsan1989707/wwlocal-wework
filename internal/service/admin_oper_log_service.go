package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type AdminOperLogService struct {
	weworkSvc    *WeWorkService
	adminLogRepo *repository.AdminOperLogRepository
	cfg          *config.WeWorkConfig
}

func NewAdminOperLogService(weworkSvc *WeWorkService, adminLogRepo *repository.AdminOperLogRepository, cfg *config.WeWorkConfig) *AdminOperLogService {
	return &AdminOperLogService{
		weworkSvc:    weworkSvc,
		adminLogRepo: adminLogRepo,
		cfg:          cfg,
	}
}

func (s *AdminOperLogService) SyncLogs(startTime, endTime int64) (int, error) {
	token, err := s.weworkSvc.GetToken()
	if err != nil {
		return 0, fmt.Errorf("get token failed: %w", err)
	}

	limit := 1000
	start := 0
	totalSynced := 0

	for {
		apiLogs, nextStart, err := s.fetchLogsFromAPI(token, startTime, endTime, start, limit)
		if err != nil {
			return totalSynced, fmt.Errorf("fetch logs failed: %w", err)
		}

		if len(apiLogs) == 0 {
			break
		}

		var logs []model.AdminOperLog

		// 批量检查已存在的记录，避免 N+1 查询
		existing, err := s.adminLogRepo.BatchExistByOperTimeAndUserIDs(apiLogs)
		if err != nil {
			slog.Warn(fmt.Sprintf("batch check exists failed: %v, falling back to insert-ignore", err))
			existing = make(map[[2]string]bool)
		}

		for _, apiLog := range apiLogs {
			key := [2]string{fmt.Sprint(apiLog.OperTime), apiLog.OperUserID}
			if existing[key] {
				continue
			}

			var operDesc, appID string
			if apiLog.OperData != "" {
				var operData model.OperData
				if err := json.Unmarshal([]byte(apiLog.OperData), &operData); err == nil {
					operDesc = operData.Content
					appID = operData.AppID
				}
			}

			logs = append(logs, model.AdminOperLog{
				OperTime:   apiLog.OperTime,
				OperTypeID: apiLog.OperTypeID,
				OperType:   apiLog.OperType,
				OperUserID: apiLog.OperUserID,
				OperName:   apiLog.OperName,
				OperData:   apiLog.OperData,
				OperDesc:   operDesc,
				AppID:      appID,
			})
		}

		if len(logs) > 0 {
			if err := s.adminLogRepo.BatchSave(logs); err != nil {
				slog.Error(fmt.Sprintf("batch save admin oper logs failed: %v", err))
				return totalSynced, fmt.Errorf("batch save failed: %w", err)
			}
			totalSynced += len(logs)
			slog.Info(fmt.Sprintf("synced %d admin oper logs (total: %d)", len(logs), totalSynced))
		}

		if nextStart == 0 {
			break
		}
		start = nextStart
		time.Sleep(100 * time.Millisecond)
	}

	return totalSynced, nil
}

func (s *AdminOperLogService) fetchLogsFromAPI(token string, startTime, endTime int64, start, limit int) ([]model.AdminOperLogAPI, int, error) {
	path := "/cgi-bin/corp/get_admin_oper_log"
	reqBody := map[string]interface{}{
		"start_time": startTime,
		"end_time":  endTime,
		"start":     start,
		"limit":     limit,
	}

	resp, err := s.weworkSvc.DoRequest("POST", path, reqBody, token)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		ErrCode    int                       `json:"errcode"`
		ErrMsg     string                    `json:"errmsg"`
		OperList   []model.AdminOperLogAPI   `json:"oper_list"`
		NextStart  int                       `json:"next_start"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, 0, fmt.Errorf("parse response failed: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, 0, fmt.Errorf("API error: %s (errcode: %d)", result.ErrMsg, result.ErrCode)
	}

	return result.OperList, result.NextStart, nil
}

func (s *AdminOperLogService) SyncIncremental() (int, error) {
	latestTime := s.adminLogRepo.GetLatestOperTime()
	if latestTime == 0 {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if loc == nil {
			loc = time.FixedZone("CST", 8*3600)
		}
		now := time.Now().In(loc)
		latestTime = now.AddDate(0, 0, -7).Unix()
		slog.Info(fmt.Sprintf("first sync admin oper logs, pulling last 7 days"))
	} else {
		latestTime = latestTime + 1
	}

	endTime := time.Now().Unix()
	slog.Info(fmt.Sprintf("SyncIncremental: startTime=%d, endTime=%d", latestTime, endTime))

	return s.SyncLogs(latestTime, endTime)
}

func (s *AdminOperLogService) Query(operType, operUserID string, startTime, endTime int64, page, pageSize int) ([]model.AdminOperLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.adminLogRepo.List(operType, operUserID, startTime, endTime, page, pageSize)
}

func (s *AdminOperLogService) GetStats(startTime, endTime int64) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	byType, err := s.adminLogRepo.GetOperStatsByType(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get stats by type failed: %w", err)
	}
	stats["by_type"] = byType

	byUser, err := s.adminLogRepo.GetOperStatsByUser(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get stats by user failed: %w", err)
	}
	stats["by_user"] = byUser

	dailyStats, err := s.adminLogRepo.GetDailyStats(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get daily stats failed: %w", err)
	}
	stats["daily"] = dailyStats

	total, err := s.adminLogRepo.Count("", "", startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get total count failed: %w", err)
	}
	stats["total"] = total

	return stats, nil
}

func (s *AdminOperLogService) GetOperTypes() ([]string, error) {
	return s.adminLogRepo.GetDistinctOperTypes()
}

func (s *AdminOperLogService) GetOperUsers() ([]string, error) {
	return s.adminLogRepo.GetDistinctOperUsers()
}

func (s *AdminOperLogService) Cleanup(beforeDays int) (int64, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	beforeTime := time.Now().In(loc).AddDate(0, 0, -beforeDays).Unix()
	return s.adminLogRepo.DeleteBefore(beforeTime)
}

func (s *AdminOperLogService) GetStatus() (bool, int64, string, error) {
	running := false
	total, err := s.adminLogRepo.Count("", "", 0, 0)
	if err != nil {
		return false, 0, "", err
	}

	latestTime := s.adminLogRepo.GetLatestOperTime()
	lastTime := ""
	if latestTime > 0 {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if loc == nil {
			loc = time.FixedZone("CST", 8*3600)
		}
		lastTime = time.Unix(latestTime, 0).In(loc).Format(time.RFC3339)
	}

	return running, total, lastTime, nil
}

// SyncAdminLogsTask 处理来自队列的任务
func (s *AdminOperLogService) SyncAdminLogsTask(task *model.SyncTask) (map[string]interface{}, error) {
	var synced int
	var err error

	if task.StartTime > 0 && task.EndTime > 0 {
		synced, err = s.SyncLogs(task.StartTime, task.EndTime)
	} else {
		synced, err = s.SyncIncremental()
	}

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"synced": synced,
	}, nil
}
