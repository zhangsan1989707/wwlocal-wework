package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
)

func newJSONContext(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

type fakeLogQueryService struct {
	page     int
	pageSize int
}

func (f *fakeLogQueryService) QueryContext(ctx context.Context, req *model.QueryRequest) (*model.QueryResult, error) {
	f.page = req.Page
	f.pageSize = req.PageSize
	return &model.QueryResult{Page: req.Page, PageSize: req.PageSize, Data: []map[string]interface{}{}}, nil
}

func (f *fakeLogQueryService) QueryByCursorContext(ctx context.Context, req *model.QueryRequest) (*model.CursorQueryResult, error) {
	f.pageSize = req.PageSize
	return &model.CursorQueryResult{Data: []map[string]interface{}{}}, nil
}

func (f *fakeLogQueryService) PrepareExportCSV(req *model.QueryRequest) error {
	f.page = req.Page
	f.pageSize = req.PageSize
	return nil
}

func (f *fakeLogQueryService) ExportCSVStreamContext(ctx context.Context, req *model.QueryRequest, writeRow func(map[string]interface{}) error) error {
	return nil
}

func (f *fakeLogQueryService) GetFeatureIDs() []int {
	return []int{90000031}
}

func (f *fakeLogQueryService) GetFeatureName(featureID int) string {
	return "登录"
}

func (f *fakeLogQueryService) GetFieldPaths() []string {
	return nil
}

func validLogQueryBody(extra string) string {
	body := `{"feature_ids":[90000031],"start_time":100,"end_time":200`
	if extra != "" {
		body += "," + extra
	}
	return body + "}"
}

func TestLogQueryValidatesRequest(t *testing.T) {
	tests := []string{
		`{"feature_ids":[],"start_time":100,"end_time":200}`,
		validLogQueryBody(`"page":-1`),
		validLogQueryBody(`"page_size":1001`),
		validLogQueryBody(`"start_time":300`),
		validLogQueryBody(`"feature_ids":[0]`),
	}

	for _, body := range tests {
		c, rec := newJSONContext(http.MethodPost, "/logs/query", body)
		h := &LogHandler{querySvc: &fakeLogQueryService{}}
		if err := h.Query(c); err != nil {
			t.Fatalf("Query(%s): %v", body, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Query(%s) status = %d, want 400", body, rec.Code)
		}
	}
}

func TestLogQueryDefaultsPagination(t *testing.T) {
	svc := &fakeLogQueryService{}
	c, rec := newJSONContext(http.MethodPost, "/logs/query", validLogQueryBody(""))
	h := &LogHandler{querySvc: svc}

	if err := h.Query(c); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.page != defaultPage || svc.pageSize != defaultLogPageSize {
		t.Fatalf("page/pageSize = %d/%d", svc.page, svc.pageSize)
	}
}

func TestLogCursorQueryValidatesCursor(t *testing.T) {
	c, rec := newJSONContext(http.MethodPost, "/logs/query/cursor", validLogQueryBody(`"cursor":-1`))
	h := &LogHandler{querySvc: &fakeLogQueryService{}}

	if err := h.QueryByCursor(c); err != nil {
		t.Fatalf("QueryByCursor: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLogExportAllowsLargerPageSize(t *testing.T) {
	svc := &fakeLogQueryService{}
	c, rec := newJSONContext(http.MethodPost, "/logs/export", validLogQueryBody(`"page_size":50000`))
	h := &LogHandler{querySvc: svc}

	if err := h.ExportCSV(c); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.pageSize != 50000 {
		t.Fatalf("pageSize = %d, want 50000", svc.pageSize)
	}
}

func TestLogExportRejectsOversizedPageSize(t *testing.T) {
	c, rec := newJSONContext(http.MethodPost, "/logs/export", validLogQueryBody(`"page_size":50001`))
	h := &LogHandler{querySvc: &fakeLogQueryService{}}

	if err := h.ExportCSV(c); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

type fakeLogSyncService struct {
	started bool
}

func (f *fakeLogSyncService) IsRunning() bool {
	return false
}

func (f *fakeLogSyncService) StartSync(fn func()) {
	f.started = true
}

func (f *fakeLogSyncService) SyncAllFeatures(startTime, endTime int64) map[int]int {
	return map[int]int{}
}

func (f *fakeLogSyncService) SyncMultipleFeatures(featureIDs []int, startTime, endTime int64) map[int]int {
	return map[int]int{}
}

func (f *fakeLogSyncService) GetStatus() *service.SyncStatus {
	return &service.SyncStatus{}
}

func (f *fakeLogSyncService) Cancel() {}

func TestSyncRejectsInvalidRequests(t *testing.T) {
	tests := []string{
		`{}`,
		`{"feature_ids":[]}`,
		`{"feature_ids":[0]}`,
		`{"feature_ids":[90000031],"start_time":200,"end_time":100}`,
		`{"feature_ids":[90000031],"start_time":100}`,
	}

	for _, body := range tests {
		svc := &fakeLogSyncService{}
		c, rec := newJSONContext(http.MethodPost, "/logs/sync", body)
		h := &SyncHandler{syncSvc: svc}
		if err := h.Sync(c); err != nil {
			t.Fatalf("Sync(%s): %v", body, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Sync(%s) status = %d, want 400", body, rec.Code)
		}
		if svc.started {
			t.Fatalf("Sync(%s) started unexpectedly", body)
		}
	}
}

func TestSyncAcceptsFeatureRequest(t *testing.T) {
	svc := &fakeLogSyncService{}
	c, rec := newJSONContext(http.MethodPost, "/logs/sync", `{"feature_ids":[90000031]}`)
	h := &SyncHandler{syncSvc: svc}

	if err := h.Sync(c); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !svc.started {
		t.Fatalf("sync was not started")
	}
}
