// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNumberedPaginationFetchesAllPages(t *testing.T) {
	cases := []struct {
		name      string
		start     string
		pages     []string
		wantPages []int
		want      string
	}{
		{"pay by bank partial final page", "", []string{
			`{"data":[{"id":"a"},{"id":"b"}],"meta":{"page_number":1,"page_size":2,"total_items":3,"total_pages":2}}`,
			`{"data":[{"id":"c"}],"meta":{"page_number":2,"page_size":2,"total_items":3,"total_pages":2}}`,
		}, []int{1, 2}, `[{"id":"a"},{"id":"b"},{"id":"c"}]`},
		{"embed full final page", "", []string{
			`{"data":[{"id":"a"},{"id":"b"}],"meta":{"page_number":1,"page_size":2,"total_rows":4}}`,
			`{"data":[{"id":"c"},{"id":"d"}],"meta":{"page_number":2,"page_size":2,"total_rows":4}}`,
		}, []int{1, 2}, `[{"id":"a"},{"id":"b"},{"id":"c"},{"id":"d"}]`},
		{"total items without total pages", "", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_items":2}}`,
			`{"data":[{"id":"b"}],"meta":{"page_number":2,"page_size":1,"total_items":2}}`,
		}, []int{1, 2}, `[{"id":"a"},{"id":"b"}]`},
		{"explicit starting page", "3", []string{
			`{"data":[{"id":"c"}],"meta":{"page_number":3,"page_size":1,"total_items":4,"total_pages":4}}`,
			`{"data":[{"id":"d"}],"meta":{"page_number":4,"page_size":1,"total_items":4,"total_pages":4}}`,
		}, []int{3, 4}, `[{"id":"c"},{"id":"d"}]`},
		{"empty pay by bank", "", []string{`{"data":[],"meta":{"page_number":1,"page_size":2,"total_items":0,"total_pages":0}}`}, []int{1}, `[]`},
		{"empty embed", "", []string{`{"data":[],"meta":{"page_number":1,"page_size":2,"total_rows":0}}`}, []int{1}, `[]`},
		{"single full page", "", []string{`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_items":1,"total_pages":1}}`}, []int{1}, `[{"id":"a"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capturePaginationStderr(t)
			params := url.Values{"status": {"active", "review"}, "page_number": {tc.start}}
			original := params.Encode()
			var requested []int
			got, err := paginateValues(params, true, "page_number", "", "", func(query url.Values) (json.RawMessage, error) {
				page := 1
				if value := query.Get("page_number"); value != "" {
					var err error
					page, err = strconv.Atoi(value)
					if err != nil {
						return nil, err
					}
				}
				requested = append(requested, page)
				if !reflect.DeepEqual(query["status"], []string{"active", "review"}) {
					t.Errorf("repeated status filter = %q", query["status"])
				}
				if len(requested) > len(tc.pages) {
					return nil, errors.New("requested a page beyond the advertised total")
				}
				return json.RawMessage(tc.pages[len(requested)-1]), nil
			})
			if err != nil {
				t.Fatalf("--all: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("items = %s, want %s", got, tc.want)
			}
			if !reflect.DeepEqual(requested, tc.wantPages) {
				t.Errorf("requested pages = %v, want %v", requested, tc.wantPages)
			}
			if params.Encode() != original {
				t.Errorf("caller query changed from %s to %s", original, params.Encode())
			}
		})
	}
}

func TestNumberedPaginationRejectsUnknownOrInvalidCompletion(t *testing.T) {
	cases := []struct {
		name      string
		responses []string
	}{
		{"missing metadata", []string{`{"data":[{"id":"a"}]}`}},
		{"nullable embed metadata", []string{`{"data":[{"id":"a"}],"meta":{"page_number":null,"page_size":null,"total_rows":null}}`}},
		{"missing totals and page size", []string{`{"data":[{"id":"a"}],"meta":{"page_number":1}}`}},
		{"invalid page size", []string{`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":0,"total_rows":2}}`}},
		{"negative total", []string{`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":-1}}`}},
		{"fractional page count", []string{`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_pages":1.5}}`}},
		{"nonnumeric page number", []string{`{"data":[{"id":"a"}],"meta":{"page_number":"one","page_size":1,"total_pages":2}}`}},
		{"nonadvancing response", []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":2}}`,
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":2}}`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := capturePaginationStderr(t)
			calls := 0
			guard := errors.New("test guard: pagination did not stop")
			got, err := paginateValues(nil, true, "page_number", "", "", func(url.Values) (json.RawMessage, error) {
				calls++
				if calls > len(tc.responses) {
					return nil, guard
				}
				return json.RawMessage(tc.responses[calls-1]), nil
			})
			if err == nil {
				t.Errorf("--all returned success with untrustworthy pagination: %s", got)
			}
			if errors.Is(err, guard) {
				t.Errorf("pagination relied on test guard to stop: %v", err)
			}
			if strings.Contains(stderr(), `"event":"complete"`) {
				t.Error("invalid pagination emitted a complete event")
			}
		})
	}
}

