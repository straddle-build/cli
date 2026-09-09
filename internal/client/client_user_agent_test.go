// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/straddle-build/straddle-cli/internal/config"
)

// The Straddle API answers HTTP 500 "FormatException" to a User-Agent
// that is not an RFC 9110 product token, which is what the previous
// module-path value ("github.com/straddle-build/straddle-cli/v1")
// produced. Every request must carry the product/version form.
func TestClient_SendsProductTokenUserAgent(t *testing.T) {
	t.Parallel()

	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	client.cacheDir = t.TempDir()
	client.Version = "1.2.3"

	if _, err := client.Get("/v1/ping", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "straddle-cli/1.2.3" {
		t.Fatalf("User-Agent = %q, want straddle-cli/1.2.3", got)
	}
}

func TestUserAgent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		version string
		comment string
		want    string
	}{
		{"product only", "", "", "straddle-cli"},
		{"product and version", "1.0.2", "", "straddle-cli/1.0.2"},
		{"product, version, and comment", "dev", "deliver", "straddle-cli/dev (deliver)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := UserAgent(tc.version, tc.comment); got != tc.want {
				t.Fatalf("UserAgent(%q, %q) = %q, want %q", tc.version, tc.comment, got, tc.want)
			}
		})
	}
}

func TestClient_PreservesConfiguredUserAgentAndAuth(t *testing.T) {
	headers := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c := New(&config.Config{BaseURL: server.URL, AuthHeaderVal: "Bearer fixture-auth", Headers: map[string]string{"User-Agent": "custom-client/2"}}, time.Second, 0)
	c.NoCache = true
	c.Version = "1.2.3"
	if _, err := c.Get("/test", nil); err != nil {
		t.Fatal(err)
	}
	got := <-headers
	if got.Get("User-Agent") != "custom-client/2" || got.Get("Authorization") != "Bearer fixture-auth" {
		t.Fatal("configured User-Agent or authentication changed")
	}
}
