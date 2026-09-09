// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeliverWebhookContentTypeMatchesBody(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		compact                 bool
	}{
		{"pretty JSON compact", "[\n  {\"id\":\"one\"},\n  {\"id\":\"two\"}\n]\n", "application/json", true},
		{"JSON object", "{\"id\":\"one\"}\n", "application/json", false},
		{"NDJSON compact", "{\"id\":\"one\"}\n{\"id\":\"two\"}\n", "application/x-ndjson", true},
		{"NDJSON", "{\"id\":\"one\"}\n{\"id\":\"two\"}\n", "application/x-ndjson", false},
		{"human output", "Archived 2 items\n", "text/plain; charset=utf-8", true},
		{"empty output", "", "text/plain; charset=utf-8", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body, contentType, method string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read webhook body: %v", err)
				}
				body, contentType, method = string(captured), r.Header.Get("Content-Type"), r.Method
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			if err := Deliver(DeliverSink{Scheme: "webhook", Target: server.URL}, []byte(tc.body), tc.compact); err != nil {
				t.Fatal(err)
			}
			if body != tc.body || contentType != tc.contentType || method != http.MethodPost {
				t.Fatalf("received %s %q with body %q; want POST %q with unchanged body", method, contentType, body, tc.contentType)
			}
		})
	}
}
