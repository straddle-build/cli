// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

// Human-mode "detail card": a single resource rendered as a titled, bordered
// box with aligned label:value rows, nested objects flattened one level, and
// nested arrays rendered as sub-tables. This is the framed renderer the human
// `get` path uses; agent/JSON/piped output is never routed here (see
// shouldRenderDetailCard), so machine bytes stay identical.
//
// Field order comes from the response's own JSON key order (Straddle serializes
// in schema order), decoded with a streaming token reader rather than a Go map
// — Go maps lose order, which would make the card non-deterministic. buildCard
// Blocks is the seam where per-resource field curation could be added later.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// currentResource is the resource name for the command in flight (e.g.
// "charges"), set in the root PersistentPreRunE from the command's
// straddle:endpoint annotation. Used only to title the human detail card; empty
// for commands without an endpoint annotation. Read-only after pre-run, and
// only consulted on the --human-friendly TTY path (never in agent mode).
var currentResource string

// resourceFromEndpoint extracts the resource segment from a straddle:endpoint
// annotation value like "charges.get" -> "charges".
func resourceFromEndpoint(ep string) string {
	if ep == "" {
		return ""
	}
	if i := strings.IndexByte(ep, '.'); i >= 0 {
		return ep[:i]
	}
	return ep
}

// kvPair is one object member in document order.
type kvPair struct {
	key string
	raw json.RawMessage
}

// blockKind tags the three shapes a card body row group can take.
type blockKind int

const (
	blockRow     blockKind = iota // a single label:value row
	blockSection                  // a nested object flattened one level
	blockTable                    // a nested array of objects as a sub-table
)

// cardKV is a flattened child of a nested object (blockSection).
type cardKV struct {
	label string
	value string
}

// cardBlock is one renderable group in the card body.
type cardBlock struct {
	kind    blockKind
	label   string
	value   string     // blockRow
	rows    []cardKV   // blockSection
	headers []string   // blockTable
	cells   [][]string // blockTable
}

// shouldRenderDetailCard reports whether the human detail card should be used
// for output written to w. It mirrors wantsHumanTable's flag gate exactly,
// plus the --human-friendly opt-in: the card is an explicit human affordance,
// so it stays off by default (agent-safe). Color within the card is decided
// separately by colorEnabled(); a plain (no-ANSI) box is still a box.
func shouldRenderDetailCard(w io.Writer, flags *rootFlags) bool {
	if !humanFriendly {
		return false
	}
	if flags.asJSON || flags.csv || flags.compact || flags.quiet || flags.plain {
		return false
	}
	if flags.selectFields != "" {
		return false
	}
	return isTerminal(w)
}

const (
	minCardWidth = 80
	maxCardWidth = 120
)

// detailCardWidth picks the outer box width, clamped to [80,120]. It honors
// $COLUMNS when present and valid; otherwise defaults to 100. Clamping up to
// 80 can exceed a very narrow terminal (the box then soft-wraps), which the
// task accepts in exchange for a readable minimum.
func detailCardWidth() int {
	w := 0
	if c := strings.TrimSpace(os.Getenv("COLUMNS")); c != "" {
		if n, err := strconv.Atoi(c); err == nil {
			w = n
		}
	}
	if w <= 0 {
		w = 100
	}
	if w > maxCardWidth {
		w = maxCardWidth
	}
	if w < minCardWidth {
		w = minCardWidth
	}
	return w
}

// renderDetailCard writes a framed detail card for a single-object response to
// w. It returns false (rendering nothing) when data is not a single object —
// an array, scalar, or empty object — so the caller falls back to the normal
// table/JSON path. The Straddle {meta,response_type,data} envelope is unwrapped
// to its inner resource first.
func renderDetailCard(w io.Writer, data json.RawMessage) bool {
	inner := unwrapEnvelope(data)
	kvs, ok := decodeOrderedObject(inner)
	if !ok || len(kvs) == 0 {
		return false
	}
	blocks := buildCardBlocks(kvs)
	if len(blocks) == 0 {
		return false
	}
	out := renderCard(cardTitle(kvs), blocks, detailCardWidth(), colorEnabled())
	fmt.Fprintln(w, out)
	return true
}

// unwrapEnvelope returns the inner resource of a Straddle item envelope
// ({meta, response_type, data}) when data has that shape and its "data"
// member is itself a JSON object. Otherwise it returns data unchanged.
func unwrapEnvelope(data json.RawMessage) json.RawMessage {
	kvs, ok := decodeOrderedObject(data)
	if !ok {
		return data
	}
	var dataRaw json.RawMessage
	hasResponseType := false
	for _, kv := range kvs {
		switch kv.key {
		case "response_type":
			hasResponseType = true
		case "data":
			dataRaw = kv.raw
		}
	}
	// Unwrap to the inner data whenever the envelope signature is present,
	// even when data is an array or null: renderDetailCard's object check then
	// declines list/empty responses, so they fall through to the table/JSON
	// path unchanged instead of being mis-rendered as a single-object card.
	if hasResponseType && dataRaw != nil {
		return dataRaw
	}
	return data
}

