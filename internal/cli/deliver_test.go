// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliverWebhookContentTypeMatchesBody(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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

// TestPrintNDJSON verifies the NDJSON serializer directly: arrays become
// newline-delimited JSON (one compact value per line, no enclosing array),
// while non-arrays and empty arrays fall through to the standard JSON path.
func TestPrintNDJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		data    json.RawMessage
		handled bool
		want    string
	}{
		{"object not handled", json.RawMessage(`{"id":"a"}`), false, ""},
		{"empty array not handled", json.RawMessage(`[]`), false, ""},
		{"scalar not handled", json.RawMessage(`"hello"`), false, ""},
		{"array of objects", json.RawMessage(`[{"id":"a"},{"id":"b"}]`), true, "{\"id\":\"a\"}\n{\"id\":\"b\"}\n"},
		{"newline in string stays single-line", json.RawMessage(`[{"d":"line1\nline2"}]`), true, "{\"d\":\"line1\\nline2\"}\n"},
		{"array of primitives is valid ndjson", json.RawMessage(`[1,2,3]`), true, "1\n2\n3\n"},
		{"nested object preserved", json.RawMessage(`[{"id":"a","inner":{"k":1}}]`), true, "{\"id\":\"a\",\"inner\":{\"k\":1}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			handled, err := printNDJSON(&buf, tc.data)
			if err != nil {
				t.Fatalf("printNDJSON: %v", err)
			}
			if handled != tc.handled {
				t.Fatalf("handled = %v, want %v", handled, tc.handled)
			}
			if !tc.handled {
				if buf.Len() != 0 {
					t.Fatalf("non-handled case wrote %q; want no writes", buf.String())
				}
				return
			}
			if buf.String() != tc.want {
				t.Fatalf("got %q, want %q", buf.String(), tc.want)
			}
			// Every emitted line must be a single, complete JSON value.
			for i, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
				if line == "" {
					continue
				}
				if !json.Valid([]byte(line)) {
					t.Errorf("line %d not valid JSON: %q", i, line)
				}
			}
		})
	}
}

// TestCompactDeliverEmitsNDJSONEndToEnd is the regression test for the
// --compact --deliver webhook:<url> bug. It runs the real output pipeline
// (printOutputWithFlags feeding the --deliver MultiWriter) and then posts
// the captured buffer via Deliver to an httptest server, asserting the
// Content-Type and body framing agree. The compact multi-record list must
// arrive as real NDJSON advertised as application/x-ndjson; single objects,
// empty lists, non-compact lists, csv, and quiet keep their existing
// framing and Content-Type (no regressions from the previous stopgap).
func TestCompactDeliverEmitsNDJSONEndToEnd(t *testing.T) {
	cases := []struct {
		name           string
		data           json.RawMessage
		flags          rootFlags
		wantCT         string
		wantNDJSON     bool
		wantSingleJSON bool
	}{
		{
			name:       "compact multi-record list -> NDJSON",
			data:       json.RawMessage(`[{"id":"a","status":"active","description":"x"},{"id":"b","status":"pending","description":"y"}]`),
			flags:      rootFlags{compact: true},
			wantCT:     "application/x-ndjson",
			wantNDJSON: true,
		},
		{
			name:           "compact single object -> JSON",
			data:           json.RawMessage(`{"id":"a","status":"active"}`),
			flags:          rootFlags{compact: true},
			wantCT:         "application/json",
			wantSingleJSON: true,
		},
		{
			name:           "compact empty list -> JSON",
			data:           json.RawMessage(`[]`),
			flags:          rootFlags{compact: true},
			wantCT:         "application/json",
			wantSingleJSON: true,
		},
		{
			name:           "non-compact list -> JSON array",
			data:           json.RawMessage(`[{"id":"a"},{"id":"b"}]`),
			flags:          rootFlags{},
			wantCT:         "application/json",
			wantSingleJSON: true,
		},
		{
			name:   "compact+csv list -> text/plain CSV body",
			data:   json.RawMessage(`[{"id":"a"},{"id":"b"}]`),
			flags:  rootFlags{compact: true, csv: true},
			wantCT: "text/plain; charset=utf-8",
		},
		{
			name:   "compact+quiet -> empty body text/plain",
			data:   json.RawMessage(`[{"id":"a"},{"id":"b"}]`),
			flags:  rootFlags{compact: true, quiet: true},
			wantCT: "text/plain; charset=utf-8",
		},
		{
			name:       "compact+select list -> NDJSON (compact framing honored with select)",
			data:       json.RawMessage(`[{"id":"a","name":"x","verbose":"drop"},{"id":"b","name":"y","verbose":"drop"}]`),
			flags:      rootFlags{compact: true, selectFields: "id,name"},
			wantCT:     "application/x-ndjson",
			wantNDJSON: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.flags.deliverBuf = &bytes.Buffer{}
			w := io.MultiWriter(io.Discard, tc.flags.deliverBuf)

			prev := isTerminalOverride
			off := false
			isTerminalOverride = &off
			defer func() { isTerminalOverride = prev }()

			if err := printOutputWithFlags(w, tc.data, &tc.flags); err != nil {
				t.Fatalf("printOutputWithFlags: %v", err)
			}
			body := tc.flags.deliverBuf.Bytes()

			var gotCT, gotMethod string
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				gotCT = r.Header.Get("Content-Type")
				gotMethod = r.Method
				rw.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			if err := Deliver(DeliverSink{Scheme: "webhook", Target: server.URL}, body, tc.flags.compact); err != nil {
				t.Fatalf("Deliver: %v", err)
			}

			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotCT != tc.wantCT {
				t.Errorf("Content-Type = %q, want %q (body=%q)", gotCT, tc.wantCT, gotBody)
			}
			switch {
			case tc.wantNDJSON:
				if len(gotBody) == 0 {
					t.Fatalf("expected NDJSON body, got empty")
				}
				trimmed := strings.TrimSpace(string(gotBody))
				if strings.HasPrefix(trimmed, "[") {
					t.Errorf("NDJSON body must not be an enclosing JSON array; got %q", gotBody)
				}
				lines := strings.Split(strings.TrimRight(string(gotBody), "\n"), "\n")
				if len(lines) < 2 {
					t.Errorf("expected >=2 NDJSON records, got %d (%q)", len(lines), gotBody)
				}
				for i, ln := range lines {
					if ln == "" {
						continue
					}
					if !json.Valid([]byte(ln)) {
						t.Errorf("NDJSON line %d not valid JSON: %q", i, ln)
					}
				}
			case tc.wantSingleJSON:
				if !json.Valid(gotBody) {
					t.Errorf("expected a single valid JSON value; got %q", gotBody)
				}
			}
		})
	}
}

