// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

const (
	contractMockSpecPath = "../../spec.yaml"
	contractMockStartup  = 60 * time.Second
)

func contractSpecPath() string {
	if path := os.Getenv("STRADDLE_CONTRACT_SPEC"); path != "" {
		return path
	}
	return contractMockSpecPath
}

type contractDocument struct {
	Paths      map[string]map[string]contractOperation `json:"paths"`
	Components struct {
		Schemas  map[string]contractSchema  `json:"schemas"`
		Examples map[string]contractExample `json:"examples"`
	} `json:"components"`
}

type contractOperation struct {
	RequestBody contractRequestBody `json:"requestBody"`
}

type contractRequestBody struct {
	Content map[string]contractMediaType `json:"content"`
}

type contractMediaType struct {
	Schema   contractSchemaReference    `json:"schema"`
	Example  json.RawMessage            `json:"example"`
	Examples map[string]contractExample `json:"examples"`
}

type contractSchemaReference struct {
	Ref string `json:"$ref"`
}

type contractSchema struct {
	Example  json.RawMessage   `json:"example"`
	Examples []json.RawMessage `json:"examples"`
}

type contractExample struct {
	Ref   string          `json:"$ref"`
	Value json.RawMessage `json:"value"`
}

type recordedContractResponse struct {
	statusCode    int
	body          []byte
	authorization string
}

type contractRecordingTransport struct {
	base http.RoundTripper

	mu        sync.Mutex
	responses []recordedContractResponse
}

func (t *contractRecordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	t.mu.Lock()
	t.responses = append(t.responses, recordedContractResponse{
		statusCode:    resp.StatusCode,
		body:          append([]byte(nil), body...),
		authorization: req.Header.Get("Authorization"),
	})
	t.mu.Unlock()
	return resp, nil
}

func (t *contractRecordingTransport) take() []recordedContractResponse {
	t.mu.Lock()
	defer t.mu.Unlock()
	responses := append([]recordedContractResponse(nil), t.responses...)
	t.responses = nil
	return responses
}

func TestContractMockServer(t *testing.T) {
	requireContractMock(t)

	const path = "/v1/customers"
	body := contractRequestExample(t, http.MethodPost, path)
	baseURL := startContractMockServer(t)
	isolateContractMockConfig(t, baseURL)
	recorder := installContractRecorder(t)

	root := RootCmd()
	var stderr bytes.Buffer
	root.SetOut(io.Discard)
	root.SetErr(&stderr)
	root.SetIn(bytes.NewReader(body))
	root.SetArgs([]string{"customers", "create", "--stdin", "--no-cache"})

	err := root.Execute()
	responses := recorder.take()
	if err != nil {
		t.Fatalf("execute customers create: %v\nstderr:\n%s", err, stderr.String())
	}
	if len(responses) != 1 {
		t.Fatalf("customers create sent %d requests, want 1", len(responses))
	}

	response := responses[0]
	if response.statusCode < http.StatusOK || response.statusCode >= http.StatusMultipleChoices {
		t.Fatalf("customers create returned HTTP %d:\n%s", response.statusCode, formatContractProblem(response.body))
	}
	if response.authorization != "Bearer test-key" {
		t.Fatalf("Authorization header = %q, want %q", response.authorization, "Bearer test-key")
	}
}

func TestContractRequestExampleUsesNamedMediaExample(t *testing.T) {
	fixture := `{
		"paths": {"/v1/customers": {"post": {"requestBody": {"content": {"application/json": {
			"schema": {"$ref": "#/components/schemas/Customer"},
			"examples": {"individual": {"$ref": "#/components/examples/Individual"}}
		}}}}}},
		"components": {
			"schemas": {"Customer": {"example": {"name": "schema fallback"}}},
			"examples": {"Individual": {"value": {"name": "Alex Example"}}}
		}
	}`
	path := filepath.Join(t.TempDir(), "contract.json")
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STRADDLE_CONTRACT_SPEC", path)
	body := contractRequestExample(t, http.MethodPost, "/v1/customers")
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "Alex Example" {
		t.Fatalf("request example = %s, want the named media example", body)
	}
}

func requireContractMock(t *testing.T) {
	t.Helper()
	if os.Getenv("STRADDLE_CONTRACT_MOCK") != "1" {
		t.Skip("set STRADDLE_CONTRACT_MOCK=1 to run Scalar contract validation")
	}
}

