// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/cli/internal/straddleacct"
)

func runRootForAPITest(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func isolateAPIConfig(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("STRADDLE_CONFIG", filepath.Join(configDir, "config.toml"))
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(configDir, "platform.toml"))
	t.Setenv("STRADDLE_API_KEY", "")
	t.Setenv("STRADDLE_BASE_URL", "")
	t.Setenv("STRADDLE_VERIFY", "")
	t.Setenv("STRADDLE_VERIFY_LIVE_HTTP", "")
}

func decodeAPIEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("output is not a JSON envelope: %v\nraw: %s", err, raw)
	}
	return env
}

func TestAPIDiscoveryPreserved(t *testing.T) {
	isolateAPIConfig(t)

	stdout, _, err := runRootForAPITest(t, []string{"api"}, "")
	if err != nil {
		t.Fatalf("api discovery returned error: %v", err)
	}
	if !strings.Contains(stdout, "Available API interfaces") || !strings.Contains(stdout, "charges") {
		t.Fatalf("api discovery output did not list hidden interfaces; output: %s", stdout)
	}

	stdout, _, err = runRootForAPITest(t, []string{"api", "charges"}, "")
	if err != nil {
		t.Fatalf("api charges discovery returned error: %v", err)
	}
	if !strings.Contains(stdout, "Methods:") || !strings.Contains(stdout, "charges create") {
		t.Fatalf("api <interface> output did not list methods; output: %s", stdout)
	}
}

func TestAPIPassthroughGETWithParamsHeadersAndSelect(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/charges" {
			t.Fatalf("path = %s, want /v1/charges", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("limit query = %q, want 1", got)
		}
		if got := r.URL.Query().Get("cursor"); got != "abc=def" {
			t.Fatalf("cursor query = %q, want abc=def", got)
		}
		if got := r.Header.Get("X-Test"); got != "abc=def" {
			t.Fatalf("X-Test header = %q, want abc=def", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test_key" {
			t.Fatalf("Authorization header = %q, want Bearer test_key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ch_1","description":"long","amount":123}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	stdout, _, err := runRootForAPITest(t, []string{
		"--json", "--select", "id", "api", "get", "/v1/charges",
		"--param", "limit=1", "--param", "cursor=abc=def", "--header", "X-Test=abc=def",
	}, "")
	if err != nil {
		t.Fatalf("api get passthrough returned error: %v", err)
	}

	env := decodeAPIEnvelope(t, stdout)
	if env["method"] != "GET" || env["path"] != "/v1/charges" {
		t.Fatalf("unexpected envelope target: %v", env)
	}
	if env["success"] != true {
		t.Fatalf("success = %v, want true", env["success"])
	}
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not an object: %v", env["data"])
	}
	if data["id"] != "ch_1" {
		t.Fatalf("selected data id = %v, want ch_1", data["id"])
	}
	if _, ok := data["description"]; ok {
		t.Fatalf("--select id should remove description from data: %v", data)
	}
}

func TestListFilterFlagsPassThrough(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		path  string
		query map[string]string
	}{
		{
			name:  "accounts external ID",
			args:  []string{"accounts", "list", "--external-id", "account_external_123"},
			path:  "/v1/accounts",
			query: map[string]string{"external_id": "account_external_123"},
		},
		{
			name:  "paykeys creation range",
			args:  []string{"paykeys", "list", "--created-from", "2026-07-01T00:00:00Z", "--created-to", "2026-07-18T00:00:00Z"},
			path:  "/v1/paykeys",
			query: map[string]string{"created_from": "2026-07-01T00:00:00Z", "created_to": "2026-07-18T00:00:00Z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAPIConfig(t)
			t.Setenv("STRADDLE_API_KEY", "test_key")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", r.Method)
				}
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tt.path)
				}
				for name, want := range tt.query {
					if got := r.URL.Query().Get(name); got != want {
						t.Fatalf("query %s = %q, want %q", name, got, want)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()
			t.Setenv("STRADDLE_BASE_URL", server.URL)

			args := append([]string{"--json", "--data-source", "live", "--no-cache"}, tt.args...)
			if _, _, err := runRootForAPITest(t, args, ""); err != nil {
				t.Fatalf("%s returned error: %v", strings.Join(tt.args, " "), err)
			}
		})
	}
}

