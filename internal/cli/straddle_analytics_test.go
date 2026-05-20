// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the hand-authored Straddle analytics helpers.
package cli

import (
	"testing"
	"time"
)

func TestIsCancelableStatus(t *testing.T) {
	cases := map[string]bool{
		"created": true, "scheduled": true, "on_hold": true,
		"pending": false, "paid": false, "failed": false,
		"reversed": false, "cancelled": false, "": false,
	}
	for status, want := range cases {
		if got := isCancelableStatus(status); got != want {
			t.Errorf("isCancelableStatus(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestIsReturnStatus(t *testing.T) {
	cases := map[string]bool{
		"failed": true, "reversed": true,
		"paid": false, "pending": false, "created": false, "": false,
	}
	for status, want := range cases {
		if got := isReturnStatus(status); got != want {
			t.Errorf("isReturnStatus(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestParseStraddleTime(t *testing.T) {
	if _, ok := parseStraddleTime(""); ok {
		t.Error("empty string should not parse")
	}
	if _, ok := parseStraddleTime("not-a-time"); ok {
		t.Error("garbage should not parse")
	}
	// Includes Straddle's timezone-less variants (real sandbox data).
	for _, s := range []string{
		"2026-05-01T12:00:00Z", "2026-05-01T12:00:00.123Z", "2026-05-01",
		"2026-02-13T06:38:39", "2026-02-13T06:34:00.1680914", "2026-02-13 06:38:39",
	} {
		if _, ok := parseStraddleTime(s); !ok {
			t.Errorf("expected %q to parse", s)
		}
	}
}

func TestDollars(t *testing.T) {
	cases := map[int64]string{
		0: "$0.00", 10000: "$100.00", 2550: "$25.50",
		-2550: "-$25.50", 123456789: "$1,234,567.89", 5: "$0.05",
	}
	for cents, want := range cases {
		if got := dollars(cents); got != want {
			t.Errorf("dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestAgeInDays(t *testing.T) {
	now := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	if got := ageInDays("", now); got != -1 {
		t.Errorf("unknown created_at should be -1, got %d", got)
	}
	if got := ageInDays("2026-05-10T00:00:00Z", now); got != 10 {
		t.Errorf("age = %d, want 10", got)
	}
	// Future timestamps clamp to 0 rather than going negative.
	if got := ageInDays("2026-06-01T00:00:00Z", now); got != 0 {
		t.Errorf("future created_at should clamp to 0, got %d", got)
	}
}

func TestReturnReason(t *testing.T) {
	if got := returnReason(straddleStatusDetails{Reason: "nsf", Message: "m"}); got != "nsf" {
		t.Errorf("reason should win, got %q", got)
	}
	if got := returnReason(straddleStatusDetails{Message: "m"}); got != "m" {
		t.Errorf("message fallback, got %q", got)
	}
	if got := returnReason(straddleStatusDetails{}); got != "" {
		t.Errorf("empty details, got %q", got)
	}
}

func TestRollWeekly(t *testing.T) {
	days := make([]cashflowBucket, 10)
	for i := range days {
		days[i] = cashflowBucket{Date: time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ChargeIn: 100, PayoutOut: 40}
		days[i].Net = days[i].ChargeIn - days[i].PayoutOut
	}
	weeks := rollWeekly(days)
	if len(weeks) != 2 {
		t.Fatalf("10 days should roll into 2 weeks, got %d", len(weeks))
	}
	if weeks[0].ChargeIn != 700 || weeks[0].PayoutOut != 280 || weeks[0].Net != 420 {
		t.Errorf("week 0 totals wrong: %+v", weeks[0])
	}
	if weeks[1].ChargeIn != 300 || weeks[1].Net != 180 {
		t.Errorf("week 1 totals wrong: %+v", weeks[1])
	}
	if got := rollWeekly(nil); got == nil {
		t.Error("rollWeekly(nil) should return empty slice, not nil")
	}
}

func TestDaysUntil(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	if _, ok := daysUntil("", now); ok {
		t.Error("empty timestamp should be ok=false")
	}
	if d, _ := daysUntil("2026-06-03T00:00:00Z", now); d != 13 {
		t.Errorf("13.5 days ahead should floor to 13, got %d", d)
	}
	// Expired a few hours ago must read as -1 (expired), not 0 (expiring).
	if d, _ := daysUntil("2026-05-20T06:00:00Z", now); d != -1 {
		t.Errorf("just-expired should be -1, got %d", d)
	}
	// Expires later today reads as 0 (expiring), not -1.
	if d, _ := daysUntil("2026-05-20T20:00:00Z", now); d != 0 {
		t.Errorf("expiring-today should be 0, got %d", d)
	}
}

func TestPaymentSettled(t *testing.T) {
	if (straddlePayment{}).settled() {
		t.Error("payment with no funding refs should be unsettled")
	}
	if !(straddlePayment{FundingID: "fe_1"}).settled() {
		t.Error("funding_id should mark settled")
	}
	if !(straddlePayment{FundingIDs: []string{"fe_1"}}).settled() {
		t.Error("funding_ids should mark settled")
	}
	refs := (straddlePayment{FundingID: "fe_2", FundingIDs: []string{"fe_1"}}).fundingRefs()
	if len(refs) != 2 {
		t.Errorf("fundingRefs should include both, got %v", refs)
	}
}
