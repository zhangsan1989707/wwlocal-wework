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
