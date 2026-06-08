package service

import (
	"errors"
	"reflect"
	"testing"

	"wwlocal-wework/internal/model"
)

func TestDashboardV2ValidateDashboardDate(t *testing.T) {
	got, err := validateDashboardDate("2026-06-08")
	if err != nil {
		t.Fatalf("validateDashboardDate: %v", err)
	}
	if got != "2026-06-08" {
		t.Fatalf("date = %q, want 2026-06-08", got)
	}

	if _, err := validateDashboardDate("2026/06/08"); !errors.Is(err, ErrDashboardInvalidParam) {
		t.Fatalf("invalid date error = %v, want ErrDashboardInvalidParam", err)
	}
}

func TestDashboardV2ResolveDateRangeValidatesInput(t *testing.T) {
	start, end, err := resolveDateRange("", "2026-06-08", "month")
	if err != nil {
		t.Fatalf("resolveDateRange: %v", err)
	}
	if start != "2026-06-01" || end != "2026-06-08" {
		t.Fatalf("range = %s..%s, want 2026-06-01..2026-06-08", start, end)
	}

	if _, _, err := resolveDateRange("2026-06-09", "2026-06-08", "day"); !errors.Is(err, ErrDashboardInvalidParam) {
		t.Fatalf("reversed range error = %v, want ErrDashboardInvalidParam", err)
	}
	if _, _, err := resolveDateRange("", "2026-06-08", "year"); !errors.Is(err, ErrDashboardInvalidParam) {
		t.Fatalf("invalid granularity error = %v, want ErrDashboardInvalidParam", err)
	}
}

func TestDashboardV2NormalizeListType(t *testing.T) {
	got, err := normalizeListType("")
	if err != nil {
		t.Fatalf("normalizeListType blank: %v", err)
	}
	if got != model.ListTypeInactive {
		t.Fatalf("blank list type = %q, want inactive", got)
	}

	got, err = normalizeListType("not_login")
	if err != nil {
		t.Fatalf("normalizeListType not_login: %v", err)
	}
	if got != model.ListTypeNoLogin {
		t.Fatalf("not_login normalized to %q, want no_login", got)
	}

	if _, err := normalizeListType("disabled"); !errors.Is(err, ErrDashboardInvalidParam) {
		t.Fatalf("invalid list type error = %v, want ErrDashboardInvalidParam", err)
	}
}

func TestDashboardV2ValidateMetricType(t *testing.T) {
	if err := validateMetricType("login_users"); err != nil {
		t.Fatalf("validateMetricType login_users: %v", err)
	}
	if err := validateMetricType(model.MetricMsgCount); err != nil {
		t.Fatalf("validateMetricType msg_count: %v", err)
	}
	if err := validateMetricType("unknown_metric"); !errors.Is(err, ErrDashboardInvalidParam) {
		t.Fatalf("unknown metric error = %v, want ErrDashboardInvalidParam", err)
	}
}

func TestDashboardV2DevicePayloadShape(t *testing.T) {
	got := buildDevicePayload("2026-06-08", map[int]int64{
		131073: 2,
		65537:  1,
	})

	if got["date"] != "2026-06-08" {
		t.Fatalf("date = %v", got["date"])
	}
	if got["total"] != int64(3) {
		t.Fatalf("total = %v, want 3", got["total"])
	}
	items, ok := got["types"].([]map[string]interface{})
	if !ok {
		t.Fatalf("types has type %T", got["types"])
	}
	if len(items) == 0 {
		t.Fatalf("types should not be empty")
	}
	if !reflect.DeepEqual(items[0], map[string]interface{}{
		"type":       model.MetricDeviceAndroid,
		"name":       model.DeviceTypeName[model.MetricDeviceAndroid],
		"count":      int64(2),
		"percentage": float64(2) / float64(3) * 100,
	}) {
		t.Fatalf("first item = %#v", items[0])
	}
}
