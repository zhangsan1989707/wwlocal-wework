package service

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type QueryService struct {
	logRepo    *repository.LogRepository
	weworkSvc  *WeWorkService
	decryptSvc *DecryptService
	cfg        *config.Config
}

func NewQueryService(logRepo *repository.LogRepository, weworkSvc *WeWorkService, decryptSvc *DecryptService, cfg *config.Config) *QueryService {
	return &QueryService{logRepo: logRepo, weworkSvc: weworkSvc, decryptSvc: decryptSvc, cfg: cfg}
}

type QueryRequest struct {
	FeatureIDs []int                  `json:"feature_ids"`
	StartTime  int64                  `json:"start_time"`
	EndTime    int64                  `json:"end_time"`
	Conditions map[string]interface{} `json:"conditions"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	Realtime   bool                   `json:"realtime"` // 是否实时查询
}

type QueryResult struct {
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Data     []map[string]interface{} `json:"data"`
}

func (s *QueryService) Query(req *QueryRequest) (*QueryResult, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	var allData []map[string]interface{}
	var total int64

	for _, featureID := range req.FeatureIDs {
		// 先从数据库查询
		entries, count, err := s.logRepo.Query(featureID, req.StartTime, req.EndTime, req.Page, req.PageSize)
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}

		// 如果数据库中没有数据且启用了实时查询，从政务微信 API 查询
		if count == 0 && req.Realtime {
			log.Printf("No data in database for feature %d, querying realtime from API", featureID)
			realtimeEntries, err := s.queryRealtime(featureID, req.StartTime, req.EndTime)
			if err != nil {
				log.Printf("Realtime query failed for feature %d: %v", featureID, err)
				// 实时查询失败不影响整体查询，继续处理其他 feature
			} else {
				entries = realtimeEntries
				count = int64(len(realtimeEntries))
			}
		}

		total += count

		for _, entry := range entries {
			data := s.parseEntry(&entry, req.Conditions)
			if data != nil {
				allData = append(allData, data)
			}
		}
	}

	// 如果有条件过滤，需要重新计算总数和分页
	if req.Conditions != nil && len(req.Conditions) > 0 {
		filtered := make([]map[string]interface{}, 0)
		for _, data := range allData {
			if s.matchConditions(data, req.Conditions) {
				filtered = append(filtered, data)
			}
		}
		allData = filtered
		total = int64(len(allData))
	}

	// 应用分页
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > len(allData) {
		start = len(allData)
	}
	if end > len(allData) {
		end = len(allData)
	}
	pageData := allData[start:end]

	return &QueryResult{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Data:     pageData,
	}, nil
}

func (s *QueryService) queryRealtime(featureID int, startTime, endTime int64) ([]model.LogEntry, error) {
	// 从政务微信 API 获取数据
	logItems, err := s.weworkSvc.GetLogList(featureID, startTime, endTime, 0, 100)
	if err != nil {
		return nil, fmt.Errorf("fetch from API failed: %w", err)
	}

	var entries []model.LogEntry
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
		entries = append(entries, *entry)
	}

	return entries, nil
}

func (s *QueryService) parseEntry(entry *model.LogEntry, conditions map[string]interface{}) map[string]interface{} {
	if entry.ParsedJSON == "" {
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(entry.ParsedJSON), &parsed); err != nil {
		return nil
	}

	result := map[string]interface{}{
		"id":         entry.ID,
		"feature_id": entry.FeatureID,
		"log_time":   entry.LogTime,
		"log_date":   time.Unix(entry.LogTime, 0).Format("2006-01-02 15:04:05"),
		"idc":        entry.IDC,
	}

	for k, v := range parsed {
		result[k] = v
	}

	return result
}

func (s *QueryService) matchConditions(data map[string]interface{}, conditions map[string]interface{}) bool {
	for key, expected := range conditions {
		if !s.matchField(data, key, expected) {
			return false
		}
	}
	return true
}

func (s *QueryService) matchField(data map[string]interface{}, key string, expected interface{}) bool {
	value, ok := data[key]
	if !ok {
		return false
	}

	switch exp := expected.(type) {
	case string:
		valStr, valOk := value.(string)
		if !valOk {
			return false
		}
		if strings.Contains(strings.ToLower(valStr), strings.ToLower(exp)) {
			return true
		}
	case int, int64, float64:
		return reflect.DeepEqual(value, exp)
	default:
		return value == expected
	}

	return false
}

func (s *QueryService) GetFeatureIDs() []int {
	return s.cfg.Features.IDs
}

func (s *QueryService) GetFeatureName(featureID int) string {
	if name, ok := s.cfg.Features.Names[featureID]; ok {
		return name
	}
	return fmt.Sprintf("未知(%d)", featureID)
}