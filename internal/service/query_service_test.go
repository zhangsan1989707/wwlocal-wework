package service

import (
	"context"
	"errors"
	"testing"

	"wwlocal-wework/internal/model"
)

type fakeLogQueryRepo struct {
	calls []exportCall
	rows  map[int][]model.LogEntry
}

type exportCall struct {
	FeatureID int
	Page      int
	PageSize  int
}

func (r *fakeLogQueryRepo) QueryAcrossMonthsContext(ctx context.Context, featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error) {
	return nil, 0, nil
}

func (r *fakeLogQueryRepo) QueryAcrossMonthsWithConditionsContext(ctx context.Context, featureID int, startTime, endTime int64, conditions map[string]interface{}, mobile string, page, pageSize int) ([]model.LogEntry, int64, error) {
	r.calls = append(r.calls, exportCall{FeatureID: featureID, Page: page, PageSize: pageSize})
	rows := r.rows[featureID]
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []model.LogEntry{}, int64(len(rows)), nil
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], int64(len(rows)), nil
}

func (r *fakeLogQueryRepo) QueryByCursorContext(ctx context.Context, featureID int, startTime, endTime int64, cursor int64, pageSize int, conditions map[string]interface{}, mobile string) ([]model.LogEntry, int64, int64, error) {
	return nil, 0, 0, nil
}

func (r *fakeLogQueryRepo) SampleParsedJSON(featureIDs []int, limit int) []string {
	return nil
}

func TestQueryContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queryContextError(ctx); !errors.Is(err, ErrQueryCanceled) {
		t.Fatalf("canceled context error = %v, want ErrQueryCanceled", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if err := queryContextError(ctx); err != nil {
		t.Fatalf("active context error = %v, want nil", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 0)
	defer cancel()
	if err := queryContextError(ctx); !errors.Is(err, ErrQueryTimeout) {
		t.Fatalf("deadline context error = %v, want ErrQueryTimeout", err)
	}
}

func TestExportCSVStreamContextPagesRowsByBatch(t *testing.T) {
	repo := &fakeLogQueryRepo{rows: map[int][]model.LogEntry{
		90000031: makeLogEntries(1205, 90000031),
	}}
	svc := &QueryService{logRepo: repo}

	var got []map[string]interface{}
	err := svc.ExportCSVStreamContext(context.Background(), &model.QueryRequest{
		FeatureIDs: []int{90000031},
		StartTime:  1,
		EndTime:    2,
		PageSize:   1205,
	}, func(row map[string]interface{}) error {
		got = append(got, row)
		return nil
	})
	if err != nil {
		t.Fatalf("ExportCSVStreamContext: %v", err)
	}
	if len(got) != 1205 {
		t.Fatalf("got %d rows, want 1205", len(got))
	}
	wantCalls := []exportCall{
		{FeatureID: 90000031, Page: 1, PageSize: 1000},
		{FeatureID: 90000031, Page: 2, PageSize: 1000},
	}
	if len(repo.calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", repo.calls, wantCalls)
	}
	for i := range wantCalls {
		if repo.calls[i] != wantCalls[i] {
			t.Fatalf("call %d = %+v, want %+v", i, repo.calls[i], wantCalls[i])
		}
	}
}

func TestExportCSVStreamContextAppliesPageSizePerFeature(t *testing.T) {
	repo := &fakeLogQueryRepo{rows: map[int][]model.LogEntry{
		90000031: makeLogEntries(3, 90000031),
		90000036: makeLogEntries(3, 90000036),
	}}
	svc := &QueryService{logRepo: repo}

	count := 0
	err := svc.ExportCSVStreamContext(context.Background(), &model.QueryRequest{
		FeatureIDs: []int{90000031, 90000036},
		StartTime:  1,
		EndTime:    2,
		PageSize:   2,
	}, func(row map[string]interface{}) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ExportCSVStreamContext: %v", err)
	}
	if count != 4 {
		t.Fatalf("count = %d, want 4", count)
	}
}

func TestExportCSVStreamContextMergesFeaturesByTime(t *testing.T) {
	repo := &fakeLogQueryRepo{rows: map[int][]model.LogEntry{
		90000031: {
			{ID: 3, FeatureID: 90000031, LogTime: 30, ParsedJSON: `{"openid":"u3"}`},
			{ID: 1, FeatureID: 90000031, LogTime: 10, ParsedJSON: `{"openid":"u1"}`},
		},
		90000036: {
			{ID: 4, FeatureID: 90000036, LogTime: 40, ParsedJSON: `{"openid":"u4"}`},
			{ID: 2, FeatureID: 90000036, LogTime: 20, ParsedJSON: `{"openid":"u2"}`},
		},
	}}
	svc := &QueryService{logRepo: repo}

	var got []int64
	err := svc.ExportCSVStreamContext(context.Background(), &model.QueryRequest{
		FeatureIDs: []int{90000031, 90000036},
		StartTime:  1,
		EndTime:    50,
		PageSize:   2,
	}, func(row map[string]interface{}) error {
		got = append(got, row["log_time"].(int64))
		return nil
	})
	if err != nil {
		t.Fatalf("ExportCSVStreamContext: %v", err)
	}
	want := []int64{40, 30, 20, 10}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func makeLogEntries(count int, featureID int) []model.LogEntry {
	rows := make([]model.LogEntry, count)
	for i := range rows {
		rows[i] = model.LogEntry{
			ID:         int64(i + 1),
			FeatureID:  featureID,
			LogTime:    int64(1000 + i),
			ParsedJSON: `{"openid":"u1","action":"login"}`,
		}
	}
	return rows
}
