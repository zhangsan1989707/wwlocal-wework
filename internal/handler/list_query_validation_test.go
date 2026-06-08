package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/internal/model"
)

func newListQueryContext(target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

type fakeOperationLogService struct {
	page       int
	pageSize   int
	statusCode int
}

func (f *fakeOperationLogService) List(page, pageSize int, action string, statusCode int) ([]model.OperationLog, int64, error) {
	f.page = page
	f.pageSize = pageSize
	f.statusCode = statusCode
	return []model.OperationLog{}, 0, nil
}

func (f *fakeOperationLogService) GetDistinctActions() ([]string, error) {
	return []string{}, nil
}

func TestOperationLogListValidatesQueryParams(t *testing.T) {
	tests := []string{
		"/operation-logs?page=abc",
		"/operation-logs?page_size=101",
		"/operation-logs?status_code=bad",
	}

	for _, target := range tests {
		c, rec := newListQueryContext(target)
		h := &OperationLogHandler{svc: &fakeOperationLogService{}}
		if err := h.List(c); err != nil {
			t.Fatalf("List(%s): %v", target, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("List(%s) status = %d, want 400", target, rec.Code)
		}
	}
}

func TestOperationLogListUsesParsedPagination(t *testing.T) {
	svc := &fakeOperationLogService{}
	c, rec := newListQueryContext("/operation-logs?page=2&page_size=50&status_code=200")
	h := &OperationLogHandler{svc: svc}

	if err := h.List(c); err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.page != 2 || svc.pageSize != 50 || svc.statusCode != 200 {
		t.Fatalf("parsed page/pageSize/status = %d/%d/%d", svc.page, svc.pageSize, svc.statusCode)
	}
}

type fakeAdminOperLogService struct{}

func (f *fakeAdminOperLogService) Query(operType, operUserID string, startTime, endTime int64, page, pageSize int) ([]model.AdminOperLog, int64, error) {
	return []model.AdminOperLog{}, 0, nil
}

func (f *fakeAdminOperLogService) SyncLogs(startTime, endTime int64) (int, error) {
	return 0, nil
}

func (f *fakeAdminOperLogService) SyncIncremental() (int, error) {
	return 0, nil
}

func (f *fakeAdminOperLogService) GetStats(startTime, endTime int64) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f *fakeAdminOperLogService) GetOperTypes() ([]string, error) {
	return []string{}, nil
}

func (f *fakeAdminOperLogService) GetOperUsers() ([]string, error) {
	return []string{}, nil
}

func (f *fakeAdminOperLogService) Cleanup(beforeDays int) (int64, error) {
	return 0, nil
}

func (f *fakeAdminOperLogService) GetStatus() (bool, int64, string, error) {
	return false, 0, "", nil
}

func TestAdminOperLogListValidatesQueryParams(t *testing.T) {
	tests := []string{
		"/admin-oper-logs?start_time=bad",
		"/admin-oper-logs?start_time=20&end_time=10",
	}

	for _, target := range tests {
		c, rec := newListQueryContext(target)
		h := &AdminOperLogHandler{svc: &fakeAdminOperLogService{}}
		if err := h.List(c); err != nil {
			t.Fatalf("List(%s): %v", target, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("List(%s) status = %d, want 400", target, rec.Code)
		}
	}
}

type fakeSyncHistoryRepository struct{}

func (f *fakeSyncHistoryRepository) List(syncType string, page, pageSize int) ([]model.SyncHistory, int64, error) {
	return []model.SyncHistory{}, 0, nil
}

func TestSyncHistoryListValidatesPagination(t *testing.T) {
	c, rec := newListQueryContext("/sync-history?page_size=101")
	h := &SyncHistoryHandler{repo: &fakeSyncHistoryRepository{}}

	if err := h.List(c); err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

type fakeContactRepository struct{}

func (f *fakeContactRepository) QueryContacts(name, mobile string, page, pageSize int) ([]model.Contact, int64, error) {
	return []model.Contact{}, 0, nil
}

func (f *fakeContactRepository) GetAllDepartments() ([]model.Department, error) {
	return []model.Department{}, nil
}

func (f *fakeContactRepository) GetMemberCountByDepartmentIDs(deptIDs []int) (map[int]int, error) {
	return map[int]int{}, nil
}

func (f *fakeContactRepository) GetTotalContacts() (int64, error) {
	return 0, nil
}

func (f *fakeContactRepository) GetContactsByDepartmentID(deptID int, page, pageSize int) ([]model.Contact, int64, error) {
	return []model.Contact{}, 0, nil
}

func (f *fakeContactRepository) GetNamesByUserIDs(userIDs []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (f *fakeContactRepository) GetContactByUserID(userID string) (*model.Contact, error) {
	return nil, nil
}

func TestContactListValidatesPagination(t *testing.T) {
	c, rec := newListQueryContext("/contacts?page=abc")
	h := &ContactHandler{contactRepo: &fakeContactRepository{}}

	if err := h.List(c); err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestContactDeptMembersValidatesPagination(t *testing.T) {
	c, rec := newListQueryContext("/contacts/departments/1/members?page_size=101")
	c.SetParamNames("id")
	c.SetParamValues("1")
	h := &ContactHandler{contactRepo: &fakeContactRepository{}}

	if err := h.GetDeptMembers(c); err != nil {
		t.Fatalf("GetDeptMembers: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
