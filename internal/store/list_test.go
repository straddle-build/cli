// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestList_LimitZeroReturnsAllRows is the direct regression for the silent-
// truncation bug: resolveLocal (internal/cli/data_source.go) and runGroupBy
// (internal/cli/analytics.go) both call db.List(rt, 0) expecting "no limit,
// return all synced data", but store.List rewrote limit<=0 to a hard cap of
// 200. Seed >200 rows for a resource type and assert List(rt, 0) returns
// every row, matching sync_state.total_count and Count(rt) — the three-way
// agreement that was broken before the fix.
func TestList_LimitZeroReturnsAllRows(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Custom resource type with no typed-table dispatch so the test
	// exercises List's generic-read path in isolation: List only reads the
	// generic resources table; the typed dispatch lives in the write path.
	const rt = "list_test"
	const N = 250 // > 200, the old silent cap

	items := make([]json.RawMessage, 0, N)
	for i := 0; i < N; i++ {
		items = append(items, json.RawMessage(
			fmt.Sprintf(`{"id":"item_%03d","amount":%d}`, i, 100+i),
		))
	}
	stored, extractFailures, err := s.UpsertBatch(rt, items)
	if err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if stored != N {
		t.Fatalf("UpsertBatch stored %d, want %d", stored, N)
	}
	if extractFailures != 0 {
		t.Fatalf("UpsertBatch extractFailures = %d, want 0", extractFailures)
	}
	if err := s.SaveSyncState(rt, "", N); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}

	// The smoking-gun mismatch from the bug report: sync_state.total_count
	// and the on-disk row count both report N, but List(rt, 0) used to
	// return only 200. After the fix, all three agree.
	_, _, syncCount, err := s.GetSyncState(rt)
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if syncCount != N {
		t.Fatalf("GetSyncState total_count = %d, want %d", syncCount, N)
	}
	count, err := s.Count(rt)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != N {
		t.Fatalf("Count = %d, want %d", count, N)
	}

	got, err := s.List(rt, 0)
	if err != nil {
		t.Fatalf("List(rt, 0): %v", err)
	}
	if len(got) != N {
		t.Fatalf("List(rt, 0) returned %d rows, want %d (limit<=0 must mean no limit, not a 200-row cap)", len(got), N)
	}
}

