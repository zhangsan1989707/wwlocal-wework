package service

import (
	"reflect"
	"testing"
	"time"

	"wwlocal-wework/internal/model"
)

func TestSortAndLimitTasksSortsBeforeLimit(t *testing.T) {
	oldest := &model.SyncTask{ID: "oldest", CreatedAt: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)}
	newest := &model.SyncTask{ID: "newest", CreatedAt: time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)}
	middle := &model.SyncTask{ID: "middle", CreatedAt: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)}

	got := sortAndLimitTasks([]*model.SyncTask{oldest, newest, middle}, 2)

	var ids []string
	for _, task := range got {
		ids = append(ids, task.ID)
	}
	if !reflect.DeepEqual(ids, []string{"newest", "middle"}) {
		t.Fatalf("ids = %v, want newest then middle", ids)
	}
}

func TestSortAndLimitTasksKeepsAllWhenLimitIsZero(t *testing.T) {
	tasks := []*model.SyncTask{
		{ID: "a", CreatedAt: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CreatedAt: time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)},
	}

	got := sortAndLimitTasks(tasks, 0)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "b" {
		t.Fatalf("first task = %s, want b", got[0].ID)
	}
}

func TestTaskPayloadAcceptsStringAndBytes(t *testing.T) {
	if got, ok := taskPayload(map[string]interface{}{"task": "payload"}); !ok || got != "payload" {
		t.Fatalf("string payload = %q, %v; want payload, true", got, ok)
	}
	if got, ok := taskPayload(map[string]interface{}{"task": []byte("payload")}); !ok || got != "payload" {
		t.Fatalf("bytes payload = %q, %v; want payload, true", got, ok)
	}
}

func TestTaskPayloadRejectsInvalidPayload(t *testing.T) {
	cases := []map[string]interface{}{
		{},
		{"task": ""},
		{"task": []byte{}},
		{"task": 123},
	}

	for _, tc := range cases {
		if got, ok := taskPayload(tc); ok {
			t.Fatalf("payload %v accepted as %q", tc, got)
		}
	}
}