// cardTitle picks the box title: the humanized resource name when known
// (e.g. "Charge"), else the object's most identifying value, else "Details".
func cardTitle(kvs []kvPair) string {
	if t := humanizeResource(currentResource); t != "" {
		return t
	}
	m := map[string]json.RawMessage{}
	for _, kv := range kvs {
		m[kv.key] = kv.raw
	}
	for _, k := range []string{"name", "label", "id"} {
		if raw, ok := m[k]; ok {
			if v, ok := formatScalarValue(k, raw, nil); ok {
				return v
			}
		}
	}
	return "Details"
}

// buildCardBlocks turns ordered object members into renderable blocks, in
// document order. Null/empty members are skipped. Scalars become rows; arrays
// of objects become sub-tables; arrays of scalars become a joined row; nested
// objects are flattened one level into a section (their own nested children
// are omitted). This is the seam for any future per-resource field curation.
func buildCardBlocks(kvs []kvPair) []cardBlock {
	sib := make(map[string]json.RawMessage, len(kvs))
	for _, kv := range kvs {
		sib[kv.key] = kv.raw
	}
	var blocks []cardBlock
	for _, kv := range kvs {
		switch jsonScalarKind(kv.raw) {
		case 0, 'n': // empty or null
			continue
		case '{':
			if b, ok := sectionBlock(kv.key, kv.raw); ok {
				blocks = append(blocks, b)
			}
		case '[':
			if b, ok := arrayBlock(kv.key, kv.raw); ok {
				blocks = append(blocks, b)
			}
		default: // scalar
			if v, ok := formatScalarValue(kv.key, kv.raw, sib); ok {
				blocks = append(blocks, cardBlock{kind: blockRow, label: humanizeLabel(kv.key), value: v})
			}
		}
	}
	return blocks
}

// sectionBlock flattens a nested object one level: its scalar children become
// section rows; nested objects/arrays within it are omitted. Returns ok=false
// when the object has no renderable scalar children.
func sectionBlock(key string, raw json.RawMessage) (cardBlock, bool) {
	child, ok := decodeOrderedObject(raw)
	if !ok || len(child) == 0 {
		return cardBlock{}, false
	}
	csib := make(map[string]json.RawMessage, len(child))
	for _, c := range child {
		csib[c.key] = c.raw
	}
	var rows []cardKV
	for _, c := range child {
		switch jsonScalarKind(c.raw) {
		case '{', '[', 'n', 0:
			continue // one-level flatten: skip nested and null
		}
		if v, ok := formatScalarValue(c.key, c.raw, csib); ok {
			rows = append(rows, cardKV{label: humanizeLabel(c.key), value: v})
		}
	}
	if len(rows) == 0 {
		return cardBlock{}, false
	}
	return cardBlock{kind: blockSection, label: humanizeLabel(key), rows: rows}, true
}

// arrayBlock renders an array member: an array of objects becomes a sub-table,
// an array of scalars becomes a single comma-joined row. Returns ok=false for
// empty or unrenderable arrays.
func arrayBlock(key string, raw json.RawMessage) (cardBlock, bool) {
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
		return cardBlock{}, false
	}
	if jsonScalarKind(arr[0]) == '{' {
		headers, cells := buildSubTable(arr)
		if len(headers) == 0 {
			return cardBlock{}, false
		}
		return cardBlock{kind: blockTable, label: humanizeLabel(key), headers: headers, cells: cells}, true
	}
	var parts []string
	for _, e := range arr {
		if v, ok := formatScalarValue(key, e, nil); ok {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return cardBlock{}, false
	}
	return cardBlock{kind: blockRow, label: humanizeLabel(key), value: strings.Join(parts, ", ")}, true
}

// maxSubTableCols caps sub-table columns so wide row objects stay inside the
// box; the first N keys in document order win.
const maxSubTableCols = 6

// buildSubTable derives ordered, deduplicated column headers (document order,
// union across rows) and a formatted cell matrix from an array of objects.
func buildSubTable(arr []json.RawMessage) ([]string, [][]string) {
	var order []string
	seen := map[string]bool{}
	rowMaps := make([]map[string]json.RawMessage, 0, len(arr))
	for _, e := range arr {
		kvs, ok := decodeOrderedObject(e)
		if !ok {
			continue
		}
		m := make(map[string]json.RawMessage, len(kvs))
		for _, kv := range kvs {
			m[kv.key] = kv.raw
			if !seen[kv.key] {
				seen[kv.key] = true
				order = append(order, kv.key)
			}
		}
		rowMaps = append(rowMaps, m)
	}
	if len(order) == 0 {
		return nil, nil
	}
	if len(order) > maxSubTableCols {
		order = order[:maxSubTableCols]
	}
	headers := make([]string, len(order))
	for i, k := range order {
		headers[i] = strings.ToUpper(humanizeLabel(k))
	}
	cells := make([][]string, 0, len(rowMaps))
	for _, m := range rowMaps {
		row := make([]string, len(order))
		for i, k := range order {
			if raw, ok := m[k]; ok {
				if v, ok := formatScalarValue(k, raw, m); ok {
					row[i] = v
				}
			}
		}
		cells = append(cells, row)
	}
	return headers, cells
}

