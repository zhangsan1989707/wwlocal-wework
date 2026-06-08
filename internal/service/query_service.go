package service

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

var ErrQueryTimeout = errors.New("query timeout")
var ErrQueryCanceled = errors.New("query canceled")

const exportBatchSize = 1000

type QueryService struct {
	logRepo         logQueryRepository
	contactRepo     *repository.ContactRepository
	weworkSvc       *WeWorkService
	decryptSvc      *DecryptService
	syncFeatureRepo *repository.SyncFeatureRepository
	cfg             *config.Config
}

type logQueryRepository interface {
	QueryAcrossMonthsContext(ctx context.Context, featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error)
	QueryAcrossMonthsWithConditionsContext(ctx context.Context, featureID int, startTime, endTime int64, conditions map[string]interface{}, mobile string, page, pageSize int) ([]model.LogEntry, int64, error)
	QueryByCursorContext(ctx context.Context, featureID int, startTime, endTime int64, cursor int64, pageSize int, conditions map[string]interface{}, mobile string) ([]model.LogEntry, int64, int64, error)
	SampleParsedJSON(featureIDs []int, limit int) []string
}

func NewQueryService(logRepo *repository.LogRepository, contactRepo *repository.ContactRepository, weworkSvc *WeWorkService, decryptSvc *DecryptService, syncFeatureRepo *repository.SyncFeatureRepository, cfg *config.Config) *QueryService {
	return &QueryService{logRepo: logRepo, contactRepo: contactRepo, weworkSvc: weworkSvc, decryptSvc: decryptSvc, syncFeatureRepo: syncFeatureRepo, cfg: cfg}
}

func (s *QueryService) Query(req *model.QueryRequest) (*model.QueryResult, error) {
	return s.QueryContext(context.Background(), req)
}

