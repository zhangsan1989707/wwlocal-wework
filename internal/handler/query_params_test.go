package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newQueryContext(target string) echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}

func TestParsePaginationDefaults(t *testing.T) {
	page, pageSize, err := parsePagination(newQueryContext("/"))
	if err != nil {
		t.Fatalf("parsePagination: %v", err)
	}
	if page != 1 || pageSize != 20 {
		t.Fatalf("page=%d pageSize=%d, want 1/20", page, pageSize)
	}
}

func TestParsePaginationRejectsInvalidPage(t *testing.T) {
	_, _, err := parsePagination(newQueryContext("/?page=abc"))
	if err == nil {
		t.Fatalf("parsePagination error = nil, want error")
	}
}

func TestParsePaginationRejectsOversizedPageSize(t *testing.T) {
	_, _, err := parsePagination(newQueryContext("/?page_size=101"))
	if err == nil {
		t.Fatalf("parsePagination error = nil, want error")
	}
}

func TestParseOptionalInt64Query(t *testing.T) {
	got, err := parseOptionalInt64Query(newQueryContext("/?start_time=1780898409"), "start_time")
	if err != nil {
		t.Fatalf("parseOptionalInt64Query: %v", err)
	}
	if got != 1780898409 {
		t.Fatalf("got %d", got)
	}
	if _, err := parseOptionalInt64Query(newQueryContext("/?start_time=nope"), "start_time"); err == nil {
		t.Fatalf("parseOptionalInt64Query error = nil, want error")
	}
}

func TestParseOptionalNonNegativeInt64Query(t *testing.T) {
	got, err := parseOptionalNonNegativeInt64Query(newQueryContext("/?start_time=1780898409"), "start_time")
	if err != nil {
		t.Fatalf("parseOptionalNonNegativeInt64Query: %v", err)
	}
	if got != 1780898409 {
		t.Fatalf("got %d", got)
	}
	if _, err := parseOptionalNonNegativeInt64Query(newQueryContext("/?start_time=-1"), "start_time"); err == nil {
		t.Fatalf("parseOptionalNonNegativeInt64Query error = nil, want error")
	}
}

func TestParseOptionalNonNegativeIntQuery(t *testing.T) {
	got, err := parseOptionalNonNegativeIntQuery(newQueryContext("/?dept_id=12"), "dept_id")
	if err != nil {
		t.Fatalf("parseOptionalNonNegativeIntQuery: %v", err)
	}
	if got != 12 {
		t.Fatalf("got %d, want 12", got)
	}
	if _, err := parseOptionalNonNegativeIntQuery(newQueryContext("/?dept_id=-1"), "dept_id"); err == nil {
		t.Fatalf("parseOptionalNonNegativeIntQuery error = nil, want error")
	}
}
