// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestNumberedPaginationRejectsIncompletePages(t *testing.T) {
	cases := []struct {
		name      string
		responses []string
	}{
		{"zero total with records and no page size", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"total_rows":0}}`,
		}},
		{"page count contradicts record count", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_items":3,"total_pages":2}}`,
		}},
		{"pay by bank missing final record", []string{
			`{"data":[{"id":"a"},{"id":"b"}],"meta":{"page_number":1,"page_size":2,"total_items":4,"total_pages":2}}`,
			`{"data":[{"id":"c"}],"meta":{"page_number":2,"page_size":2,"total_items":4,"total_pages":2}}`,
		}},
		{"embed missing final record", []string{
			`{"data":[{"id":"a"},{"id":"b"}],"meta":{"page_number":1,"page_size":2,"total_rows":4}}`,
			`{"data":[{"id":"c"}],"meta":{"page_number":2,"page_size":2,"total_rows":4}}`,
		}},
		{"missing intermediate record", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":2,"total_items":3,"total_pages":2}}`,
		}},
		{"advertised single page is empty", []string{
			`{"data":[],"meta":{"page_number":1,"page_size":2,"total_items":1,"total_pages":1}}`,
		}},
		{"advertised embed final page is empty", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":2}}`,
			`{"data":[],"meta":{"page_number":2,"page_size":1,"total_rows":2}}`,
		}},
		{"advertised intermediate page is empty", []string{
			`{"data":[],"meta":{"page_number":1,"page_size":2,"total_items":3,"total_pages":2}}`,
		}},
		{"page size changes while reading", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":3}}`,
			`{"data":[{"id":"b"},{"id":"c"}],"meta":{"page_number":2,"page_size":2,"total_rows":3}}`,
		}},
		{"data repeats without a response page number", []string{
			`{"data":[{"id":"a"}],"meta":{"page_size":1,"total_rows":2}}`,
			`{"data":[{"id":"a"}],"meta":{"page_size":1,"total_rows":2}}`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := capturePaginationStderr(t)
			calls := 0
			guard := errors.New("test guard: requested another page after invalid response")
			got, err := paginateValues(nil, true, "page_number", "", "", func(url.Values) (json.RawMessage, error) {
				calls++
				if calls > len(tc.responses) {
					return nil, guard
				}
				return json.RawMessage(tc.responses[calls-1]), nil
			})
			if err == nil {
				t.Errorf("incomplete --all returned success: %s", got)
			}
			if errors.Is(err, guard) {
				t.Error("continued reading after a page already proved results incomplete")
			}
			if len(got) != 0 {
				t.Errorf("incomplete --all returned partial results: %s", got)
			}
			if strings.Contains(stderr(), `"event":"complete"`) {
				t.Error("incomplete --all emitted a complete event")
			}
		})
	}
}

func TestNumberedPaginationRejectsMalformedData(t *testing.T) {
	cases := []struct{ name, response string }{
		{"missing data", `{"meta":{"page_number":1,"total_pages":1}}`},
		{"null data", `{"data":null,"meta":{"page_number":1,"total_pages":1}}`},
		{"object data", `{"data":{"id":"a"},"meta":{"page_number":1,"total_pages":1}}`},
		{"string data", `{"data":"broken","meta":{"page_number":1,"total_pages":1}}`},
		{"invalid JSON", `{"data":[`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := capturePaginationStderr(t)
			calls := 0
			got, err := paginateValues(nil, true, "page_number", "", "", func(url.Values) (json.RawMessage, error) {
				calls++
				if calls > 1 {
					return nil, errors.New("test guard: fetched after malformed data")
				}
				return json.RawMessage(tc.response), nil
			})
			if err == nil || len(got) != 0 || calls != 1 {
				t.Errorf("malformed data returned %s, error %v after %d calls", got, err, calls)
			}
			if strings.Contains(stderr(), `"event":"complete"`) {
				t.Error("malformed response emitted a complete event")
			}
		})
	}
}

func TestNumberedPaginationNormalizesNonpositiveStartingPages(t *testing.T) {
	for _, start := range []string{"0", "-1", "-100"} {
		t.Run(start, func(t *testing.T) {
			capturePaginationStderr(t)
			calls := 0
			got, err := paginateValues(url.Values{"page_number": {start}}, true, "page_number", "", "", func(query url.Values) (json.RawMessage, error) {
				calls++
				if query.Get("page_number") != "1" {
					t.Errorf("requested page_number = %q, want 1", query.Get("page_number"))
				}
				if calls > 1 {
					return nil, errors.New("test guard: requested a page after the end")
				}
				return json.RawMessage(`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_items":1,"total_pages":1}}`), nil
			})
			if err != nil || string(got) != `[{"id":"a"}]` || calls != 1 {
				t.Errorf("normalized read returned %s, error %v after %d calls", got, err, calls)
			}
		})
	}
}

func TestNumberedPaginationBoundsPageRequests(t *testing.T) {
	for _, finishesAtLimit := range []bool{false, true} {
		t.Run(fmt.Sprintf("finishes_at_limit=%t", finishesAtLimit), func(t *testing.T) {
			stderr := capturePaginationStderr(t)
			calls := 0
			guard := errors.New("test guard: exceeded maximum page requests")
			got, err := paginateValues(nil, true, "page_number", "", "", func(query url.Values) (json.RawMessage, error) {
				calls++
				if calls > maxNumberedPages {
					return nil, guard
				}
				if query.Get("page_number") != strconv.Itoa(calls) {
					t.Errorf("requested page_number = %q, want %d", query.Get("page_number"), calls)
				}
				totalPages := maxNumberedPages + 1
				if finishesAtLimit {
					totalPages = maxNumberedPages
				}
				return json.RawMessage(fmt.Sprintf(`{"data":[{"id":%d}],"meta":{"page_number":%d,"page_size":1,"total_pages":%d}}`, calls, calls, totalPages)), nil
			})
			if calls != maxNumberedPages || errors.Is(err, guard) {
				t.Errorf("page limit: %d requests, error %v", calls, err)
			}
			if finishesAtLimit {
				var items []json.RawMessage
				if err != nil || json.Unmarshal(got, &items) != nil || len(items) != maxNumberedPages {
					t.Errorf("complete read at limit: %d items, error %v", len(items), err)
				}
				return
			}
			if err == nil || len(got) != 0 {
				t.Errorf("unfinished read at limit: %d output bytes, error %v", len(got), err)
			}
			if strings.Contains(stderr(), `"event":"complete"`) {
				t.Error("page limit failure emitted a complete event")
			}
		})
	}
}
