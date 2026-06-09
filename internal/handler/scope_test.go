package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/service"
)

type fakeScopeChecker struct {
	scope *service.DataScope
	ok    bool
	err   error
}

func (f *fakeScopeChecker) IdentifierInDataScope(userID int64, identifier string) (*service.DataScope, bool, error) {
	if f.scope == nil {
		return &service.DataScope{}, f.ok, f.err
	}
	return f.scope, f.ok, f.err
}

func newScopeContext(userID int64) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID > 0 {
		c.Set("user_id", userID)
	}
	return c, rec
}

func TestLogCheckQueryScopeRejectsMissingUser(t *testing.T) {
	c, rec := newScopeContext(0)
	h := &LogHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{Unrestricted: true}}}

	err := h.checkQueryScope(c, &model.QueryRequest{Mobile: "u1"})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogCheckQueryScopeAllowsSuperAdminWithoutIdentifier(t *testing.T) {
	c, rec := newScopeContext(1)
	h := &LogHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{Unrestricted: true}}}

	err := h.checkQueryScope(c, &model.QueryRequest{})
	if err != nil {
		t.Fatalf("checkQueryScope: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want default %d", rec.Code, http.StatusOK)
	}
}

func TestLogCheckQueryScopeRejectsDeptAdminWithoutIdentifier(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &LogHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}}}

	err := h.checkQueryScope(c, &model.QueryRequest{})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogCheckQueryScopeRejectsOutOfScopeIdentifier(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &LogHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}, ok: false}}

	err := h.checkQueryScope(c, &model.QueryRequest{Mobile: "outside"})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogCheckQueryScopeAllowsInScopeIdentifier(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &LogHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}, ok: true}}

	err := h.checkQueryScope(c, &model.QueryRequest{Mobile: "inside"})
	if err != nil {
		t.Fatalf("checkQueryScope: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want default %d", rec.Code, http.StatusOK)
	}
}

func TestLogCheckQueryScopeHandlesScopeErrors(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &LogHandler{userSvc: &fakeScopeChecker{err: errors.New("db down")}}

	err := h.checkQueryScope(c, &model.QueryRequest{Mobile: "u1"})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLogQueryErrorMapsTimeout(t *testing.T) {
	c, rec := newScopeContext(1)
	h := &LogHandler{}

	err := h.queryError(c, service.ErrQueryTimeout, "查询失败")
	if err != nil {
		t.Fatalf("queryError returned error: %v", err)
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

func TestLogQueryErrorMapsCanceled(t *testing.T) {
	c, rec := newScopeContext(1)
	h := &LogHandler{}

	err := h.queryError(c, service.ErrQueryCanceled, "查询失败")
	if err != nil {
		t.Fatalf("queryError returned error: %v", err)
	}
	if rec.Code != 499 {
		t.Fatalf("status = %d, want 499", rec.Code)
	}
}

func TestFormatLogCSVRow(t *testing.T) {
	h := &LogHandler{querySvc: service.NewQueryService(nil, nil, nil, nil, nil, &config.Config{
		Features: config.FeaturesConfig{Names: map[int]string{90000031: "登录"}},
	})}
	row := map[string]interface{}{
		"feature_id": int64(90000031),
		"log_date":   "2026-06-08 10:00:00",
		"openid":     "u1",
		"action":     "login",
		"id":         int64(1),
		"log_time":   int64(1780912800),
	}

	got := h.formatLogCSVRow(row)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0] != "90000031" {
		t.Fatalf("feature id = %q, want 90000031", got[0])
	}
	if got[1] != "登录" {
		t.Fatalf("feature name = %q, want 登录", got[1])
	}
	if got[2] != "2026-06-08 10:00:00" {
		t.Fatalf("date = %q", got[2])
	}
	if got[3] != "u1" {
		t.Fatalf("openid = %q", got[3])
	}
	if !strings.Contains(got[4], `"action":"login"`) {
		t.Fatalf("content = %q, want action json", got[4])
	}
	if strings.Contains(got[4], "log_time") {
		t.Fatalf("content = %q, should omit internal fields", got[4])
	}
}

func TestBehaviorCheckQueryScopeRejectsBlankOpenID(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &BehaviorQueryHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}}}

	err := h.checkQueryScope(c, &model.BehaviorQueryRequest{OpenID: " "})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestBehaviorCheckQueryScopeRejectsOutOfScopeOpenID(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &BehaviorQueryHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}, ok: false}}

	err := h.checkQueryScope(c, &model.BehaviorQueryRequest{OpenID: "outside"})
	if err != nil {
		t.Fatalf("checkQueryScope returned error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestBehaviorCheckQueryScopeAllowsInScopeOpenID(t *testing.T) {
	c, rec := newScopeContext(2)
	h := &BehaviorQueryHandler{userSvc: &fakeScopeChecker{scope: &service.DataScope{DeptIDs: []int{10}}, ok: true}}

	err := h.checkQueryScope(c, &model.BehaviorQueryRequest{OpenID: "inside"})
	if err != nil {
		t.Fatalf("checkQueryScope: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want default %d", rec.Code, http.StatusOK)
	}
}
