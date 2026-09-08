// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/straddle-build/straddle-cli/internal/client"
	"github.com/straddle-build/straddle-cli/internal/config"
)

// newTailTestClient builds a *client.Client pointed at baseURL the same way
// flags.newClient() does, with caching disabled so each call hits the
// server. Auth is a static bearer token the test server ignores.
func newTailTestClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	cfg := &config.Config{BaseURL: baseURL, StraddleApiKey: "test_key"}
	c := client.New(cfg, 30*time.Second, 0)
	c.NoCache = true
	return c
}

// newTailTestServer returns an httptest.Server that answers 200 + body for
// exactly wantPath and 404 for any other path. tail poll requests depend on
// the path the client builds, so a 200 only at wantPath proves the URL was
// constructed correctly end-to-end through the client's buildURL helper.
func newTailTestServer(t *testing.T, wantPath, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// decodeTailEvents splits the NDJSON stream fetchAndEmit writes into parsed
// event objects so tests can count events and inspect their payloads.
func decodeTailEvents(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("invalid NDJSON event: %v\nline=%s", err, string(line))
		}
		events = append(events, ev)
	}
	return events
}

// TestTailKnownResourcesAdvertisesOnlyStreamable verifies the no-arg help
// envelope only lists resources tail can actually stream: the flat-listable
// set (defaultSyncResources == the keys of syncResourcePath). The six
// resources without a flat GET list endpoint that the old code advertised
// must be absent so `tail --json` no longer sends agents toward resources
// that would always fail.
func TestTailKnownResourcesAdvertisesOnlyStreamable(t *testing.T) {
	t.Parallel()
	got := tailKnownResources()
	want := defaultSyncResources()
	if len(got) != len(want) {
		t.Fatalf("tailKnownResources has %d entries, want %d; got=%v", len(got), len(want), got)
	}
	gotSet := make(map[string]bool, len(got))
	for _, r := range got {
		gotSet[r] = true
	}
	for _, r := range want {
		if !gotSet[r] {
			t.Errorf("tailKnownResources missing streamable resource %q", r)
		}
	}
	for _, r := range got {
		if _, err := syncResourcePath(r); err != nil {
			t.Errorf("tailKnownResources lists %q but syncResourcePath rejects it: %v", r, err)
		}
	}
	for _, r := range []string{"account-settings", "bridge", "charges", "funding-event-payments", "payouts", "reports"} {
		if gotSet[r] {
			t.Errorf("tailKnownResources must not advertise non-listable resource %q", r)
		}
	}
	if !gotSet["funding-events"] || !gotSet["linked-bank-accounts"] {
		t.Errorf("tailKnownResources must keep the hyphenated flat-listable resources; got=%v", got)
	}
}

// TestTailNoArgJSONListsStreamableResources drives the real RunE for the
// `tail --json` (no resource) discovery branch and asserts the JSON
// envelope advertises exactly the streamable set (the 8 flat-listable
// resources, not the old 14).
func TestTailNoArgJSONListsStreamableResources(t *testing.T) {
	isolateAPIConfig(t)

	stdout, _, err := runRootForAPITest(t, []string{"--json", "tail"}, "")
	if err != nil {
		t.Fatalf("tail --json returned error: %v", err)
	}
	env := decodeAPIEnvelope(t, stdout)
	resources, ok := env["resources"].([]any)
	if !ok {
		t.Fatalf("envelope missing resources array; got: %v", env)
	}
	want := defaultSyncResources()
	if len(resources) != len(want) {
		t.Fatalf("envelope lists %d resources, want %d; resources=%v", len(resources), len(want), resources)
	}
	gotSet := make(map[string]bool, len(resources))
	for _, r := range resources {
		s, _ := r.(string)
		gotSet[s] = true
	}
	for _, r := range want {
		if !gotSet[r] {
			t.Errorf("envelope missing streamable resource %q; got=%v", r, resources)
		}
	}
	for _, r := range []string{"account-settings", "bridge", "charges", "funding-event-payments", "payouts", "reports"} {
		if gotSet[r] {
			t.Errorf("envelope must not advertise non-listable resource %q", r)
		}
	}
}

// TestTailRejectsUnknownResource asserts the six previously-advertised
// non-listable resources (and a bogus name) now produce a clear up-front
// error from RunE that names the resource, before any network call,
// rather than silently looping on "poll failed" warnings with no events.
func TestTailRejectsUnknownResource(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")

	for _, resource := range []string{
		"account-settings",
		"bridge",
		"charges",
		"funding-event-payments",
		"payouts",
		"reports",
		"nonexistent-thing",
	} {
		resource := resource
		t.Run(resource, func(t *testing.T) {
			_, _, err := runRootForAPITest(t, []string{"tail", resource}, "")
			if err == nil {
				t.Fatal("tail accepted an unknown resource; expected a clear error")
			}
			if !strings.Contains(err.Error(), "unknown tail resource") {
				t.Errorf("error = %q, want it to mention \"unknown tail resource\"", err.Error())
			}
			if !strings.Contains(err.Error(), resource) {
				t.Errorf("error = %q, want it to name the resource %q", err.Error(), resource)
			}
		})
	}
}

