// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel-feature support for the Straddle CLI. Survives regen as a
// whole file (not generator-emitted). Provides the local-store record types,
// loaders, status helpers, and output plumbing shared by the reconcile,
// pipeline, returns, review-queue, cashflow, and expiring commands. All data is
// read from the SQLite store populated by `sync`; nothing here calls the API.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/straddle-build/cli/internal/store"
)

// straddleStatusDetails mirrors the status_details object Straddle attaches to
// payments and paykeys. Reason/code carry the ACH return semantics (R01, R02,
// R05, ...) the returns command ranks.
type straddleStatusDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Source  string `json:"source"`
}

// straddleCustomerRef is the customer_details sub-object embedded on payments.
type straddleCustomerRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// straddlePayment is the subset of PaymentSummaryV1 the analytics commands use.
// The payments table (populated by `sync` from GET /v1/payments) is the unified
// source for both charges and payouts; payment_type distinguishes them.
type straddlePayment struct {
	ID              string                `json:"id"`
	PaymentType     string                `json:"payment_type"` // "charge" or "payout"
	Status          string                `json:"status"`
	Amount          int64                 `json:"amount"` // cents
	Currency        string                `json:"currency"`
	Paykey          string                `json:"paykey"`
	ExternalID      string                `json:"external_id"`
	CreatedAt       string                `json:"created_at"`
	PaymentDate     string                `json:"payment_date"`
	FundingID       string                `json:"funding_id"`
	FundingIDs      []string              `json:"funding_ids"`
	StatusDetails   straddleStatusDetails `json:"status_details"`
	CustomerDetails straddleCustomerRef   `json:"customer_details"`
}

// settled reports whether this payment has been tied to a funding event.
func (p straddlePayment) settled() bool {
	return p.FundingID != "" || len(p.FundingIDs) > 0
}

// fundingRefs returns the distinct funding-event ids this payment references.
// Deduped so a payment is never counted twice within the same funding event
// (e.g. when FundingID duplicates an entry in FundingIDs).
func (p straddlePayment) fundingRefs() []string {
	seen := map[string]bool{}
	var refs []string
	for _, id := range p.FundingIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			refs = append(refs, id)
		}
	}
	if p.FundingID != "" && !seen[p.FundingID] {
		refs = append(refs, p.FundingID)
	}
	return refs
}

type straddlePaykey struct {
	ID              string                `json:"id"`
	Status          string                `json:"status"`
	CustomerID      string                `json:"customer_id"`
	ExpiresAt       string                `json:"expires_at"`
	CreatedAt       string                `json:"created_at"`
	UnblockEligible bool                  `json:"unblock_eligible"`
	Label           string                `json:"label"`
	InstitutionName string                `json:"institution_name"`
	StatusDetails   straddleStatusDetails `json:"status_details"`
}

type straddleCustomer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}

type straddleFundingEvent struct {
	ID           string `json:"id"`
	Amount       int64  `json:"amount"`
	Direction    string `json:"direction"`
	Status       string `json:"status"`
	PaymentCount int    `json:"payment_count"`
	CreatedAt    string `json:"created_at"`
	TransferDate string `json:"transfer_date"`
}

// openStraddleStore opens the local SQLite store, returning an actionable error
// pointing at `sync` when it cannot be opened.
func openStraddleStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("straddle")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'straddle sync' first.", err)
	}
	return db, nil
}

// straddleScanJSON streams `SELECT id, data FROM <table>` rows. id and data are
// NOT NULL on every typed table, so a bare scan is safe; a per-row scan error
// skips that row rather than aborting the whole query.
func straddleScanJSON(ctx context.Context, db *store.Store, query string, fn func(id string, data []byte)) error {
	rows, err := db.DB().QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		fn(id, data)
	}
	return rows.Err()
}

func loadStraddlePayments(ctx context.Context, db *store.Store) ([]straddlePayment, error) {
	var out []straddlePayment
	err := straddleScanJSON(ctx, db, `SELECT id, data FROM payments`, func(id string, data []byte) {
		var p straddlePayment
		if json.Unmarshal(data, &p) != nil {
			return
		}
		if p.ID == "" {
			p.ID = id
		}
		out = append(out, p)
	})
	return out, err
}

