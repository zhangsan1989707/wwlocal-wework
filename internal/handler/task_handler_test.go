package handler

import (
	"strings"
	"testing"

	"wwlocal-wework/internal/model"
)

func TestValidateSubmitTaskRequestAcceptsValidRequests(t *testing.T) {
	tests := []*SubmitTaskRequest{
		{Type: model.TaskTypeContactSync},
		{Type: model.TaskTypeLogSync, FeatureIDs: []int{90000031}, StartTime: 100, EndTime: 200},
		{Type: model.TaskTypeAdminLogSync, StartTime: 100, EndTime: 100},
	}

	for _, req := range tests {
		if err := validateSubmitTaskRequest(req); err != nil {
			t.Fatalf("validateSubmitTaskRequest(%+v): %v", req, err)
		}
	}
}

func TestValidateSubmitTaskRequestRejectsInvalidType(t *testing.T) {
	err := validateSubmitTaskRequest(&SubmitTaskRequest{Type: model.TaskType("unknown")})
	if err == nil || !strings.Contains(err.Error(), "type must be") {
		t.Fatalf("error = %v, want type validation error", err)
	}
}

func TestValidateSubmitTaskRequestRejectsInvalidFeatureIDs(t *testing.T) {
	err := validateSubmitTaskRequest(&SubmitTaskRequest{Type: model.TaskTypeLogSync, FeatureIDs: []int{90000031, 0}})
	if err == nil || !strings.Contains(err.Error(), "feature_ids") {
		t.Fatalf("error = %v, want feature_ids validation error", err)
	}
}

func TestValidateSubmitTaskRequestRejectsFeatureIDsForNonLogTask(t *testing.T) {
	err := validateSubmitTaskRequest(&SubmitTaskRequest{Type: model.TaskTypeContactSync, FeatureIDs: []int{90000031}})
	if err == nil || !strings.Contains(err.Error(), "log_sync") {
		t.Fatalf("error = %v, want feature_ids task type validation error", err)
	}
}

func TestValidateSubmitTaskRequestRejectsPartialTimeRange(t *testing.T) {
	err := validateSubmitTaskRequest(&SubmitTaskRequest{Type: model.TaskTypeLogSync, StartTime: 100})
	if err == nil || !strings.Contains(err.Error(), "provided together") {
		t.Fatalf("error = %v, want paired time validation error", err)
	}
}

func TestValidateSubmitTaskRequestRejectsReversedTimeRange(t *testing.T) {
	err := validateSubmitTaskRequest(&SubmitTaskRequest{Type: model.TaskTypeAdminLogSync, StartTime: 200, EndTime: 100})
	if err == nil || !strings.Contains(err.Error(), "end_time") {
		t.Fatalf("error = %v, want reversed time validation error", err)
	}
}