// TestTailFetchAndEmitVersionedPathEmitsEvents is the end-to-end httptest
// from the bug report: a real client.Client pointed at a test server that
// returns 200 + a bare JSON array for /v1/accounts and 404 for /accounts.
// With the versioned path fetchAndEmit emits one NDJSON event per array
// element; with the unversioned path the 4xx makes c.Get return an error
// and fetchAndEmit emits zero events. This is the exact chain tail's
// polling loop drives (syncResourcePath -> fetchAndEmit -> client.Get ->
// buildURL -> http), so it pins the version-segment half of the fix.
func TestTailFetchAndEmitVersionedPathEmitsEvents(t *testing.T) {
	server := newTailTestServer(t, "/v1/accounts", `[{"id":"acc_1"},{"id":"acc_2"}]`)
	defer server.Close()
	c := newTailTestClient(t, server.URL)

	t.Run("versioned path emits one event per item", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := fetchAndEmit(c, "/v1/accounts", enc); err != nil {
			t.Fatalf("fetchAndEmit returned error: %v", err)
		}
		events := decodeTailEvents(t, buf.Bytes())
		if len(events) != 2 {
			t.Fatalf("got %d events, want 2; output=%s", len(events), buf.String())
		}
	})

	t.Run("unversioned path 404s and emits zero events", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		err := fetchAndEmit(c, "/accounts", enc)
		if err == nil {
			t.Fatal("fetchAndEmit(/accounts) returned nil error; want a 404 error")
		}
		if buf.Len() != 0 {
			t.Fatalf("expected zero events on 404, got: %s", buf.String())
		}
	})
}

// TestTailFetchAndEmitHyphenUnderscoreMapping pins the hyphen->underscore
// half of the fix: the API only answers /v1/funding_events (underscore),
// not /v1/funding-events (hyphen). tail now resolves "funding-events" to
// the underscored path via syncResourcePath, so polling succeeds; the old
// "/" + resource construction kept the hyphen and missed the endpoint.
func TestTailFetchAndEmitHyphenUnderscoreMapping(t *testing.T) {
	server := newTailTestServer(t, "/v1/funding_events", `[{"id":"fe_1"},{"id":"fe_2"},{"id":"fe_3"}]`)
	defer server.Close()
	c := newTailTestClient(t, server.URL)

	t.Run("underscore path emits events", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := fetchAndEmit(c, "/v1/funding_events", enc); err != nil {
			t.Fatalf("fetchAndEmit returned error: %v", err)
		}
		if got := len(decodeTailEvents(t, buf.Bytes())); got != 3 {
			t.Fatalf("got %d events, want 3; output=%s", got, buf.String())
		}
	})

	t.Run("hyphen path 404s and emits zero events", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := fetchAndEmit(c, "/v1/funding-events", enc); err == nil {
			t.Fatal("fetchAndEmit(/v1/funding-events) returned nil error; want a 404 error")
		}
		if buf.Len() != 0 {
			t.Fatalf("expected zero events on hyphen path, got: %s", buf.String())
		}
	})
}

// TestTailFetchAndEmitLinkedBankAccountsUnderscoreMapping repeats the
// hyphen->underscore assertion for the other hyphenated flat-listable
// resource (linked-bank-accounts -> /v1/linked_bank_accounts), guarding
// against a fix that hard-codes funding-events alone.
func TestTailFetchAndEmitLinkedBankAccountsUnderscoreMapping(t *testing.T) {
	server := newTailTestServer(t, "/v1/linked_bank_accounts", `[{"id":"lba_1"}]`)
	defer server.Close()
	c := newTailTestClient(t, server.URL)

	t.Run("underscore path emits events", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := fetchAndEmit(c, "/v1/linked_bank_accounts", enc); err != nil {
			t.Fatalf("fetchAndEmit returned error: %v", err)
		}
		if got := len(decodeTailEvents(t, buf.Bytes())); got != 1 {
			t.Fatalf("got %d events, want 1; output=%s", got, buf.String())
		}
	})

	t.Run("hyphen path 404s and emits zero events", func(t *testing.T) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := fetchAndEmit(c, "/v1/linked-bank-accounts", enc); err == nil {
			t.Fatal("fetchAndEmit(/v1/linked-bank-accounts) returned nil error; want a 404 error")
		}
		if buf.Len() != 0 {
			t.Fatalf("expected zero events on hyphen path, got: %s", buf.String())
		}
	})
}

// TestTailResourceToPathIntegration composes the two steps tail's RunE
// performs — syncResourcePath(resource) then fetchAndEmit(c, path, enc) —
// for every advertised resource against a server that 200s only at the
// resolved path. This is the closest deterministic check that "tail
// <resource>" reaches the API's versioned, correctly-underscored endpoint
// for each of the 8 flat-listable resources, without driving the polling
// loop (which would require an OS signal to stop).
func TestTailResourceToPathIntegration(t *testing.T) {
	for _, resource := range defaultSyncResources() {
		resource := resource
		path, err := syncResourcePath(resource)
		if err != nil {
			t.Fatalf("syncResourcePath(%q) error: %v", resource, err)
		}
		// Per-resource server that answers 200 only at the resolved path,
		// so a 200 + events proves the URL tail would build is correct.
		body := `[{"id":"x_1"},{"id":"x_2"}]`
		t.Run(resource, func(t *testing.T) {
			server := newTailTestServer(t, path, body)
			defer server.Close()
			c := newTailTestClient(t, server.URL)

			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err := fetchAndEmit(c, path, enc); err != nil {
				t.Fatalf("fetchAndEmit(%q) error: %v", path, err)
			}
			if got := len(decodeTailEvents(t, buf.Bytes())); got != 2 {
				t.Fatalf("fetchAndEmit(%q) emitted %d events, want 2; output=%s", path, got, buf.String())
			}
		})
	}
}
