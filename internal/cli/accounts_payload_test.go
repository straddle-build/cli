// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountAddressPayloadIncludesLegacyAndCanonicalFields(t *testing.T) {
	address := map[string]string{
		"city":        "Denver",
		"state":       "CO",
		"line1":       "123 Main St",
		"address1":    "123 Main St",
		"postal_code": "80202",
		"zip":         "80202",
	}
	tests := []struct {
		name   string
		method string
		path   string
		args   []string
		status int
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/v1/accounts",
			status: http.StatusCreated,
			args: []string{
				"accounts", "create",
				"--access-level", "standard",
				"--account-type", "business",
				"--business-profile-address-city", "Denver",
				"--business-profile-address-line1", "123 Main St",
				"--business-profile-address-postal-code", "80202",
				"--business-profile-address-state", "CO",
				"--business-profile-name", "Acme",
				"--business-profile-website", "https://example.com",
				"--organization-id", "org_1",
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/v1/accounts/acct_1",
			status: http.StatusOK,
			args: []string{
				"accounts", "update", "acct_1",
				"--business-profile-address-city", "Denver",
				"--business-profile-address-line1", "123 Main St",
				"--business-profile-address-postal-code", "80202",
				"--business-profile-address-state", "CO",
				"--business-profile-name", "Acme",
				"--business-profile-website", "https://example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAPIConfig(t)
			t.Setenv("STRADDLE_API_KEY", "test_key")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Fatalf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tt.path)
				}

				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				businessProfile, ok := body["business_profile"].(map[string]any)
				if !ok {
					t.Fatalf("business_profile = %T, want object", body["business_profile"])
				}
				gotAddress, ok := businessProfile["address"].(map[string]any)
				if !ok {
					t.Fatalf("address = %T, want object", businessProfile["address"])
				}
				for name, want := range address {
					if got := gotAddress[name]; got != want {
						t.Fatalf("address[%q] = %v, want %s", name, got, want)
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"id":"acct_1"}`))
			}))
			defer server.Close()
			t.Setenv("STRADDLE_BASE_URL", server.URL)

			args := append([]string{"--json", "--data-source", "live", "--no-cache"}, tt.args...)
			if _, _, err := runRootForAPITest(t, args, ""); err != nil {
				t.Fatalf("command returned error: %v", err)
			}
		})
	}
}