func loadStraddlePaykeys(ctx context.Context, db *store.Store) ([]straddlePaykey, error) {
	var out []straddlePaykey
	err := straddleScanJSON(ctx, db, `SELECT id, data FROM paykeys`, func(id string, data []byte) {
		var p straddlePaykey
		if json.Unmarshal(data, &p) != nil {
			return
		}
		if p.ID == "" {
			p.ID = id
		}
		out = append(out, p)
	})
	return out, err
}

func loadStraddleCustomers(ctx context.Context, db *store.Store) ([]straddleCustomer, error) {
	var out []straddleCustomer
	err := straddleScanJSON(ctx, db, `SELECT id, data FROM customers`, func(id string, data []byte) {
		var c straddleCustomer
		if json.Unmarshal(data, &c) != nil {
			return
		}
		if c.ID == "" {
			c.ID = id
		}
		out = append(out, c)
	})
	return out, err
}

func loadStraddleFundingEvents(ctx context.Context, db *store.Store) ([]straddleFundingEvent, error) {
	var out []straddleFundingEvent
	err := straddleScanJSON(ctx, db, `SELECT id, data FROM funding_events`, func(id string, data []byte) {
		var f straddleFundingEvent
		if json.Unmarshal(data, &f) != nil {
			return
		}
		if f.ID == "" {
			f.ID = id
		}
		out = append(out, f)
	})
	return out, err
}

// Payment lifecycle helpers, grounded in Straddle's documented status machine:
// created -> validating -> scheduled -> pending -> paid -> reversed, with
// on_hold -> released/cancelled. Once a payment reaches pending it is locked.
var straddleCancelableStatuses = map[string]bool{
	"created":   true,
	"scheduled": true,
	"on_hold":   true,
}

// isCancelableStatus reports whether a payment in this status can still be
// held, released, or cancelled. Anything from pending onward is locked.
func isCancelableStatus(status string) bool {
	return straddleCancelableStatuses[status]
}

var straddleReturnStatuses = map[string]bool{
	"failed":   true,
	"reversed": true,
}

// isReturnStatus reports whether a payment failed before funding or reversed
// after funding (the ACH-return outcomes the returns command analyzes).
func isReturnStatus(status string) bool {
	return straddleReturnStatuses[status]
}

// parseStraddleTime parses the RFC3339 timestamps Straddle returns. ok is false
// for empty or unparseable values so callers can treat them as "unknown".
func parseStraddleTime(s string) (t time.Time, ok bool) {
	if s == "" {
		return time.Time{}, false
	}
	// Straddle is inconsistent: some fields carry a zone (created_at:
	// "2026-02-13T06:34:00.168Z"), others omit it (created_at:
	// "2026-02-13T06:38:39"). Try zoned layouts first, then the bare forms
	// (parsed as UTC, which is fine for day/age math).
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// daysUntil returns the floored whole days from now until the given timestamp.
// ok is false when the timestamp is empty or unparseable. A negative result
// means the timestamp is already in the past; flooring (not truncating toward
// zero) ensures something that expired a few hours ago reads as -1, not 0.
func daysUntil(ts string, now time.Time) (int, bool) {
	parsed, ok := parseStraddleTime(ts)
	if !ok {
		return 0, false
	}
	return int(math.Floor(parsed.Sub(now).Hours() / 24)), true
}

// straddleWantsJSON decides between machine output (JSON/CSV/--select/agent/
// piped) and a human table. Mirrors the generated commands' output gating so
// novel commands behave identically under --agent and in pipes.
func straddleWantsJSON(cmd *cobra.Command, flags *rootFlags) bool {
	if flags.asJSON || flags.agent || flags.csv || flags.quiet || flags.plain || flags.selectFields != "" {
		return true
	}
	return !isTerminal(cmd.OutOrStdout()) && !humanFriendly
}

// dollars formats integer cents as a dollar string for human tables.
func dollars(cents int64) string {
	neg := ""
	if cents < 0 {
		neg = "-"
		cents = -cents
	}
	return neg + "$" + addThousands(cents/100) + "." + twoDigits(cents%100)
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func addThousands(n int64) string {
	s := itoa(n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	pre := len(s) % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
	}
	for i := pre; i < len(s); i += 3 {
		if len(out) > 0 {
			out = append(out, ',')
		}
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}
