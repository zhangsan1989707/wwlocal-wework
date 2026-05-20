package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
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

func (s *QueryService) Query(req *model.QueryRequest) (*model.QueryResult, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	// 限制最大时间跨度 90 天，防止内存溢出
	const maxRangeSeconds = 90 * 24 * 3600
	if req.EndTime-req.StartTime > int64(maxRangeSeconds) {
		return nil, fmt.Errorf("时间范围不能超过 90 天")
	}
	if len(req.FeatureIDs) > 10 {
		return nil, fmt.Errorf("最多同时查询 10 个数据类型")
	}

	hasConditions := len(req.Conditions) > 0 || req.Mobile != ""

	perPage := req.PageSize
	if len(req.FeatureIDs) > 1 {
		perPage = req.Page * req.PageSize
		if perPage > 5000 {
			perPage = 5000
		}
	}

	var allData []map[string]interface{}
	var total int64

	for _, featureID := range req.FeatureIDs {
		var entries []model.LogEntry
		var count int64
		var err error

		if hasConditions {
			entries, count, err = s.logRepo.QueryAcrossMonthsWithConditions(featureID, req.StartTime, req.EndTime, req.Conditions, req.Mobile, 1, perPage)
		} else {
			entries, count, err = s.logRepo.QueryAcrossMonths(featureID, req.StartTime, req.EndTime, 1, perPage)
		}
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}

		if count == 0 && req.Realtime {
			slog.Info(fmt.Sprintf("No data in database for feature %d, querying realtime from API", featureID))
			realtimeEntries, err := s.queryRealtime(featureID, req.StartTime, req.EndTime)
			if err != nil {
				slog.Info(fmt.Sprintf("Realtime query failed for feature %d: %v", featureID, err))
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

	return &model.QueryResult{
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
			slog.Info(fmt.Sprintf("decrypt failed for feature %d, log_time %d: %v", featureID, item.LogTime, err))
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
		slog.Info(fmt.Sprintf("get feature ids failed: %v", err))
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

// QueryByCursor 使用游标查询，更高效的分页方式
func (s *QueryService) QueryByCursor(req *model.QueryRequest) (*model.CursorQueryResult, error) {
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	// 限制最大时间跨度 90 天，防止内存溢出
	const maxRangeSeconds = 90 * 24 * 3600
	if req.EndTime-req.StartTime > int64(maxRangeSeconds) {
		return nil, fmt.Errorf("time range cannot exceed 90 days")
	}
	if len(req.FeatureIDs) > 10 {
		return nil, fmt.Errorf("cannot query more than 10 feature types at the same time")
	}

	var allData []map[string]interface{}
	var total int64
	var minCursor int64 = 0

	// 使用游标查询每个 feature
	for _, featureID := range req.FeatureIDs {
		entries, count, nextCursor, err := s.logRepo.QueryByCursor(
			featureID,
			req.StartTime,
			req.EndTime,
			req.Cursor,
			req.PageSize,
			req.Conditions,
			req.Mobile,
		)
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}
		total += count
		
		// 解析数据
		for _, entry := range entries {
			allData = append(allData, s.parseEntry(&entry))
		}
		
		// 找到最小的 cursor 作为下一页的 cursor
		if nextCursor > 0 && (minCursor == 0 || nextCursor < minCursor) {
			minCursor = nextCursor
		}
	}

	// 所有 feature 都没有更多数据了
	if len(allData) == 0 {
		return &model.CursorQueryResult{
			Total:  total,
			Cursor: 0,
			Data:   allData,
		}, nil
	}

	// 在合并后的数据上进行排序（仅在当前页，避免全量排序）
	sort.Slice(allData, func(i, j int) bool {
		ti, _ := allData[i]["log_time"].(int64)
		tj, _ := allData[j]["log_time"].(int64)
		return ti > tj
	})

	// 截断到正确的页面大小
	if len(allData) > req.PageSize {
		allData = allData[:req.PageSize]
		// 更新 cursor 为最后一个元素的 log_time
		if len(allData) > 0 {
			minCursor = allData[len(allData)-1]["log_time"].(int64)
		}
	}

	// 如果数据量少于页面大小，说明没有更多数据了
	if len(allData) < req.PageSize {
		minCursor = 0
	}

	return &model.CursorQueryResult{
		Total:  total,
		Cursor: minCursor,
		Data:   allData,
	}, nil
}

func (s *QueryService) ExportCSV(req *model.QueryRequest) ([]map[string]interface{}, error) {
	if req.PageSize <= 0 {
		req.PageSize = 50000
	}
	if req.PageSize > 50000 {
		req.PageSize = 50000
	}

	const maxRangeSeconds = 90 * 24 * 3600
	if req.EndTime-req.StartTime > int64(maxRangeSeconds) {
		return nil, fmt.Errorf("time range cannot exceed 90 days")
	}
	if len(req.FeatureIDs) > 10 {
		return nil, fmt.Errorf("cannot query more than 10 feature types at the same time")
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	var allData []map[string]interface{}

	for _, featureID := range req.FeatureIDs {
		entries, _, err := s.logRepo.QueryAcrossMonths(featureID, req.StartTime, req.EndTime, req.Page, req.PageSize)
		if err != nil {
			return nil, fmt.Errorf("query feature %d failed: %w", featureID, err)
		}
		for _, entry := range entries {
			allData = append(allData, s.parseEntry(&entry))
		}
	}

	sort.Slice(allData, func(i, j int) bool {
		ti, _ := allData[i]["log_time"].(int64)
		tj, _ := allData[j]["log_time"].(int64)
		return ti > tj
	})

	return allData, nil
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
