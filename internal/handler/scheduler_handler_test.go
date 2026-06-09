package handler

import "testing"

func TestValidateIncrementalSyncRequest(t *testing.T) {
	valid := []*IncrementalSyncRequest{
		{SyncAll: true},
		{FeatureIDs: []int{90000031}},
	}
	for _, req := range valid {
		if err := validateIncrementalSyncRequest(req); err != nil {
			t.Fatalf("validateIncrementalSyncRequest(%+v): %v", req, err)
		}
	}
}

func TestValidateIncrementalSyncRequestRejectsInvalidRequests(t *testing.T) {
	tooMany := make([]int, maxLogFeatureCount+1)
	for i := range tooMany {
		tooMany[i] = 90000031 + i
	}

	invalid := []*IncrementalSyncRequest{
		{},
		{FeatureIDs: []int{}},
		{FeatureIDs: []int{0}},
		{FeatureIDs: tooMany},
	}
	for _, req := range invalid {
		if err := validateIncrementalSyncRequest(req); err == nil {
			t.Fatalf("validateIncrementalSyncRequest(%+v) error = nil, want error", req)
		}
	}
}
