// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/store"
)

func TestWriteThroughCacheUnwrapsObjectEnvelope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeThroughCache(context.Background(), "charges", json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"ch_123","status":"pending"}}`))
	db, err := store.OpenWithContext(context.Background(), filepath.Join(home, ".local", "share", "straddle", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := db.Get("charges", "ch_123")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"id":"ch_123","status":"pending"}` {
		t.Fatalf("cached object = %s", got)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Join(home, ".local", "share", "straddle", "data.db"))); err != nil {
		t.Fatal(err)
	}
}

func TestWriteThroughCacheSkipsSensitiveObjectEnvelope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	payload := json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"secret-123","ssn":"masked"}}`)
	writeThroughCache(context.Background(), "unmask", payload)
	db, err := store.OpenWithContext(context.Background(), filepath.Join(home, ".local", "share", "straddle", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Get("unmask", "secret-123"); err == nil {
		t.Fatal("sensitive detail unexpectedly cached")
	}
}

func TestWriteThroughCachePreservesBareObjectWithDataField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeThroughCache(context.Background(), "charges", json.RawMessage(`{"id":"outer","data":{"id":"inner"},"status":"pending"}`))
	db, err := store.OpenWithContext(context.Background(), filepath.Join(home, ".local", "share", "straddle", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Get("charges", "outer"); err != nil {
		t.Fatalf("outer resource not cached: %v", err)
	}
	if _, err := db.Get("charges", "inner"); err == nil {
		t.Fatal("inner collision object was cached")
	}
}

func TestWriteThroughCacheSkipsRevealEnvelope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	payload := json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"secret-456"}}`)
	writeThroughCache(context.Background(), "reveal", payload)
	db, err := store.OpenWithContext(context.Background(), filepath.Join(home, ".local", "share", "straddle", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Get("reveal", "secret-456"); err == nil {
		t.Fatal("revealed detail unexpectedly cached")
	}
}

func TestResolveReadRequestWritesDetailForOfflineLookup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	flags := &rootFlags{dataSource: "auto"}
	live := func() (json.RawMessage, error) {
		return json.RawMessage(`{"meta":{},"response_type":"object","data":{"id":"ch_auto","status":"pending"}}`), nil
	}
	if _, _, err := resolveReadRequest(context.Background(), flags, "charges", false, "/v1/charges/ch_auto", nil, live); err != nil {
		t.Fatal(err)
	}
	flags.dataSource = "local"
	got, _, err := resolveReadRequest(context.Background(), flags, "charges", false, "/v1/charges/ch_auto", nil, func() (json.RawMessage, error) { t.Fatal("local read called live function"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"id":"ch_auto","status":"pending"}` {
		t.Fatalf("offline result = %s", got)
	}
}
