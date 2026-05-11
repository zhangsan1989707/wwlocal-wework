package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type QueryService struct {
	logRepo *repository.LogRepository
	cfg     *config.Config
}

func NewQueryService(logRepo *repository.LogRepository, cfg *config.Config) *QueryService {
	return &QueryService{logRepo: logRepo, cfg: cfg}
}

type QueryRequest struct {
	FeatureIDs []int                  `json:"feature_ids"`
	StartTime  int64                  `json:"start_time"`
	EndTime    int64                  `json:"end_time"`
	Conditions map[string]interface{} `json:"conditions"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
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
		// 使用数据库分页，每次只取当前页需要的数据
		entries, count, err := s.logRepo.Query(featureID, req.StartTime, req.EndTime, req.Page, req.PageSize)
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
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
		// 注意：有条件过滤时，总数可能不准确，因为过滤是在应用层进行的
		// 这是一个已知的限制，对于精确分页需要在数据库层面实现 JSON 查询
		total = int64(len(allData))
	}

	return &QueryResult{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Data:     allData,
	}, nil
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