func TestAPIPassthroughAccountFlagUsesRawPathPolicy(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
		t.Fatalf("save platform context: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(straddleacct.Header); got != "acct_flag" {
			t.Fatalf("%s header = %q, want acct_flag", straddleacct.Header, got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ch_scoped"}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	stdout, _, err := runRootForAPITest(t, []string{"--json", "--account", "acct_flag", "api", "get", "/v1/charges"}, "")
	if err != nil {
		t.Fatalf("api get passthrough with --account returned error: %v", err)
	}
	env := decodeAPIEnvelope(t, stdout)
	if env["success"] != true {
		t.Fatalf("success = %v, want true; envelope: %v", env["success"], env)
	}
}

func TestAPIPassthroughAccountFlagUsesRawPathPolicyWithQueryAndFragment(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
		t.Fatalf("save platform context: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(straddleacct.Header); got != "acct_flag" {
			t.Fatalf("%s header = %q, want acct_flag", straddleacct.Header, got)
		}
		if r.URL.Path != "/v1/charges" {
			t.Fatalf("path = %s, want /v1/charges", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("limit query = %q, want 10", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ch_query"}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	stdout, _, err := runRootForAPITest(t, []string{"--json", "--account", "acct_flag", "api", "get", "/v1/charges?limit=10#frag"}, "")
	if err != nil {
		t.Fatalf("api get passthrough with query path returned error: %v", err)
	}
	env := decodeAPIEnvelope(t, stdout)
	if env["success"] != true {
		t.Fatalf("success = %v, want true; envelope: %v", env["success"], env)
	}
}

func TestAPIPassthroughStickyAccountUsesRawPathPolicy(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS, CurrentAccount: "acct_sticky"}); err != nil {
		t.Fatalf("save platform context: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(straddleacct.Header); got != "acct_sticky" {
			t.Fatalf("%s header = %q, want acct_sticky", straddleacct.Header, got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ch_sticky"}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	if _, _, err := runRootForAPITest(t, []string{"--json", "api", "get", "/v1/charges"}, ""); err != nil {
		t.Fatalf("api get passthrough with sticky account returned error: %v", err)
	}
}

func TestAPIPassthroughStickyAccountUsesRawPathPolicyWithQuery(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_API_KEY", "test_key")
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS, CurrentAccount: "acct_sticky"}); err != nil {
		t.Fatalf("save platform context: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(straddleacct.Header); got != "acct_sticky" {
			t.Fatalf("%s header = %q, want acct_sticky", straddleacct.Header, got)
		}
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("limit query = %q, want 10", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ch_sticky_query"}`))
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	if _, _, err := runRootForAPITest(t, []string{"--json", "api", "get", "/v1/charges?limit=10"}, ""); err != nil {
		t.Fatalf("api get passthrough with sticky account and query path returned error: %v", err)
	}
}

func TestAPIPassthroughRejectsReservedAccountHeader(t *testing.T) {
	isolateAPIConfig(t)

	_, _, err := runRootForAPITest(t, []string{"api", "get", "/v1/charges", "--header", "sTrAdDlE-aCcOuNt-Id=acct_override"}, "")
	if err == nil {
		t.Fatal("reserved account header returned nil error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(reserved account header) = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "use --account") {
		t.Fatalf("reserved account header error = %q, want use --account", err.Error())
	}
}

func TestAPIPassthroughPOSTStdinVerifyShortCircuit(t *testing.T) {
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_VERIFY", "1")

	stdout, _, err := runRootForAPITest(t, []string{"--agent", "api", "post", "/v1/charges", "--stdin"}, `{}`)
	if err != nil {
		t.Fatalf("api post verify passthrough returned error without API key: %v", err)
	}
	env := decodeAPIEnvelope(t, stdout)
	if env["method"] != "POST" || env["path"] != "/v1/charges" {
		t.Fatalf("unexpected envelope target: %v", env)
	}
	if env["verify_noop"] != true {
		t.Fatalf("verify_noop = %v, want true; envelope: %v", env["verify_noop"], env)
	}
	if env["success"] != false {
		t.Fatalf("success = %v, want false for verify noop", env["success"])
	}
	if env["status"] != float64(http.StatusOK) {
		t.Fatalf("status = %v, want %d", env["status"], http.StatusOK)
	}
	data, ok := env["data"].(map[string]any)
	if !ok || data["reason"] != "verify_short_circuit" {
		t.Fatalf("verify data missing reason: %v", env["data"])
	}
}

func TestAPIPassthroughMalformedTokensAreUsageErrors(t *testing.T) {
	isolateAPIConfig(t)

	_, _, err := runRootForAPITest(t, []string{"api", "get", "/v1/charges", "--param", "missing_equals"}, "")
	if err == nil {
		t.Fatal("malformed --param returned nil error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(malformed --param) = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "invalid --param") {
		t.Fatalf("malformed --param error = %q, want invalid --param", err.Error())
	}

	_, _, err = runRootForAPITest(t, []string{"api", "get", "/v1/charges", "--header", "=value"}, "")
	if err == nil {
		t.Fatal("malformed --header returned nil error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(malformed --header) = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "invalid --header") {
		t.Fatalf("malformed --header error = %q, want invalid --header", err.Error())
	}
}
