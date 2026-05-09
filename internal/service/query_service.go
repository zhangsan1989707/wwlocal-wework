package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type QueryService struct {
	logRepo *repository.LogRepository
}

func NewQueryService(logRepo *repository.LogRepository) *QueryService {
	return &QueryService{logRepo: logRepo}
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
		entries, _, err := s.logRepo.Query(featureID, req.StartTime, req.EndTime, 1, 10000)
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}

		for _, entry := range entries {
			data := s.parseEntry(&entry, req.Conditions)
			if data != nil {
				allData = append(allData, data)
				total++
			}
		}
	}

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
	return []int{
		90000031, 90000032, 90000033, 90000034, 90000035,
		90000036, 90000037, 90000038, 90000039, 90000040,
		90000041, 90000042, 90000043, 90000044, 90000047,
		90000048, 90000054, 90000055, 90000058, 90000059,
		90000061, 90000062, 90000063, 90000066,
	}
}

func (s *QueryService) GetFeatureName(featureID int) string {
	names := map[int]string{
		90000031: "登录",
		90000032: "唤醒",
		90000033: "访问应用",
		90000034: "应用推送消息",
		90000035: "聊天消息发送",
		90000036: "单聊聊天数据",
		90000037: "群聊聊天数据",
		90000038: "创建群聊",
		90000039: "群加人",
		90000040: "群踢人",
		90000041: "退群",
		90000042: "转让群主",
		90000043: "解散群",
		90000044: "群改名",
		90000047: "添加",
		90000048: "激活",
		90000054: "客户端安装信息",
		90000055: "客户端更新信息",
		90000058: "通讯录更新",
		90000059: "客户端网络请求统计",
		90000061: "微盘文件操作",
		90000062: "微盘账号行为",
		90000063: "微盘空间操作",
		90000066: "API接口调用日志",
	}
	if name, ok := names[featureID]; ok {
		return name
	}
	return fmt.Sprintf("未知(%d)", featureID)
}