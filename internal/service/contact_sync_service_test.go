package service

import (
	"reflect"
	"testing"

	"wwlocal-wework/internal/model"
)

func TestChunkStringSliceSplitsBySize(t *testing.T) {
	got := chunkStringSlice([]string{"u1", "u2", "u3", "u4", "u5"}, 2)
	want := [][]string{{"u1", "u2"}, {"u3", "u4"}, {"u5"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestChunkStringSliceUsesSingleChunkForInvalidSize(t *testing.T) {
	got := chunkStringSlice([]string{"u1", "u2"}, 0)
	want := [][]string{{"u1", "u2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestContactUserIDsSkipsEmptyIDs(t *testing.T) {
	got := contactUserIDs([]model.Contact{
		{UserID: "u1"},
		{},
		{UserID: "u2"},
	})
	want := []string{"u1", "u2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeSimpleUserKeepsDepartmentsAndPositions(t *testing.T) {
	got := mergeSimpleUser(
		model.SimpleUser{
			UserID:     "u1",
			Name:       "张三",
			Department: []int{1, 2},
			Positions:  []string{"主任"},
		},
		model.SimpleUser{
			UserID:     "u1",
			Name:       "张三新",
			Department: []int{2, 3},
			Positions:  []string{"主任", "委员"},
		},
	)

	if !reflect.DeepEqual(got.Department, []int{1, 2, 3}) {
		t.Fatalf("departments got %v", got.Department)
	}
	if !reflect.DeepEqual(got.Positions, []string{"主任", "委员"}) {
		t.Fatalf("positions got %v", got.Positions)
	}
	if got.Name != "张三" {
		t.Fatalf("name got %q", got.Name)
	}
}

func TestSimpleUserIDsSkipsEmptyIDs(t *testing.T) {
	got := simpleUserIDs([]model.SimpleUser{
		{UserID: "u1"},
		{},
		{UserID: "u2"},
	})
	want := []string{"u1", "u2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
