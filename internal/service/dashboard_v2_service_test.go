package service

import (
	"errors"
	"reflect"
	"testing"

	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type fakeDashboardV2StatsRepo struct {
	distinctErrByFeature     map[int]error
	distinctThroughByFeature map[int]int64
	logRowsErrByFeature      map[int]error
	deviceErr                error
}

func (f *fakeDashboardV2StatsRepo) CountDistinctUsersFromDailyStats(featureIDs []int, startDate, endDate string, deptIDs []int, unrestricted bool) (int64, error) {
	if len(featureIDs) > 0 && f.distinctErrByFeature != nil {
		if err := f.distinctErrByFeature[featureIDs[0]]; err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (f *fakeDashboardV2StatsRepo) CountDistinctUsersFromDailyStatsThroughDate(featureIDs []int, endDate string, deptIDs []int, unrestricted bool) (int64, error) {
	if len(featureIDs) > 0 && f.distinctErrByFeature != nil {
		if err := f.distinctErrByFeature[featureIDs[0]]; err != nil {
			return 0, err
		}
	}
	if len(featureIDs) > 0 && f.distinctThroughByFeature != nil {
		if count, ok := f.distinctThroughByFeature[featureIDs[0]]; ok {
			return count, nil
		}
	}
	return 1, nil
}

func (f *fakeDashboardV2StatsRepo) CountLogRowsScoped(featureIDs []int, startDate, endDate, userField string, deptIDs []int, unrestricted bool) (int64, error) {
	if len(featureIDs) > 0 && f.logRowsErrByFeature != nil {
		if err := f.logRowsErrByFeature[featureIDs[0]]; err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (f *fakeDashboardV2StatsRepo) GetPeopleTrend(featureIDs []int, startDate, endDate, granularity string, deptIDs []int, unrestricted bool) ([]repository.AggregatedStat, error) {
	return nil, nil
}

func (f *fakeDashboardV2StatsRepo) GetEventTrendScoped(featureIDs []int, startDate, endDate, granularity, userField string, deptIDs []int, unrestricted bool) ([]repository.AggregatedStat, error) {
	return nil, nil
}

func (f *fakeDashboardV2StatsRepo) GetDeviceStatsScoped(statDate string, deptIDs []int, unrestricted bool) (map[int]int64, error) {
	if f.deviceErr != nil {
		return nil, f.deviceErr
	}
	return map[int]int64{}, nil
}

func (f *fakeDashboardV2StatsRepo) GetScopedUserList(statDate, listType string, activeFeatureIDs []int, deptIDs []int, unrestricted bool, limit, offset int) ([]model.DashboardDailyUserList, int64, error) {
	return nil, 0, nil
}

func (f *fakeDashboardV2StatsRepo) GetLatestDate() (string, error) {
	return "2026-06-08", nil
}

type fakeDashboardV2ContactRepo struct{}

func (f *fakeDashboardV2ContactRepo) GetScopedContactCount(deptIDs []int, unrestricted bool) (int64, error) {
	return 10, nil
}

func (f *fakeDashboardV2ContactRepo) GetAllDepartments() ([]model.Department, error) {
	return nil, nil
}

func (f *fakeDashboardV2ContactRepo) GetDeptMemberCounts() ([]repository.DeptMemberCount, error) {
	return nil, nil
}

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

func TestDashboardV2OverviewReturnsGroupCreatedError(t *testing.T) {
	svc := &DashboardV2Service{
		statsRepo: &fakeDashboardV2StatsRepo{logRowsErrByFeature: map[int]error{
			90000038: errors.New("group query failed"),
		}},
		contactRepo: &fakeDashboardV2ContactRepo{},
	}

	if _, err := svc.GetOverview("2026-06-08", &DataScope{Unrestricted: true}); err == nil {
		t.Fatalf("GetOverview error = nil, want group query error")
	}
}

func TestDashboardV2OverviewUsesCumulativeActivation(t *testing.T) {
	svc := &DashboardV2Service{
		statsRepo: &fakeDashboardV2StatsRepo{distinctThroughByFeature: map[int]int64{
			90000048: 8,
		}},
		contactRepo: &fakeDashboardV2ContactRepo{},
	}

	got, err := svc.GetOverview("2026-06-08", &DataScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetOverview: %v", err)
	}
	if got["activated"] != int64(8) {
		t.Fatalf("activated = %v, want 8", got["activated"])
	}
	if got["rate_activation"] != int64(800) {
		t.Fatalf("rate_activation = %v, want 800", got["rate_activation"])
	}
}

func TestDashboardV2ExportOverviewCSV(t *testing.T) {
	svc := &DashboardV2Service{
		statsRepo:   &fakeDashboardV2StatsRepo{},
		contactRepo: &fakeDashboardV2ContactRepo{},
	}

	rows, err := svc.ExportOverviewCSV("2026-06-08", &DataScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("ExportOverviewCSV: %v", err)
	}
	if !csvRowsContain(rows, "设备总数", "0") {
		t.Fatalf("CSV rows should include device total 0: %#v", rows)
	}
	if csvRowsContainKey(rows, "活跃群数") || csvRowsContainKey(rows, "群活跃率") {
		t.Fatalf("CSV rows should not include unsupported group active metrics: %#v", rows)
	}
}

func TestOverviewDeviceTotalDefaultsToZero(t *testing.T) {
	if got := overviewDeviceTotal(map[string]interface{}{}); got != 0 {
		t.Fatalf("missing devices total = %v, want 0", got)
	}
	if got := overviewDeviceTotal(map[string]interface{}{"devices": "bad"}); got != 0 {
		t.Fatalf("invalid devices total = %v, want 0", got)
	}
}

func TestDashboardV2OverviewReturnsAppAccessError(t *testing.T) {
	svc := &DashboardV2Service{
		statsRepo: &fakeDashboardV2StatsRepo{distinctErrByFeature: map[int]error{
			90000033: errors.New("app access query failed"),
		}},
		contactRepo: &fakeDashboardV2ContactRepo{},
	}

	if _, err := svc.GetOverview("2026-06-08", &DataScope{Unrestricted: true}); err == nil {
		t.Fatalf("GetOverview error = nil, want app access query error")
	}
}

func TestDashboardV2OverviewReturnsDeviceError(t *testing.T) {
	svc := &DashboardV2Service{
		statsRepo:   &fakeDashboardV2StatsRepo{deviceErr: errors.New("device query failed")},
		contactRepo: &fakeDashboardV2ContactRepo{},
	}

	if _, err := svc.GetOverview("2026-06-08", &DataScope{Unrestricted: true}); err == nil {
		t.Fatalf("GetOverview error = nil, want device query error")
	}
}

func csvRowsContain(rows [][]string, key, value string) bool {
	for _, row := range rows {
		if len(row) >= 2 && row[0] == key && row[1] == value {
			return true
		}
	}
	return false
}

func csvRowsContainKey(rows [][]string, key string) bool {
	for _, row := range rows {
		if len(row) > 0 && row[0] == key {
			return true
		}
	}
	return false
}
