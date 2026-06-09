package repository

import "testing"

func TestMapStructuredFieldsSingleChat(t *testing.T) {
	parsed := `{
		"sender": {"openid": "u1", "type": 1},
		"receiver": {"openid": "u2", "type": 1},
		"send_time": 1710000000,
		"msg_type": 1,
		"msg_id": "m1"
	}`

	got := mapStructuredFields(90000036, parsed)
	if got["sender_openid"] != "u1" {
		t.Fatalf("sender_openid = %v, want u1", got["sender_openid"])
	}
	if got["receiver_openid"] != "u2" {
		t.Fatalf("receiver_openid = %v, want u2", got["receiver_openid"])
	}
	if got["msgid"] != "m1" {
		t.Fatalf("msgid = %v, want m1", got["msgid"])
	}
}

func TestMapStructuredFieldsUnmappedFeature(t *testing.T) {
	if got := mapStructuredFields(90000066, `{"openid":"u1"}`); got != nil {
		t.Fatalf("unmapped feature returned %#v, want nil", got)
	}
}

func TestExtractMobileFromParsedUserOpenID(t *testing.T) {
	parsed := map[string]interface{}{
		"user": map[string]interface{}{
			"openid": "u1",
			"type":   float64(0),
		},
	}

	if got := extractMobileFromParsed(parsed); got != "u1" {
		t.Fatalf("extractMobileFromParsed() = %q, want u1", got)
	}
}

func TestMapStructuredFieldsWeDriveOper(t *testing.T) {
	parsed := `{
		"time": 1619586572,
		"oper_type": 1,
		"oper_sub_type": 121,
		"oper_name": "u1",
		"ext": {"content": "doc", "ip": "127.0.0.1", "cli_type": 3}
	}`

	for _, featureID := range []int{90000061, 90000062, 90000063} {
		t.Run("feature", func(t *testing.T) {
			got := mapStructuredFields(featureID, parsed)
			if got["oper_name"] != "u1" {
				t.Fatalf("oper_name = %v, want u1", got["oper_name"])
			}
			if got["operate_time"] != int64(1619586572) {
				t.Fatalf("operate_time = %v, want 1619586572", got["operate_time"])
			}
			if got["oper_sub_type"] != 121 {
				t.Fatalf("oper_sub_type = %v, want 121", got["oper_sub_type"])
			}
			if got["ext"] == "" {
				t.Fatalf("ext should be serialized")
			}
		})
	}
}

func TestBehaviorFieldsIncludeWeDriveOper(t *testing.T) {
	for _, featureID := range []int{90000061, 90000062, 90000063} {
		fields := behaviorFieldsByFeature[featureID]
		if len(fields) != 1 || fields[0].Column != "oper_name" {
			t.Fatalf("feature %d fields = %#v, want oper_name", featureID, fields)
		}
	}
}

func TestDeviceLogSourcesIncludeUsageFeatures(t *testing.T) {
	got := deviceLogSources()
	byFeature := make(map[int]string)
	for _, source := range got {
		byFeature[source.FeatureID] = source.OpenIDColumn
	}

	want := map[int]string{
		90000031: "login_user_openid",
		90000032: "login_user_openid",
		90000033: "user_openid",
		90000035: "sender_openid",
		90000054: "openid",
		90000055: "openid",
		90000058: "openid",
		90000059: "openid",
	}
	for featureID, openIDColumn := range want {
		if byFeature[featureID] != openIDColumn {
			t.Fatalf("feature %d openid column = %q, want %q; sources=%#v", featureID, byFeature[featureID], openIDColumn, got)
		}
	}
}

func TestJSONTextContainsOpenID(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "string array", text: `["u1","u2"]`, want: true},
		{name: "object array openid", text: `[{"openid":"u1","type":1}]`, want: true},
		{name: "object array userid", text: `[{"userid":"u1"}]`, want: true},
		{name: "missing", text: `[{"openid":"u2"}]`, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonTextContainsOpenID(tc.text, "u1"); got != tc.want {
				t.Fatalf("jsonTextContainsOpenID() = %v, want %v", got, tc.want)
			}
		})
	}
}