// TestCompactDeliverScopedToSink verifies NDJSON emission only happens when
// a --deliver sink is wired (deliverBuf != nil). Plain --compact piped to a
// non-terminal (no --deliver) must keep the existing pretty JSON array, so
// the fix does not change behavior for `straddle <list> --compact > out.json`.
//
// No t.Parallel(): mutates the package-global isTerminalOverride.
func TestCompactDeliverScopedToSink(t *testing.T) {
	data := json.RawMessage(`[{"id":"a"},{"id":"b"}]`)
	var buf bytes.Buffer

	prev := isTerminalOverride
	off := false
	isTerminalOverride = &off
	defer func() { isTerminalOverride = prev }()

	flags := &rootFlags{compact: true} // deliverBuf nil => no --deliver sink
	if err := printOutputWithFlags(&buf, data, flags); err != nil {
		t.Fatalf("printOutputWithFlags: %v", err)
	}
	got := buf.String()
	trimmed := strings.TrimSpace(got)
	if !strings.HasPrefix(trimmed, "[") {
		t.Fatalf("compact-without-deliver must stay a JSON array; got %q", got)
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("compact-without-deliver output not a single valid JSON value: %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected indented JSON array on non-terminal, got single line %q", got)
	}
}

// isolateDeliverConfig wires the minimal config isolation needed for a
// RootCmd() execution that hits a mock API + webhook receiver, mirroring
// isolateAPIConfig from api_passthrough_test.go.
func isolateDeliverConfig(t *testing.T, apiURL string) {
	t.Helper()
	t.Setenv("STRADDLE_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	t.Setenv("STRADDLE_API_KEY", "test_key")
	t.Setenv("STRADDLE_BASE_URL", apiURL)
	t.Setenv("STRADDLE_VERIFY", "")
	t.Setenv("STRADDLE_VERIFY_LIVE_HTTP", "")
	t.Setenv("STRADDLE_ENVIRONMENT", "sandbox")
}

// captureStdout swaps os.Stdout to a buffer for the duration of fn so the
// --deliver MultiWriter (which writes to os.Stdout + deliverBuf) does not
// pollute test output. Returns whatever was written to stdout.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() { _, _ = io.Copy(&buf, r); close(done) }()
	runErr := fn()
	_ = w.Close()
	os.Stdout = orig
	<-done
	_ = r.Close()
	return buf.String(), runErr
}

// runDeliverCommand is reserved for future consolidation; the per-test
// helpers below inline the newRootCmd+Deliver pattern directly so each test
// owns its webhook capture closures.

