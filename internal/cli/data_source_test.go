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
	writeThroughCache(context.Background(), "charges", json.RawMessage(`{"meta":{},"response_type":"charge","data":{"id":"ch_123","status":"pending"}}`))
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
	payload := json.RawMessage(`{"meta":{},"response_type":"customer","data":{"id":"secret-123","ssn":"masked"}}`)
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