// decodeOrderedObject reads a JSON object's members in document order using a
// streaming token decoder. Returns ok=false when data is not a JSON object.
// This is how the card recovers spec/serialization order that a Go map drops.
func decodeOrderedObject(data json.RawMessage) ([]kvPair, bool) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false
	}
	var out []kvPair
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false
		}
		out = append(out, kvPair{key: key, raw: raw})
	}
	return out, true
}

// jsonScalarKind returns the first non-whitespace byte of a JSON value, used
// to dispatch on type cheaply: '{' object, '[' array, '"' string, 't'/'f'
// bool, 'n' null, a digit or '-' for numbers, 0 for empty/blank.
func jsonScalarKind(raw json.RawMessage) byte {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return b
	}
	return 0
}

// formatScalarValue formats a JSON scalar for display and reports whether it
// should be shown at all (false for null and empty strings, which are
// skipped). Integers named "amount" or "*_amount" are treated as Straddle
// minor-unit (cents) and rendered as currency using a sibling "currency"
// field — the one resource-aware formatting rule; everything else renders
// faithfully. This function is the seam for richer formatting (date-time,
// more enums) later.
func formatScalarValue(key string, raw json.RawMessage, sib map[string]json.RawMessage) (string, bool) {
	switch jsonScalarKind(raw) {
	case 0, 'n':
		return "", false
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return "", false
	}
	switch t := v.(type) {
	case nil:
		return "", false
	case bool:
		return strconv.FormatBool(t), true
	case string:
		if t == "" {
			return "", false
		}
		return t, true
	case float64:
		if isCentsField(key) && t == math.Trunc(t) {
			return formatCents(int64(t), siblingCurrency(sib)), true
		}
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'f', 2, 64), true
	default:
		// An object or array sitting in a scalar slot (e.g. a sub-table cell):
		// fall back to compact JSON so the value is still visible.
		b, _ := json.Marshal(t)
		return string(b), true
	}
}

// isCentsField reports whether a numeric field holds Straddle minor units.
func isCentsField(key string) bool {
	return key == "amount" || strings.HasSuffix(key, "_amount") || key == "balance" || key == "account_balance"
}

// formatCents renders integer minor units as major-unit currency, e.g.
// 1000 -> "$10.00 USD".
func formatCents(cents int64, currency string) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	s := fmt.Sprintf("$%d.%02d", cents/100, cents%100)
	if neg {
		s = "-" + s
	}
	if currency == "" {
		currency = "USD"
	}
	return s + " " + currency
}

// siblingCurrency returns the uppercased "currency" sibling value, or "USD".
func siblingCurrency(sib map[string]json.RawMessage) string {
	if sib != nil {
		if raw, ok := sib["currency"]; ok {
			var c string
			if json.Unmarshal(raw, &c) == nil && c != "" {
				return strings.ToUpper(c)
			}
		}
	}
	return "USD"
}

// labelAcronyms upper-cases domain initialisms when humanizing field names.
var labelAcronyms = map[string]string{
	"id": "ID", "url": "URL", "ip": "IP", "ssn": "SSN", "dob": "DOB",
	"api": "API", "ach": "ACH", "uuid": "UUID", "tan": "TAN", "kyc": "KYC",
	"mx": "MX", "us": "US", "ein": "EIN",
}

// humanizeLabel turns a snake_case/kebab/camelCase field name into a Title
// Case label, e.g. external_id -> "External ID", ip_address -> "IP Address".
func humanizeLabel(key string) string {
	words := splitCamelCase(key) // splits on _ - and camel boundaries, lowercased
	for i, w := range words {
		if a, ok := labelAcronyms[w]; ok {
			words[i] = a
			continue
		}
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// humanizeResource singularizes and humanizes a resource name for the card
// title, e.g. "charges" -> "Charge", "linked-bank-accounts" -> "Linked Bank
// Account". Returns "" for an empty input.
func humanizeResource(res string) string {
	if res == "" {
		return ""
	}
	return humanizeLabel(singularize(res))
}

// singularize is a minimal pluralization inverse sufficient for this CLI's
// resource set (trailing -s, -ies, -ses).
func singularize(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	default:
		return s
	}
}