// TestCompactDeliverGeneratedListNDJSON drives the full command tree for a
// generated list command (customers list) against a mock API returning a JSON
// array, with --compact --deliver webhook:<receiver>. It asserts the webhook
// receives real NDJSON advertised as application/x-ndjson (header/body
// agreement on the primary user-facing list path, which builds a provenance
// envelope and calls printOutput directly — bypassing printOutputWithFlags).
//
// No t.Parallel(): swaps os.Stdout via captureStdout.
func TestCompactDeliverGeneratedListNDJSON(t *testing.T) {
	isolateDeliverConfig(t, "")

	apiHit := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/customers" {
			t.Errorf("api path = %s, want /v1/customers", r.URL.Path)
		}
		apiHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"cus_a","name":"Alice","description":"drop"},{"id":"cus_b","name":"Bob","description":"drop"}]`))
	}))
	defer api.Close()
	t.Setenv("STRADDLE_BASE_URL", api.URL)

	var gotCT string
	var gotBody []byte
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hook.Close()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{"--compact", "--no-cache", "--data-source", "live", "--deliver", "webhook:" + hook.URL, "customers", "list"})
	if _, err := captureStdout(t, cmd.Execute); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if flags.deliverBuf == nil {
		t.Fatal("deliverBuf not wired")
	}
	if err := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !apiHit {
		t.Fatal("mock API was not hit")
	}
	assertNDJSONDelivery(t, gotCT, gotBody, 2, []string{`"id":"cus_a"`, `"id":"cus_b"`})
}

// TestCompactDeliverAPIPassthroughNDJSON drives `straddle api get` (which
// builds a {data,method,path,success} envelope via printAPIPassthroughEnvelope)
// with --compact --deliver webhook:<receiver> and asserts real NDJSON is
// delivered as application/x-ndjson.
//
// No t.Parallel(): swaps os.Stdout via captureStdout.
func TestCompactDeliverAPIPassthroughNDJSON(t *testing.T) {
	isolateDeliverConfig(t, "")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"x_1","name":"One","verbose":"drop"},{"id":"x_2","name":"Two","verbose":"drop"}]`))
	}))
	defer api.Close()
	t.Setenv("STRADDLE_BASE_URL", api.URL)

	var gotCT string
	var gotBody []byte
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hook.Close()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{"--compact", "--no-cache", "--data-source", "live", "--deliver", "webhook:" + hook.URL, "api", "get", "/v1/widgets"})
	if _, err := captureStdout(t, cmd.Execute); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	assertNDJSONDelivery(t, gotCT, gotBody, 2, []string{`"id":"x_1"`, `"id":"x_2"`})
}

// TestCompactDeliverSingleObjectStaysJSON verifies a single-object get via the
// generated path under --compact --deliver still delivers a JSON envelope
// (application/json), not NDJSON — the fix must not collapse single-object
// responses to NDJSON.
//
// No t.Parallel(): swaps os.Stdout via captureStdout.
func TestCompactDeliverSingleObjectStaysJSON(t *testing.T) {
	isolateDeliverConfig(t, "")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cus_a","name":"Alice","description":"drop"}`))
	}))
	defer api.Close()
	t.Setenv("STRADDLE_BASE_URL", api.URL)

	var gotCT string
	var gotBody []byte
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hook.Close()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{"--compact", "--no-cache", "--data-source", "live", "--deliver", "webhook:" + hook.URL, "customers", "get", "cus_a"})
	if _, err := captureStdout(t, cmd.Execute); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json for single object", gotCT)
	}
	if !json.Valid(gotBody) {
		t.Errorf("expected a single valid JSON value; got %q", gotBody)
	}
	if !bytes.Contains(gotBody, []byte(`cus_a`)) {
		t.Errorf("delivered body missing the record: %q", gotBody)
	}
}

// assertNDJSONDelivery asserts a webhook received real NDJSON (Content-Type
// application/x-ndjson, nRecord lines each a valid JSON value, each expected
// substring present, no enclosing array).
func assertNDJSONDelivery(t *testing.T, ct string, body []byte, nRecord int, wantSubstrings []string) {
	t.Helper()
	if ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", ct)
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		t.Errorf("NDJSON body must not be an enclosing JSON array; got %q", body)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != nRecord {
		t.Errorf("expected %d NDJSON records, got %d (%q)", nRecord, len(lines), body)
	}
	for i, ln := range lines {
		if ln == "" {
			continue
		}
		if !json.Valid([]byte(ln)) {
			t.Errorf("NDJSON line %d not valid JSON: %q", i, ln)
		}
	}
	for _, want := range wantSubstrings {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("NDJSON body missing %q; got %q", want, body)
		}
	}
}