func (s *QueryService) QueryContext(ctx context.Context, req *model.QueryRequest) (*model.QueryResult, error) {
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
		if err := queryContextError(ctx); err != nil {
			return nil, err
		}
		var entries []model.LogEntry
		var count int64
		var err error

		if hasConditions {
			entries, count, err = s.logRepo.QueryAcrossMonthsWithConditionsContext(ctx, featureID, req.StartTime, req.EndTime, req.Conditions, req.Mobile, 1, perPage)
		} else {
			entries, count, err = s.logRepo.QueryAcrossMonthsContext(ctx, featureID, req.StartTime, req.EndTime, 1, perPage)
		}
		if err != nil {
			if ctxErr := queryContextError(ctx); ctxErr != nil {
				return nil, ctxErr
			}
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
	return s.QueryByCursorContext(context.Background(), req)
}

func (s *QueryService) QueryByCursorContext(ctx context.Context, req *model.QueryRequest) (*model.CursorQueryResult, error) {
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
		if err := queryContextError(ctx); err != nil {
			return nil, err
		}
		entries, count, nextCursor, err := s.logRepo.QueryByCursorContext(
			ctx,
			featureID,
			req.StartTime,
			req.EndTime,
			req.Cursor,
			req.PageSize,
			req.Conditions,
			req.Mobile,
		)
		if err != nil {
			if ctxErr := queryContextError(ctx); ctxErr != nil {
				return nil, ctxErr
			}
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
	return s.ExportCSVContext(context.Background(), req)
}

func (s *QueryService) PrepareExportCSV(req *model.QueryRequest) error {
	return s.prepareExportRequest(req)
}

func (s *QueryService) prepareExportRequest(req *model.QueryRequest) error {
	if req.PageSize <= 0 {
		req.PageSize = 50000
	}
	if req.PageSize > 50000 {
		req.PageSize = 50000
	}

	const maxRangeSeconds = 90 * 24 * 3600
	if req.EndTime-req.StartTime > int64(maxRangeSeconds) {
		return fmt.Errorf("time range cannot exceed 90 days")
	}
	if len(req.FeatureIDs) > 10 {
		return fmt.Errorf("cannot query more than 10 feature types at the same time")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	return nil
}

func (s *QueryService) ExportCSVContext(ctx context.Context, req *model.QueryRequest) ([]map[string]interface{}, error) {
	if err := s.prepareExportRequest(req); err != nil {
		return nil, err
	}

	var allData []map[string]interface{}

	for _, featureID := range req.FeatureIDs {
		if err := queryContextError(ctx); err != nil {
			return nil, err
		}
		entries, _, err := s.logRepo.QueryAcrossMonthsWithConditionsContext(ctx, featureID, req.StartTime, req.EndTime, req.Conditions, req.Mobile, req.Page, req.PageSize)
		if err != nil {
			if ctxErr := queryContextError(ctx); ctxErr != nil {
				return nil, ctxErr
			}
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

func (s *QueryService) ExportCSVStreamContext(ctx context.Context, req *model.QueryRequest, writeRow func(map[string]interface{}) error) error {
	if writeRow == nil {
		return fmt.Errorf("writeRow is required")
	}
	if err := s.prepareExportRequest(req); err != nil {
		return err
	}

	streams := make([]exportFeatureStream, 0, len(req.FeatureIDs))
	items := exportEntryHeap{}
	for _, featureID := range req.FeatureIDs {
		offset := (req.Page - 1) * req.PageSize
		stream := exportFeatureStream{
			featureID: featureID,
			page:      offset/exportBatchSize + 1,
			skip:      offset % exportBatchSize,
		}
		streams = append(streams, stream)
		index := len(streams) - 1
		entry, err := s.nextExportEntry(ctx, req, &streams[index])
		if err != nil {
			return err
		}
		if entry != nil {
			heap.Push(&items, exportEntryItem{streamIndex: index, entry: *entry})
		}
	}

	for items.Len() > 0 {
		item := heap.Pop(&items).(exportEntryItem)
		if err := writeRow(s.parseEntry(&item.entry)); err != nil {
			return err
		}
		entry, err := s.nextExportEntry(ctx, req, &streams[item.streamIndex])
		if err != nil {
			return err
		}
		if entry != nil {
			heap.Push(&items, exportEntryItem{streamIndex: item.streamIndex, entry: *entry})
		}
	}

	return nil
}

type exportFeatureStream struct {
	featureID int
	page      int
	skip      int
	written   int
	done      bool
	buffer    []model.LogEntry
	pos       int
}

func (s *QueryService) nextExportEntry(ctx context.Context, req *model.QueryRequest, stream *exportFeatureStream) (*model.LogEntry, error) {
	if stream.written >= req.PageSize {
		return nil, nil
	}
	for stream.pos >= len(stream.buffer) {
		if stream.done {
			return nil, nil
		}
		if err := queryContextError(ctx); err != nil {
			return nil, err
		}
		entries, _, err := s.logRepo.QueryAcrossMonthsWithConditionsContext(ctx, stream.featureID, req.StartTime, req.EndTime, req.Conditions, req.Mobile, stream.page, exportBatchSize)
		if err != nil {
			if ctxErr := queryContextError(ctx); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("query feature %d failed: %w", stream.featureID, err)
		}
		stream.page++
		if len(entries) < exportBatchSize {
			stream.done = true
		}
		if stream.skip > 0 {
			if stream.skip >= len(entries) {
				stream.skip -= len(entries)
				continue
			}
			entries = entries[stream.skip:]
			stream.skip = 0
		}
		stream.buffer = entries
		stream.pos = 0
		if len(stream.buffer) == 0 && stream.done {
			return nil, nil
		}
	}

	entry := stream.buffer[stream.pos]
	stream.pos++
	stream.written++
	return &entry, nil
}

type exportEntryItem struct {
	streamIndex int
	entry       model.LogEntry
}

type exportEntryHeap []exportEntryItem

func (h exportEntryHeap) Len() int { return len(h) }

func (h exportEntryHeap) Less(i, j int) bool {
	if h[i].entry.LogTime == h[j].entry.LogTime {
		return h[i].entry.ID > h[j].entry.ID
	}
	return h[i].entry.LogTime > h[j].entry.LogTime
}

func (h exportEntryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *exportEntryHeap) Push(x interface{}) {
	*h = append(*h, x.(exportEntryItem))
}

func (h *exportEntryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func queryContextError(ctx context.Context) error {
	switch err := ctx.Err(); {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return ErrQueryTimeout
	case errors.Is(err, context.Canceled):
		return ErrQueryCanceled
	default:
		return err
	}
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
