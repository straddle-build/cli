// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/straddleacct"
)

func TestCustomersListAllUsesNumberedPagination(t *testing.T) {
	for _, failLaterPage := range []bool{false, true} {
		name := "all remaining customers"
		if failLaterPage {
			name = "later page HTTP failure"
		}
		t.Run(name, func(t *testing.T) {
			stderr := capturePaginationStderr(t)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				call := calls.Add(1)
				if request.Method != http.MethodGet || request.URL.Path != "/v1/customers" {
					t.Errorf("request = %s %s, want GET /v1/customers", request.Method, request.URL.Path)
				}
				query := request.URL.Query()
				if !reflect.DeepEqual(query["status"], []string{"pending", "review"}) {
					t.Errorf("repeated status filter = %q", query["status"])
				}
				if query.Get("page_size") != "1" {
					t.Errorf("page_size = %q, want 1", query.Get("page_size"))
				}
				if request.Header.Get("Request-Id") != "pagination-test" {
					t.Errorf("Request-Id = %q", request.Header.Get("Request-Id"))
				}
				if request.Header.Get(straddleacct.Header) != accountHeaderTestID {
					t.Errorf("account header = %q", request.Header.Get(straddleacct.Header))
				}
				if request.Header.Get("Authorization") == "" {
					t.Error("request lost authorization header")
				}
				w.Header().Set("Content-Type", "application/json")
				switch call {
				case 1:
					if query.Get("page_number") != "2" {
						t.Errorf("initial page_number = %q, want 2", query.Get("page_number"))
					}
					_, _ = w.Write([]byte(`{"data":[{"id":"customer-b"}],"meta":{"page_number":2,"page_size":1,"total_items":3,"total_pages":3}}`))
				case 2:
					if query.Get("page_number") != "3" {
						t.Errorf("next page_number = %q, want 3", query.Get("page_number"))
					}
					if failLaterPage {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"error":{"message":"access revoked"}}`))
						return
					}
					_, _ = w.Write([]byte(`{"data":[{"id":"customer-c"}],"meta":{"page_number":3,"page_size":1,"total_items":3,"total_pages":3}}`))
				default:
					t.Error("requested a page beyond total_pages")
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			isolateSurfaceConfig(t, server.URL)
			if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
				t.Fatal(err)
			}
			stdout, _, err := runRootForAPITest(t, []string{
				"--agent", "--data-source", "live", "--no-cache", "--account", accountHeaderTestID,
				"customers", "list", "--all", "--page-number", "2", "--page-size", "1",
				"--status", "pending,review", "--request-id", "pagination-test",
			}, "")
			if calls.Load() != 2 {
				t.Errorf("HTTP requests = %d, want 2", calls.Load())
			}
			if failLaterPage {
				if err == nil || !strings.Contains(err.Error(), "403") {
					t.Errorf("error = %v, want later-page HTTP 403", err)
				}
				if strings.TrimSpace(stdout) != "" {
					t.Errorf("failed --all printed partial results as success: %s", stdout)
				}
				if strings.Contains(stderr(), `"event":"complete"`) {
					t.Error("failed --all emitted a complete event")
				}
				return
			}
			if err != nil {
				t.Fatalf("customers list --all: %v", err)
			}
			var envelope struct {
				Results []struct {
					ID string `json:"id"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("decode output %s: %v", stdout, err)
			}
			var ids []string
			for _, customer := range envelope.Results {
				ids = append(ids, customer.ID)
			}
			if !reflect.DeepEqual(ids, []string{"customer-b", "customer-c"}) {
				t.Errorf("customer IDs = %v, want both remaining pages; output: %s", ids, stdout)
			}
		})
	}
}
