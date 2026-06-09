package handler

import (
	"strings"
	"testing"
)

func TestValidateUpdateSyncFeaturesRequestAcceptsValidUpdates(t *testing.T) {
	req := &UpdateSyncFeaturesRequest{
		Features: []SyncFeatureUpdate{
			{FeatureID: 90000031, Enabled: true},
			{FeatureID: 90000033, Enabled: false},
		},
	}

	if err := validateUpdateSyncFeaturesRequest(req); err != nil {
		t.Fatalf("validateUpdateSyncFeaturesRequest: %v", err)
	}
}

func TestValidateUpdateSyncFeaturesRequestRejectsInvalidUpdates(t *testing.T) {
	tests := []struct {
		name string
		req  *UpdateSyncFeaturesRequest
		want string
	}{
		{
			name: "empty",
			req:  &UpdateSyncFeaturesRequest{},
			want: "features",
		},
		{
			name: "invalid id",
			req:  &UpdateSyncFeaturesRequest{Features: []SyncFeatureUpdate{{FeatureID: 0}}},
			want: "positive",
		},
		{
			name: "duplicate id",
			req:  &UpdateSyncFeaturesRequest{Features: []SyncFeatureUpdate{{FeatureID: 90000031}, {FeatureID: 90000031}}},
			want: "duplicate",
		},
	}

	for _, tc := range tests {
		err := validateUpdateSyncFeaturesRequest(tc.req)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s error = %v, want containing %q", tc.name, err, tc.want)
		}
	}
}