// TestList_NegativeLimitReturnsAllRows pins the limit<=0 contract: any
// non-positive limit means "no limit", not "default to 200". Covers -1
// (the sentinel used by the fix's SQL LIMIT -1) and an arbitrary negative
// value, so a future refactor of the sentinel cannot silently reintroduce
// a cap.
func TestList_NegativeLimitReturnsAllRows(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const rt = "list_neg"
	const N = 250
	items := make([]json.RawMessage, 0, N)
	for i := 0; i < N; i++ {
		items = append(items, json.RawMessage(fmt.Sprintf(`{"id":"n_%03d"}`, i)))
	}
	if _, _, err := s.UpsertBatch(rt, items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	for _, limit := range []int{-1, -2, -100} {
		got, err := s.List(rt, limit)
		if err != nil {
			t.Fatalf("List(rt, %d): %v", limit, err)
		}
		if len(got) != N {
			t.Fatalf("List(rt, %d) returned %d rows, want %d (negative limit must mean no limit)", limit, len(got), N)
		}
	}
}

// TestList_ExplicitPositiveLimitHonored ensures the fix to the limit<=0
// branch did not change the behavior of an explicit positive limit. The
// {201} case is load-bearing: it sits just past the old 200 cap and must
// return 201, not snap back to 200.
func TestList_ExplicitPositiveLimitHonored(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const rt = "list_pos"
	const N = 250
	items := make([]json.RawMessage, 0, N)
	for i := 0; i < N; i++ {
		items = append(items, json.RawMessage(fmt.Sprintf(`{"id":"p_%03d"}`, i)))
	}
	if _, _, err := s.UpsertBatch(rt, items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	cases := []struct {
		limit int
		want  int
	}{
		{1, 1},
		{50, 50},
		{200, 200},
		{201, 201}, // just past the old cap — must not snap back to 200
		{1000, N},  // larger than present → all rows
	}
	for _, c := range cases {
		got, err := s.List(rt, c.limit)
		if err != nil {
			t.Fatalf("List(rt, %d): %v", c.limit, err)
		}
		if len(got) != c.want {
			t.Fatalf("List(rt, %d) returned %d rows, want %d", c.limit, len(got), c.want)
		}
	}
}

// TestList_ScopedToSingleResourceType asserts List filters by
// resource_type so a no-limit read of one type never leaks rows from
// another. Guards a future refactor that drops the WHERE clause while
// widening the LIMIT.
func TestList_ScopedToSingleResourceType(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const rtA = "list_scope_a"
	const rtB = "list_scope_b"
	const NA, NB = 10, 12
	itemsA := make([]json.RawMessage, 0, NA)
	for i := 0; i < NA; i++ {
		itemsA = append(itemsA, json.RawMessage(fmt.Sprintf(`{"id":"a_%02d"}`, i)))
	}
	itemsB := make([]json.RawMessage, 0, NB)
	for i := 0; i < NB; i++ {
		itemsB = append(itemsB, json.RawMessage(fmt.Sprintf(`{"id":"b_%02d"}`, i)))
	}
	if _, _, err := s.UpsertBatch(rtA, itemsA); err != nil {
		t.Fatalf("UpsertBatch A: %v", err)
	}
	if _, _, err := s.UpsertBatch(rtB, itemsB); err != nil {
		t.Fatalf("UpsertBatch B: %v", err)
	}

	gotA, err := s.List(rtA, 0)
	if err != nil {
		t.Fatalf("List(A): %v", err)
	}
	if len(gotA) != NA {
		t.Fatalf("List(A) = %d rows, want %d (must not leak B rows)", len(gotA), NA)
	}
	gotB, err := s.List(rtB, 0)
	if err != nil {
		t.Fatalf("List(B): %v", err)
	}
	if len(gotB) != NB {
		t.Fatalf("List(B) = %d rows, want %d (must not leak A rows)", len(gotB), NB)
	}
}

// TestList_OrdersByUpdatedAtDesc pins the ORDER BY that the bug report
// relies on for "the dropped rows are the oldest". The fix must preserve
// updated_at DESC ordering. Seeds rows with distinct updated_at values
// (via direct insert, since UpsertBatch stamps every row in a batch with
// the same time.Now() inside one transaction) and asserts List returns
// them newest-first. With a correct no-limit read, no rows are dropped, so
// every seeded id is present in the expected descending order.
func TestList_OrdersByUpdatedAtDesc(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const rt = "list_order"
	// Three rows with strictly increasing updated_at, inserted out of
	// order so an accidental insertion-order sort would not match.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stamps := []time.Time{
		base.Add(2 * time.Hour), // newest  → id row_0
		base.Add(0 * time.Hour), // oldest  → id row_1
		base.Add(1 * time.Hour), // middle  → id row_2
	}
	db := s.DB()
	for i, st := range stamps {
		if _, err := db.Exec(
			`INSERT INTO resources (id, resource_type, data, synced_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			fmt.Sprintf("row_%d", i), rt,
			json.RawMessage(fmt.Sprintf(`{"id":"row_%d"}`, i)),
			st, st,
		); err != nil {
			t.Fatalf("seed insert %d: %v", i, err)
		}
	}

	got, err := s.List(rt, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List returned %d rows, want 3", len(got))
	}
	var ids []string
	for _, r := range got {
		var obj map[string]any
		if err := json.Unmarshal(r, &obj); err != nil {
			t.Fatalf("unmarshal row: %v", err)
		}
		ids = append(ids, fmt.Sprintf("%v", obj["id"]))
	}
	// stamps[0]=newest(id row_0), stamps[2]=middle(id row_2), stamps[1]=oldest(id row_1).
	wantOrder := []string{"row_0", "row_2", "row_1"}
	if fmt.Sprint(ids) != fmt.Sprint(wantOrder) {
		t.Fatalf("List order = %v, want %v (updated_at DESC)", ids, wantOrder)
	}
}
