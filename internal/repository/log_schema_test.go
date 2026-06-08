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
