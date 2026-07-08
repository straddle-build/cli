// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/straddle-build/cli/internal/config"
)

// TestClient_CheckRedirectCredentialPolicy pins the redirect credential
// boundary. Go copies Authorization before CheckRedirect and only strips it
// on host changes, so the downgrade case must start with the header already
// present and prove our hook actively deletes it.
func TestClient_CheckRedirectCredentialPolicy(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{
		BaseURL:       "https://api.straddle.com",
		AuthHeaderVal: "Bearer secret123",
	}, time.Second, 0)

	cases := []struct {
		name      string
		from      string
		to        string
		startAuth string
		wantAuth  string
	}{
		{
			name:      "same host https downgrade drops copied credential",
			from:      "https://api.straddle.com/original",
			to:        "http://api.straddle.com/redirected",
			startAuth: "Bearer secret123",
		},
		{
			name:      "same host http keeps credential for loopback-style mocks",
			from:      "http://api.straddle.com/original",
			to:        "http://api.straddle.com/redirected",
			startAuth: "Bearer secret123",
			wantAuth:  "Bearer secret123",
		},
		{
			name:      "same host https keeps credential",
			from:      "https://api.straddle.com/original",
			to:        "https://api.straddle.com/redirected",
			startAuth: "Bearer secret123",
			wantAuth:  "Bearer secret123",
		},
		{
			name: "cross host https does not add credential",
			from: "https://api.straddle.com/original",
			to:   "https://evil.example/redirected",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := http.NewRequest(http.MethodGet, tc.from, nil)
			if err != nil {
				t.Fatalf("original request: %v", err)
			}
			target, err := http.NewRequest(http.MethodGet, tc.to, nil)
			if err != nil {
				t.Fatalf("target request: %v", err)
			}
			if tc.startAuth != "" {
				target.Header.Set("Authorization", tc.startAuth)
			}

			if err := client.HTTPClient.CheckRedirect(target, []*http.Request{original}); err != nil {
				t.Fatalf("CheckRedirect() error = %v", err)
			}
			if got := target.Header.Get("Authorization"); got != tc.wantAuth {
				t.Fatalf("Authorization after redirect = %q, want %q", got, tc.wantAuth)
			}
		})
	}
}

// TestClient_APIErrorRedactsReflectedBearerToken exercises the real HTTP error
// path: a hostile or misconfigured server that reflects a credential in a 4xx
// body must not leak that token through APIError.Error().
func TestClient_APIErrorRedactsReflectedBearerToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"msg":"echo Authorization: Bearer abc123"}`))
	}))
	defer server.Close()

	client := New(&config.Config{
		BaseURL:       server.URL,
		AuthHeaderVal: "Bearer abc123",
	}, time.Second, 0)
	client.NoCache = true

	_, err := client.Get("/leak", nil)
	if err == nil {
		t.Fatal("Get() error = nil, want APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("Get() error type = %T, want *APIError", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "abc123") {
		t.Fatalf("API error leaked bearer token: %q", msg)
	}
	if !strings.Contains(msg, "[REDACTED]") {
		t.Fatalf("API error = %q, want redaction marker", msg)
	}
}

func TestClient_GETCacheKeyIncludesHeaderOverrides(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tenant":%q}`, r.Header.Get("X-Tenant"))
	}))
	defer server.Close()

	client := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	client.cacheDir = t.TempDir()

	first, err := client.GetWithHeaders("/cache", nil, map[string]string{"X-Tenant": "one"})
	if err != nil {
		t.Fatalf("first GetWithHeaders: %v", err)
	}
	second, err := client.GetWithHeaders("/cache", nil, map[string]string{"X-Tenant": "two"})
	if err != nil {
		t.Fatalf("second GetWithHeaders: %v", err)
	}
	if !strings.Contains(string(first), `"tenant":"one"`) {
		t.Fatalf("first response = %s, want tenant one", first)
	}
	if !strings.Contains(string(second), `"tenant":"two"`) {
		t.Fatalf("second response = %s, want tenant two", second)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 distinct cache entries", requests)
	}
}

func TestClient_GETCacheKeyIncludesConfigHeaders(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"account":%q}`, r.Header.Get("Straddle-Account-Id"))
	}))
	defer server.Close()

	cfg := &config.Config{
		BaseURL: server.URL,
		Headers: map[string]string{
			"Straddle-Account-Id": "acct_one",
		},
	}
	client := New(cfg, time.Second, 0)
	client.cacheDir = t.TempDir()

	first, err := client.Get("/cache", nil)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	cfg.Headers["Straddle-Account-Id"] = "acct_two"
	second, err := client.Get("/cache", nil)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if !strings.Contains(string(first), `"account":"acct_one"`) {
		t.Fatalf("first response = %s, want acct_one", first)
	}
	if !strings.Contains(string(second), `"account":"acct_two"`) {
		t.Fatalf("second response = %s, want acct_two", second)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 distinct cache entries", requests)
	}
}
