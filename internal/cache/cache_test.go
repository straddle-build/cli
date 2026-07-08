// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestStoreSet_PrivateFilesystemPermissions verifies cached API responses are
// written owner-only. A regression to 0755/0644 would expose customer and
// payment data to other local users.
func TestStoreSet_PrivateFilesystemPermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not stable on Windows")
	}

	store := New(filepath.Join(t.TempDir(), "http"), time.Hour)
	store.Set("charges:list", json.RawMessage(`{"id":"ch_123"}`))

	dirInfo, err := os.Stat(store.Dir)
	if err != nil {
		t.Fatalf("stat cache dir: %v", err)
	}
	if got, want := dirInfo.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("cache dir mode = %o, want %o", got, want)
	}

	entryInfo, err := os.Stat(store.path("charges:list"))
	if err != nil {
		t.Fatalf("stat cache entry: %v", err)
	}
	if got, want := entryInfo.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("cache entry mode = %o, want %o", got, want)
	}
}
