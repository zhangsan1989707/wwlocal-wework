package repository

import (
	"reflect"
	"testing"

	"wwlocal-wework/internal/model"
)

func TestExpandDepartmentIDsFromListIncludesChildren(t *testing.T) {
	depts := []model.Department{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
		{ID: 4, ParentID: 0},
	}

	got := ExpandDepartmentIDsFromList(depts, []int{2})
	want := []int{2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUniqueStringsTrimsAndDeduplicates(t *testing.T) {
	got := uniqueStrings([]string{" u1 ", "", "u2", "u1", "  "})
	want := []string{"u1", "u2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExpandDepartmentIDsFromListDeduplicates(t *testing.T) {
	depts := []model.Department{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 1},
	}

	got := ExpandDepartmentIDsFromList(depts, []int{1, 2})
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