func TestNumberedPaginationFallsBackToShortPage(t *testing.T) {
	cases := []struct {
		name      string
		params    url.Values
		responses []string
	}{
		{"optional embed totals omitted", nil, []string{
			`{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":null}}`,
			`{"data":[],"meta":{"page_number":2,"page_size":1,"total_rows":null}}`,
		}},
		{"explicit page size without metadata", url.Values{"page_size": {"1"}}, []string{
			`{"data":[{"id":"a"}]}`, `{"data":[]}`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capturePaginationStderr(t)
			calls := 0
			got, err := paginateValues(tc.params, true, "page_number", "", "", func(query url.Values) (json.RawMessage, error) {
				calls++
				if calls > len(tc.responses) {
					return nil, errors.New("requested another page after the short page")
				}
				if calls == 2 && query.Get("page_number") != "2" {
					t.Errorf("next page_number = %q, want 2", query.Get("page_number"))
				}
				return json.RawMessage(tc.responses[calls-1]), nil
			})
			if err != nil || calls != 2 || string(got) != `[{"id":"a"}]` {
				t.Fatalf("items = %s, error = %v, calls = %d", got, err, calls)
			}
		})
	}
}

func TestNumberedPaginationWarnsWhenMorePagesExist(t *testing.T) {
	cases := []struct {
		name, response string
		more           bool
	}{
		{"pay by bank", `{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_items":2,"total_pages":2}}`, true},
		{"embed", `{"data":[{"id":"a"}],"meta":{"page_number":1,"page_size":1,"total_rows":2}}`, true},
		{"last page", `{"data":[{"id":"b"}],"meta":{"page_number":2,"page_size":1,"total_items":2,"total_pages":2}}`, false},
		{"unknown total", `{"data":[{"id":"a"}]}`, false},
	}
	for _, human := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/human=%t", tc.name, human), func(t *testing.T) {
				stderr := capturePaginationStderr(t)
				humanFriendly = human
				calls := 0
				got, err := paginateValues(nil, false, "page_number", "", "", func(url.Values) (json.RawMessage, error) { calls++; return json.RawMessage(tc.response), nil })
				if err != nil || string(got) != tc.response || calls != 1 {
					t.Fatalf("single page returned %s, %v after %d calls", got, err, calls)
				}
				output := stderr()
				if strings.Contains(output, "truncated") != tc.more {
					t.Errorf("truncation warning = %q, more pages = %t", output, tc.more)
				}
				if tc.more && !strings.Contains(output, "--all") {
					t.Errorf("warning has no actionable --all hint: %q", output)
				}
			})
		}
	}
}

func TestCursorPaginationStillAdvances(t *testing.T) {
	capturePaginationStderr(t)
	calls := 0
	got, err := paginateValues(nil, true, "cursor", "meta.next", "", func(query url.Values) (json.RawMessage, error) {
		calls++
		switch calls {
		case 1:
			return json.RawMessage(`{"data":[{"id":"a"}],"meta":{"next":"second"}}`), nil
		case 2:
			if query.Get("cursor") != "second" {
				t.Errorf("cursor = %q, want second", query.Get("cursor"))
			}
			return json.RawMessage(`{"data":[{"id":"b"}],"meta":{"next":""}}`), nil
		default:
			return nil, errors.New("requested a page after cursor exhaustion")
		}
	})
	if err != nil || string(got) != `[{"id":"a"},{"id":"b"}]` || calls != 2 {
		t.Fatalf("items = %s, error = %v, calls = %d", got, err, calls)
	}
}

func capturePaginationStderr(t *testing.T) func() string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	previous, previousHuman := os.Stderr, humanFriendly
	os.Stderr, humanFriendly = file, false
	t.Cleanup(func() { os.Stderr, humanFriendly = previous, previousHuman; _ = file.Close() })
	return func() string {
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}
