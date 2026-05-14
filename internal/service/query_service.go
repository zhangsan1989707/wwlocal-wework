package service

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type QueryService struct {
	logRepo         *repository.LogRepository
	contactRepo     *repository.ContactRepository
	weworkSvc       *WeWorkService
	decryptSvc      *DecryptService
	syncFeatureRepo *repository.SyncFeatureRepository
	cfg             *config.Config
}

func NewQueryService(logRepo *repository.LogRepository, contactRepo *repository.ContactRepository, weworkSvc *WeWorkService, decryptSvc *DecryptService, syncFeatureRepo *repository.SyncFeatureRepository, cfg *config.Config) *QueryService {
	return &QueryService{logRepo: logRepo, contactRepo: contactRepo, weworkSvc: weworkSvc, decryptSvc: decryptSvc, syncFeatureRepo: syncFeatureRepo, cfg: cfg}
}

type QueryRequest struct {
	FeatureIDs []int                  `json:"feature_ids"`
	StartTime  int64                  `json:"start_time"`
	EndTime    int64                  `json:"end_time"`
	Conditions map[string]interface{} `json:"conditions"`
	Mobile     string                 `json:"mobile"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	Realtime   bool                   `json:"realtime"`
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

	hasConditions := len(req.Conditions) > 0 || req.Mobile != ""

	var allData []map[string]interface{}
	var total int64

	for _, featureID := range req.FeatureIDs {
		var entries []model.LogEntry
		var count int64
		var err error

		if hasConditions {
			entries, count, err = s.logRepo.QueryAcrossMonthsWithConditions(featureID, req.StartTime, req.EndTime, req.Conditions, req.Mobile, req.Page, req.PageSize)
		} else {
			entries, count, err = s.logRepo.QueryAcrossMonths(featureID, req.StartTime, req.EndTime, req.Page, req.PageSize)
		}
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}

		if count == 0 && req.Realtime {
			log.Printf("No data in database for feature %d, querying realtime from API", featureID)
			realtimeEntries, err := s.queryRealtime(featureID, req.StartTime, req.EndTime)
			if err != nil {
				log.Printf("Realtime query failed for feature %d: %v", featureID, err)
			} else {
				entries = realtimeEntries
				count = int64(len(realtimeEntries))
			}
		}

		total += count

		for _, entry := range entries {
			allData = append(allData, s.parseEntry(&entry))
		}
	}

	sort.Slice(allData, func(i, j int) bool {
		ti, _ := allData[i]["log_time"].(int64)
		tj, _ := allData[j]["log_time"].(int64)
		return ti > tj
	})

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

func (s *QueryService) parseEntry(entry *model.LogEntry) map[string]interface{} {
	result := map[string]interface{}{
		"id":         entry.ID,
		"feature_id": entry.FeatureID,
		"log_time":   entry.LogTime,
		"log_date":   time.Unix(entry.LogTime, 0).Format("2006-01-02 15:04:05"),
		"idc":        entry.IDC,
	}

	if entry.ParsedJSON == "" {
		result["_decrypt_failed"] = true
		return result
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(entry.ParsedJSON), &parsed); err != nil {
		result["_decrypt_failed"] = true
		return result
	}

	for k, v := range parsed {
		result[k] = v
	}

	return result
}

func (s *QueryService) GetFeatureIDs() []int {
	ids, err := s.syncFeatureRepo.GetEnabledIDs()
	if err != nil {
		log.Printf("get feature ids failed: %v", err)
		return s.cfg.Features.IDs
	}
	if len(ids) == 0 {
		return s.cfg.Features.IDs
	}
	return ids
}

func (s *QueryService) GetFeatureName(featureID int) string {
	if name, ok := s.cfg.Features.Names[featureID]; ok {
		return name
	}
	return fmt.Sprintf("未知(%d)", featureID)
}

func (s *QueryService) GetFieldPaths() []string {
	samples := s.logRepo.SampleParsedJSON(s.GetFeatureIDs(), 20)
	pathSet := make(map[string]struct{})
	for _, sample := range samples {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(sample), &data); err != nil {
			continue
		}
		extractPaths(data, "", pathSet)
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func extractPaths(data map[string]interface{}, prefix string, result map[string]struct{}) {
	for key, val := range data {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		result[path] = struct{}{}
		if nested, ok := val.(map[string]interface{}); ok {
			extractPaths(nested, path, result)
		}
	}
}
