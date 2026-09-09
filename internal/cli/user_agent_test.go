// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every API command must report the CLI version through a product-token
// User-Agent; the Straddle API rejects module-path values with HTTP 500.
func TestAPICommands_SendVersionedUserAgent(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")

	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"ch_1"},"meta":{},"response_type":"object"}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	if _, _, err := runRootForAPITest(t, []string{"--json", "--data-source", "live", "--no-cache", "charges", "get", "ch_1"}, ""); err != nil {
		t.Fatalf("charges get returned error: %v", err)
	}
	if want := "straddle-cli/" + Version(); got != want {
		t.Fatalf("User-Agent = %q, want %q", got, want)
	}
}

func TestNonAPIRequestsSendVersionedUserAgent(t *testing.T) {
	for _, subsystem := range []string{"deliver", "feedback"} {
		t.Run(subsystem, func(t *testing.T) {
			headers := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Get("User-Agent")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			var err error
			if subsystem == "deliver" {
				err = deliverWebhook(server.URL, []byte(`{}`), false)
			} else {
				err = postFeedback(server.URL, FeedbackEntry{Text: "fixture"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, want := <-headers, "straddle-cli/"+Version()+" ("+subsystem+")"; got != want {
				t.Fatalf("User-Agent = %q, want %q", got, want)
			}
		})
	}
}
