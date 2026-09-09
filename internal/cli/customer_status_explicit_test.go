// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomerUpdateRequiresExplicitStatus(t *testing.T) {
	cases := []struct {
		name       string
		profile    map[string]string
		args       []string
		stdin      string
		wantStatus string
		wantError  bool
		dryRun     bool
	}{
		{name: "omitted", wantError: true},
		{name: "omitted dry run", wantError: true, dryRun: true},
		{name: "profile status", profile: map[string]string{"status": "review"}, wantStatus: "review"},
		{name: "flag overrides profile", profile: map[string]string{"status": "review"}, args: []string{"--status", "verified"}, wantStatus: "verified"},
		{name: "stdin ignores profile status", profile: map[string]string{"status": "review"}, args: []string{"--stdin"}, stdin: `{}`, wantError: true},
		{name: "verified", args: []string{"--status", "verified"}, wantStatus: "verified"},
		{name: "review", args: []string{"--status", "review"}, wantStatus: "review"},
		{name: "invalid", args: []string{"--status", "invalid"}, wantError: true},
		{name: "empty", args: []string{"--status="}, wantError: true},
		{name: "stdin omitted", args: []string{"--stdin"}, stdin: `{"email":"updated@example.com"}`, wantError: true},
		{name: "stdin cannot use flag", args: []string{"--stdin", "--status", "verified"}, stdin: `{}`, wantError: true},
		{name: "stdin null", args: []string{"--stdin"}, stdin: `{"status":null}`, wantError: true},
		{name: "stdin invalid", args: []string{"--stdin"}, stdin: `{"status":"invalid"}`, wantError: true},
		{name: "stdin wrong type", args: []string{"--stdin"}, stdin: `{"status":true}`, wantError: true},
		{name: "stdin explicit", args: []string{"--stdin"}, stdin: `{"status":"review"}`, wantStatus: "review"},
		{name: "dry run explicit", args: []string{"--status", "review"}, wantStatus: "review", dryRun: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				received <- body
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"id":"updated"}}`))
			}))
			defer server.Close()
			isolateSurfaceConfig(t, server.URL)
			t.Setenv("HOME", t.TempDir())
			if tc.profile != nil {
				if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{"update": {Name: "update", Values: tc.profile}}}); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--agent", "--no-cache", "customers", "update", goldenUUID,
				"--email", "updated@example.com", "--name", "Updated Customer", "--phone", "+15555550100", "--device-ip-address", "192.0.2.1"}
			args = append(args, tc.args...)
			if tc.profile != nil {
				args = append(args, "--profile", "update")
			}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}
			_, _, err := runRootForAPITest(t, args, tc.stdin)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "status") {
					t.Errorf("error = %v, want explicit status error", err)
				}
				if len(received) != 0 {
					t.Error("invalid status reached the API")
				}
				return
			}
			if err != nil {
				t.Fatalf("update: %v", err)
			}
			if tc.dryRun {
				if len(received) != 0 {
					t.Fatal("dry run reached the API")
				}
				return
			}
			select {
			case body := <-received:
				if body["status"] != tc.wantStatus {
					t.Errorf("status = %v, want %s", body["status"], tc.wantStatus)
				}
			default:
				t.Fatal("update did not reach the API")
			}
		})
	}
}
