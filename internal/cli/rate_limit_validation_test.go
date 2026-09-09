// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRateLimitFlagRejectsInvalidValuesBeforeRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "negative", value: "-1", want: "non-negative"},
		{name: "nan", value: "NaN", want: "finite"},
		{name: "positive infinity", value: "+Inf", want: "finite"},
		{name: "negative infinity", value: "-Inf", want: "non-negative"},
		{name: "subnormal", value: "1e-300", want: "representable pacing range"},
		{name: "too fast for nanosecond pacing", value: "1e12", want: "representable pacing range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requests.Add(1)
			}))
			defer server.Close()
			isolateSurfaceConfig(t, server.URL)

			_, _, err := runRootForAPITest(t, []string{
				"--rate-limit", tc.value,
				"--data-source", "live",
				"customers", "list",
			}, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("HTTP requests = %d, want 0", got)
			}
		})
	}
}

func TestRateLimitFlagAcceptsDisabledAndFiniteValues(t *testing.T) {
	for _, value := range []string{"0", "0.25", "1e9"} {
		t.Run(value, func(t *testing.T) {
			isolateAPIConfig(t)
			_, _, err := runRootForAPITest(t, []string{
				"--rate-limit", value,
				"--dry-run",
				"api", "get", "/v1/customers",
			}, "")
			if err != nil {
				t.Fatalf("rate-limit %s: %v", value, err)
			}
		})
	}
}