func contractRequestExample(t *testing.T, method, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(contractSpecPath())
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var document contractDocument
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse contract: %v", err)
	}

	pathItem, ok := document.Paths[path]
	if !ok {
		t.Fatalf("%s is absent from the contract", path)
	}
	operation, ok := pathItem[strings.ToLower(method)]
	if !ok {
		t.Fatalf("%s %s is absent from the contract", method, path)
	}
	mediaType, ok := operation.RequestBody.Content["application/json"]
	if !ok {
		t.Fatalf("%s %s has no application/json request body", method, path)
	}
	if len(mediaType.Example) > 0 {
		return mediaType.Example
	}
	if len(mediaType.Examples) > 0 {
		var name string
		chosen := false
		for candidate := range mediaType.Examples {
			if !chosen || candidate < name {
				name, chosen = candidate, true
			}
		}
		example := mediaType.Examples[name]
		if example.Ref != "" {
			refName, ok := strings.CutPrefix(example.Ref, "#/components/examples/")
			if !ok {
				t.Fatalf("%s %s example %q has unsupported reference %q", method, path, name, example.Ref)
			}
			refName = strings.ReplaceAll(strings.ReplaceAll(refName, "~1", "/"), "~0", "~")
			example, ok = document.Components.Examples[refName]
			if !ok {
				t.Fatalf("%s %s references missing example %q", method, path, refName)
			}
		}
		if len(example.Value) == 0 {
			t.Fatalf("%s %s example %q has no value", method, path, name)
		}
		return example.Value
	}

	const schemaRefPrefix = "#/components/schemas/"
	schemaName, ok := strings.CutPrefix(mediaType.Schema.Ref, schemaRefPrefix)
	if !ok || schemaName == "" {
		t.Fatalf("%s %s request body does not reference a component schema", method, path)
	}
	schema, ok := document.Components.Schemas[schemaName]
	if !ok {
		t.Fatalf("%s %s references missing schema %q", method, path, schemaName)
	}

	switch {
	case len(schema.Example) > 0:
		return schema.Example
	case len(schema.Examples) > 0:
		return schema.Examples[0]
	default:
		t.Fatalf("%s %s schema %q has no request example", method, path, schemaName)
		return nil
	}
}

func formatContractProblem(body []byte) string {
	var formatted bytes.Buffer
	if json.Indent(&formatted, body, "", "  ") == nil {
		return formatted.String()
	}
	return string(body)
}

func installContractRecorder(t *testing.T) *contractRecordingTransport {
	t.Helper()
	original := http.DefaultTransport
	recorder := &contractRecordingTransport{base: original}
	http.DefaultTransport = recorder
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
	return recorder
}

func isolateContractMockConfig(t *testing.T, baseURL string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("HOME", configDir)
	t.Setenv("STRADDLE_CONFIG", filepath.Join(configDir, "config.toml"))
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(configDir, "platform.toml"))
	t.Setenv("STRADDLE_API_KEY", "test-key")
	t.Setenv("STRADDLE_BASE_URL", baseURL)
	t.Setenv("STRADDLE_VERIFY", "")
	t.Setenv("STRADDLE_VERIFY_LIVE_HTTP", "")
}

func startContractMockServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve mock port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release mock port: %v", err)
	}

	stderr, err := os.CreateTemp(t.TempDir(), "scalar-stderr-*.log")
	if err != nil {
		t.Fatalf("create Scalar stderr log: %v", err)
	}
	t.Cleanup(func() {
		_ = stderr.Close()
	})
	cmd := exec.Command("npx", "--yes", "@scalar/cli@2.1.0", "document", "mock", contractSpecPath(), "--port", strconv.Itoa(port))
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start Scalar mock server: %v", err)
	}
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
			return
		default:
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.NewTimer(contractMockStartup)
	defer deadline.Stop()
	retry := time.NewTicker(50 * time.Millisecond)
	defer retry.Stop()
	for {
		conn, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return "http://" + address
		}
		select {
		case <-done:
			t.Fatalf("Scalar mock server exited before accepting connections: %v\nstderr:\n%s", waitErr, readContractMockStderr(stderr.Name()))
		case <-deadline.C:
			t.Fatalf("Scalar mock server did not accept connections within %s\nstderr:\n%s", contractMockStartup, readContractMockStderr(stderr.Name()))
		case <-retry.C:
		}
	}
}

func readContractMockStderr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("read stderr log: %v", err)
	}
	return string(data)
}
