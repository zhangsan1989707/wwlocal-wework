package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type fakeSystemSyncStateRepo struct {
	pingErr error
	states  []model.SyncState
	err     error
}

func (f *fakeSystemSyncStateRepo) Ping() error {
	return f.pingErr
}

func (f *fakeSystemSyncStateRepo) GetAll() ([]model.SyncState, error) {
	return f.states, f.err
}

type fakeSystemKeyRepo struct {
	active    *model.RSAKeyVersion
	activeErr error
	keys      []model.RSAKeyVersion
	keysErr   error
}

func (f *fakeSystemKeyRepo) GetActive() (*model.RSAKeyVersion, error) {
	return f.active, f.activeErr
}

func (f *fakeSystemKeyRepo) GetAll() ([]model.RSAKeyVersion, error) {
	return f.keys, f.keysErr
}

type fakeSystemContactRepo struct {
	count    int64
	countErr error
	lastSync *time.Time
	syncErr  error
}

func (f *fakeSystemContactRepo) GetTotalContacts() (int64, error) {
	return f.count, f.countErr
}

func (f *fakeSystemContactRepo) GetLastSyncTime() (*time.Time, error) {
	return f.lastSync, f.syncErr
}

type fakeSystemLogRepo struct {
	tables        []repository.TableSizeInfo
	tableErr      error
	schemaQuality []repository.SchemaQualityInfo
	schemaErr     error
}

func (f *fakeSystemLogRepo) GetTableSizes(limit int) ([]repository.TableSizeInfo, error) {
	return f.tables, f.tableErr
}

func (f *fakeSystemLogRepo) GetSchemaQuality() ([]repository.SchemaQualityInfo, error) {
	return f.schemaQuality, f.schemaErr
}

func newSystemTestContext() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func newFakeSystemHandler() *SystemHandler {
	return &SystemHandler{
		syncStateRepo: &fakeSystemSyncStateRepo{},
		keyRepo:       &fakeSystemKeyRepo{},
		contactRepo:   &fakeSystemContactRepo{},
		logRepo:       &fakeSystemLogRepo{},
		startTime:     time.Now(),
	}
}

func TestSystemStatusAllowsMissingActiveKey(t *testing.T) {
	h := newFakeSystemHandler()
	h.keyRepo = &fakeSystemKeyRepo{
		activeErr: gorm.ErrRecordNotFound,
		keys:      []model.RSAKeyVersion{{Version: "v1"}},
	}
	h.contactRepo = &fakeSystemContactRepo{count: 3}

	c, rec := newSystemTestContext()
	if err := h.GetStatus(c); err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestSystemStatusReturnsErrorForSyncCoverageFailure(t *testing.T) {
	h := newFakeSystemHandler()
	h.syncStateRepo = &fakeSystemSyncStateRepo{err: errors.New("db down")}

	c, rec := newSystemTestContext()
	if err := h.GetStatus(c); err != nil {
		t.Fatalf("GetStatus returned echo error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestSystemStatusReturnsErrorForKeyListFailure(t *testing.T) {
	h := newFakeSystemHandler()
	h.keyRepo = &fakeSystemKeyRepo{
		activeErr: gorm.ErrRecordNotFound,
		keysErr:   errors.New("key query failed"),
	}

	c, rec := newSystemTestContext()
	if err := h.GetStatus(c); err != nil {
		t.Fatalf("GetStatus returned echo error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
