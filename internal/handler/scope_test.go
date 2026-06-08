package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
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
