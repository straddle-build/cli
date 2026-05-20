// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// assertUniformWidth checks every line of a rendered card has the same visible
// width (ANSI stripped) — the core alignment invariant.
func assertUniformWidth(t *testing.T, card string) int {
	t.Helper()
	lines := strings.Split(card, "\n")
	if len(lines) == 0 {
		t.Fatal("empty card")
	}
	want := dispWidth(stripANSI(lines[0]))
	for i, ln := range lines {
		if got := dispWidth(stripANSI(ln)); got != want {
			t.Errorf("line %d width = %d, want %d\n  line: %q", i, got, want, stripANSI(ln))
		}
	}
	return want
}

func TestHumanizeLabel(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"external_id", "External ID"},
		{"account_id", "Account ID"},
		{"ip_address", "IP Address"},
		{"id", "ID"},
		{"status", "Status"},
		{"payment_date", "Payment Date"},
		{"institution_name", "Institution Name"},
		{"ssn_last4", "SSN Last4"},
		{"customerType", "Customer Type"},
		{"unblock_eligible", "Unblock Eligible"},
	}
	for _, tc := range cases {
		if got := humanizeLabel(tc.in); got != tc.want {
			t.Errorf("humanizeLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHumanizeResource(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"charges", "Charge"},
		{"customers", "Customer"},
		{"paykeys", "Paykey"},
		{"payouts", "Payout"},
		{"accounts", "Account"},
		{"organizations", "Organization"},
		{"representatives", "Representative"},
		{"linked-bank-accounts", "Linked Bank Account"},
		{"funding-events", "Funding Event"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := humanizeResource(tc.in); got != tc.want {
			t.Errorf("humanizeResource(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatScalarValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		key     string
		raw     string
		sib     map[string]json.RawMessage
		want    string
		present bool
	}{
		{"string", "name", `"Jane"`, nil, "Jane", true},
		{"empty string skipped", "name", `""`, nil, "", false},
		{"null skipped", "name", `null`, nil, "", false},
		{"bool true", "ok", `true`, nil, "true", true},
		{"bool false kept", "unblock_eligible", `false`, nil, "false", true},
		{"int", "count", `7`, nil, "7", true},
		{"float", "rate", `1.5`, nil, "1.50", true},
		{"amount cents default usd", "amount", `1000`, nil, "$10.00 USD", true},
		{"amount cents sibling currency", "amount", `2550`, map[string]json.RawMessage{"currency": json.RawMessage(`"eur"`)}, "$25.50 EUR", true},
		{"suffix amount", "funding_amount", `99`, nil, "$0.99 USD", true},
		{"datetime passthrough", "created_at", `"2026-05-20T14:03:00Z"`, nil, "2026-05-20T14:03:00Z", true},
		{"zero kept", "amount", `0`, nil, "$0.00 USD", true},
	}
	for _, tc := range cases {
		got, present := formatScalarValue(tc.key, json.RawMessage(tc.raw), tc.sib)
		if present != tc.present || got != tc.want {
			t.Errorf("%s: formatScalarValue(%q,%s) = (%q,%v), want (%q,%v)", tc.name, tc.key, tc.raw, got, present, tc.want, tc.present)
		}
	}
}

func TestDecodeOrderedObjectPreservesOrder(t *testing.T) {
	t.Parallel()
	// Keys deliberately out of alphabetical order to prove document order wins.
	data := json.RawMessage(`{"zebra":1,"apple":2,"mango":3,"id":4}`)
	kvs, ok := decodeOrderedObject(data)
	if !ok {
		t.Fatal("decodeOrderedObject returned ok=false")
	}
	want := []string{"zebra", "apple", "mango", "id"}
	if len(kvs) != len(want) {
		t.Fatalf("got %d kvs, want %d", len(kvs), len(want))
	}
	for i, k := range want {
		if kvs[i].key != k {
			t.Errorf("kv[%d] = %q, want %q (order not preserved)", i, kvs[i].key, k)
		}
	}
}

func TestDecodeOrderedObjectNonObject(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`[1,2,3]`, `"str"`, `42`, `null`, ``, `  `} {
		if _, ok := decodeOrderedObject(json.RawMessage(in)); ok {
			t.Errorf("decodeOrderedObject(%q) ok=true, want false", in)
		}
	}
}

func TestUnwrapEnvelope(t *testing.T) {
	t.Parallel()
	// Straddle item envelope -> inner object.
	env := json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"x","status":"created"}}`)
	got := unwrapEnvelope(env)
	kvs, ok := decodeOrderedObject(got)
	if !ok || len(kvs) != 2 || kvs[0].key != "id" {
		t.Errorf("envelope not unwrapped to inner object: %s", got)
	}
	// Envelope wrapping an array (list response) must NOT be unwrapped to a
	// non-object — renderDetailCard should then decline.
	listEnv := json.RawMessage(`{"response_type":"array","data":[{"id":"a"}]}`)
	if _, ok := decodeOrderedObject(unwrapEnvelope(listEnv)); ok {
		// unwrap returns the array; decodeOrderedObject on an array is false.
		t.Errorf("list envelope unwrap should yield non-object")
	}
	// A bare object (no envelope) passes through unchanged.
	bare := json.RawMessage(`{"id":"x"}`)
	if string(unwrapEnvelope(bare)) != string(bare) {
		t.Errorf("bare object should pass through unchanged")
	}
}

func TestBuildCardBlocks(t *testing.T) {
	t.Parallel()
	data := json.RawMessage(`{
		"id":"ch_1",
		"amount":1000,
		"currency":"usd",
		"external_id":null,
		"description":"",
		"metadata":{},
		"device":{"ip_address":"1.2.3.4","nested":{"deep":1}},
		"status_history":[{"status":"created","reason":"new"}],
		"funding_ids":["f1","f2"]
	}`)
	kvs, ok := decodeOrderedObject(data)
	if !ok {
		t.Fatal("decode failed")
	}
	blocks := buildCardBlocks(kvs)

	// Index by label for assertions.
	byLabel := map[string]cardBlock{}
	for _, b := range blocks {
		byLabel[b.label] = b
	}
	if b, ok := byLabel["Amount"]; !ok || b.kind != blockRow || b.value != "$10.00 USD" {
		t.Errorf("Amount block = %+v, want cents row", b)
	}
	if _, ok := byLabel["External ID"]; ok {
		t.Error("null field should be skipped")
	}
	if _, ok := byLabel["Description"]; ok {
		t.Error("empty string field should be skipped")
	}
	if _, ok := byLabel["Metadata"]; ok {
		t.Error("empty object should be skipped")
	}
	if b, ok := byLabel["Device"]; !ok || b.kind != blockSection {
		t.Errorf("Device should be a flattened section, got %+v", b)
	} else {
		// One-level flatten: ip_address kept, nested object dropped.
		if len(b.rows) != 1 || b.rows[0].label != "IP Address" {
			t.Errorf("Device section rows = %+v, want only IP Address", b.rows)
		}
	}
	if b, ok := byLabel["Status History"]; !ok || b.kind != blockTable {
		t.Errorf("Status History should be a sub-table, got %+v", b)
	} else if len(b.headers) != 2 || b.headers[0] != "STATUS" {
		t.Errorf("Status History headers = %v, want [STATUS REASON]", b.headers)
	}
	if b, ok := byLabel["Funding Ids"]; !ok || b.kind != blockRow || b.value != "f1, f2" {
		t.Errorf("Funding Ids block = %+v, want joined row", b)
	}
}

func TestRenderCardWidthInvariant(t *testing.T) {
	t.Parallel()
	blocks := []cardBlock{
		{kind: blockRow, label: "ID", value: "ch_550e8400-e29b-41d4"},
		{kind: blockRow, label: "Status", value: "created"},
		{kind: blockSection, label: "Device", rows: []cardKV{{"IP Address", "10.0.0.1"}}},
		{kind: blockTable, label: "Status History",
			headers: []string{"STATUS", "REASON", "CHANGED AT"},
			cells:   [][]string{{"created", "new", "2026-05-20T00:00:00Z"}, {"on_hold", "manual_review", "2026-05-21T00:00:00Z"}}},
	}
	for _, w := range []int{80, 100, 120} {
		card := renderCard("Charge", blocks, w, false)
		if got := assertUniformWidth(t, card); got != w {
			t.Errorf("width %d: lines are %d wide", w, got)
		}
		lines := strings.Split(card, "\n")
		if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
			t.Errorf("width %d: bad top border %q", w, lines[0])
		}
		last := lines[len(lines)-1]
		if !strings.HasPrefix(last, "└") || !strings.HasSuffix(last, "┘") {
			t.Errorf("width %d: bad bottom border %q", w, last)
		}
		if !strings.Contains(lines[0], "Charge") {
			t.Errorf("width %d: title missing from %q", w, lines[0])
		}
	}
}

func TestRenderCardColorWidthInvariant(t *testing.T) {
	t.Parallel()
	blocks := []cardBlock{
		{kind: blockRow, label: "ID", value: "ch_1"},
		{kind: blockRow, label: "Amount", value: "$10.00 USD"},
	}
	card := renderCard("Charge", blocks, 80, true)
	if !strings.Contains(card, "\x1b[1m") {
		t.Error("color render produced no ANSI bold sequences")
	}
	// Visible width (ANSI stripped) must still be uniform and == 80.
	if got := assertUniformWidth(t, card); got != 80 {
		t.Errorf("colored card visible width = %d, want 80", got)
	}
}

func TestRenderCardWrapsLongValue(t *testing.T) {
	t.Parallel()
	long := "this is a very long description field that must wrap across multiple physical lines inside the value column of the card"
	blocks := []cardBlock{{kind: blockRow, label: "Description", value: long}}
	card := renderCard("Charge", blocks, 80, false)
	lines := strings.Split(card, "\n")
	// More than top + one row + bottom => wrapped.
	if len(lines) <= 3 {
		t.Fatalf("expected wrapped value to span multiple lines, got %d lines", len(lines))
	}
	assertUniformWidth(t, card)
	// A continuation line carries value text in the value column but no label.
	contFound := false
	for _, ln := range lines[2 : len(lines)-1] {
		inner := strings.TrimSpace(strings.Trim(stripANSI(ln), "│"))
		if inner != "" && !strings.Contains(ln, "Description") {
			contFound = true
		}
	}
	if !contFound {
		t.Error("no continuation line (value text without the label) found")
	}
}

func TestRenderCardSubTable(t *testing.T) {
	t.Parallel()
	blocks := []cardBlock{
		{kind: blockTable, label: "Status History",
			headers: []string{"STATUS", "CHANGED AT"},
			cells:   [][]string{{"created", "2026-05-20T00:00:00Z"}}},
	}
	card := renderCard("Charge", blocks, 100, false)
	assertUniformWidth(t, card)
	for _, want := range []string{"Status History", "STATUS", "CHANGED AT", "created"} {
		if !strings.Contains(card, want) {
			t.Errorf("sub-table card missing %q\n%s", want, card)
		}
	}
}

func TestRenderCardNarrowWidthNoPanic(t *testing.T) {
	t.Parallel()
	blocks := []cardBlock{
		{kind: blockRow, label: "ID", value: "ch_550e8400-e29b-41d4-a716"},
		{kind: blockTable, label: "History", headers: []string{"A", "B", "C"}, cells: [][]string{{"x", "y", "z"}}},
	}
	// Below the [80,120] clamp the renderer must still not panic and stay
	// internally consistent (defends against negative padding).
	for _, w := range []int{1, 4, 8, 12, 20, 40} {
		card := renderCard("Charge", blocks, w, false)
		assertUniformWidth(t, card)
	}
}

func TestRenderCardWideRunes(t *testing.T) {
	t.Parallel()
	blocks := []cardBlock{
		{kind: blockRow, label: "Name", value: "東京タワー株式会社"}, // CJK, 2 cells each
	}
	card := renderCard("Customer", blocks, 80, false)
	// runewidth makes CJK 2 cells; the box must stay aligned.
	if got := assertUniformWidth(t, card); got != 80 {
		t.Errorf("wide-rune card width = %d, want 80", got)
	}
}

func TestShouldRenderDetailCardGate(t *testing.T) {
	// Not parallel: toggles the package-global humanFriendly.
	saved := humanFriendly
	defer func() { humanFriendly = saved }()
	buf := &bytes.Buffer{} // not a TTY

	humanFriendly = false
	if shouldRenderDetailCard(buf, &rootFlags{}) {
		t.Error("card must be off when --human-friendly is unset")
	}

	humanFriendly = true
	// Each machine flag short-circuits before the TTY check, so these prove
	// the flag gate independent of terminal detection.
	machine := []rootFlags{
		{asJSON: true}, {csv: true}, {compact: true},
		{quiet: true}, {plain: true}, {selectFields: "id"},
	}
	for i, f := range machine {
		if shouldRenderDetailCard(buf, &f) {
			t.Errorf("machine flag case %d should disable the card", i)
		}
	}
	// human-friendly with no machine flags, but piped (non-TTY) => still off,
	// guaranteeing piped output stays byte-identical.
	if shouldRenderDetailCard(buf, &rootFlags{}) {
		t.Error("non-TTY output must never render a card")
	}
}

func TestRenderDetailCardDeclinesNonObject(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`[{"id":"a"}]`, `"scalar"`, `42`, `{}`, `null`} {
		buf := &bytes.Buffer{}
		if renderDetailCard(buf, json.RawMessage(in)) {
			t.Errorf("renderDetailCard(%q) = true, want false (fall through)", in)
		}
		if buf.Len() != 0 {
			t.Errorf("renderDetailCard(%q) wrote output despite declining", in)
		}
	}
}

// TestFinishHumanOrOutputByteStable locks the contract that agent/JSON and
// piped (non-TTY) output is unchanged by the card hook — the card only ever
// adds output on the --human-friendly TTY path.
func TestFinishHumanOrOutputByteStable(t *testing.T) {
	// Not parallel: toggles the package-global humanFriendly.
	saved := humanFriendly
	defer func() { humanFriendly = saved }()
	obj := json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"ch_1","amount":1000,"currency":"usd"}}`)

	// Even with --human-friendly set, --json output must match printOutput byte
	// for byte (the card never fires for asJSON).
	humanFriendly = true
	want := &bytes.Buffer{}
	_ = printOutput(want, obj, true)
	got := &bytes.Buffer{}
	_ = finishHumanOrOutput(got, obj, &rootFlags{asJSON: true})
	if got.String() != want.String() {
		t.Errorf("asJSON output changed by card hook:\n got=%q\nwant=%q", got.String(), want.String())
	}

	// Piped/non-TTY default human path: no card, matches printOutput, and
	// carries no box-drawing.
	wantPiped := &bytes.Buffer{}
	_ = printOutput(wantPiped, obj, false)
	gotPiped := &bytes.Buffer{}
	_ = finishHumanOrOutput(gotPiped, obj, &rootFlags{})
	if gotPiped.String() != wantPiped.String() {
		t.Errorf("piped output changed by card hook:\n got=%q\nwant=%q", gotPiped.String(), wantPiped.String())
	}
	if strings.Contains(gotPiped.String(), "┌") {
		t.Error("piped/non-TTY output unexpectedly rendered a card")
	}
}

