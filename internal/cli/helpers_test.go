// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// TestExtractResponseData covers the Straddle envelope unwrap helper used by
// the human display path of `straddle reports` and the generated
// unwrap:true read commands. The unwrap recognizes the Straddle envelope by
// meta+data co-presence (not a top-level "status"), so these cases pin the
// intended behavior and guard against the documented false positives.
func TestExtractResponseData(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// (a) Straddle array envelope {meta, response_type, data:[...]} → inner array
		{
			"straddle_array_envelope",
			`{"meta":{"api_request_id":"req-1"},"response_type":"array","data":[{"id":"a"},{"id":"b"}]}`,
			`[{"id":"a"},{"id":"b"}]`,
		},
		// (b) Straddle object envelope {meta, response_type, data:{...}} → inner object
		{
			"straddle_object_envelope",
			`{"meta":{"api_request_id":"req-1"},"response_type":"object","data":{"id":"abc","status":"created"}}`,
			`{"id":"abc","status":"created"}`,
		},
		// (c) Internal {meta, data} envelope (no response_type) → inner data
		// — mirrors the committed reports.create fixture shape.
		{
			"internal_meta_data_envelope",
			`{"meta":{"page":1,"page_size":2,"total_count":2},"data":[{"status":"verified","total":2},{"status":"review","total":1}]}`,
			`[{"status":"verified","total":2},{"status":"review","total":1}]`,
		},
		// (d) Straddle error envelope {meta, response_type:"error", error:{...}} (no data) → unchanged
		{
			"straddle_error_envelope",
			`{"meta":{"api_request_id":"req-1"},"response_type":"error","error":{"message":"bad"}}`,
			`{"meta":{"api_request_id":"req-1"},"response_type":"error","error":{"message":"bad"}}`,
		},
		// (e) Bare array (no envelope) → unchanged
		{
			"bare_array",
			`[{"id":"a"},{"id":"b"}]`,
			`[{"id":"a"},{"id":"b"}]`,
		},
		// (f) Stripe-style {"data":[...],"has_more":true} (no meta) → unchanged
		// — confirms the meta gate defeats the documented false positive where
		// "data" is the resource list, not an envelope wrapper.
		{
			"stripe_style_no_meta",
			`{"data":[{"id":"a"}],"has_more":true}`,
			`{"data":[{"id":"a"}],"has_more":true}`,
		},
		// (g) Webhook event payload {event_type, event_id, account_id, data:{...}} (no meta) → unchanged
		// — confirms the unreachable counterexample (inbound webhook bodies) is
		// also safe: lack of meta keeps the payload intact even if it ever
		// reached the helper.
		{
			"webhook_event_payload",
			`{"event_type":"account.event.v1","event_id":"evt-1","account_id":"acct-1","data":{"id":"acct-1"}}`,
			`{"event_type":"account.event.v1","event_id":"evt-1","account_id":"acct-1","data":{"id":"acct-1"}}`,
		},
		// meta present but data absent → unchanged (no inner data to unwrap to)
		{
			"meta_without_data",
			`{"meta":{"api_request_id":"req-1"},"response_type":"array"}`,
			`{"meta":{"api_request_id":"req-1"},"response_type":"array"}`,
		},
		// data present but meta absent → unchanged (meta gate prevents unwrap of
		// payloads where "data" may be a regular field)
		{
			"data_without_meta",
			`{"data":[{"id":"a"}],"has_more":true}`,
			`{"data":[{"id":"a"}],"has_more":true}`,
		},
		// Single-level unwrap only: envelope whose data itself contains a nested
		// "data" field is unwrapped once (outer envelope removed, inner preserved).
		{
			"single_level_unwrap",
			`{"meta":{"api_request_id":"req-1"},"data":{"data":[{"id":"a"}]}}`,
			`{"data":[{"id":"a"}]}`,
		},
		// Invalid JSON → unchanged
		{
			"invalid_json",
			`not-json`,
			`not-json`,
		},
		// Empty input → unchanged
		{
			"empty",
			``,
			``,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractResponseData(json.RawMessage(tc.in))
			// Compare by JSON value when both sides are valid JSON so whitespace
			// and key-order differences don't cause spurious failures; fall back to
			// a byte comparison for non-JSON inputs (invalid-empty cases).
			if json.Valid([]byte(tc.want)) && json.Valid(got) {
				var gv, wv any
				if err := json.Unmarshal(got, &gv); err != nil {
					t.Fatalf("extractResponseData(%s) returned invalid JSON: %v", tc.in, err)
				}
				if err := json.Unmarshal([]byte(tc.want), &wv); err != nil {
					t.Fatalf("want value is not valid JSON: %v", err)
				}
				gb, _ := json.Marshal(gv)
				wb, _ := json.Marshal(wv)
				if string(gb) != string(wb) {
					t.Errorf("extractResponseData(%s) = %s, want %s", tc.in, string(got), tc.want)
				}
				return
			}
			if string(got) != tc.want {
				t.Errorf("extractResponseData(%q) = %q, want %q", tc.in, string(got), tc.want)
			}
		})
	}
}