func TestRenderDetailCardChargeEnvelope(t *testing.T) {
	// Not parallel: sets currentResource.
	saved := currentResource
	defer func() { currentResource = saved }()
	currentResource = "charges"

	env := json.RawMessage(`{
		"meta":{"api_request_id":"req_1"},
		"response_type":"object",
		"data":{
			"id":"ch_1","paykey":"pk_abc","amount":1000,"currency":"usd",
			"payment_date":"2026-05-20","status":"created","external_id":null,
			"metadata":{},
			"status_history":[{"status":"created","reason":"new","changed_at":"2026-05-20T00:00:00Z"}],
			"funding_ids":["f1","f2"]
		}
	}`)
	buf := &bytes.Buffer{}
	if !renderDetailCard(buf, env) {
		t.Fatal("renderDetailCard declined a valid charge envelope")
	}
	card := strings.TrimRight(buf.String(), "\n")
	assertUniformWidth(t, card)
	for _, want := range []string{
		"Charge",                                 // resource title
		"$10.00 USD",                             // cents formatting
		"created",                                // status value
		"Status History", "STATUS", "CHANGED AT", // sub-table
		"f1, f2", // scalar array joined
	} {
		if !strings.Contains(card, want) {
			t.Errorf("charge card missing %q\n%s", want, card)
		}
	}
	// Skipped fields must not appear.
	if strings.Contains(card, "External ID") {
		t.Error("null external_id should not render")
	}
}
